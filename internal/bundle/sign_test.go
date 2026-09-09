/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package bundle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSign_MissingBinaryReturnsClearError(t *testing.T) {
	// Save and restore both package vars so test order cannot affect other tests.
	origFinder := signerFinder
	origRunner := signRunner
	t.Cleanup(func() {
		signerFinder = origFinder
		signRunner = origRunner
	})

	// Point PATH at an empty dir so the cosign lookup fails deterministically.
	t.Setenv("PATH", t.TempDir())
	signerFinder = lookCosign // use the real finder against the empty PATH

	dir := t.TempDir()
	bundlePath := filepath.Join(dir, bundleBundleZip)
	require.NoError(t, os.WriteFile(bundlePath, []byte("zip-bytes"), 0o600))

	_, err := Sign(context.Background(), bundlePath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cosign")
}

func TestRunCosign_ExecutesCommand(t *testing.T) {
	// Create a shell script that acts as a fake cosign: it just writes the
	// signature file so runCosign can succeed.
	dir := t.TempDir()
	fakeCosign := filepath.Join(dir, "cosign")
	sigPath := filepath.Join(dir, "bundle.zip.sig")
	blobPath := filepath.Join(dir, bundleBundleZip)
	require.NoError(t, os.WriteFile(blobPath, []byte("data"), 0o600))

	// Write a tiny shell script that creates the sig file.
	script := "#!/bin/sh\ntouch \"$4\"\n" // $4 is sigPath in: cosign sign-blob --yes --output-signature <sig> <blob>
	require.NoError(t, os.WriteFile(fakeCosign, []byte(script), 0o700))

	err := runCosign(context.Background(), fakeCosign, blobPath, sigPath)
	require.NoError(t, err)
	_, statErr := os.Stat(sigPath)
	assert.NoError(t, statErr, "signature file must be created by cosign")
}

func TestSign_InvokesSignerAndReturnsSigPath(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, bundleBundleZip)
	require.NoError(t, os.WriteFile(bundlePath, []byte("zip-bytes"), 0o600))

	// Save and restore both package vars so test order cannot affect other tests.
	origFinder := signerFinder
	origRunner := signRunner
	t.Cleanup(func() {
		signerFinder = origFinder
		signRunner = origRunner
	})

	var gotBlob, gotSig string
	signRunner = func(_ context.Context, _, blobPath, sigPath string) error {
		gotBlob, gotSig = blobPath, sigPath
		return os.WriteFile(sigPath, []byte("signature"), 0o600)
	}
	signerFinder = func() (string, error) { return "/usr/bin/cosign", nil }

	sig, err := Sign(context.Background(), bundlePath)
	require.NoError(t, err)
	assert.Equal(t, bundlePath+".sig", sig)
	assert.Equal(t, bundlePath, gotBlob)
	assert.Equal(t, bundlePath+".sig", gotSig)
	_, statErr := os.Stat(sig)
	assert.NoError(t, statErr)
}

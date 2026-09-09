/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// newFakeOptions wires an Options with a Factory whose Client uses a
// controller-runtime fake client seeded with the given objects.
func newFakeOptions(t *testing.T, objs ...ctrlclient.Object) (*cli.Options, *bytes.Buffer) {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	o := &cli.Options{
		In: bytes.NewReader(nil), Out: out, ErrOut: errOut,
		Printer: render.NewPrinter(out, errOut),
		Factory: k8s.NewFactoryWithClient(&k8s.Client{Typed: fc, Scheme: scheme}),
	}
	return o, out
}

func TestApply_FromFile(t *testing.T) {
	o, out := newFakeOptions(t)
	dir := t.TempDir()
	mf := filepath.Join(dir, "platform.yaml")
	require.NoError(t, os.WriteFile(mf, []byte(`apiVersion: otilm.com/v1alpha1
kind: Platform
metadata:
  name: ilm
  namespace: ilm
spec:
  database:
    mode: external
  messaging:
    mode: external
`), 0o600))

	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{"-f", mf})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Platform/ilm")
}

func TestApply_RequiresFile(t *testing.T) {
	o, out := newFakeOptions(t)
	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestApply_DryRunClient(t *testing.T) {
	o, out := newFakeOptions(t)
	dir := t.TempDir()
	mf := filepath.Join(dir, "platform.yaml")
	require.NoError(t, os.WriteFile(mf, []byte(`apiVersion: otilm.com/v1alpha1
kind: Platform
metadata:
  name: ilm-dry
  namespace: ilm
spec:
  database:
    mode: external
  messaging:
    mode: external
`), 0o600))

	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{"-f", mf, "--dry-run=client"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "dry-run")

	// Client dry-run must not persist anything: the object must be absent.
	c, err := o.Factory.Client()
	require.NoError(t, err)
	_, getErr := c.GetPlatform(context.Background(), platformName, "ilm-dry")
	require.Error(t, getErr, "Platform must not exist after client dry-run")
	assert.True(t, apierrors.IsNotFound(getErr), "expected NotFound, got: %v", getErr)
}

func TestApply_InvalidYAML(t *testing.T) {
	o, out := newFakeOptions(t)
	dir := t.TempDir()
	mf := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(mf, []byte("{{invalid yaml{{"), 0o600))
	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{"-f", mf})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestApply_EmptyManifest(t *testing.T) {
	o, out := newFakeOptions(t)
	dir := t.TempDir()
	mf := filepath.Join(dir, "empty.yaml")
	require.NoError(t, os.WriteFile(mf, []byte("---\n"), 0o600))
	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{"-f", mf})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestApply_FromStdin(t *testing.T) {
	manifest := `apiVersion: otilm.com/v1alpha1
kind: Platform
metadata:
  name: stdin-plat
  namespace: ilm
spec:
  database:
    mode: external
  messaging:
    mode: external
`
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	fc := fake.NewClientBuilder().WithScheme(scheme).Build()
	o := &cli.Options{
		In:      bytes.NewBufferString(manifest),
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
		Factory: k8s.NewFactoryWithClient(&k8s.Client{Typed: fc, Scheme: scheme}),
	}
	cmd := NewApplyCommand(o)
	cmd.SetArgs([]string{"-f", "-"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Platform/")
}

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package bundle

import (
	"archive/zip"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeMinimalZip(t *testing.T, path, manifestJSON string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	w, err := zw.Create(ManifestName)
	require.NoError(t, err)
	_, err = w.Write([]byte(manifestJSON))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
}

func writeEmptyZip(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	require.NoError(t, zw.Close())
}

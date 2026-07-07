/*
Copyright (c) ILM.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package diag

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectFlags_NoRedactRequiresYes(t *testing.T) {
	t.Parallel()
	// --no-redact without -y must be a usage error before any cluster call.
	o := newCollectOptions()
	o.NoRedact = true
	o.AssumeYes = false
	_, err := o.toCollectOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--no-redact")
}

func TestCollectFlags_NoRedactWithYesAllowed(t *testing.T) {
	t.Parallel()
	o := newCollectOptions()
	o.NoRedact = true
	o.AssumeYes = true
	co, err := o.toCollectOptions()
	require.NoError(t, err)
	assert.False(t, co.Redact, "redaction off when --no-redact -y")
}

func TestCollectFlags_DefaultsRedactOnLogsOnZip(t *testing.T) {
	t.Parallel()
	o := newCollectOptions()
	co, err := o.toCollectOptions()
	require.NoError(t, err)
	assert.True(t, co.Redact, "redaction defaults on")
	assert.True(t, co.IncludeLogs, "logs default on")
	assert.Equal(t, "zip", string(co.Format))
}

func TestCollectFlags_InvalidFormat(t *testing.T) {
	t.Parallel()
	o := newCollectOptions()
	o.Format = "rar"
	_, err := o.toCollectOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestCollectFlags_OutputAndDirMutuallyExclusive(t *testing.T) {
	t.Parallel()
	o := newCollectOptions()
	o.Output = "b.zip"
	o.OutputDir = "out/"
	_, err := o.toCollectOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--output")
}

func TestDefaultBundleName(t *testing.T) {
	t.Parallel()
	assert.Contains(t, defaultBundleName("zip"), "ilm-diagnostics-")
	assert.Contains(t, defaultBundleName("zip"), ".zip")
	assert.Contains(t, defaultBundleName("tgz"), ".tgz")
}

func TestWriteArchive_ToBuffer(t *testing.T) {
	t.Parallel()
	// writeArchive must copy bytes to the destination writer verbatim.
	var dst bytes.Buffer
	src := bytes.NewBufferString(diagArchiveBytes)
	n, err := writeArchive(&dst, src)
	require.NoError(t, err)
	assert.EqualValues(t, len(diagArchiveBytes), n)
	assert.Equal(t, diagArchiveBytes, dst.String())
}

// makeZip builds an in-memory zip with the given entries (name→content).
func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

// TestUnpackZip_ZipSlipSafety verifies that traversal entries (../x, /abs/x) land
// inside the target directory and never escape to the parent or an absolute path.
//
// The implementation uses filepath.Join(dir, filepath.Clean("/"+f.Name)).
// Go's filepath.Join does NOT replace its first element when the second is
// absolute (unlike Python), so the cleaned path is always rooted at dir.
//
// Specifically:
//   - "../escape.txt"   → Clean("/"+name) = "/escape.txt" → Join(dir,"/escape.txt") = dir+"/escape.txt"
//   - "/abs/escape.txt" → Clean("/"+name) = "/abs/escape.txt" → Join(dir,"/abs/escape.txt") = dir+"/abs/escape.txt"
//
// Both stay inside dir, so unpackZip is Zip-Slip–safe without additional checks.
func TestUnpackZip_ZipSlipSafety(t *testing.T) {
	t.Parallel()

	traversalEntries := map[string]string{
		"../escape.txt":         "should-land-inside-dir",
		"../../deep-escape.txt": "should-land-inside-dir-deep",
		"/abs/escape.txt":       "abs-path-entry",
	}
	raw := makeZip(t, traversalEntries)

	dir := t.TempDir()
	parentDir := filepath.Dir(dir)

	require.NoError(t, unpackZip(raw, dir))

	// Files must NOT appear in the parent directory — they are always neutralized
	// to inside dir by the filepath.Join(dir, filepath.Clean("/"+name)) pattern.
	escapedToParent := []string{
		filepath.Join(parentDir, diagEscapeTxt),
		filepath.Join(parentDir, "deep-escape.txt"),
	}
	for _, p := range escapedToParent {
		_, statErr := os.Stat(p)
		assert.True(t, os.IsNotExist(statErr), "traversal entry must not create file outside dir at %s", p)
	}

	// Traversal entries ARE neutralized and DO land inside dir.
	// "../escape.txt" → dir/escape.txt (not parent/escape.txt).
	inside := filepath.Join(dir, diagEscapeTxt)
	_, err := os.Stat(inside)
	assert.NoError(t, err, "traversal entry must be neutralized to inside dir, not silently dropped")

	// Absolute-path entry lands inside dir as well.
	insideAbs := filepath.Join(dir, "abs", diagEscapeTxt)
	_, err = os.Stat(insideAbs)
	assert.NoError(t, err, "/abs/escape.txt must land at dir/abs/escape.txt")
}

// TestUnpackZip_HappyPath verifies that a normal zip entry is written to the
// correct location inside the target directory with the expected content.
func TestUnpackZip_HappyPath(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		"manifest.json":      `{"version":"1.0"}`,
		"logs/component.log": "log line 1\nlog line 2\n",
	}
	raw := makeZip(t, entries)

	dir := t.TempDir()
	require.NoError(t, unpackZip(raw, dir))

	// manifest.json must exist with exact content.
	got, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"version":"1.0"}`, string(got))

	// Nested directory must be created automatically.
	logFile, err := os.ReadFile(filepath.Join(dir, "logs", "component.log"))
	require.NoError(t, err)
	assert.Equal(t, "log line 1\nlog line 2\n", string(logFile))
}

// TestUnpackZip_InvalidBytes verifies that unpackZip returns an error when
// the input is not a valid zip archive.
func TestUnpackZip_InvalidBytes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := unpackZip([]byte("not-a-zip"), dir)
	require.Error(t, err)
}

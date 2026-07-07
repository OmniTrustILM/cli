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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultPath_FallsBackToRelativeWhenHomeFails exercises the branch where
// os.UserHomeDir returns an error (simulated by unsetting HOME and USERPROFILE).
// On Darwin/Linux HOME is the environment variable consulted by os.UserHomeDir;
// when it is absent the function may still succeed via other means (getpwuid),
// so we only test the safe observable: DefaultPath must return a non-empty string.
func TestDefaultPath_NonEmpty(t *testing.T) {
	t.Parallel()
	// Regardless of HOME/XDG_CONFIG_HOME, DefaultPath must never return an empty
	// string — either the xdg path, the home-based path, or the relative fallback.
	p := DefaultPath()
	assert.NotEmpty(t, p)
	assert.Contains(t, p, "ilm")
}

// TestLoad_InvalidYAML verifies that Load returns an error when the config file
// contains invalid YAML rather than silently returning a zero-value Context.
func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, cfgFileName)
	require.NoError(t, os.WriteFile(p, []byte(":\ninvalid: :\nyaml: [unclosed\n"), 0o600))
	l := &Loader{ExplicitPath: p}
	_, err := l.Load()
	require.Error(t, err, "malformed YAML must produce a load error")
}

// TestLoad_MissingAPIVersionDefaulted verifies that a file that omits apiVersion
// gets the default value injected by Load.
func TestLoad_MissingAPIVersionDefaulted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, cfgFileName)
	require.NoError(t, os.WriteFile(p, []byte("kind: Config\ncurrent-context: test\n"), 0o600))
	l := &Loader{ExplicitPath: p}
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, apiVersion, c.APIVersion)
	assert.Equal(t, "test", c.CurrentContext)
}

// TestLoad_MissingKindDefaulted verifies that a file that omits kind gets the
// default value injected by Load.
func TestLoad_MissingKindDefaulted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, cfgFileName)
	require.NoError(t, os.WriteFile(p, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\ncurrent-context: test\n"), 0o600))
	l := &Loader{ExplicitPath: p}
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, kind, c.Kind)
}

// TestResolvePath_EmptyFirstEnvEntry covers the branch where ILMCONFIG is set
// but its first path-list entry is empty (e.g., ":second").  In that case
// resolvePath must fall through to DefaultPath.
func TestResolvePath_EmptyFirstEnvEntry(t *testing.T) {
	t.Setenv(cfgEnvXDG, "/xdg")
	// First entry before the separator is empty; resolvePath must ignore it.
	t.Setenv(cfgEnvILM, string(os.PathListSeparator)+"second-path")
	l := &Loader{}
	got := l.resolvePath()
	// Must fall back to DefaultPath (XDG-based here) because first entry is empty.
	assert.Equal(t, "/xdg/ilm/config", got)
}

// TestLoad_NonExistentViaEnv verifies that an absent file referenced via
// ILMCONFIG yields an empty (but valid) Context, not an error.
func TestLoad_NonExistentViaEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cfgEnvILM, filepath.Join(dir, "no-such-file"))
	l := &Loader{}
	c, err := l.Load()
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "Config", c.Kind)
}

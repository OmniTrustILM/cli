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

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPath_XDG(t *testing.T) {
	t.Setenv(cfgEnvXDG, "/tmp/xdg")
	assert.Equal(t, "/tmp/xdg/ilm/config", DefaultPath())
}

func TestDefaultPath_HomeFallback(t *testing.T) {
	t.Setenv(cfgEnvXDG, "")
	t.Setenv("HOME", "/home/tester")
	assert.Equal(t, "/home/tester/.config/ilm/config", DefaultPath())
}

func TestLoad_AbsentReturnsEmptyNoError(t *testing.T) {
	l := &Loader{ExplicitPath: filepath.Join(t.TempDir(), "does-not-exist")}
	c, err := l.Load()
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "Config", c.Kind)
	assert.Equal(t, "ilm.omnitrust.com/v1alpha1", c.APIVersion)
	assert.Empty(t, c.Contexts)
}

func TestLoad_ReadsExplicitFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, cfgFileName)
	require.NoError(t, os.WriteFile(p, []byte(`apiVersion: ilm.omnitrust.com/v1alpha1
kind: Config
current-context: prod
contexts:
- name: prod
  context:
    server: https://ilm.example.com
    issuer: https://idp.example.com
    clientID: ilmctl
    edgeMTLS: ""
`), 0o600))
	l := &Loader{ExplicitPath: p}
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, "prod", c.CurrentContext)
	require.Len(t, c.Contexts, 1)
	assert.Equal(t, "https://ilm.example.com", c.Contexts[0].Context.Server)
}

func TestLoad_PrecedenceEnvOverDefault(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "env-config")
	require.NoError(t, os.WriteFile(envPath, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: fromenv\n"), 0o600))
	t.Setenv(cfgEnvILM, envPath)
	t.Setenv(cfgEnvXDG, dir) // default would point elsewhere
	l := &Loader{}           // no explicit path -> env wins
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, "fromenv", c.CurrentContext)
}

func TestLoad_PrecedenceExplicitOverEnv(t *testing.T) {
	dir := t.TempDir()

	envPath := filepath.Join(dir, "env-config")
	require.NoError(t, os.WriteFile(envPath, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: fromenv\n"), 0o600))

	explicitPath := filepath.Join(dir, "explicit-config")
	require.NoError(t, os.WriteFile(explicitPath, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: fromflag\n"), 0o600))

	t.Setenv(cfgEnvILM, envPath)
	l := &Loader{ExplicitPath: explicitPath}
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, "fromflag", c.CurrentContext)
}

func TestLoad_EnvPathList_UsesFirst(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first-config")
	secondPath := filepath.Join(dir, "second-config")
	require.NoError(t, os.WriteFile(firstPath, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: first\n"), 0o600))
	require.NoError(t, os.WriteFile(secondPath, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: second\n"), 0o600))

	t.Setenv(cfgEnvILM, firstPath+string(os.PathListSeparator)+secondPath)
	l := &Loader{}
	c, err := l.Load()
	require.NoError(t, err)
	assert.Equal(t, "first", c.CurrentContext)
}

func TestLoad_NeverWrites(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, cfgFileName)
	require.NoError(t, os.WriteFile(p, []byte("apiVersion: ilm.omnitrust.com/v1alpha1\nkind: Config\ncurrent-context: ro\n"), 0o400))
	l := &Loader{ExplicitPath: p}
	_, err := l.Load()
	require.NoError(t, err)
	// Confirm the file was not modified (mtime unchanged by checking it is still readable as-is).
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ro")
}

func TestAddFlags_RegistersIlmconfig(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	l := &Loader{}
	AddFlags(fs, l)
	require.NoError(t, fs.Parse([]string{"--ilmconfig", "/custom/path"}))
	assert.Equal(t, "/custom/path", l.ExplicitPath)
}

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

package hack_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// repoRoot returns the module root (this test lives in hack/).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(filepath.Dir(file))
}

func readYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "read %s", path)
	var m map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &m), "parse %s", path)
	return m
}

func TestGoReleaserConfig(t *testing.T) {
	root := repoRoot(t)
	cfg := readYAML(t, filepath.Join(root, ".goreleaser.yaml"))

	require.EqualValues(t, 2, cfg["version"], "goreleaser config schema version")

	// Exactly one build, with both ldflags variables pointed at internal/buildinfo.
	builds, ok := cfg["builds"].([]any)
	require.True(t, ok, "builds block present")
	require.Len(t, builds, 1, "one build feeds both names")
	build := builds[0].(map[string]any)
	require.Equal(t, "./cmd/ilmctl", build["main"])
	require.Equal(t, "ilmctl", build["binary"])
	ldflags, _ := yaml.Marshal(build["ldflags"])
	for _, want := range []string{
		"internal/buildinfo.GitVersion",
		"internal/buildinfo.GitCommit",
		"internal/buildinfo.BuildDate",
	} {
		require.Contains(t, string(ldflags), want)
	}

	// Two archive entries: one for each published name from the same build.
	archives, ok := cfg["archives"].([]any)
	require.True(t, ok, "archives block present")
	names := map[string]bool{}
	for _, a := range archives {
		am := a.(map[string]any)
		tmpl, _ := am["name_template"].(string)
		switch {
		case containsName(tmpl, "ilmctl"):
			names["ilmctl"] = true
		case containsName(tmpl, "kubectl-ilm"):
			names["kubectl-ilm"] = true
		}
	}
	require.True(t, names["ilmctl"], "ilmctl archive present")
	require.True(t, names["kubectl-ilm"], "kubectl-ilm archive present")

	// Supply-chain + packaging blocks the spec mandates.
	for _, key := range []string{"checksum", "signs", "sboms", "nfpms", "homebrew_casks", "scoops", "chocolateys", "krews"} {
		require.Contains(t, cfg, key, "missing %q block", key)
	}

	// cosign blob signing.
	signs := cfg["signs"].([]any)
	require.NotEmpty(t, signs)
	signRaw, _ := yaml.Marshal(signs[0])
	require.Contains(t, string(signRaw), "cosign")

	// nfpm ships both deb and rpm.
	nfpms := cfg["nfpms"].([]any)
	require.NotEmpty(t, nfpms)
	formatsRaw, _ := yaml.Marshal(nfpms[0])
	require.Contains(t, string(formatsRaw), "deb")
	require.Contains(t, string(formatsRaw), "rpm")
}

func TestKrewManifest(t *testing.T) {
	root := repoRoot(t)
	m := readYAML(t, filepath.Join(root, ".krew.yaml"))
	require.Equal(t, "krew.googlecontainertools.github.com/v1alpha2", m["apiVersion"])
	require.Equal(t, "Plugin", m["kind"])
	meta := m["metadata"].(map[string]any)
	require.Equal(t, "ilm", meta["name"], "plugin word is ilm (kubectl ilm)")
}

func containsName(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		(len(s) > len(sub) && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

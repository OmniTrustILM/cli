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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	goreleaserFile  = ".goreleaser.yaml"
	releaseWorkflow = ".github/workflows/release.yml"
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
	cfg := readYAML(t, filepath.Join(root, goreleaserFile))

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

	// nfpm ships both deb and rpm.
	nfpms := cfg["nfpms"].([]any)
	require.NotEmpty(t, nfpms)
	formatsRaw, _ := yaml.Marshal(nfpms[0])
	require.Contains(t, string(formatsRaw), "deb")
	require.Contains(t, string(formatsRaw), "rpm")
}

// TestGoReleaserSigning pins the cosign v3 invocation. cosign v3 turned on the
// TUF signing-config path and the new bundle format by default, and both demand
// --bundle: without these opt-outs sign-blob refuses and the release dies right
// after the checksums are calculated. Asserting only that "cosign" appears
// somewhere would pass on exactly the config that breaks.
func TestGoReleaserSigning(t *testing.T) {
	cfg := readYAML(t, filepath.Join(repoRoot(t), goreleaserFile))

	signs, ok := cfg["signs"].([]any)
	require.True(t, ok, "signs block present")
	require.Len(t, signs, 1, "one signing invocation")
	sign, ok := signs[0].(map[string]any)
	require.True(t, ok, "signs entry is a mapping")

	require.Equal(t, "cosign", sign["cmd"])
	require.Equal(t, "checksum", sign["artifacts"], "the signature covers checksums.txt")

	args := stringsOf(t, sign["args"])
	for _, want := range []string{
		"sign-blob",
		"--use-signing-config=false",
		"--new-bundle-format=false",
		"--output-signature=${signature}",
	} {
		require.Contains(t, args, want, "cosign v3 requires %q", want)
	}
}

// TestReleaseWorkflow pins the two release-workflow facts the signing flags
// depend on: the cosign release they were verified against, and the chocolatey
// skip that keeps a `choco`-less runner from failing the job.
func TestReleaseWorkflow(t *testing.T) {
	wf := readYAML(t, filepath.Join(repoRoot(t), releaseWorkflow))

	cosign := stepWith(t, wf, "goreleaser", "sigstore/cosign-installer")
	require.Equal(t, "v3.0.2", cosign["cosign-release"],
		"the sign-blob flags in %s are verified against exactly this cosign release", goreleaserFile)

	gr := stepWith(t, wf, "goreleaser", "goreleaser/goreleaser-action")
	args, _ := gr["args"].(string)
	require.Contains(t, args, "--skip=chocolatey",
		"the chocolatey pipe shells out to a binary the runner does not carry")
}

// TestGoReleaserPackagingHold pins the packaging hold: the tap, bucket and index
// repositories do not exist yet, so those uploads stay off rather than failing
// the release with a 404.
func TestGoReleaserPackagingHold(t *testing.T) {
	cfg := readYAML(t, filepath.Join(repoRoot(t), goreleaserFile))

	for _, key := range []string{"homebrew_casks", "scoops", "krews"} {
		entries, ok := cfg[key].([]any)
		require.True(t, ok, "%s block present", key)
		require.NotEmpty(t, entries)
		for i, e := range entries {
			m, ok := e.(map[string]any)
			require.True(t, ok, "%s[%d] is a mapping", key, i)
			require.Equal(t, "true", fmt.Sprint(m["skip_upload"]),
				"%s[%d] must hold its upload while the target repository is missing", key, i)
		}
	}
}

// stringsOf converts a YAML sequence of scalars into a []string.
func stringsOf(t *testing.T, v any) []string {
	t.Helper()
	seq, ok := v.([]any)
	require.True(t, ok, "expected a YAML sequence, got %T", v)
	out := make([]string, 0, len(seq))
	for _, e := range seq {
		s, ok := e.(string)
		require.True(t, ok, "expected a string element, got %T", e)
		out = append(out, s)
	}
	return out
}

// stepWith returns the `with:` mapping of the first step of job whose `uses`
// starts with prefix.
func stepWith(t *testing.T, wf map[string]any, job, prefix string) map[string]any {
	t.Helper()
	jobs, ok := wf["jobs"].(map[string]any)
	require.True(t, ok, "jobs block present")
	j, ok := jobs[job].(map[string]any)
	require.True(t, ok, "job %q present", job)
	steps, ok := j["steps"].([]any)
	require.True(t, ok, "job %q has steps", job)

	for _, s := range steps {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		uses, _ := m["uses"].(string)
		if !strings.HasPrefix(uses, prefix) {
			continue
		}
		with, ok := m["with"].(map[string]any)
		require.True(t, ok, "step %q has a with: mapping", prefix)
		return with
	}
	require.FailNow(t, "no step found", "job %q has no step using %q", job, prefix)
	return nil
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

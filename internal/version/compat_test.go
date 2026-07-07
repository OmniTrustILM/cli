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

package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	compatV018 = "v0.18.0"
	compatV099 = "v0.99.0"
)

func TestCompatFromList(t *testing.T) {
	// No t.Parallel(): subtests run serially; CompatFromList is pure so no race.
	list := []string{compatV018, "v0.19.0", "v0.20.0"}
	tests := []struct {
		name      string
		ver       string
		supported []string
		wantOK    bool
		wantMsg   string
	}{
		{"exact match", compatV018, list, true, ""},
		{"last in list", "v0.20.0", list, true, ""},
		{"not in list", compatV099, list, false, compatV099},
		{"empty list", compatV018, nil, false, ""},
		{"empty list msg empty", compatV018, []string{}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := CompatFromList(tt.ver, tt.supported)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantMsg == "" {
				assert.Empty(t, msg)
			} else {
				assert.Contains(t, msg, tt.wantMsg)
			}
		})
	}
}

func TestCompatFromList_MessageContainsList(t *testing.T) {
	// Pure function — no globals mutated.
	list := []string{compatV018, "v0.19.0"}
	_, msg := CompatFromList(compatV099, list)
	assert.Contains(t, msg, compatV018, "message must mention the supported set")
}

// TestResolvedBuildVars_AllSet must NOT run in parallel: it mutates package-level
// vars (GitVersion/GitCommit/BuildDate) that are also used by other tests.
func TestResolvedBuildVars_AllSet(t *testing.T) {
	oldVer, oldCommit, oldDate := GitVersion, GitCommit, BuildDate
	t.Cleanup(func() { GitVersion, GitCommit, BuildDate = oldVer, oldCommit, oldDate })

	GitVersion = "v1.2.3"
	GitCommit = "abc123"
	BuildDate = "2026-01-01"

	ver, commit, date := resolvedBuildVars()
	assert.Equal(t, "v1.2.3", ver)
	assert.Equal(t, "abc123", commit)
	assert.Equal(t, "2026-01-01", date)
}

// TestResolvedBuildVars_DefaultsWhenEmpty must NOT run in parallel: mutates globals.
func TestResolvedBuildVars_DefaultsWhenEmpty(t *testing.T) {
	oldVer, oldCommit, oldDate := GitVersion, GitCommit, BuildDate
	t.Cleanup(func() { GitVersion, GitCommit, BuildDate = oldVer, oldCommit, oldDate })

	GitVersion = ""
	GitCommit = ""
	BuildDate = ""

	ver, commit, date := resolvedBuildVars()
	// Any of: real VCS value or the static fallbacks.
	assert.NotEmpty(t, ver, "version must be non-empty (dev or real tag)")
	assert.NotEmpty(t, commit, "commit must be non-empty (none or real sha)")
	assert.NotEmpty(t, date, "date must be non-empty (unknown or real date)")
}

// TestClient_AllFieldsPopulated must NOT run in parallel: mutates globals.
func TestClient_AllFieldsPopulated(t *testing.T) {
	oldVer, oldCommit, oldDate := GitVersion, GitCommit, BuildDate
	t.Cleanup(func() { GitVersion, GitCommit, BuildDate = oldVer, oldCommit, oldDate })

	GitVersion = "v9.8.7"
	GitCommit = "deadbeef"
	BuildDate = "2026-06-19"

	info := Client()
	assert.Equal(t, "v9.8.7", info.ClientVersion)
	assert.Equal(t, "deadbeef", info.GitCommit)
	assert.Equal(t, "2026-06-19", info.BuildDate)
	assert.True(t, strings.HasPrefix(info.Platform, "darwin") || strings.HasPrefix(info.Platform, "linux") || strings.HasPrefix(info.Platform, "windows"),
		"Platform must be GOOS/GOARCH, got %q", info.Platform)
	assert.NotEmpty(t, info.GoVersion)
}

// TestClient_CommitAndDateDefaultValues must NOT run in parallel: mutates globals.
func TestClient_CommitAndDateDefaultValues(t *testing.T) {
	oldVer, oldCommit, oldDate := GitVersion, GitCommit, BuildDate
	t.Cleanup(func() { GitVersion, GitCommit, BuildDate = oldVer, oldCommit, oldDate })

	GitVersion = ""
	GitCommit = ""
	BuildDate = ""

	info := Client()
	// When none of the ldflags vars are set and there are no VCS settings
	// available (running under `go test`), the fallbacks are "dev"/"none"/"unknown".
	// When VCS settings ARE available the values are real — either way, non-empty.
	require.NotEmpty(t, info.ClientVersion)
	require.NotEmpty(t, info.GitCommit)
	require.NotEmpty(t, info.BuildDate)
}

func TestOrDefault(t *testing.T) {
	// orDefault is pure — safe to run concurrently, but no need to mark parallel
	// given other tests in this package are serial due to global mutation.
	assert.Equal(t, "a", orDefault("a", "b"))
	assert.Equal(t, "b", orDefault("", "b"))
}

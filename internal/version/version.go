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

// Package version reports the CLI client version (ldflags-injected via
// internal/buildinfo) and checks an operator/platform version against the
// embedded BOM-supported range.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/OmniTrustILM/operator/pkg/bom"

	"github.com/OmniTrustILM/cli/internal/buildinfo"
)

// GitVersion, GitCommit, and BuildDate shadow the buildinfo package-level vars
// so that tests can mutate them without modifying the ldflags target, and so
// that the version package is self-contained at call sites. They are resolved
// lazily in Client() via resolvedBuildVars() so that the fallback order is
// always: (1) ldflags, (2) VCS build settings from runtime/debug, (3) static
// defaults. Tests may override these vars directly.
var (
	GitVersion = buildinfo.GitVersion
	GitCommit  = buildinfo.GitCommit
	BuildDate  = buildinfo.BuildDate
)

// orDefault returns s if non-empty, otherwise fallback.
func orDefault(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// applyDebugBuildInfo fills any empty build vars from runtime/debug.ReadBuildInfo.
func applyDebugBuildInfo(ver, commit, date string) (string, string, string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ver, commit, date
	}
	if ver == "" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		ver = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" && s.Value != "" {
				commit = s.Value
			}
		case "vcs.time":
			if date == "" && s.Value != "" {
				date = s.Value
			}
		}
	}
	return ver, commit, date
}

// resolvedBuildVars returns the effective GitVersion/GitCommit/BuildDate
// following the priority: (1) package-level vars (ldflags or test overrides),
// (2) VCS settings from runtime/debug.ReadBuildInfo(), (3) static defaults.
func resolvedBuildVars() (ver, commit, date string) {
	ver, commit, date = GitVersion, GitCommit, BuildDate

	// Only consult debug.ReadBuildInfo when at least one var is still empty;
	// if ldflags injected everything we avoid the ReadBuildInfo call entirely.
	if ver == "" || commit == "" || date == "" {
		ver, commit, date = applyDebugBuildInfo(ver, commit, date)
	}

	// Static defaults as final fallback.
	ver = orDefault(ver, "dev")
	commit = orDefault(commit, "none")
	date = orDefault(date, "unknown")
	return ver, commit, date
}

// Info captures the version surface the `version` command renders.
type Info struct {
	ClientVersion string `json:"clientVersion"`
	GitCommit     string `json:"gitCommit"`
	BuildDate     string `json:"buildDate"`
	GoVersion     string `json:"goVersion"`
	Platform      string `json:"platform"`

	// OperatorVersion and PlatformVersions are filled at runtime from the cluster
	// (see the version command's fillClusterVersions); they are empty when no
	// cluster is reachable or none is running.
	OperatorVersion  string   `json:"operatorVersion,omitempty"`
	PlatformVersions []string `json:"platformVersions,omitempty"`
}

// Client returns the client-side build info.
func Client() Info {
	ver, commit, date := resolvedBuildVars()
	return Info{
		ClientVersion: ver,
		GitCommit:     commit,
		BuildDate:     date,
		GoVersion:     runtime.Version(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// CompatResult reports whether an operator/platform version is within the
// CLI's supported range.
type CompatResult struct {
	Supported         bool     `json:"supported"`
	Requested         string   `json:"requested"`
	SupportedVersions []string `json:"supportedVersions"`
	Message           string   `json:"message"`
}

// CheckOperator validates an operator/platform version against the embedded
// BOM. Semantics are warning-not-failure: an unsupported version is reported,
// never fatal.
func CheckOperator(ver string) CompatResult {
	requested := ver
	if requested == "" {
		requested = bom.DefaultVersion
	}
	_, ok := bom.BundleFor(requested)
	res := CompatResult{
		Supported:         ok,
		Requested:         requested,
		SupportedVersions: bom.SupportedVersions(),
	}
	if ok {
		res.Message = fmt.Sprintf("%s is supported by this CLI build", requested)
	} else {
		res.Message = fmt.Sprintf("%s is outside the supported range %v; this CLI may not understand it (warning, not an error)",
			requested, bom.SupportedVersions())
	}
	return res
}

// SupportedVersions returns the BOM-supported version list.
func SupportedVersions() []string { return bom.SupportedVersions() }

// Default returns the BOM's default platform version.
func Default() string { return bom.DefaultVersion }

// CompatFromList is a pure membership check: it reports whether ver appears in
// the supported list. Callers that already hold the list (e.g. an offline
// snapshot replay) use this instead of CheckOperator so no BOM lookup occurs.
// Returns (true, "") when ver is in supported; (false, message) otherwise.
// If supported is empty the result is (false, "") — unknown, not unsupported.
func CompatFromList(ver string, supported []string) (ok bool, msg string) {
	if len(supported) == 0 {
		return false, ""
	}
	for _, s := range supported {
		if s == ver {
			return true, ""
		}
	}
	return false, fmt.Sprintf("%s is not in the supported set %v", ver, supported)
}

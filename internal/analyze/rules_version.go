/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"fmt"
	"strings"

	"github.com/OmniTrustILM/cli/internal/version"
)

// versionAnalyzer checks each platform's observed version against the supported
// set recorded in the snapshot. Per the version-skew policy this is a WARNING,
// never a hard failure.
type versionAnalyzer struct{}

func (versionAnalyzer) Name() string { return "version" }

func (a versionAnalyzer) Analyze(s *Snapshot) []Finding {
	// SupportedVersions holds the platform versions of the CLI that built the
	// snapshot; when it is empty the supported set is unknown, so stay silent.
	if len(s.SupportedVersions) == 0 {
		return nil
	}
	var out []Finding
	for _, p := range s.Platforms {
		if p.ObservedVersion == "" {
			continue
		}
		if ok, _ := version.CompatFromList(p.ObservedVersion, s.SupportedVersions); ok {
			continue
		}
		out = append(out, Finding{
			Severity:    SeverityWarn,
			Rule:        a.Name(),
			Title:       "platform version skew",
			Resource:    p.ResourceRef(),
			Evidence:    fmt.Sprintf("platform version %s is outside the supported set: %s", p.ObservedVersion, strings.Join(s.SupportedVersions, ", ")),
			Remediation: "use an ilmctl release built for this platform version",
		})
	}
	return out
}

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

// versionAnalyzer checks the observed operator version against the supported
// set recorded in the snapshot. Per the version-skew policy this is a WARNING,
// never a hard failure.
type versionAnalyzer struct{}

func (versionAnalyzer) Name() string { return "version" }

func (a versionAnalyzer) Analyze(s *Snapshot) []Finding {
	if s.OperatorVersion == "" {
		return nil
	}
	// Decide from the snapshot's own SupportedVersions — recorded at collection
	// time — so bundle replay is deterministic across CLI versions. When the
	// field is empty the supported set is unknown; stay silent.
	ok, msg := version.CompatFromList(s.OperatorVersion, s.SupportedVersions)
	if ok || len(s.SupportedVersions) == 0 {
		return nil
	}
	if msg == "" {
		msg = fmt.Sprintf("operator version %s is outside the supported range", s.OperatorVersion)
	}
	return []Finding{{
		Severity:    SeverityWarn,
		Rule:        a.Name(),
		Title:       "operator version skew",
		Evidence:    fmt.Sprintf("%s (supported: %s)", msg, strings.Join(s.SupportedVersions, ", ")),
		Remediation: "upgrade the CLI or the operator so the versions are compatible",
	}}
}

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

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

// Package analyze holds the diagnostic data model for the analyzer subsystem:
// Finding, Severity, Snapshot, the Analyzer interface, and the rule registry.
package analyze

// Severity classifies how serious a finding is.
type Severity string

const (
	// SeverityOK means no problem was detected.
	SeverityOK Severity = "ok"
	// SeverityInfo is an informational note; no action required.
	SeverityInfo Severity = "info"
	// SeverityWarn is a condition that may require attention.
	SeverityWarn Severity = "warn"
	// SeverityFail is a condition that needs to be addressed.
	SeverityFail Severity = "fail"
)

// Finding is a single diagnostic result produced by an Analyzer.
type Finding struct {
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Resource    string   `json:"resource,omitempty"` // kind/namespace/name
	Evidence    string   `json:"evidence,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
	DocsURL     string   `json:"docsURL,omitempty"`
	Rule        string   `json:"rule"` // analyzer name that produced it
}

// severityRank orders severities; unknown values rank as OK so a malformed
// finding can never spuriously dominate the exit-code decision.
func severityRank(s Severity) int {
	switch s {
	case SeverityFail:
		return 3
	case SeverityWarn:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// Worst returns the highest severity in findings (SeverityOK for an empty set),
// used by check/analyze to choose the process exit code.
func Worst(findings []Finding) Severity {
	worst := SeverityOK
	for _, f := range findings {
		if severityRank(f.Severity) > severityRank(worst) {
			worst = f.Severity
		}
	}
	return worst
}

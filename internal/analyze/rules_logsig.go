/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"fmt"
	"regexp"
)

// LogSignature is one regex-to-finding rule over collected component logs. The
// set is extensible: callers can add their own signatures.
type LogSignature struct {
	Name        string
	Pattern     *regexp.Regexp
	Severity    Severity
	Title       string
	Remediation string
}

// logsigAnalyzer matches its signatures against each resource's collected logs.
type logsigAnalyzer struct {
	sigs []LogSignature
}

// newLogsigAnalyzer builds a logsig analyzer. When called with no signatures it
// uses DefaultLogSignatures(); callers may pass explicit signatures for testing
// or extension.
func newLogsigAnalyzer(sigs ...LogSignature) logsigAnalyzer {
	if len(sigs) == 0 {
		sigs = DefaultLogSignatures()
	}
	return logsigAnalyzer{sigs: sigs}
}

func (logsigAnalyzer) Name() string { return "logsig" }

func (a logsigAnalyzer) Analyze(s *Snapshot) []Finding {
	var out []Finding
	for _, r := range s.Resources() {
		ref := r.ResourceRef()
		for component, tail := range r.Logs {
			for _, sig := range a.sigs {
				if sig.Pattern == nil || !sig.Pattern.MatchString(tail) {
					continue
				}
				out = append(out, Finding{
					Severity:    sig.Severity,
					Rule:        a.Name(),
					Resource:    ref,
					Title:       sig.Title,
					Evidence:    fmt.Sprintf("%s: matched /%s/", component, sig.Pattern.String()),
					Remediation: sig.Remediation,
				})
			}
		}
	}
	return out
}

// DefaultLogSignatures is the curated starter set of log-signature rules.
func DefaultLogSignatures() []LogSignature {
	return []LogSignature{
		{
			Name:        "panic",
			Pattern:     regexp.MustCompile(`(?m)^panic:`),
			Severity:    SeverityFail,
			Title:       "component panicked",
			Remediation: "inspect the stack trace in the component logs and file a bug if it recurs",
		},
		{
			Name:        "oom",
			Pattern:     regexp.MustCompile(`OOMKilled`),
			Severity:    SeverityFail,
			Title:       "out-of-memory kill in logs",
			Remediation: "raise the container memory limit or reduce its footprint",
		},
		{
			Name:        "x509",
			Pattern:     regexp.MustCompile(`x509: certificate signed by unknown authority`),
			Severity:    SeverityFail,
			Title:       "TLS trust failure in logs",
			Remediation: "ensure the trusted-CA bundle includes the peer's signing CA",
		},
		{
			Name:        "conn-refused",
			Pattern:     regexp.MustCompile(`connection refused`),
			Severity:    SeverityWarn,
			Title:       "dependency connection refused",
			Remediation: "verify the dependency (DB/broker/Keycloak) is reachable and ready",
		},
	}
}

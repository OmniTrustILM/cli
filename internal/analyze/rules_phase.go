/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"fmt"
	"time"

	"github.com/OmniTrustILM/cli/internal/health"
)

// defaultStuckAfter is how long a resource may sit in Progressing before it
// is flagged as stuck.
const defaultStuckAfter = 10 * time.Minute

// phaseAnalyzer maps the CR phase to a finding: Degraded is always a fail, and a
// Progressing phase becomes a warn once it has lingered past stuckAfter.
type phaseAnalyzer struct {
	now        func() time.Time
	stuckAfter time.Duration
}

func newPhaseAnalyzer() phaseAnalyzer {
	return phaseAnalyzer{now: time.Now, stuckAfter: defaultStuckAfter}
}

func (phaseAnalyzer) Name() string { return "phase" }

// phaseDegraded is the Degraded status PHASE. The Degraded condition type shares
// the string but is a separate API surface with its own constant (condDegraded).
const phaseDegraded = "Degraded"

func (a phaseAnalyzer) Analyze(s *Snapshot) []Finding {
	now := a.now
	if now == nil {
		now = time.Now
	}
	stuck := a.stuckAfter
	if stuck <= 0 {
		stuck = defaultStuckAfter
	}

	var out []Finding
	for _, r := range s.Resources() {
		switch r.Phase {
		case phaseDegraded:
			out = append(out, Finding{
				Severity: SeverityFail,
				Rule:     a.Name(),
				Resource: r.ResourceRef(),
				Title:    fmt.Sprintf("%s is Degraded", shortKind(r.GVK)),
				Evidence: "status.phase=Degraded",
			})
		case "Failed":
			out = append(out, Finding{
				Severity: SeverityFail,
				Rule:     a.Name(),
				Resource: r.ResourceRef(),
				Title:    fmt.Sprintf("%s is in Failed phase", shortKind(r.GVK)),
				Evidence: "status.phase=Failed",
			})
		case "Progressing":
			c := health.Condition(r.Conditions, "Progressing")
			if c == nil || c.LastTransitionTime.IsZero() {
				continue // no age signal; not yet "stuck"
			}
			age := now().Sub(c.LastTransitionTime.Time)
			if age <= stuck {
				continue
			}
			out = append(out, Finding{
				Severity:    SeverityWarn,
				Rule:        a.Name(),
				Resource:    r.ResourceRef(),
				Title:       fmt.Sprintf("%s stuck in Progressing", shortKind(r.GVK)),
				Evidence:    fmt.Sprintf("status.phase=Progressing for %s", age.Round(time.Minute)),
				Remediation: "inspect component logs and events; check the operator is reconciling",
			})
		}
	}
	return out
}

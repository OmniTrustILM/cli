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
		case "Degraded":
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

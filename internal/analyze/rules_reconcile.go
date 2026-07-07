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

	"github.com/OmniTrustILM/cli/internal/health"
)

// reconcileAnalyzer flags two generation-related conditions:
//   - Generation > 0 and ObservedGen == 0: the CR has never been reconciled (informational).
//   - ObservedGen > 0 but trailing Generation: the operator has not yet observed
//     the latest spec change (warn).
type reconcileAnalyzer struct{}

func (reconcileAnalyzer) Name() string { return "reconcile" }

func (a reconcileAnalyzer) Analyze(s *Snapshot) []Finding {
	var out []Finding
	for _, r := range s.Resources() {
		switch {
		case r.Generation > 0 && r.ObservedGen == 0:
			out = append(out, Finding{
				Severity:    SeverityInfo,
				Rule:        a.Name(),
				Resource:    r.ResourceRef(),
				Title:       fmt.Sprintf("%s has not been reconciled yet", shortKind(r.GVK)),
				Evidence:    fmt.Sprintf("metadata.generation=%d but status.observedGeneration=0", r.Generation),
				Remediation: "check that the operator is healthy and running",
			})
		case health.ReconcileLagged(r.ObservedGen, r.Generation):
			out = append(out, Finding{
				Severity: SeverityWarn,
				Rule:     a.Name(),
				Resource: r.ResourceRef(),
				Title:    fmt.Sprintf("%s reconcile lag", shortKind(r.GVK)),
				Evidence: fmt.Sprintf(
					"metadata.generation=%d but status.observedGeneration=%d", r.Generation, r.ObservedGen),
				Remediation: "the operator has not observed the latest change; check operator readiness and logs",
			})
		}
	}
	return out
}

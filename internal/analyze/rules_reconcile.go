/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import "fmt"

// referenceAnalyzer turns pre-resolved missing references (secret/issuer/
// configmap names the live builder could not find) into findings. A bundle
// leaves Snapshot.MissingRefs nil, so this rule is correctly silent offline.
type referenceAnalyzer struct{}

func (referenceAnalyzer) Name() string { return "reference" }

func (a referenceAnalyzer) Analyze(s *Snapshot) []Finding {
	out := make([]Finding, 0, len(s.MissingRefs))
	for _, ref := range s.MissingRefs {
		out = append(out, Finding{
			Severity:    SeverityFail,
			Rule:        a.Name(),
			Resource:    ref,
			Title:       "referenced object not found",
			Evidence:    fmt.Sprintf("a Platform/Connector/Proxy spec references %s, which does not exist", ref),
			Remediation: "create the missing Secret/Issuer/ConfigMap or correct the reference in the CR spec",
		})
	}
	return out
}

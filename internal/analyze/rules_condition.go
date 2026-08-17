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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/health"
)

// conditionAnalyzer is data-driven over EVERY published condition: it never
// hardcodes a subset, so a new operator condition surfaces automatically. The
// health.KnownConditions catalog only layers curated docs/remediation on top.
type conditionAnalyzer struct{}

func newConditionAnalyzer() conditionAnalyzer { return conditionAnalyzer{} }

func (conditionAnalyzer) Name() string { return "condition" }

// condDegraded is the operator's Degraded CONDITION type. It shares the string with
// the Degraded status phase (phaseDegraded) but is a separate API surface — the
// operator declares them as separate constants too — so the two are deliberately
// not collapsed into one constant here.
const condDegraded = "Degraded"

func (a conditionAnalyzer) Analyze(s *Snapshot) []Finding {
	var out []Finding
	for _, r := range s.Resources() {
		for i := range r.Conditions {
			if f, ok := a.evaluate(r, r.Conditions[i]); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

// evaluate classifies a single condition. Returns ok=false when the condition is
// healthy or not a fault signal.
func (a conditionAnalyzer) evaluate(r ResourceSnapshot, c metav1.Condition) (Finding, bool) {
	var sev Severity
	switch {
	case c.Type == condDegraded && c.Status == metav1.ConditionTrue:
		sev = SeverityFail
	case strings.HasSuffix(c.Type, "UpgradeBlocked") && c.Status == metav1.ConditionTrue:
		sev = SeverityWarn
	case strings.HasSuffix(c.Type, "Ready") && c.Status == metav1.ConditionFalse:
		sev = SeverityFail
	default:
		return Finding{}, false
	}

	f := Finding{
		Severity: sev,
		Rule:     a.Name(),
		Resource: r.ResourceRef(),
		Title:    fmt.Sprintf("%s condition %s=%s", shortKind(r.GVK), c.Type, c.Status),
		Evidence: fmt.Sprintf("%s=%s (%s): %s", c.Type, c.Status, c.Reason, c.Message),
	}
	if meta, ok := health.KnownConditions[c.Type]; ok {
		f.DocsURL = meta.DocsURL
		if sev == SeverityFail && meta.FailRemediation != "" {
			f.Remediation = meta.FailRemediation
		}
	}
	if f.Remediation == "" && strings.HasSuffix(c.Type, "UpgradeBlocked") {
		infra := strings.ToLower(strings.TrimSuffix(c.Type, "UpgradeBlocked"))
		f.Remediation = fmt.Sprintf(
			"acknowledge the managed-infra upgrade (e.g. ilmctl platform upgrade <name> --ack-%s)", infra)
	}
	return f, true
}

// shortKind extracts "Platform" from "Platform.otilm.com/v1alpha1".
func shortKind(gvk string) string {
	if i := strings.IndexByte(gvk, '.'); i > 0 {
		return gvk[:i]
	}
	return gvk
}

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cond(typ string, status metav1.ConditionStatus, reason, msg string) metav1.Condition {
	return metav1.Condition{Type: typ, Status: status, Reason: reason, Message: msg}
}

func TestConditionAnalyzer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		conds     []metav1.Condition
		wantSev   map[string]Severity // finding title (or condition type) -> severity
		wantCount int
	}{
		{
			name:      "all healthy emits nothing",
			conds:     []metav1.Condition{cond("Available", metav1.ConditionTrue, "AllComponentsReady", "")},
			wantCount: 0,
		},
		{
			name: "DatabaseReady=False is a fail with curated remediation",
			conds: []metav1.Condition{
				cond("DatabaseReady", metav1.ConditionFalse, "CNPGNotReady", "cluster not ready"),
			},
			wantSev:   map[string]Severity{"DatabaseReady": SeverityFail},
			wantCount: 1,
		},
		{
			name: "Degraded=True is a fail",
			conds: []metav1.Condition{
				cond("Degraded", metav1.ConditionTrue, "ComponentDown", "core crashlooping"),
			},
			wantSev:   map[string]Severity{"Degraded": SeverityFail},
			wantCount: 1,
		},
		{
			name: "DatabaseUpgradeBlocked=True is a warn",
			conds: []metav1.Condition{
				cond("DatabaseUpgradeBlocked", metav1.ConditionTrue, "AwaitingAck", "major bump"),
			},
			wantSev:   map[string]Severity{"DatabaseUpgradeBlocked": SeverityWarn},
			wantCount: 1,
		},
		{
			name: "unknown new *Ready condition still fires data-driven",
			conds: []metav1.Condition{
				cond("FutureThingReady", metav1.ConditionFalse, "Nope", "not yet"),
			},
			wantSev:   map[string]Severity{"FutureThingReady": SeverityFail},
			wantCount: 1,
		},
		{
			name: "Progressing=False is not a fault signal",
			conds: []metav1.Condition{
				cond("Progressing", metav1.ConditionFalse, "Done", ""),
			},
			wantCount: 0,
		},
	}
	a := newConditionAnalyzer()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Snapshot{Platforms: []ResourceSnapshot{{
				GVK: "Platform.otilm.com/v1alpha1", Namespace: "ns", Name: "ilm",
				Conditions: tt.conds,
			}}}
			got := a.Analyze(s)
			require.Len(t, got, tt.wantCount)
			for _, f := range got {
				assert.Equal(t, "condition", f.Rule)
				assert.Equal(t, "Platform/ns/ilm", f.Resource)
				assert.NotEmpty(t, f.Evidence)
				// The evidence cites the condition type.
				for typ, sev := range tt.wantSev {
					if assertContains(f.Evidence, typ) {
						assert.Equal(t, sev, f.Severity)
					}
				}
			}
		})
	}
}

func TestConditionAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "condition", newConditionAnalyzer().Name())
}

// assertContains is a small helper to avoid importing strings in the table loop.
func assertContains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		(haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ = time.Now // keep time import live for sibling files in this package

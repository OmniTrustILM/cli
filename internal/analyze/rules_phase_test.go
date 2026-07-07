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

func TestPhaseAnalyzer(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	progressingCond := func(age time.Duration) []metav1.Condition {
		return []metav1.Condition{{
			Type:               "Progressing",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(base.Add(-age)),
		}}
	}

	tests := []struct {
		name      string
		phase     string
		conds     []metav1.Condition
		wantCount int
		wantSev   Severity
	}{
		{"Running emits nothing", "Running", nil, 0, ""},
		{"empty phase emits nothing", "", nil, 0, ""},
		{"Degraded is fail", "Degraded", nil, 1, SeverityFail},
		{"Failed is fail", analyzeFailed, nil, 1, SeverityFail},
		{"short Progressing emits nothing", analyzeProgressing, progressingCond(2 * time.Minute), 0, ""},
		{"long Progressing is warn", analyzeProgressing, progressingCond(30 * time.Minute), 1, SeverityWarn},
		{"Progressing without condition emits nothing", analyzeProgressing, nil, 0, ""},
	}
	a := newPhaseAnalyzer()
	a.now = func() time.Time { return base }
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Snapshot{Platforms: []ResourceSnapshot{{
				GVK: analyzeAPIGroup, Namespace: "ns", Name: "ilm",
				Phase: tt.phase, Conditions: tt.conds,
			}}}
			got := a.Analyze(s)
			require.Len(t, got, tt.wantCount)
			if tt.wantCount == 1 {
				assert.Equal(t, analyzePhaseRule, got[0].Rule)
				assert.Equal(t, tt.wantSev, got[0].Severity)
				assert.Equal(t, "Platform/ns/ilm", got[0].Resource)
			}
		})
	}
}

// TestPhaseAnalyzerConnectorFailed verifies that a Connector in Failed phase
// produces exactly one SeverityFail finding. Connectors express their degraded
// state with phase "Failed" rather than "Degraded".
func TestPhaseAnalyzerConnectorFailed(t *testing.T) {
	t.Parallel()
	a := newPhaseAnalyzer()
	s := &Snapshot{Connectors: []ResourceSnapshot{{
		GVK: "Connector.otilm.com/v1alpha1", Namespace: "ns", Name: "conn1",
		Phase: analyzeFailed,
	}}}
	got := a.Analyze(s)
	require.Len(t, got, 1)
	assert.Equal(t, SeverityFail, got[0].Severity)
	assert.Equal(t, analyzePhaseRule, got[0].Rule)
	assert.Equal(t, "Connector/ns/conn1", got[0].Resource)
}

func TestPhaseAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, analyzePhaseRule, newPhaseAnalyzer().Name())
}

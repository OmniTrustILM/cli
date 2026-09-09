/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileAnalyzer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		gen       int64
		observed  int64
		wantCount int
		wantSev   Severity
	}{
		{"in sync emits nothing", 3, 3, 0, ""},
		{"lagging emits warn", 4, 3, 1, SeverityWarn},
		{"never reconciled (gen>0, observed 0) is info", 2, 0, 1, SeverityInfo},
		{"gen=1 never reconciled is info", 1, 0, 1, SeverityInfo},
		{"fresh CR gen 0 skipped", 0, 0, 0, ""},
	}
	a := reconcileAnalyzer{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Snapshot{Connectors: []ResourceSnapshot{{
				GVK: GVKConnector, Namespace: "ns", Name: "c",
				Generation: tt.gen, ObservedGen: tt.observed,
			}}}
			got := a.Analyze(s)
			require.Len(t, got, tt.wantCount)
			if tt.wantCount == 1 {
				assert.Equal(t, "reconcile", got[0].Rule)
				assert.Equal(t, tt.wantSev, got[0].Severity)
				assert.Equal(t, "Connector/ns/c", got[0].Resource)
			}
		})
	}
}

func TestReconcileAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "reconcile", reconcileAnalyzer{}.Name())
}

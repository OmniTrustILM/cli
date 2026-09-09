/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeverityRank(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		sev  Severity
		want int
	}{
		{"ok lowest", SeverityOK, 0},
		{"info", SeverityInfo, 1},
		{"warn", SeverityWarn, 2},
		{"fail highest", SeverityFail, 3},
		{"unknown sorts as ok", Severity("bogus"), 0},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, severityRank(tt.sev))
		})
	}
}

func TestWorst(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   []Finding
		want Severity
	}{
		{"empty is ok", nil, SeverityOK},
		{"single info", []Finding{{Severity: SeverityInfo}}, SeverityInfo},
		{
			name: "fail dominates warn+info",
			in: []Finding{
				{Severity: SeverityInfo},
				{Severity: SeverityFail},
				{Severity: SeverityWarn},
			},
			want: SeverityFail,
		},
		{
			name: "warn dominates ok+info",
			in: []Finding{
				{Severity: SeverityOK},
				{Severity: SeverityWarn},
				{Severity: SeverityInfo},
			},
			want: SeverityWarn,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Worst(tt.in))
		})
	}
}

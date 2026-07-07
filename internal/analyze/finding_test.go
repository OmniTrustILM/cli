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

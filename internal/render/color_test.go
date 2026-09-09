/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package render

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OmniTrustILM/cli/internal/analyze"
)

func TestColorEnabled(t *testing.T) {
	// A *bytes.Buffer is never a char device, so Auto resolves to false.
	tests := []struct {
		name string
		mode ColorMode
		want bool
	}{
		{"never", ColorNever, false},
		{"always", ColorAlways, true},
		{"auto on non-tty", ColorAuto, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ColorEnabled(tt.mode, &bytes.Buffer{}))
		})
	}
}

func TestColorEnabled_NoColorEnvOverridesAlwaysOnlyForAuto(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// NO_COLOR suppresses Auto, but an explicit --color (Always) still wins.
	assert.False(t, ColorEnabled(ColorAuto, &bytes.Buffer{}))
	assert.True(t, ColorEnabled(ColorAlways, &bytes.Buffer{}))
}

func TestPrinter_UseColor_HonoursFlags(t *testing.T) {
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	assert.False(t, p.UseColor(), "auto on a buffer is off")

	pNever := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	pNever.Color = ColorNever
	assert.False(t, pNever.UseColor())

	pAlways := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	pAlways.Color = ColorAlways
	assert.True(t, pAlways.UseColor())
}

func TestSeveritySymbol(t *testing.T) {
	tests := []struct {
		sev  analyze.Severity
		want string
	}{
		{analyze.SeverityOK, "✓"},
		{analyze.SeverityInfo, "ℹ"},
		{analyze.SeverityWarn, "⚠"},
		{analyze.SeverityFail, "✗"},
		{"unknown", "?"},
	}
	for _, tt := range tests {
		t.Run(string(tt.sev), func(t *testing.T) {
			assert.Equal(t, tt.want, SeveritySymbol(tt.sev))
		})
	}
}

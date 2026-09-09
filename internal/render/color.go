/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package render

import "github.com/OmniTrustILM/cli/internal/analyze"

// UseColor resolves whether this Printer should emit ANSI color on its Out
// stream, folding the --color/--no-color flags (via resolveColor) and the TTY +
// NO_COLOR rules (via ColorEnabled). The explicit Color field, if set away from
// ColorAuto, takes precedence over the flags.
func (p *Printer) UseColor() bool {
	mode := p.Color
	if mode == ColorAuto {
		mode = p.resolveColor()
	}
	return ColorEnabled(mode, p.Out)
}

// SeveritySymbol returns the display symbol for a Severity level, shared by the
// check and diagnostics analyze table renderers.
func SeveritySymbol(s analyze.Severity) string {
	switch s {
	case analyze.SeverityOK:
		return "✓"
	case analyze.SeverityInfo:
		return "ℹ"
	case analyze.SeverityWarn:
		return "⚠"
	case analyze.SeverityFail:
		return "✗"
	default:
		return "?"
	}
}

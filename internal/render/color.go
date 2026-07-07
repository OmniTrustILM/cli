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

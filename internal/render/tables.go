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

// Package render – tables.go provides aligned human-readable table output and
// the findings renderer used by check/analyze views.
package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"sigs.k8s.io/yaml"

	"github.com/OmniTrustILM/cli/internal/analyze"
)

// ansi escape sequences for severity colouring.
const (
	ansiReset  = "\x1b[0m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiBlue   = "\x1b[34m"
)

// Table is a column-aligned human view used by status, list, and version output.
type Table struct {
	Columns []string
	Rows    [][]string
}

// PrintTable writes the table to the Printer's Out using a tabwriter for
// aligned columns. When a structured -o format is active the caller is expected
// to pass the typed object directly to PrintObject; PrintTable skips emission
// in that case and returns nil so callers can safely call it unconditionally.
func (p *Printer) PrintTable(t Table) error {
	if p.Structured() {
		return nil
	}
	tw := tabwriter.NewWriter(p.Out, 0, 4, 3, ' ', 0)
	if len(t.Columns) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(t.Columns, "\t")); err != nil {
			return err
		}
	}
	for _, row := range t.Rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// severityColor returns the ANSI foreground color sequence for a severity.
func severityColor(s analyze.Severity) string {
	switch s {
	case analyze.SeverityOK:
		return ansiGreen
	case analyze.SeverityWarn:
		return ansiYellow
	case analyze.SeverityFail:
		return ansiRed
	case analyze.SeverityInfo:
		return ansiBlue
	default:
		return ""
	}
}

// PrintFindings renders the findings produced by check/analyze. When a
// structured -o (json or yaml) is active the findings are serialised directly;
// other structured formats (name/jsonpath/go-template) are not meaningful for a
// plain-struct slice so they fall through to the human view. The human view
// prints one line per finding with its severity symbol, title, resource,
// evidence and remediation.
func (p *Printer) PrintFindings(f []analyze.Finding) error {
	switch p.Format() {
	case formatJSON:
		return printFindingsJSON(p, f)
	case formatYAML:
		return printFindingsYAML(p, f)
	}
	return printFindingsHuman(p, f)
}

func printFindingsJSON(p *Printer, f []analyze.Finding) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(p.Out, string(b))
	return err
}

func printFindingsYAML(p *Printer, f []analyze.Finding) error {
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(p.Out, string(b))
	return err
}

func printFindingsHuman(p *Printer, f []analyze.Finding) error {
	if len(f) == 0 {
		_, err := fmt.Fprintln(p.Out, "No findings.")
		return err
	}
	color := p.UseColor()
	for i := range f {
		if err := printOneFinding(p, f[i], color); err != nil {
			return err
		}
	}
	return nil
}

func printOneFinding(p *Printer, fi analyze.Finding, color bool) error {
	sym := SeveritySymbol(fi.Severity)
	if color {
		sym = severityColor(fi.Severity) + sym + ansiReset
	}
	line := sym + " " + fi.Title
	if fi.Resource != "" {
		line += "  (" + fi.Resource + ")"
	}
	if _, err := fmt.Fprintln(p.Out, line); err != nil {
		return err
	}
	if fi.Evidence != "" {
		if _, err := fmt.Fprintln(p.Out, "    evidence:    "+fi.Evidence); err != nil {
			return err
		}
	}
	if fi.Remediation != "" {
		if _, err := fmt.Fprintln(p.Out, "    remediation: "+fi.Remediation); err != nil {
			return err
		}
	}
	if fi.DocsURL != "" {
		if _, err := fmt.Fprintln(p.Out, "    docs:        "+fi.DocsURL); err != nil {
			return err
		}
	}
	return nil
}

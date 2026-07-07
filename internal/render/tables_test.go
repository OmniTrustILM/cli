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

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/analyze"
)

const (
	tableColPhase   = "PHASE"
	tablePending    = "Pending"
	tablePlatAlpha  = "Platform/ilm/alpha"
	tableRunning    = "Running"
	tableFullName   = "full-name"
	tableInstallCmd = "ilmctl deps install --only cnpg"
)

func TestPrintTable_AlignsColumns(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorNever
	// Use rows whose first-column widths differ so that naive space-joining
	// would produce different second-column start offsets on each line.
	// tabwriter must produce identical offsets on every line.
	err := p.PrintTable(Table{
		Columns: []string{"NAME", tableColPhase, "VERSION"},
		Rows: [][]string{
			{"alpha", tableRunning, "2.18.0"},
			{"a-much-longer-name", "Degraded", "2.18.0"},
		},
	})
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	require.Len(t, lines, 3) // header + 2 rows

	// Find the byte offset of the second-column value on every line.
	secondColValues := []string{tableColPhase, tableRunning, "Degraded"}
	offsets := make([]int, len(lines))
	for i, ln := range lines {
		offsets[i] = bytes.Index(ln, []byte(secondColValues[i]))
		require.GreaterOrEqualf(t, offsets[i], 0,
			"second-column value %q not found on line %d: %q", secondColValues[i], i, ln)
	}

	// All three offsets must be identical — this is the tabwriter guarantee.
	// Under naive space-joining "alpha Running" would have offset 6 while
	// "a-much-longer-name Running" would have offset 19, so the assertion
	// proves tabwriter alignment and cannot pass with space-joined output.
	assert.Equal(t, offsets[0], offsets[1], "second-column offset differs between header and row 0")
	assert.Equal(t, offsets[0], offsets[2], "second-column offset differs between header and row 1")

	// The offset must be strictly beyond the longest first-column cell.
	longestFirstCol := len("a-much-longer-name")
	assert.Greater(t, offsets[0], longestFirstCol,
		"second-column should start after the longest first-column cell")
}

// TestPrintTable_RaggedRowDoesNotPanic documents that PrintTable tolerates rows
// shorter than the column header slice without panicking. tabwriter receives
// whatever strings.Join produces for the short slice; it does not index into
// Columns itself, so no bounds-check failure is possible.
//
// Alignment semantics with ragged rows: tabwriter aligns column N across all
// rows that contain column N. A row with a wider cell in column 0 than the
// header will push the second column further right on that row and on the
// header line (tabwriter widens all rows in the same "elastic tabstop" pass).
// The short row (1 cell) does not participate in column-2 alignment at all.
// The well-formed row's second column must still start at least minPad (3)
// bytes after its first-column cell ends — proving tabwriter-aligned padding
// rather than concatenation.
func TestPrintTable_RaggedRowDoesNotPanic(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorNever

	// 3 column headers, one row with only 1 cell, one complete row.
	tbl := Table{
		Columns: []string{"NAME", tableColPhase, "VERSION"},
		Rows: [][]string{
			{"short-only"},                         // fewer cells than columns
			{tableFullName, tablePending, "3.0.0"}, // complete row
		},
	}
	// Must not panic.
	require.NotPanics(t, func() {
		require.NoError(t, p.PrintTable(tbl))
	})

	lines := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	require.Len(t, lines, 3) // header + 2 rows

	// The well-formed row (lines[2]) must have tablePending separated from
	// tableFullName by at least minPad=3 spaces (tabwriter configuration).
	// This proves tabwriter padding was applied and not mere concatenation.
	const minPad = 3
	fullRowOffset := bytes.Index(lines[2], []byte(tablePending))
	require.Greater(t, fullRowOffset, 0, "Pending not found in full row")
	assert.GreaterOrEqual(t, fullRowOffset, len(tableFullName)+minPad,
		"second-column value must be at least minPad bytes after the first-column cell")

	// The short row must be emitted as-is (its single cell present, no panic).
	assert.Contains(t, string(lines[1]), "short-only")
}

func TestPrintTable_EmptyRows(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	require.NoError(t, p.PrintTable(Table{Columns: []string{"NAME", tableColPhase}}))
	assert.Contains(t, out.String(), "NAME")
}

func TestPrintTable_StructuredFormatSkips(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)
	require.NoError(t, fs.Parse([]string{"-o", "json"}))

	require.NoError(t, p.PrintTable(Table{
		Columns: []string{"NAME", tableColPhase},
		Rows:    [][]string{{"alpha", tableRunning}},
	}))
	// Nothing should be written when a structured format is active.
	assert.Empty(t, out.String())
}

func testFindings() []analyze.Finding {
	return []analyze.Finding{
		{Severity: analyze.SeverityOK, Title: "Operator is healthy", Rule: "workload"},
		{Severity: analyze.SeverityWarn, Title: "Platform progressing", Resource: tablePlatAlpha, Rule: "phase"},
		{Severity: analyze.SeverityFail, Title: "DatabaseReady is False", Resource: tablePlatAlpha,
			Evidence: "reason=CNPGNotInstalled", Remediation: tableInstallCmd, Rule: "condition"},
	}
}

func TestPrintFindings_HumanTable(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorNever
	require.NoError(t, p.PrintFindings(testFindings()))
	s := out.String()
	assert.Contains(t, s, "✓")
	assert.Contains(t, s, "⚠")
	assert.Contains(t, s, "✗")
	assert.Contains(t, s, "Operator is healthy")
	assert.Contains(t, s, "DatabaseReady is False")
	assert.Contains(t, s, tablePlatAlpha)
	// Remediation for the failing finding is surfaced.
	assert.Contains(t, s, tableInstallCmd)
}

func TestPrintFindings_JSONPassthrough(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)
	require.NoError(t, fs.Parse([]string{"-o", "json"}))

	require.NoError(t, p.PrintFindings(testFindings()))
	var got []analyze.Finding
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Len(t, got, 3)
	assert.Equal(t, analyze.SeverityFail, got[2].Severity)
	assert.Equal(t, tableInstallCmd, got[2].Remediation)
}

func TestPrintFindings_YAMLPassthrough(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)
	require.NoError(t, fs.Parse([]string{"-o", "yaml"}))

	require.NoError(t, p.PrintFindings(testFindings()))
	s := out.String()
	assert.Contains(t, s, "severity: fail")
	assert.Contains(t, s, tableInstallCmd)
}

func TestPrintFindings_Empty(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorNever
	require.NoError(t, p.PrintFindings(nil))
	assert.Contains(t, out.String(), "No findings")
}

func TestPrintFindings_ColorWrapsSymbols(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorAlways
	require.NoError(t, p.PrintFindings(testFindings()))
	// ANSI escape introducer present when color is forced.
	assert.Contains(t, out.String(), "\x1b[")
}

func TestPrintFindings_WithDocsURL(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	p.Color = ColorNever
	f := []analyze.Finding{
		{Severity: analyze.SeverityWarn, Title: "Check docs", Rule: "r",
			DocsURL: "https://docs.example.com/check"},
	}
	require.NoError(t, p.PrintFindings(f))
	assert.Contains(t, out.String(), "https://docs.example.com/check")
}

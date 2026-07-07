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

package diag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/bundle"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/health"
)

// ErrFindingsFailed is returned by runAnalyze when at least one fail-severity
// finding is present. It wraps cli.ErrFailure so that errors.Is(err, cli.ErrFailure)
// is true and cli.Run maps the exit code to ExitFailure (1).
var ErrFindingsFailed = fmt.Errorf("diagnostics analyze found fail-severity issues: %w", cli.ErrFailure)

func newAnalyzeCommand(o *cli.Options) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "analyze BUNDLE",
		Short: "Analyze a collected support bundle offline",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runAnalyze(o, args[0], format)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "md", "output format: md|html|json")
	return cmd
}

func runAnalyze(o *cli.Options, path, format string) error {
	snap, m, err := bundle.Read(path)
	if err != nil {
		return err
	}
	findings := analyze.DefaultRegistry().Run(snap)
	if err := renderReport(o.Out, format, snap, m, findings); err != nil {
		return err
	}
	return exitForFindings(findings)
}

// exitForFindings returns ErrFindingsFailed (which errors.Is matches to
// cli.ErrFailure via the wrapping done in cli.Run) when the worst finding is a
// fail severity, causing the process to exit 1.
func exitForFindings(findings []analyze.Finding) error {
	if analyze.Worst(findings) == analyze.SeverityFail {
		return ErrFindingsFailed
	}
	return nil
}

// renderReport writes the full diagnostics report. The human formats (md/html)
// lead with a Summary of the bundle (what was collected, the platform overview,
// and any RBAC-skipped items) before the findings, so a bundle produces a real
// report rather than a bare findings list. The json format stays findings-only:
// it is the machine-readable contract and mirrors `check -o json`.
func renderReport(w io.Writer, format string, snap *analyze.Snapshot, m bundle.Manifest, findings []analyze.Finding) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	case "md", "markdown":
		return renderMarkdown(w, snap, m, findings)
	case "html":
		return renderHTML(w, snap, m, findings)
	default:
		return cli.NewUsageError(fmt.Errorf("invalid --output format %q (want md|html|json)", format))
	}
}

// forEachField calls fn for every non-empty display field of a Finding.
// Shared by renderMarkdown and renderHTML to avoid duplication.
func forEachField(f analyze.Finding, fn func(label, value string)) {
	if f.Resource != "" {
		fn("Resource", f.Resource)
	}
	if f.Evidence != "" {
		fn("Evidence", f.Evidence)
	}
	if f.Remediation != "" {
		fn("Remediation", f.Remediation)
	}
	if f.DocsURL != "" {
		fn("Docs", f.DocsURL)
	}
	fn("Rule", f.Rule)
}

func renderMarkdown(w io.Writer, snap *analyze.Snapshot, m bundle.Manifest, findings []analyze.Finding) error {
	if _, err := fmt.Fprintf(w, "# ILM Diagnostics\n\n"); err != nil {
		return err
	}
	if err := writeMarkdownSummary(w, snap, m); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n## Findings\n\n"); err != nil {
		return err
	}
	if len(findings) == 0 {
		_, err := fmt.Fprintf(w, "No issues found — all analyzers passed.\n")
		return err
	}
	for _, f := range findings {
		if _, err := fmt.Fprintf(w, "### [%s] %s\n\n", f.Severity, f.Title); err != nil {
			return err
		}
		var werr error
		forEachField(f, func(label, value string) {
			if werr != nil {
				return
			}
			switch label {
			case "Rule":
				_, werr = fmt.Fprintf(w, "- _rule: %s_\n\n", value)
			case "Resource":
				_, werr = fmt.Fprintf(w, "- **%s:** `%s`\n", label, value)
			default:
				_, werr = fmt.Fprintf(w, "- **%s:** %s\n", label, value)
			}
		})
		if werr != nil {
			return werr
		}
	}
	return nil
}

// writeMarkdownSummary renders the bundle overview: what was collected, the
// operator/platform state reconstructed from the bundle, and any RBAC-skipped
// artifacts — the context that makes an offline report actionable.
func writeMarkdownSummary(w io.Writer, snap *analyze.Snapshot, m bundle.Manifest) error {
	operator := cmdutil.OrNone(snap.OperatorVersion) + readySuffix(snap.OperatorReady)
	if _, err := fmt.Fprintf(w, "## Summary\n\n"+
		"- **Collected:** %s\n"+
		"- **Client version:** %s\n"+
		"- **Operator:** %s\n"+
		"- **Bundle:** schema %s · %d files · %d not collected · redacted=%t\n",
		summaryTimestamp(m.CreatedAt), cmdutil.OrNone(m.ClientVersion), operator,
		cmdutil.OrNone(m.SchemaVersion), len(m.Files), len(m.Skipped), m.Redacted); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- **Platforms (%d):**%s\n", len(snap.Platforms), noneIfEmpty(snap.Platforms)); err != nil {
		return err
	}
	for i := range snap.Platforms {
		p := &snap.Platforms[i]
		ready, total := health.Summarize(p.Conditions)
		if _, err := fmt.Fprintf(w, "  - `%s/%s` — %s, %s, conditions %d/%d, %d log(s)\n",
			p.Namespace, p.Name, cmdutil.OrNone(p.Phase), cmdutil.OrNone(p.ObservedVersion), ready, total, len(p.Logs)); err != nil {
			return err
		}
	}
	if err := writeResourceCounts(w, "Connectors", snap.Connectors); err != nil {
		return err
	}
	if err := writeResourceCounts(w, "Proxies", snap.Proxies); err != nil {
		return err
	}
	return writeSkipped(w, m.Skipped)
}

// writeResourceCounts lists a resource kind and its namespaced names (or none).
func writeResourceCounts(w io.Writer, kind string, rs []analyze.ResourceSnapshot) error {
	if len(rs) == 0 {
		_, err := fmt.Fprintf(w, "- **%s (0):** none\n", kind)
		return err
	}
	if _, err := fmt.Fprintf(w, "- **%s (%d):**\n", kind, len(rs)); err != nil {
		return err
	}
	for i := range rs {
		if _, err := fmt.Fprintf(w, "  - `%s/%s`\n", rs[i].Namespace, rs[i].Name); err != nil {
			return err
		}
	}
	return nil
}

// writeSkipped lists the artifacts that could not be collected (RBAC gaps),
// which tells the reader the bundle is partial and why.
func writeSkipped(w io.Writer, skipped []bundle.SkippedItem) error {
	if len(skipped) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "- **Not collected (%d, e.g. RBAC-restricted):**\n", len(skipped)); err != nil {
		return err
	}
	for _, s := range skipped {
		if _, err := fmt.Fprintf(w, "  - `%s` — %s\n", s.Path, firstLine(s.Reason)); err != nil {
			return err
		}
	}
	return nil
}

func renderHTML(w io.Writer, snap *analyze.Snapshot, m bundle.Manifest, findings []analyze.Finding) error {
	if _, err := io.WriteString(w, "<!doctype html><html><head><meta charset=\"utf-8\">"+
		"<title>ILM Diagnostics</title></head><body><h1>ILM Diagnostics</h1>\n"); err != nil {
		return err
	}
	// Reuse the canonical summary, embedded as escaped preformatted text.
	var sum bytes.Buffer
	if err := writeMarkdownSummary(&sum, snap, m); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "<pre class=\"summary\">%s</pre>\n<h2>Findings</h2>\n",
		html.EscapeString(sum.String())); err != nil {
		return err
	}
	if len(findings) == 0 {
		_, err := io.WriteString(w, "<p>No issues found — all analyzers passed.</p>\n</body></html>\n")
		return err
	}
	for _, f := range findings {
		if err := writeHTMLFinding(w, f); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "</body></html>\n")
	return err
}

// writeHTMLFinding renders one finding as an HTML section (fields escaped).
func writeHTMLFinding(w io.Writer, f analyze.Finding) error {
	if _, err := fmt.Fprintf(w, "<section class=\"finding %s\"><h2>[%s] %s</h2><dl>",
		html.EscapeString(string(f.Severity)),
		html.EscapeString(string(f.Severity)),
		html.EscapeString(f.Title)); err != nil {
		return err
	}
	var werr error
	forEachField(f, func(label, value string) {
		if werr != nil {
			return
		}
		if label == "Resource" {
			_, werr = fmt.Fprintf(w, "<dt>%s</dt><dd><code>%s</code></dd>",
				html.EscapeString(label), html.EscapeString(value))
		} else {
			_, werr = fmt.Fprintf(w, "<dt>%s</dt><dd>%s</dd>",
				html.EscapeString(label), html.EscapeString(value))
		}
	})
	if werr != nil {
		return werr
	}
	_, err := io.WriteString(w, "</dl></section>\n")
	return err
}

// readySuffix annotates a version with the operator's readiness.
func readySuffix(ready bool) string {
	if ready {
		return " (ready)"
	}
	return " (not ready)"
}

// summaryTimestamp formats the bundle collection time, or a placeholder if unset.
func summaryTimestamp(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	return t.UTC().Format(time.RFC3339)
}

// noneIfEmpty returns " none" to inline after a zero-count platform heading.
func noneIfEmpty(rs []analyze.ResourceSnapshot) string {
	if len(rs) == 0 {
		return " none"
	}
	return ""
}

// firstLine returns s up to its first newline (skip-reason messages can be long).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/bundle"
	"github.com/OmniTrustILM/cli/internal/cli"
)

func failFindings() []analyze.Finding {
	return []analyze.Finding{
		{Severity: analyze.SeverityFail, Title: diagDBReadyFalse, Resource: diagPlatResource, Rule: diagCondRule, Remediation: "Inspect the CNPG Cluster"},
		{Severity: analyze.SeverityWarn, Title: "Progressing stuck", Resource: diagPlatResource, Rule: "phase"},
		{Severity: analyze.SeverityOK, Title: "OIDCConfigured", Resource: diagPlatResource, Rule: diagCondRule},
	}
}

func TestRenderFindings_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "json", failFindings())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, `"severity": "fail"`)
	assert.Contains(t, out, `"rule": "condition"`)
}

func TestRenderFindings_Markdown(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "md", failFindings())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "# ILM Diagnostics")
	assert.Contains(t, out, diagDBReadyFalse)
	assert.Contains(t, out, "Inspect the CNPG Cluster")
	// fail findings rank above ok findings in the output.
	assert.Less(t, strings.Index(out, diagDBReadyFalse), strings.Index(out, "OIDCConfigured"))
}

func TestRenderFindings_HTML(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "html", failFindings())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "<html")
	assert.Contains(t, out, diagDBReadyFalse)
	// HTML must escape, never inject raw user content as markup.
	assert.NotContains(t, out, "<script>")
}

func TestRenderFindings_UnknownFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "pdf", failFindings())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestExitForFindings(t *testing.T) {
	t.Parallel()
	err := exitForFindings(failFindings())
	// The returned error must wrap ErrFindingsFailed.
	assert.ErrorIs(t, err, ErrFindingsFailed)
	// It must also wrap cli.ErrFailure so that cli.Run maps the exit code to 1.
	assert.ErrorIs(t, err, cli.ErrFailure)
	assert.NoError(t, exitForFindings([]analyze.Finding{
		{Severity: analyze.SeverityWarn, Title: "w", Rule: "phase"},
		{Severity: analyze.SeverityOK, Title: "ok", Rule: diagCondRule},
	}))
}

// TestRunAnalyze_BadPath verifies that runAnalyze returns an error when the
// bundle path does not exist.
func TestRunAnalyze_BadPath(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	o := &cli.Options{Out: &buf, ErrOut: &buf}
	err := runAnalyze(o, "/nonexistent/bundle.zip", "md")
	require.Error(t, err)
}

// findingsWithDocsURL returns findings that include the DocsURL field, exercising
// the forEachField branch that emits "Docs" labels.
func findingsWithDocsURL() []analyze.Finding {
	return []analyze.Finding{
		{
			Severity:    analyze.SeverityFail,
			Title:       "ComponentDegraded",
			Rule:        "availability",
			Evidence:    "phase=Degraded",
			DocsURL:     diagDocsURL,
			Remediation: "Check component logs",
		},
	}
}

func TestRenderFindings_Markdown_DocsURL(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "md", findingsWithDocsURL())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), diagDocsURL)
}

func TestRenderFindings_HTML_DocsURL(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "html", findingsWithDocsURL())
	require.NoError(t, err)
	assert.Contains(t, buf.String(), diagDocsURL)
}

// TestRenderFindings_Markdown_AllFields exercises forEachField and renderMarkdown
// with a Finding that has Evidence and DocsURL set, reaching all branches.
func TestRenderFindings_Markdown_AllFields(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := renderF(&buf, "markdown", findingsWithDocsURL())
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "ComponentDegraded")
	assert.Contains(t, out, "Degraded")
	assert.Contains(t, out, "docs.example.com")
}

// errWriter is an io.Writer that always returns an error after n bytes.
type errWriter struct{ n int }

func (e *errWriter) Write(p []byte) (int, error) {
	if e.n <= 0 {
		return 0, fmt.Errorf("write error")
	}
	written := len(p)
	if written > e.n {
		written = e.n
	}
	e.n -= written
	if e.n == 0 {
		return written, fmt.Errorf("write error")
	}
	return written, nil
}

func TestRenderMarkdown_WriteError(t *testing.T) {
	t.Parallel()
	// Allow zero bytes — the very first write must fail.
	w := &errWriter{n: 0}
	err := renderMarkdown(w, &analyze.Snapshot{}, bundle.Manifest{}, failFindings())
	require.Error(t, err)
}

func TestRenderHTML_WriteError(t *testing.T) {
	t.Parallel()
	// Allow zero bytes — the very first write must fail.
	w := &errWriter{n: 0}
	err := renderHTML(w, &analyze.Snapshot{}, bundle.Manifest{}, failFindings())
	require.Error(t, err)
}

// renderF adapts the finding-only test call sites to renderReport with an empty
// snapshot/manifest, so the existing rendering assertions stay focused on findings.
func renderF(buf *bytes.Buffer, format string, findings []analyze.Finding) error {
	return renderReport(buf, format, &analyze.Snapshot{}, bundle.Manifest{}, findings)
}

// TestRenderReport_MarkdownSummary verifies the report leads with a bundle
// Summary (what was collected + the reconstructed overview) before the findings.
func TestRenderReport_MarkdownSummary(t *testing.T) {
	snap := &analyze.Snapshot{
		OperatorVersion: diagVer2180,
		OperatorReady:   true,
		Platforms: []analyze.ResourceSnapshot{{
			Namespace: diagPlatformName, Name: diagPlatformName, Phase: "Running", ObservedVersion: diagVer2180,
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue},
				{Type: "Progressing", Status: metav1.ConditionFalse},
			},
			Logs: map[string]string{"core": "…", "auth": "…"},
		}},
		Connectors: []analyze.ResourceSnapshot{{Namespace: diagPlatformName, Name: "common-credential-provider"}},
	}
	m := bundle.Manifest{
		SchemaVersion: "1", ClientVersion: "abc123", Redacted: true,
		Files:   []string{"a", "b", "c"},
		Skipped: []bundle.SkippedItem{{Path: "cluster/nodes.json", Reason: "forbidden: nodes is forbidden"}},
	}
	var buf bytes.Buffer
	require.NoError(t, renderReport(&buf, "md", snap, m, nil))
	out := buf.String()

	assert.Contains(t, out, "## Summary")
	assert.Contains(t, out, "**Operator:** 2.18.0 (ready)")
	assert.Contains(t, out, "**Platforms (1):**")
	assert.Contains(t, out, "`ilm/ilm` — Running, 2.18.0, conditions 2/2, 2 log(s)")
	assert.Contains(t, out, "`ilm/common-credential-provider`")
	assert.Contains(t, out, "**Proxies (0):** none")
	assert.Contains(t, out, "cluster/nodes.json") // skipped item surfaced
	// Summary precedes findings.
	assert.Less(t, strings.Index(out, "## Summary"), strings.Index(out, "## Findings"))
}

// TestRenderReport_NoIssues verifies a clean bundle produces a clear positive
// result rather than an empty findings list.
func TestRenderReport_NoIssues(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, renderReport(&buf, "md", &analyze.Snapshot{}, bundle.Manifest{}, nil))
	assert.Contains(t, buf.String(), "No issues found")
}

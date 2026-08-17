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

package gendocs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/gendocs"
)

// Committed generated artifacts, relative to this package's directory.
const (
	combinedPath = "../../docs/site/commands.md"
	treeDir      = "../../docs/commands"
)

// TestCombinedIsCurrent is the drift gate: the committed docs/site/commands.md must be
// byte-identical to a fresh render. If this fails, run `make docs` and commit.
func TestCombinedIsCurrent(t *testing.T) {
	want, err := gendocs.Combined()
	require.NoError(t, err)

	got, err := os.ReadFile(combinedPath)
	require.NoError(t, err, "docs/site/commands.md is missing; run make docs")

	require.Equal(t, string(want), string(got),
		"docs/site/commands.md is stale; run `make docs` and commit the result")
}

// TestTreeIsCurrent is the drift gate for the per-command tree. TestCombinedIsCurrent
// alone is not enough: docs/commands/*.md is committed, is what the generated SEE ALSO
// links and RELEASE.md point at, and can go stale, gain an orphan file for a deleted
// command, or keep a machine-dependent flag default without the combined page noticing.
// Rendering into a temp dir and diffing the COMPLETE file set catches all three.
func TestTreeIsCurrent(t *testing.T) {
	fresh := t.TempDir()
	require.NoError(t, gendocs.Tree(fresh))

	names := func(dir string) []string {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err, "%s is missing; run make docs", dir)
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Name())
		}
		sort.Strings(out)
		return out
	}

	freshNames, committedNames := names(fresh), names(treeDir)
	require.Equal(t, freshNames, committedNames,
		"docs/commands/ has missing or orphan files; run `make docs` and commit the result")

	for _, name := range freshNames {
		want, err := os.ReadFile(filepath.Join(fresh, name))
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(treeDir, name))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got),
			"docs/commands/%s is stale; run `make docs` and commit the result", name)
	}
}

// TestCombinedIsMachineIndependent proves no flag default leaks the generating
// machine's home directory, which would make TestCombinedIsCurrent pass on exactly
// one laptop and fail everywhere else.
func TestCombinedIsMachineIndependent(t *testing.T) {
	page, err := gendocs.Combined()
	require.NoError(t, err)

	body := string(page)
	require.NotContains(t, body, "/Users/", "generated page leaks an absolute home path")
	require.NotContains(t, body, "/home/", "generated page leaks an absolute home path")
	if home, ok := os.LookupEnv("HOME"); ok && home != "" {
		require.NotContains(t, body, home, "generated page leaks $HOME")
	}
}

// TestCombinedCoversEveryCommand proves the page is a complete reference: one H2
// section per available command in the tree, and no orphan sections.
func TestCombinedCoversEveryCommand(t *testing.T) {
	page, err := gendocs.Combined()
	require.NoError(t, err)

	var want []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		want = append(want, "## "+c.CommandPath())
		for _, sub := range c.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			walk(sub)
		}
	}
	walk(gendocs.Root())

	body := string(page)
	for _, h := range want {
		require.Contains(t, body, h+"\n", "missing section for %q", h)
	}
	require.Equal(t, len(want), strings.Count(body, "\n## "),
		"section count does not match the command tree")
}

// TestCombinedCrossReferencesAreAnchors proves SEE ALSO links point at same-page
// anchors, not at the per-command files, so the single page is self-contained.
func TestCombinedCrossReferencesAreAnchors(t *testing.T) {
	page, err := gendocs.Combined()
	require.NoError(t, err)

	body := string(page)
	require.NotContains(t, body, ".md)", "a cross-reference still points at a file")
	require.Contains(t, body, "(#ilmctl-platform)", "expected a same-page anchor link")
}

// TestCombinedAnchorsResolve proves every same-page anchor points at a heading
// that exists. The valid-slug set is derived from the emitted H2 headings with
// the algorithm Docusaurus and GitHub share for these names (lowercase, spaces
// to hyphens), so a future command name the anchorLink derivation cannot handle
// (uppercase, an underscore of its own) fails here with the offending anchor
// named, instead of shipping a silently dead link.
func TestCombinedAnchorsResolve(t *testing.T) {
	page, err := gendocs.Combined()
	require.NoError(t, err)

	slugs := map[string]bool{}
	for _, line := range strings.Split(string(page), "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			slugs[strings.ToLower(strings.ReplaceAll(h, " ", "-"))] = true
		}
	}
	require.NotEmpty(t, slugs, "expected H2 command headings")

	anchors := regexp.MustCompile(`\]\(#([^)]+)\)`).FindAllStringSubmatch(string(page), -1)
	require.NotEmpty(t, anchors, "expected same-page anchor links")
	for _, m := range anchors {
		require.True(t, slugs[m[1]], "anchor #%s does not match any generated heading", m[1])
	}
}

// TestCombinedIsHomeIndependent renders the page under two different HOME values
// and requires byte equality: the normalization, not the generating machine,
// decides the output. This catches the leak class itself, where the string scan
// in TestCombinedIsMachineIndependent only catches known path shapes.
func TestCombinedIsHomeIndependent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	first, err := gendocs.Combined()
	require.NoError(t, err)

	t.Setenv("HOME", t.TempDir())
	second, err := gendocs.Combined()
	require.NoError(t, err)

	require.Equal(t, string(first), string(second), "output differs across HOME values")
}

// TestCombinedCarriesSiteFrontMatter proves the page is site-ready as generated and
// never needs a hand edit after `make docs`.
func TestCombinedCarriesSiteFrontMatter(t *testing.T) {
	page, err := gendocs.Combined()
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(string(page), "---\nsidebar_position: 8\ntoc_max_heading_level: 2\n---\n"),
		"generated page must open with the site front matter")
}

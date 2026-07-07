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

package generate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_PlatformYAMLWithNotes(t *testing.T) {
	p, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: "ilm", Namespace: "ilm", Profile: ProfileManagedHA,
		DBMode: "external", Set: map[string]bool{"db-mode": true},
	})
	require.NoError(t, err)

	out, err := Render(p, notes)
	require.NoError(t, err)

	assert.Contains(t, out, "apiVersion: otilm.com/v1alpha1")
	assert.Contains(t, out, "kind: Platform")
	assert.Contains(t, out, "name: ilm")
	// effective-value comment block present
	assert.Contains(t, out, genEffectiveValues)
	assert.Contains(t, out, "# database.mode = external (flag)")
	assert.Contains(t, out, "# messaging.mode = managed (profile)")
	// comments precede the document body
	idx := strings.Index(out, "apiVersion:")
	cidx := strings.Index(out, genEffectiveValues)
	assert.True(t, cidx >= 0 && cidx < idx, "comment block must precede the CR body")
}

func TestRender_NilNotesStillRenders(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{Name: "ilm", Namespace: "ilm", Profile: ProfileMinimal, Set: map[string]bool{}})
	require.NoError(t, err)
	out, err := Render(p, nil)
	require.NoError(t, err)
	assert.Contains(t, out, "kind: Platform")
	assert.NotContains(t, out, genEffectiveValues)
}

// TestRender_PlaceholderNoteLouderComment verifies that notes with
// source=placeholder are rendered with a " # TODO: replace this placeholder"
// suffix so the user cannot miss them, while non-placeholder notes use the
// plain "(source)" format.
func TestRender_PlaceholderNoteLouderComment(t *testing.T) {
	// A scaffold with no --host produces a placeholder note for common.hostName.
	_, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: "myapp", Namespace: "ilm", Profile: ProfileMinimal, Set: map[string]bool{},
	})
	require.NoError(t, err)

	// Render with notes only (pass a minimal object; content doesn't matter here).
	p, _, err := ScaffoldPlatform(PlatformOptions{
		Name: "myapp", Namespace: "ilm", Profile: ProfileMinimal, Set: map[string]bool{},
	})
	require.NoError(t, err)

	out, err := Render(p, notes)
	require.NoError(t, err)

	// The placeholder note for common.hostName must carry the TODO suffix.
	assert.Contains(t, out, "# TODO: replace this placeholder",
		"placeholder notes must be rendered with a TODO suffix")

	// Non-placeholder notes must NOT carry the TODO suffix.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "# ") && !strings.Contains(line, "placeholder") &&
			!strings.Contains(line, "Effective values") {
			assert.NotContains(t, line, "TODO:",
				"non-placeholder note line must not carry a TODO suffix: %q", line)
		}
	}
}

// TestRender_EmptyNamespaceNoNamespaceLine verifies that when Namespace is empty
// the rendered YAML does not include a "namespace:" key. Namespace is intentionally
// left empty in the scaffold (resolved at apply time from the kubectl context).
func TestRender_EmptyNamespaceNoNamespaceLine(t *testing.T) {
	p, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: "ilm", Namespace: "", Profile: ProfileMinimal, Set: map[string]bool{},
	})
	require.NoError(t, err)
	out, err := Render(p, notes)
	require.NoError(t, err)
	assert.NotContains(t, out, "namespace:",
		"empty namespace must not appear as a key in the rendered YAML")
}

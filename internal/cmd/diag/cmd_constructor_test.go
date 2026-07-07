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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
)

func newTestOptions() *cli.Options {
	return &cli.Options{
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	}
}

// TestNewDiagnosticsCommand_UseAndAliases verifies basic shape of the built command.
func TestNewDiagnosticsCommand_UseAndAliases(t *testing.T) {
	t.Parallel()
	cmd := NewDiagnosticsCommand(newTestOptions())
	assert.Equal(t, "diagnostics", cmd.Use)
	assert.Contains(t, cmd.Aliases, "diag")
	assert.Equal(t, string(cli.GroupDiagnostics), cmd.GroupID)
}

// TestNewDiagnosticsCommand_FlagsRegistered verifies every expected flag is present.
func TestNewDiagnosticsCommand_FlagsRegistered(t *testing.T) {
	t.Parallel()
	cmd := NewDiagnosticsCommand(newTestOptions())
	fs := cmd.Flags()

	requiredFlags := []string{
		"no-redact", "yes", "include-logs", "since",
		"all-namespaces", "namespace", "format", "output", "output-dir", "sign",
	}
	for _, name := range requiredFlags {
		assert.NotNilf(t, fs.Lookup(name), "flag --%s must be registered on diagnostics", name)
	}
}

// TestNewDiagnosticsCommand_HasAnalyzeSubcommand verifies the analyze subcommand
// is registered as a child.
func TestNewDiagnosticsCommand_HasAnalyzeSubcommand(t *testing.T) {
	t.Parallel()
	cmd := NewDiagnosticsCommand(newTestOptions())
	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "analyze" {
			found = true
		}
	}
	assert.True(t, found, "diagnostics must have an 'analyze' subcommand")
}

// TestNewAnalyzeCommand_UseAndFlags verifies the analyze cobra command shape.
func TestNewAnalyzeCommand_UseAndFlags(t *testing.T) {
	t.Parallel()
	cmd := newAnalyzeCommand(newTestOptions())
	assert.Equal(t, "analyze", cmd.Name())
	assert.Contains(t, cmd.Use, "BUNDLE")
	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "analyze must have --output/-o flag")
	assert.Equal(t, "o", f.Shorthand)
	assert.Equal(t, "md", f.DefValue)
}

// TestNewDiagnosticsCommand_DefaultFlagValues verifies sensible defaults.
func TestNewDiagnosticsCommand_DefaultFlagValues(t *testing.T) {
	t.Parallel()
	cmd := NewDiagnosticsCommand(newTestOptions())
	fs := cmd.Flags()

	noRedact, err := fs.GetBool("no-redact")
	require.NoError(t, err)
	assert.False(t, noRedact, "--no-redact must default to false")

	includeLogs, err := fs.GetBool("include-logs")
	require.NoError(t, err)
	assert.True(t, includeLogs, "--include-logs must default to true")

	format, err := fs.GetString("format")
	require.NoError(t, err)
	assert.Equal(t, "zip", format, "--format must default to zip")

	sign, err := fs.GetBool("sign")
	require.NoError(t, err)
	assert.False(t, sign, "--sign must default to false")
}

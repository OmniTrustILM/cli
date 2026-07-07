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

package cli

import (
	"bytes"
	"strings"
	"testing"

	gocobra "github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/render"
)

// newAuditOptions builds Options with non-TTY buffers and no live factory so
// the audit exercises argument parsing, usage, and help paths deterministically.
// (Distinct from newTestOptions in root_test.go which uses NewDefaultOptions.)
func newAuditOptions() (*Options, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	o := NewDefaultOptions(out, errOut)
	o.In = strings.NewReader("")
	o.InvokedAs = "ilmctl"
	return o, out, errOut
}

// executeAudit runs the root command (bare, no subcommands registered) with
// args and maps the cobra error to an exit code via ExitCodeFor, asserting the
// real contract without a live cluster.
func executeAudit(t *testing.T, args ...string) int {
	t.Helper()
	o, out, errOut := newAuditOptions()
	root := NewRootCommand(o)
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(errOut)
	err := root.Execute()
	return ExitCodeFor(err)
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"help is success", []string{"--help"}, ExitOK},
		{"version is success", []string{"version", "--short"}, ExitOK},
		{"unknown command is usage", []string{"definitely-not-a-command"}, ExitUsage},
		{"unknown flag is usage", []string{"version", "--definitely-not-a-flag"}, ExitUsage},
		{"completion is success", []string{"completion", "bash"}, ExitOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, executeAudit(t, tc.args...))
		})
	}
}

// TestExitCodeFor_DirectMapping verifies the ExitCodeFor contract directly
// without driving a cobra command, covering the UsageError and ErrFailure sentinels.
func TestExitCodeFor_DirectMapping(t *testing.T) {
	assert.Equal(t, ExitOK, ExitCodeFor(nil), "nil → ExitOK")
	assert.Equal(t, ExitUsage, ExitCodeFor(NewUsageError(assert.AnError)), "UsageError → ExitUsage")
	assert.Equal(t, ExitFailure, ExitCodeFor(ErrFailure), "ErrFailure → ExitFailure")
	assert.Equal(t, ExitUsage, ExitCodeFor(UsageError{assert.AnError}), "UsageError{} → ExitUsage")
}

// simpleErr is a minimal error type used to inject cobra-style error messages.
type simpleErr string

func (s simpleErr) Error() string { return string(s) }

// TestExitCodeFor_CobraErrorStrings verifies that cobra's string-form usage
// errors (unknown command/flag, accepts N args) resolve to ExitUsage.
func TestExitCodeFor_CobraErrorStrings(t *testing.T) {
	cobraUsageErrors := []string{
		"unknown command \"foo\" for \"ilmctl\"",
		"unknown flag: --bar",
		"unknown shorthand flag: 'x' in -x",
		"required flag(s) \"name\" not set",
		"invalid argument \"bad\" for \"--count\"",
		"accepts 1 arg(s), received 0",
	}
	for _, msg := range cobraUsageErrors {
		assert.Equal(t, ExitUsage, ExitCodeFor(simpleErr(msg)), "expected ExitUsage for: %s", msg)
	}
}

// TestNonTTYNoColorNoPrompt verifies the NO_COLOR + non-TTY contract:
// colour is suppressed and commands do not block on a non-TTY input.
func TestNonTTYNoColorNoPrompt(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)

	// In auto mode a plain *bytes.Buffer is not a TTY and NO_COLOR is set, so
	// colour resolves off.
	assert.False(t, render.ColorEnabled(render.ColorAuto, out))
	// --color=never is always off.
	assert.False(t, render.ColorEnabled(render.ColorNever, out))
	// An explicit --color=always overrides NO_COLOR (explicit flag beats env).
	assert.True(t, render.ColorEnabled(render.ColorAlways, out))

	require.NoError(t, p.PrintTable(render.Table{
		Columns: []string{"NAME", "PHASE"},
		Rows:    [][]string{{"ilm", "Running"}},
	}))
	assert.NotContains(t, out.String(), "\x1b[", "no ANSI escapes on non-TTY")
}

// TestMissingRequiredArgIsUsage verifies that cobra's ExactArgs validator
// produces ExitUsage (2) when a required positional argument is omitted.
// This exercises the "accepts N arg(s), received 0" string path in ExitCodeFor.
func TestMissingRequiredArgIsUsage(t *testing.T) {
	o, out, errOut := newAuditOptions()
	root := NewRootCommand(o)

	// Register a minimal stub command requiring exactly one positional arg,
	// mirroring the shape of "platform describe NAME".
	stub := &gocobra.Command{
		Use:   "_teststub NAME",
		Short: "stub for exit-code audit",
		Args:  gocobra.ExactArgs(1),
		RunE: func(_ *gocobra.Command, _ []string) error {
			return nil
		},
	}
	root.AddCommand(stub)
	root.SetArgs([]string{"_teststub"}) // missing the NAME argument
	root.SetOut(out)
	root.SetErr(errOut)

	err := root.Execute()
	assert.Equal(t, ExitUsage, ExitCodeFor(err), "missing required arg must yield ExitUsage")
}

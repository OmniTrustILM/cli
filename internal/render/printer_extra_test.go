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
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatPtrForTest verifies that FormatPtrForTest returns the pointer that
// the AddFlags wiring uses, so cross-package tests can inject a format without
// flag parsing.
func TestFormatPtrForTest(t *testing.T) {
	t.Parallel()
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)

	ptr := p.FormatPtrForTest()
	require.NotNil(t, ptr)

	// Mutating via the pointer must be reflected in Format().
	*ptr = "yaml"
	assert.Equal(t, "yaml", p.Format())
}

// TestResolveColor_FromFlagSet_NoColor verifies that ResolveColor reads the
// --no-color flag from a flag set and sets the printer to ColorNever.
func TestResolveColor_FromFlagSet_NoColor(t *testing.T) {
	t.Parallel()
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Bool("no-color", false, "")
	fs.Bool("color", false, "")
	require.NoError(t, fs.Parse([]string{"--no-color"}))

	p.ResolveColor(fs)
	assert.Equal(t, ColorNever, p.resolveColor())
}

// TestResolveColor_FromFlagSet_Color verifies that ResolveColor reads the
// --color flag from a flag set and sets the printer to ColorAlways.
func TestResolveColor_FromFlagSet_Color(t *testing.T) {
	t.Parallel()
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Bool("no-color", false, "")
	fs.Bool("color", false, "")
	require.NoError(t, fs.Parse([]string{"--color"}))

	p.ResolveColor(fs)
	assert.Equal(t, ColorAlways, p.resolveColor())
}

// TestResolveColor_NilFlagSet verifies that ResolveColor is a no-op on a nil flag set.
func TestResolveColor_NilFlagSet(t *testing.T) {
	t.Parallel()
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	// Should not panic.
	assert.NotPanics(t, func() { p.ResolveColor(nil) })
	// State must remain unchanged (ColorAuto).
	assert.Equal(t, ColorAuto, p.resolveColor())
}

// TestColorEnabled_Auto_StdoutIsNotTTY verifies that Auto mode returns false
// when writing to os.Stdout in test (which is not a character device).
// We cannot force a TTY in unit tests so we only test the non-TTY branch.
func TestColorEnabled_Auto_StdoutIsNotTTY(t *testing.T) {
	// os.Stdout in a test process is typically a pipe, not a TTY.
	// This exercises the *os.File branch of ColorEnabled where info.Mode() does
	// not have ModeCharDevice set. The result depends on the environment (might be
	// true in a real terminal, false in CI). We only assert no panic.
	assert.NotPanics(t, func() {
		_ = ColorEnabled(ColorAuto, os.Stdout)
	})
}

// TestColorEnabled_Auto_RealFile verifies Auto mode on a regular file (not a
// TTY): must return false because a regular file is not a character device.
func TestColorEnabled_Auto_RealFile(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "colortest-*.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	// A regular file is never a character device.
	assert.False(t, ColorEnabled(ColorAuto, f), "regular file must not be detected as a TTY")
}

// TestSeverityColor_DefaultBranch verifies that severityColor returns an empty
// string for an unknown severity value, covering the default branch.
func TestSeverityColor_DefaultBranch(t *testing.T) {
	t.Parallel()
	// "unknown" is not one of OK/Warn/Fail/Info.
	assert.Equal(t, "", severityColor("unknown"))
}

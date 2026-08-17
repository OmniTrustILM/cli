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

// Package render holds the kubectl-identical printers, human tables and color
// handling. P2 completes the structured-printing surface (-o json|yaml|name|
// jsonpath|go-template) over cli-runtime PrintFlags.
package render

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

// ColorMode controls ANSI colouring.
type ColorMode int

const (
	// ColorAuto detects TTY + NO_COLOR; used when neither --color nor --no-color is set.
	ColorAuto ColorMode = iota
	// ColorAlways forces colour output regardless of TTY state.
	ColorAlways
	// ColorNever disables colour output regardless of TTY state.
	ColorNever
)

// Printer wraps output format flags plus color/TTY handling.
type Printer struct {
	// flags drives the -o/--output flag and structured printing via cli-runtime.
	flags *genericclioptions.PrintFlags

	Out    io.Writer
	ErrOut io.Writer
	Color  ColorMode

	noColor bool
	color   bool
}

// NewPrinter builds a Printer targeting the given writers.
func NewPrinter(out, errOut io.Writer) *Printer {
	return &Printer{
		flags:  genericclioptions.NewPrintFlags(""),
		Out:    out,
		ErrOut: errOut,
		Color:  ColorAuto,
	}
}

// AddFlags registers --color, --no-color, and -o/--output on the flag set.
// The root command passes root.PersistentFlags() (*pflag.FlagSet) here.
func (p *Printer) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&p.noColor, "no-color", false, "Disable colored output.")
	fs.BoolVar(&p.color, "color", false, "Force colored output even when not a TTY.")
	// cli-runtime PrintFlags.AddFlags takes a *cobra.Command. Use a staging command so
	// it can register its flags there, then merge them onto our pflag.FlagSet. The
	// staging command is throwaway; only its flag definitions are transferred.
	staging := &cobra.Command{Use: "_"}
	p.flags.AddFlags(staging)
	staging.Flags().VisitAll(func(f *pflag.Flag) {
		if fs.Lookup(f.Name) == nil {
			fs.AddFlag(f)
		}
	})
	// OutputFlagSpecified was bound to the staging command's flag set; re-point it at the
	// real flag set so callers get an accurate answer after flag parsing.
	p.flags.OutputFlagSpecified = func() bool { return fs.Changed("output") }
}

// Format returns the resolved -o value ("" means table/human view).
func (p *Printer) Format() string {
	if p.flags.OutputFormat == nil {
		return ""
	}
	return *p.flags.OutputFormat
}

// FormatPtrForTest exposes the underlying -o string pointer so cross-package
// tests can set the resolved format without parsing flags. Must only be called
// from _test.go files.
func (p *Printer) FormatPtrForTest() *string {
	return p.flags.OutputFormat
}

// ResolveColor reads the --color/--no-color flags from the given flag set and
// updates the Printer's Color field. Commands that have a flag set separate from
// the root PersistentFlags call this in their RunE so the resolved mode is available
// before the first PrintTable/PrintObject call. When the flags are bound to the
// root PersistentFlags (the common case), color is already resolved and this is a no-op.
func (p *Printer) ResolveColor(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}
	if f := fs.Lookup("no-color"); f != nil && f.Value.String() == "true" {
		p.noColor = true
	}
	if f := fs.Lookup("color"); f != nil && f.Value.String() == "true" {
		p.color = true
	}
}

// resolveColor folds the flags into a ColorMode (called by UseColor and tests).
func (p *Printer) resolveColor() ColorMode {
	switch {
	case p.noColor:
		return ColorNever
	case p.color:
		return ColorAlways
	default:
		return ColorAuto
	}
}

// ColorEnabled reports whether colour should be emitted, honouring NO_COLOR + TTY.
func ColorEnabled(mode ColorMode, out io.Writer) bool {
	switch mode {
	case ColorNever:
		return false
	case ColorAlways:
		return true
	default:
		if _, ok := os.LookupEnv("NO_COLOR"); ok {
			return false
		}
		f, ok := out.(*os.File)
		if !ok {
			return false
		}
		info, err := f.Stat()
		if err != nil {
			return false
		}
		return info.Mode()&os.ModeCharDevice != 0
	}
}

// formatJSON and formatYAML are the canonical structured -o values the printer
// and the table renderers branch on.
const (
	formatJSON = "json"
	formatYAML = "yaml"
)

// structuredFormats lists the -o values that produce machine output (table/wide are
// human views handled by the table builders).
var structuredFormats = map[string]bool{
	formatJSON: true, formatYAML: true, "name": true,
	"jsonpath": true, "jsonpath-as-json": true, "jsonpath-file": true,
	"go-template": true, "go-template-file": true, "template": true, "templatefile": true,
}

// Structured reports whether the resolved -o value is a machine format that
// PrintObject can render (everything except "" and "wide").
func (p *Printer) Structured() bool {
	f := p.Format()
	for prefix := range structuredFormats {
		if f == prefix || hasFormatPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// hasFormatPrefix reports whether f is "<prefix>=..." (jsonpath/go-template form).
func hasFormatPrefix(f, prefix string) bool {
	return len(f) > len(prefix) && f[:len(prefix)] == prefix && f[len(prefix)] == '='
}

// ToPrinter returns the cli-runtime ResourcePrinter for the resolved -o value.
func (p *Printer) ToPrinter() (printers.ResourcePrinter, error) {
	return p.flags.ToPrinter()
}

// PrintObject renders a runtime.Object using the resolved -o printer. It returns
// an error in table mode (no -o set): tables are produced by the table builders.
func (p *Printer) PrintObject(obj runtime.Object) error {
	if !p.Structured() {
		return fmt.Errorf("render: PrintObject requires a structured -o format, got %q", p.Format())
	}
	rp, err := p.ToPrinter()
	if err != nil {
		return err
	}
	return rp.PrintObj(obj, p.Out)
}

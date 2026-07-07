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

// Package cli wires the cobra root command, help-groups, global flags and the
// dual-invocation entrypoint shared by ilmctl and kubectl-ilm.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/OmniTrustILM/cli/internal/buildinfo"
	"github.com/OmniTrustILM/cli/internal/config"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// Exit codes (conventional; matches kubectl/linkerd).
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

// UsageError marks a cobra usage failure so Run maps it to ExitUsage.
type UsageError struct{ error }

// NewUsageError wraps err so Run maps it to ExitUsage (2).
func NewUsageError(err error) error { return UsageError{err} }

// ErrFailure is the sentinel a command returns to make Run exit ExitFailure (1)
// without printing a usage banner (distinct from UsageError which exits ExitUsage).
var ErrFailure = errors.New("command found failures")

// ExitCodeFor maps a command error to the conventional exit code:
// nil → 0; a usage error → 2; anything else → 1. This is the single
// authority every invocation path (Run, tests) routes through.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	if isUsageError(err) {
		return ExitUsage
	}
	if errors.Is(err, ErrFailure) {
		return ExitFailure
	}
	// cobra reports unknown command / unknown flag / missing args as usage failures.
	msg := err.Error()
	if strings.HasPrefix(msg, "unknown command") ||
		strings.HasPrefix(msg, "unknown flag") ||
		strings.HasPrefix(msg, "unknown shorthand flag") ||
		strings.Contains(msg, "required flag") ||
		strings.HasPrefix(msg, "invalid argument") ||
		strings.HasPrefix(msg, "accepts ") {
		return ExitUsage
	}
	return ExitFailure
}

// Options threads through every command: client factory, printer, config, IO.
// NewDefaultOptions constructs Factory, Printer, and ConfigLoader, so commands
// can rely on them being non-nil.
type Options struct {
	ConfigFlags  *genericclioptions.ConfigFlags
	Factory      *k8s.Factory
	Printer      *render.Printer
	ConfigLoader *config.Loader

	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	// InvokedAs is "ilmctl" or "kubectl-ilm", derived from os.Args[0].
	InvokedAs string
}

// NewDefaultOptions builds Options with default IO and config plumbing. The
// Factory is built from the same ConfigFlags pointer that the root command
// binds the kubectl-style global flags to; because the Factory reads those
// flags lazily (at Client()/RESTConfig() time), constructing it here — before
// cobra parses the flags during Execute() — still observes the parsed values.
func NewDefaultOptions(out, errOut io.Writer) *Options {
	cf := genericclioptions.NewConfigFlags(true)
	// NewFactory fails only if cf is nil (it is not) or the compiled-in scheme
	// fails to register — both unreachable build errors. Fail fast with a clear
	// message rather than leaving Factory nil, which would resurface as a nil
	// dereference in every cluster command.
	factory, err := k8s.NewFactory(cf)
	if err != nil {
		panic(fmt.Sprintf("cli: build kubernetes factory: %v", err))
	}
	return &Options{
		ConfigFlags:  cf,
		Factory:      factory,
		ConfigLoader: &config.Loader{},
		Printer:      render.NewPrinter(out, errOut),
		In:           os.Stdin,
		Out:          out,
		ErrOut:       errOut,
		InvokedAs:    buildinfo.BinaryName,
	}
}

// NewRootCommand assembles the root command: help-groups, global kubectl-style
// flags, the --ilmconfig flag, and --color/--no-color plumbing on the printer.
func NewRootCommand(o *Options) *cobra.Command {
	use := buildinfo.BinaryName
	if o.InvokedAs == buildinfo.PluginBinaryName {
		use = "kubectl " + buildinfo.PluginWord
	}

	root := &cobra.Command{
		Use:          use,
		Short:        "ilmctl — manage ILM on Kubernetes",
		Long:         "ilmctl (also installed as kubectl-ilm) installs, operates, inspects and diagnoses ILM on any Kubernetes cluster.",
		SilenceUsage: true,
		// SilenceErrors is intentionally false so cobra writes "Error: …" to the
		// configured ErrOut (set via root.SetErr). Run() therefore omits a second
		// print; commands gate their own usage printing via SilenceUsage.
		// No Args constraint here: cobra's default unknown-command handling reports
		// "unknown command" for mistyped subcommands, which is the correct UX.
		// No RunE: cobra shows help when invoked with no subcommand (default behaviour
		// for a command with subcommands and no Run/RunE).
	}
	root.SetOut(o.Out)
	root.SetErr(o.ErrOut)
	root.SetIn(o.In)

	registerGroups(root)

	root.AddCommand(newVersionCommand(o))
	root.AddCommand(newCompletionCommand(o))

	// Suppress cobra's built-in __complete / completion commands; our custom
	// completion command is the single, documented entry point.
	root.CompletionOptions.DisableDefaultCmd = true

	// kubectl-identical global flags (--kubeconfig/--context/--cluster/--user/-n ...).
	o.ConfigFlags.AddFlags(root.PersistentFlags())
	// Reserved ilmctl context file flag.
	config.AddFlags(root.PersistentFlags(), o.ConfigLoader)
	// Output + color flags (--color/--no-color) live on the printer.
	if o.Printer != nil {
		o.Printer.AddFlags(root.PersistentFlags())
	}

	return root
}

// resolveInvokedAs maps os.Args[0] to "ilmctl" or "kubectl-ilm".
func resolveInvokedAs(arg0 string) string {
	base := filepath.Base(arg0)
	base = strings.TrimSuffix(base, ".exe")
	if base == buildinfo.PluginBinaryName {
		return buildinfo.PluginBinaryName
	}
	return buildinfo.BinaryName
}

// Run is the process entrypoint: it resolves the invocation name, builds Options,
// executes the command tree, and maps errors to conventional exit codes.
func Run(args []string) int {
	return RunWithCommands(args, nil)
}

// RunWithCommands is like Run but calls register(root, o) before Execute, so
// callers (main.go) can add phase-specific subcommands without creating an
// import cycle between internal/cli and the command packages that import it.
// register may be nil.
func RunWithCommands(args []string, register func(*cobra.Command, *Options)) int {
	o := NewDefaultOptions(os.Stdout, os.Stderr)
	if len(args) > 0 {
		o.InvokedAs = resolveInvokedAs(args[0])
	}
	root := NewRootCommand(o)
	if register != nil {
		register(root, o)
	}
	if len(args) > 1 {
		root.SetArgs(args[1:])
	} else {
		root.SetArgs(nil)
	}

	// cobra already wrote "Error: …" to o.ErrOut via root.SetErr; no re-print here.
	return ExitCodeFor(root.Execute())
}

// isUsageError reports whether err (or any wrapped cause) is a UsageError.
func isUsageError(err error) bool {
	for e := err; e != nil; {
		if _, ok := e.(UsageError); ok {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

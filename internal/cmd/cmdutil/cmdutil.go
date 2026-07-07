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

// Package cmdutil provides shared helpers consumed by all resource subcommand packages.
package cmdutil

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// OrNone returns "<none>" when s is empty, otherwise s.
func OrNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// Age formats a duration since t as a human-readable string (e.g. "5m", "3h", "2d").
func Age(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// OtilmGVK returns the GroupVersionKind for the given kind in the otilm.com/v1alpha1 group.
func OtilmGVK(kind string) schema.GroupVersionKind {
	return otilmv1alpha1.GroupVersion.WithKind(kind)
}

// RunFn is the signature shared by single-NAME subcommand implementations.
type RunFn func(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error

// SingleNameOpts carries optional ClientFn and NamespaceFn seams used in hermetic
// tests. When either field is nil the command falls back to the corresponding
// o.Factory method.
type SingleNameOpts struct {
	// ClientFn overrides o.Factory.Client() when set; used in tests to inject a
	// fake client without a live apiserver.
	ClientFn func() (*k8s.Client, error)
	// NamespaceFn overrides o.Factory.Namespace() when set alongside ClientFn
	// in tests that have no live kubeconfig.
	NamespaceFn func() (string, bool, error)
}

// NewSingleNameCommand builds a cobra.Command that requires exactly one NAME argument,
// resolves the client + namespace from Options (or opts seams), and delegates to fn.
// The -o and color flags are registered on the command.
func NewSingleNameCommand(o *cli.Options, opts *SingleNameOpts, use, short string, fn RunFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Printer.ResolveColor(cmd.Flags())
			c, ns, err := resolveClientNS(o, opts)
			if err != nil {
				return err
			}
			return fn(cmd.Context(), c, o.Printer, ns, args[0])
		},
	}
	o.Printer.AddFlags(cmd.Flags())
	return cmd
}

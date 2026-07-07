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

package platform

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/health"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewGetCommand builds the `platform get` command.
func NewGetCommand(o *cli.Options) *cobra.Command {
	return newGetCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newGetCommand(o *cli.Options) *cobra.Command { return NewGetCommand(o) }

// newGetCommandFromOpts builds the get cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newGetCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewOptionalNameCommand(o, opts,
		"get [NAME]",
		"List or get Platform instances (wide: phase, version, ready, age)",
		runGet,
	)
}

// runGet lists (name=="") or gets one Platform, honouring -o for structured output.
func runGet(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	if shared.OResolved(p) {
		return getStructured(ctx, c, p, ns, name)
	}
	tbl := render.Table{Columns: []string{"NAMESPACE", "NAME", "PHASE", "VERSION", "READY", "AGE"}}
	add := func(pl otilmv1alpha1.Platform) {
		ready, total := health.Summarize(pl.Status.Conditions)
		tbl.Rows = append(tbl.Rows, []string{
			pl.Namespace, pl.Name, cmdutil.OrNone(string(pl.Status.Phase)), cmdutil.OrNone(pl.Status.ObservedVersion),
			fmt.Sprintf("%d/%d", ready, total), cmdutil.Age(pl.CreationTimestamp.Time),
		})
	}
	if name != "" {
		pl, err := c.GetPlatform(ctx, ns, name)
		if err != nil {
			return err
		}
		add(*pl)
	} else {
		list, err := c.ListPlatforms(ctx, ns)
		if err != nil {
			return err
		}
		for i := range list.Items {
			add(list.Items[i])
		}
	}
	return p.PrintTable(tbl)
}

func getStructured(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	if name != "" {
		pl, err := c.GetPlatform(ctx, ns, name)
		if err != nil {
			return err
		}
		pl.SetGroupVersionKind(cmdutil.OtilmGVK("Platform"))
		return p.PrintObject(pl)
	}
	list, err := c.ListPlatforms(ctx, ns)
	if err != nil {
		return err
	}
	list.SetGroupVersionKind(cmdutil.OtilmGVK("PlatformList"))
	return p.PrintObject(list)
}

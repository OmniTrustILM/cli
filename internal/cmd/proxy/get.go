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

package proxy

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewGetCommand builds the `proxy get` command.
func NewGetCommand(o *cli.Options) *cobra.Command {
	return newGetCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newGetCommand(o *cli.Options) *cobra.Command { return NewGetCommand(o) }

// newGetCommandFromOpts builds the get cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newGetCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewOptionalNameCommand(o, opts,
		"get [NAME]",
		"List or get Proxy instances (columns: namespace, phase, version, ready, age)",
		runGet,
	)
}

// runGet lists (name=="") or gets one Proxy, honouring -o for structured output.
func runGet(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	if shared.OResolved(p) {
		return getStructured(ctx, c, p, ns, name)
	}
	tbl := render.Table{Columns: []string{"NAMESPACE", "NAME", "PHASE", "VERSION", "READY", "AGE"}}
	add := func(prx otilmv1alpha1.Proxy) {
		tbl.Rows = append(tbl.Rows, []string{
			prx.Namespace, prx.Name, cmdutil.OrNone(string(prx.Status.Phase)),
			cmdutil.OrNone(prx.Status.ObservedVersion),
			fmt.Sprintf("%d", prx.Status.ReadyReplicas),
			cmdutil.Age(prx.CreationTimestamp.Time),
		})
	}
	if name != "" {
		prx, err := c.GetProxy(ctx, ns, name)
		if err != nil {
			return err
		}
		add(*prx)
	} else {
		list, err := c.ListProxies(ctx, ns)
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
		prx, err := c.GetProxy(ctx, ns, name)
		if err != nil {
			return err
		}
		prx.SetGroupVersionKind(cmdutil.OtilmGVK("Proxy"))
		return p.PrintObject(prx)
	}
	list, err := c.ListProxies(ctx, ns)
	if err != nil {
		return err
	}
	list.SetGroupVersionKind(cmdutil.OtilmGVK("ProxyList"))
	return p.PrintObject(list)
}

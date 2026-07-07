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

package connector

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

// NewGetCommand builds the `connector get` command.
func NewGetCommand(o *cli.Options) *cobra.Command {
	return newGetCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newGetCommand(o *cli.Options) *cobra.Command { return NewGetCommand(o) }

// newGetCommandFromOpts builds the get cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newGetCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewOptionalNameCommand(o, opts,
		"get [NAME]",
		"List or get Connector instances (wide: phase, ready, endpoint, registration status, age)",
		runGet,
	)
}

// runGet lists (name=="") or gets one Connector, honouring -o for structured output.
func runGet(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	if shared.OResolved(p) {
		return getStructured(ctx, c, p, ns, name)
	}
	tbl := render.Table{Columns: []string{"NAMESPACE", "NAME", "PHASE", "READY", "ENDPOINT", "REGISTRATION", "AGE"}}
	add := func(cn otilmv1alpha1.Connector) {
		reg := "<not registered>"
		if r := cn.Status.Registration; r != nil {
			reg = string(r.Status)
		}
		tbl.Rows = append(tbl.Rows, []string{
			cn.Namespace, cn.Name, cmdutil.OrNone(string(cn.Status.Phase)),
			fmt.Sprintf("%d/%d", cn.Status.ReadyReplicas, cn.Status.Replicas),
			cmdutil.OrNone(cn.Status.Endpoint), reg, cmdutil.Age(cn.CreationTimestamp.Time),
		})
	}
	if name != "" {
		cn, err := c.GetConnector(ctx, ns, name)
		if err != nil {
			return err
		}
		add(*cn)
	} else {
		list, err := c.ListConnectors(ctx, ns)
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
		cn, err := c.GetConnector(ctx, ns, name)
		if err != nil {
			return err
		}
		cn.SetGroupVersionKind(cmdutil.OtilmGVK("Connector"))
		return p.PrintObject(cn)
	}
	list, err := c.ListConnectors(ctx, ns)
	if err != nil {
		return err
	}
	list.SetGroupVersionKind(cmdutil.OtilmGVK("ConnectorList"))
	return p.PrintObject(list)
}

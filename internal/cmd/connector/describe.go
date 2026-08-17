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
	"io"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewDescribeCommand builds the `connector describe` command.
func NewDescribeCommand(o *cli.Options) *cobra.Command {
	return newDescribeCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newDescribeCommand(o *cli.Options) *cobra.Command { return NewDescribeCommand(o) }

// newDescribeCommandFromOpts builds the describe cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newDescribeCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"describe NAME",
		"Spec summary, conditions, child Deployment, and recent events",
		runDescribe,
	)
}

func runDescribe(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	conn, err := c.GetConnector(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		conn.SetGroupVersionKind(cmdutil.OtilmGVK("Connector"))
		return p.PrintObject(conn)
	}
	w := p.Out
	if err := printConnectorHeader(w, ns, name, conn); err != nil {
		return err
	}
	if err := printConnectorConditions(w, p, conn); err != nil {
		return err
	}
	if err := printConnectorPods(ctx, w, c, p, ns, name); err != nil {
		return err
	}
	return printConnectorEvents(ctx, w, c, p, ns, conn)
}

func printConnectorHeader(w io.Writer, ns, name string, conn *otilmv1alpha1.Connector) error {
	if _, err := fmt.Fprintf(w, "Name:      %s/%s\n", ns, name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Phase:     %s\n", cmdutil.OrNone(string(conn.Status.Phase))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Image:     %s:%s\n",
		cmdutil.OrNone(conn.Spec.Image.Repository), cmdutil.OrNone(conn.Spec.Image.Tag)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Endpoint:  %s\n", cmdutil.OrNone(conn.Status.Endpoint)); err != nil {
		return err
	}
	if reg := conn.Spec.Registration; reg != nil {
		_, err := fmt.Fprintf(w, "Registration target: platformUrl=%s name=%s authType=%s\n",
			cmdutil.OrNone(reg.PlatformURL), cmdutil.OrNone(reg.Name), cmdutil.OrNone(string(reg.AuthType)))
		return err
	}
	return nil
}

func printConnectorConditions(w io.Writer, p *render.Printer, conn *otilmv1alpha1.Connector) error {
	if len(conn.Status.Conditions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nConditions:"); err != nil {
		return err
	}
	return cmdutil.RenderConditions(p, "TYPE", conn.Status.Conditions)
}

func printConnectorPods(ctx context.Context, w io.Writer, c *k8s.Client, p *render.Printer, ns, name string) error {
	pods, perr := c.PodsFor(ctx, ns, connectorPodSelector(name))
	if perr != nil || len(pods) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nPods:"); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"NAME", "PHASE"}}
	for i := range pods {
		pod := pods[i]
		tbl.Rows = append(tbl.Rows, []string{pod.Name, string(pod.Status.Phase)})
	}
	return p.PrintTable(tbl)
}

func printConnectorEvents(ctx context.Context, w io.Writer, c *k8s.Client, p *render.Printer, ns string, conn *otilmv1alpha1.Connector) error {
	evs, eerr := c.Events(ctx, ns, conn)
	if eerr != nil || len(evs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nRecent Events:"); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"TYPE", "REASON", "MESSAGE"}}
	for i := range evs {
		e := evs[i]
		tbl.Rows = append(tbl.Rows, []string{e.Type, e.Reason, e.Message})
	}
	return p.PrintTable(tbl)
}

// connectorPodLabel is the operator-applied pod label key carrying the owning
// Connector's name.
const connectorPodLabel = "otilm.com/connector"

// connectorPodSelector returns the label selector that uniquely matches a
// connector's own pods. The workload pods carry otilm.com/connector=<name>,
// which is the canonical selector for a single connector's pods.
func connectorPodSelector(name string) map[string]string {
	return map[string]string{
		connectorPodLabel: name,
	}
}

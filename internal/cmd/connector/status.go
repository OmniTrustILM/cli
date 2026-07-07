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
)

// NewStatusCommand builds the `connector status` command.
func NewStatusCommand(o *cli.Options) *cobra.Command {
	return newStatusCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newStatusCommand(o *cli.Options) *cobra.Command { return NewStatusCommand(o) }

// newStatusCommandFromOpts builds the status cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newStatusCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"status NAME",
		"Show a Connector's phase and platform registration status\n\n"+
			"NOTE: registration status reflects a runtime handshake with a running ILM Core\n"+
			"      platform instance. The status will not show 'connected' until the platform\n"+
			"      is reachable and has approved the connector.",
		runStatus,
	)
}

func runStatus(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	conn, err := c.GetConnector(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		conn.SetGroupVersionKind(cmdutil.OtilmGVK("Connector"))
		return p.PrintObject(conn)
	}
	w := p.Out
	if _, err := fmt.Fprintf(w, "Name:     %s/%s\n", ns, name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Phase:    %s\n", cmdutil.OrNone(string(conn.Status.Phase))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Ready:    %d/%d\n", conn.Status.ReadyReplicas, conn.Status.Replicas); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Endpoint: %s\n", cmdutil.OrNone(conn.Status.Endpoint)); err != nil {
		return err
	}
	reg := "<not registered>"
	if r := conn.Status.Registration; r != nil {
		registeredAt := "<unknown>"
		if r.RegisteredAt != nil {
			registeredAt = r.RegisteredAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		reg = fmt.Sprintf("uuid=%s status=%s registeredAt=%s", cmdutil.OrNone(r.UUID), cmdutil.OrNone(string(r.Status)), registeredAt)
	}
	if _, err := fmt.Fprintf(w, "Register: %s\n", reg); err != nil {
		return err
	}
	if len(conn.Status.Conditions) > 0 {
		tbl := render.Table{Columns: []string{"CONDITION", "STATUS", "REASON", "MESSAGE"}}
		for _, cnd := range conn.Status.Conditions {
			tbl.Rows = append(tbl.Rows, []string{cnd.Type, string(cnd.Status), cnd.Reason, cnd.Message})
		}
		return p.PrintTable(tbl)
	}
	return nil
}

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
	"io"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewDescribeCommand builds the `platform describe` command.
func NewDescribeCommand(o *cli.Options) *cobra.Command {
	return newDescribeCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newDescribeCommand(o *cli.Options) *cobra.Command { return NewDescribeCommand(o) }

// newDescribeCommandFromOpts builds the describe cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newDescribeCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"describe NAME",
		"Spec summary, conditions, child resources, events and resolved endpoints",
		runDescribe,
	)
}

func runDescribe(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	pl, err := c.GetPlatform(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		pl.SetGroupVersionKind(cmdutil.OtilmGVK("Platform"))
		return p.PrintObject(pl)
	}
	w := p.Out
	if err := printPlatformHeader(w, ns, name, pl); err != nil {
		return err
	}
	if err := printPlatformEndpoints(w, p, pl); err != nil {
		return err
	}
	if err := printPlatformConditions(w, p, pl); err != nil {
		return err
	}
	if err := printPlatformDeployments(ctx, w, c, p, ns, name); err != nil {
		return err
	}
	return printPlatformEvents(ctx, w, c, p, ns, pl)
}

func printPlatformHeader(w io.Writer, ns, name string, pl *otilmv1alpha1.Platform) error {
	if _, err := fmt.Fprintf(w, "Name:      %s/%s\n", ns, name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Phase:     %s\n", cmdutil.OrNone(string(pl.Status.Phase))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Version:   %s\n", cmdutil.OrNone(pl.Status.ObservedVersion)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Database:  mode=%s\n", cmdutil.OrNone(pl.Spec.Database.Mode)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Messaging: mode=%s broker=%s\n", cmdutil.OrNone(pl.Spec.Messaging.Mode), cmdutil.OrNone(pl.Spec.Messaging.BrokerType)); err != nil {
		return err
	}
	// Keycloak is the third managed backing service. spec.keycloak is a pointer:
	// a nil block means no Keycloak is configured (mode "none").
	keycloakMode := "none"
	if pl.Spec.Keycloak != nil {
		keycloakMode = pl.Spec.Keycloak.Mode
	}
	_, err := fmt.Fprintf(w, "Keycloak:  mode=%s\n", cmdutil.OrNone(keycloakMode))
	return err
}

func printPlatformEndpoints(w io.Writer, p *render.Printer, pl *otilmv1alpha1.Platform) error {
	if _, err := fmt.Fprintln(w, "\nEndpoints:"); err != nil {
		return err
	}
	for _, ep := range resolveEndpoints(pl) {
		if _, err := fmt.Fprintf(w, "  %-12s %s\n", ep.label+":", ep.url); err != nil {
			return err
		}
	}
	_ = p // satisfy interface consistency; printer used for tables below
	return nil
}

func printPlatformConditions(w io.Writer, p *render.Printer, pl *otilmv1alpha1.Platform) error {
	if len(pl.Status.Conditions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nConditions:"); err != nil {
		return err
	}
	return cmdutil.RenderConditions(p, "TYPE", pl.Status.Conditions)
}

func printPlatformDeployments(ctx context.Context, w io.Writer, c *k8s.Client, p *render.Printer, ns, name string) error {
	deps, derr := c.DeploymentsForPlatform(ctx, ns, name)
	if derr != nil || len(deps) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nChild Deployments:"); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"NAME", "READY", "AVAILABLE"}}
	for i := range deps {
		d := deps[i]
		tbl.Rows = append(tbl.Rows, []string{
			d.Name, fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, d.Status.Replicas),
			fmt.Sprintf("%d", d.Status.AvailableReplicas),
		})
	}
	return p.PrintTable(tbl)
}

func printPlatformEvents(ctx context.Context, w io.Writer, c *k8s.Client, p *render.Printer, ns string, pl *otilmv1alpha1.Platform) error {
	evs, eerr := c.Events(ctx, ns, pl)
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

type endpoint struct{ label, url string }

// resolveEndpoints derives the externally-reachable URLs. The admin UI host comes from
// spec.edge.host, falling back to spec.common.hostName. The RabbitMQ management /mq route
// is rendered ONLY for a managed RabbitMQ broker with management exposure enabled.
func resolveEndpoints(pl *otilmv1alpha1.Platform) []endpoint {
	host := platformHost(pl)
	if host == "" {
		return []endpoint{{"admin-ui", "<no public host configured>"}}
	}
	base := "https://" + host
	out := []endpoint{{"admin-ui", base}}
	m := pl.Spec.Messaging
	if m.Mode == modeManaged && m.BrokerType == brokerRabbitMQ && m.Management.Expose {
		// The endpoint's display name is deliberately a literal: it labels the /mq route
		// in `describe` output and is not coupled to the spec.messaging.brokerType enum
		// value brokerRabbitMQ, which merely shares the string.
		out = append(out, endpoint{"rabbitmq", base + "/mq"})
	}
	return out
}

func platformHost(pl *otilmv1alpha1.Platform) string {
	if pl.Spec.Edge != nil && pl.Spec.Edge.Host != "" {
		return pl.Spec.Edge.Host
	}
	return pl.Spec.Common.HostName
}

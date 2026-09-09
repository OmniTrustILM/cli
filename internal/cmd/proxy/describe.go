/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// NewDescribeCommand builds the `proxy describe` command.
func NewDescribeCommand(o *cli.Options) *cobra.Command {
	return newDescribeCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newDescribeCommand(o *cli.Options) *cobra.Command { return NewDescribeCommand(o) }

// newDescribeCommandFromOpts builds the describe cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newDescribeCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"describe NAME",
		"Spec summary, conditions, child pods, and recent events",
		runDescribe,
	)
}

func runDescribe(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	prx, err := c.GetProxy(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		prx.SetGroupVersionKind(cmdutil.OtilmGVK("Proxy"))
		return p.PrintObject(prx)
	}
	if err := printProxyHeader(p, ns, name, prx); err != nil {
		return err
	}
	if err := printProxyConditions(p, prx); err != nil {
		return err
	}
	if err := printProxyPods(ctx, c, p, ns, name); err != nil {
		return err
	}
	return printProxyEvents(ctx, c, p, ns, prx)
}

// printProxyHeader writes the name/phase/version/token/checksum header lines.
func printProxyHeader(p *render.Printer, ns, name string, prx *otilmv1alpha1.Proxy) error {
	w := p.Out
	lines := []string{
		fmt.Sprintf("Name:            %s/%s", ns, name),
		fmt.Sprintf("Phase:           %s", cmdutil.OrNone(string(prx.Status.Phase))),
		fmt.Sprintf("Version:         %s", cmdutil.OrNone(prx.Status.ObservedVersion)),
		fmt.Sprintf("ConfigToken:     %s", cmdutil.OrNone(prx.Spec.ConfigTokenSecretRef.Name)),
		fmt.Sprintf("ConfigChecksum:  %s", cmdutil.OrNone(prx.Status.ConfigChecksum)),
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// printProxyConditions prints the conditions table when conditions are present.
func printProxyConditions(p *render.Printer, prx *otilmv1alpha1.Proxy) error {
	if len(prx.Status.Conditions) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(p.Out, "\nConditions:"); err != nil {
		return err
	}
	return cmdutil.RenderConditions(p, "TYPE", prx.Status.Conditions)
}

// printProxyPods fetches and prints the pods table when pods are present.
func printProxyPods(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	pods, err := c.PodsFor(ctx, ns, proxyPodSelector(name))
	if err != nil || len(pods) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(p.Out, "\nPods:"); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"NAME", "PHASE"}}
	for i := range pods {
		tbl.Rows = append(tbl.Rows, []string{pods[i].Name, string(pods[i].Status.Phase)})
	}
	return p.PrintTable(tbl)
}

// printProxyEvents fetches and prints the events table when events are present.
func printProxyEvents(ctx context.Context, c *k8s.Client, p *render.Printer, ns string, prx *otilmv1alpha1.Proxy) error {
	evs, err := c.Events(ctx, ns, prx)
	if err != nil || len(evs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(p.Out, "\nRecent Events:"); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"TYPE", "REASON", "MESSAGE"}}
	for i := range evs {
		tbl.Rows = append(tbl.Rows, []string{evs[i].Type, evs[i].Reason, evs[i].Message})
	}
	return p.PrintTable(tbl)
}

// proxyPodLabel is the operator-applied pod label key carrying the owning
// Proxy's name.
const proxyPodLabel = "otilm.com/proxy"

// proxyPodSelector returns the label selector that uniquely matches a proxy's
// own pods. The workload pods carry otilm.com/proxy=<name>, which is the
// canonical selector for a single proxy's pods.
func proxyPodSelector(name string) map[string]string {
	return map[string]string{
		proxyPodLabel: name,
	}
}

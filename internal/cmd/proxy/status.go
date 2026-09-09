/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
)

// NewStatusCommand builds the `proxy status` command.
func NewStatusCommand(o *cli.Options) *cobra.Command {
	return newStatusCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newStatusCommand(o *cli.Options) *cobra.Command { return NewStatusCommand(o) }

// newStatusCommandFromOpts builds the status cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newStatusCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"status NAME",
		"Show a Proxy's phase, version, config checksum, and conditions",
		runStatus,
	)
}

func runStatus(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	prx, err := c.GetProxy(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		prx.SetGroupVersionKind(cmdutil.OtilmGVK("Proxy"))
		return p.PrintObject(prx)
	}
	w := p.Out
	if _, err := fmt.Fprintf(w, "Name:           %s/%s\n", ns, name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Phase:          %s\n", cmdutil.OrNone(string(prx.Status.Phase))); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Version:        %s\n", cmdutil.OrNone(prx.Status.ObservedVersion)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Ready:          %d\n", prx.Status.ReadyReplicas); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "ConfigChecksum: %s\n", cmdutil.OrNone(prx.Status.ConfigChecksum)); err != nil {
		return err
	}
	if len(prx.Status.Conditions) > 0 {
		return cmdutil.RenderConditions(p, "CONDITION", prx.Status.Conditions)
	}
	return nil
}

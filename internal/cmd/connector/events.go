/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package connector

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// NewEventsCommand builds the `connector events` command.
func NewEventsCommand(o *cli.Options) *cobra.Command {
	return newEventsCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newEventsCommand(o *cli.Options) *cobra.Command { return NewEventsCommand(o) }

// newEventsCommandFromOpts builds the events cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newEventsCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"events NAME",
		"Show recent events for a Connector",
		runEvents,
	)
}

func runEvents(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	conn, err := c.GetConnector(ctx, ns, name)
	if err != nil {
		return err
	}
	evs, err := c.Events(ctx, ns, conn)
	if err != nil {
		return err
	}
	return cmdutil.RenderEventsTable(p, evs, "connector", ns, name)
}

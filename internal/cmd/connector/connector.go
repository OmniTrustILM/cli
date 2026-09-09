/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package connector holds the `connector` resource read/inspect subcommands.
package connector

import (
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
)

// NewConnectorCommand builds the `connector` parent command and its read subcommands.
//
// Note: registration status reflects a runtime handshake between the connector and a
// running ILM Core platform instance. The status will not show "connected" until the
// platform is reachable and has approved the connector.
func NewConnectorCommand(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connector",
		Aliases: []string{"conn", "connectors"},
		Short:   "Inspect ILM Connector instances",
		GroupID: string(cli.GroupResources),
	}
	cmd.AddCommand(
		newGetCommand(o), newStatusCommand(o), newDescribeCommand(o),
		newEventsCommand(o), newWaitCommand(o), newLogsCommand(o),
		NewGenerateCommand(o),
	)
	return cmd
}

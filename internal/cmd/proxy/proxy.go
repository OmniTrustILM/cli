/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package proxy holds the `proxy` resource read/inspect subcommands.
package proxy

import (
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
)

// proxyKind is the proxy resource noun: the command-group name and the value of
// the workload's component label.
const proxyKind = "proxy"

// NewProxyCommand builds the `proxy` parent command and its read subcommands.
func NewProxyCommand(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     proxyKind,
		Aliases: []string{"prx", "proxies"},
		Short:   "Inspect ILM Proxy instances",
		GroupID: string(cli.GroupResources),
	}
	cmd.AddCommand(
		newGetCommand(o), newStatusCommand(o), newDescribeCommand(o),
		newEventsCommand(o), newWaitCommand(o), newLogsCommand(o),
		NewGenerateCommand(o),
	)
	return cmd
}

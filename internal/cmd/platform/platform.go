/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package platform holds the `platform` resource read/inspect subcommands.
package platform

import (
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
)

// NewPlatformCommand builds the `platform` parent command and wires its read subcommands.
func NewPlatformCommand(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "platform",
		Aliases: []string{"plat", "platforms"},
		Short:   "Inspect ILM Platform instances",
		GroupID: string(cli.GroupResources),
	}
	cmd.AddCommand(
		newGetCommand(o), newStatusCommand(o), newDescribeCommand(o),
		newEventsCommand(o), newWaitCommand(o), newLogsCommand(o),
		NewGenerateCommand(o), NewMigrateCommand(o),
		NewApplyCommand(o), NewEditCommand(o), NewDeleteCommand(o),
		NewUpgradeCommand(o), NewCredentialsCommand(o),
	)
	return cmd
}

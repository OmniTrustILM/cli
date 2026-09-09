/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package deps

import (
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
)

// NewDepsCommand builds the `deps` parent command and wires its subcommands.
func NewDepsCommand(o *cli.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "deps",
		Short:   "Manage upstream operator dependencies",
		Long:    "Check for and install upstream operators that ILM managed modes depend on.",
		GroupID: string(cli.GroupInfrastructure),
	}
	cmd.AddCommand(
		NewCheckCommand(o),
		NewInstallCommand(o),
	)
	return cmd
}

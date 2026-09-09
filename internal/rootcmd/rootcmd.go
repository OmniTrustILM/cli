/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package rootcmd wires all resource and infrastructure subcommands into the
// root cobra command produced by cli.NewRootCommand.
package rootcmd

import (
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/connector"
	"github.com/OmniTrustILM/cli/internal/cmd/deps"
	"github.com/OmniTrustILM/cli/internal/cmd/diag"
	"github.com/OmniTrustILM/cli/internal/cmd/infra"
	"github.com/OmniTrustILM/cli/internal/cmd/platform"
	"github.com/OmniTrustILM/cli/internal/cmd/proxy"
)

// Register adds all infrastructure and resource subcommands to root.
// It is the single authoritative list; both New and the main entrypoint
// delegate here so command additions happen in exactly one place.
func Register(root *cobra.Command, o *cli.Options) {
	root.AddCommand(
		infra.NewInitCommand(o),
		infra.NewStatusCommand(o),
		infra.NewCheckCommand(o),
		infra.NewUpgradeCommand(o),
		infra.NewUninstallCommand(o),
		deps.NewDepsCommand(o),
		platform.NewPlatformCommand(o),
		connector.NewConnectorCommand(o),
		proxy.NewProxyCommand(o),
		diag.NewDiagnosticsCommand(o),
	)
}

// New builds the fully-wired root command: the bare root from cli.NewRootCommand
// plus all infrastructure and resource subcommands registered via Register.
func New(o *cli.Options) *cobra.Command {
	root := cli.NewRootCommand(o)
	Register(root, o)
	return root
}

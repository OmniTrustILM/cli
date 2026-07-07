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

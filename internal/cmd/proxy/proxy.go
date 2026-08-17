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

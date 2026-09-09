/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

func newLogsCommand(o *cli.Options) *cobra.Command {
	return newLogsCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

// newLogsCommandFromOpts builds the logs cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newLogsCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewLogsCommand(o, opts, "Tail a Proxy's pod logs", execLogs)
}

// proxyContainer is the default container name in a proxy pod.
const proxyContainer = "proxy"

// execLogs tails the proxy's own pods.
func execLogs(ctx context.Context, c *k8s.Client, w io.Writer, req shared.LogsRequest) error {
	return cmdutil.TailPodLogs(ctx, c, w, req,
		fmt.Sprintf("proxy %q", req.Name), proxyContainer, proxyPodSelector(req.Name))
}

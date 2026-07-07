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

package connector

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
	return cmdutil.NewLogsCommand(o, opts, "Tail a Connector's pod logs", execLogs)
}

// connectorContainer is the default container name in a connector pod.
const connectorContainer = "connector"

// execLogs tails the connector's own pods.
func execLogs(ctx context.Context, c *k8s.Client, w io.Writer, req shared.LogsRequest) error {
	return cmdutil.TailPodLogs(ctx, c, w, req,
		fmt.Sprintf("connector %q", req.Name), connectorContainer, connectorPodSelector(req.Name))
}

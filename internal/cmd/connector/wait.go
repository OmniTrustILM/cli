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
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

// NewWaitCommand builds the `connector wait` command.
func NewWaitCommand(o *cli.Options) *cobra.Command {
	return newWaitCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newWaitCommand(o *cli.Options) *cobra.Command { return NewWaitCommand(o) }

// newWaitCommandFromOpts builds the wait cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newWaitCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewWaitCommand(o, opts, "Connector", connectorConditions)
}

// connectorConditions reports the wait-relevant status of a single Connector.
func connectorConditions(ctx context.Context, c *k8s.Client, ns, name string) ([]metav1.Condition, string, int64, int64, error) {
	conn, err := c.GetConnector(ctx, ns, name)
	if err != nil {
		return nil, "", 0, 0, err
	}
	return conn.Status.Conditions, string(conn.Status.Phase), conn.Generation, conn.Status.ObservedGeneration, nil
}

// runWait blocks until the Connector meets target; it is the testable seam over
// shared.Wait and the connectorConditions getter.
func runWait(ctx context.Context, c *k8s.Client, ns, name string, target shared.WaitTarget, timeout time.Duration) error {
	return shared.Wait(ctx, func() ([]metav1.Condition, string, int64, int64, error) {
		return connectorConditions(ctx, c, ns, name)
	}, target, timeout)
}

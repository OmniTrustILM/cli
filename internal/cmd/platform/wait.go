/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

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

// NewWaitCommand builds the `platform wait` command.
func NewWaitCommand(o *cli.Options) *cobra.Command {
	return newWaitCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newWaitCommand(o *cli.Options) *cobra.Command { return NewWaitCommand(o) }

// newWaitCommandFromOpts builds the wait cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newWaitCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewWaitCommand(o, opts, "Platform", platformConditions)
}

// platformConditions reports the wait-relevant status of a single Platform.
func platformConditions(ctx context.Context, c *k8s.Client, ns, name string) ([]metav1.Condition, string, int64, int64, error) {
	pl, err := c.GetPlatform(ctx, ns, name)
	if err != nil {
		return nil, "", 0, 0, err
	}
	return pl.Status.Conditions, string(pl.Status.Phase), pl.Generation, pl.Status.ObservedGeneration, nil
}

// runWait blocks until the Platform meets target; it is the testable seam over
// shared.Wait and the platformConditions getter.
func runWait(ctx context.Context, c *k8s.Client, ns, name string, target shared.WaitTarget, timeout time.Duration) error {
	return shared.Wait(ctx, func() ([]metav1.Condition, string, int64, int64, error) {
		return platformConditions(ctx, c, ns, name)
	}, target, timeout)
}

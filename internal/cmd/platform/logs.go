/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

// logsOpts carries the optional clientFn and namespaceFn seams for the logs subcommand.
type logsOpts struct {
	// clientFn overrides o.Factory.Client() when set; used in tests to inject a
	// fake client without a live apiserver.
	clientFn func() (*k8s.Client, error)
	// namespaceFn overrides o.Factory.Namespace() when set alongside clientFn.
	namespaceFn func() (string, bool, error)
}

// defaultLogComponent is the component `platform logs` tails by default.
const defaultLogComponent = "core"

func newLogsCommand(o *cli.Options) *cobra.Command {
	return newLogsCommandFromOpts(o, &logsOpts{})
}

// newLogsCommandFromOpts builds the logs cobra.Command from caller-supplied opts.
// Tests pre-populate opts.clientFn to exercise RunE hermetically.
func newLogsCommandFromOpts(o *cli.Options, opts *logsOpts) *cobra.Command {
	var (
		component string
		follow    bool
		since     time.Duration
		tail      int64
	)
	cmd := &cobra.Command{
		Use:   "logs NAME --component <component>",
		Short: "Tail a Platform component's logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cf := opts.clientFn
			if cf == nil {
				cf = o.Factory.Client
			}
			c, err := cf()
			if err != nil {
				return err
			}
			nf := opts.namespaceFn
			if nf == nil {
				nf = o.Factory.Namespace
			}
			ns, _, err := nf()
			if err != nil {
				return err
			}
			if err := resolveLogComponent(component); err != nil {
				return cli.NewUsageError(err)
			}
			return execLogs(cmd.Context(), c, o.Printer.Out, shared.LogsRequest{
				Namespace: ns, Name: args[0], Component: component,
				Follow: follow, Since: since, Tail: tail,
			})
		},
	}
	cmd.Flags().StringVar(&component, "component", defaultLogComponent, "component to tail: "+strings.Join(shared.PlatformLogComponents, ", "))
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream logs")
	cmd.Flags().DurationVar(&since, "since", 0, "only return logs newer than this duration")
	cmd.Flags().Int64Var(&tail, "tail", 100, "lines of recent log to show (-1 = all)")
	return cmd
}

// resolveLogComponent validates --component against the known platform component set.
func resolveLogComponent(component string) error {
	if !slices.Contains(shared.PlatformLogComponents, component) {
		return fmt.Errorf("unknown component %q: valid components are %s",
			component, strings.Join(shared.PlatformLogComponents, ", "))
	}
	return nil
}

// execLogs tails a platform component's pods; the component name is also the
// container name inside each pod.
func execLogs(ctx context.Context, c *k8s.Client, w io.Writer, req shared.LogsRequest) error {
	return cmdutil.TailPodLogs(ctx, c, w, req,
		fmt.Sprintf("component %q of platform %q", req.Component, req.Name),
		req.Component, shared.ComponentSelector(req.Name, req.Component))
}

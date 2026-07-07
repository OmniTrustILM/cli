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

package cmdutil

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// resolveClientNS returns the client and default namespace, using the opts
// seams when set and otherwise falling back to o.Factory. It centralises the
// client/namespace resolution shared by every resource subcommand.
func resolveClientNS(o *cli.Options, opts *SingleNameOpts) (*k8s.Client, string, error) {
	cf := opts.ClientFn
	if cf == nil {
		cf = o.Factory.Client
	}
	c, err := cf()
	if err != nil {
		return nil, "", err
	}
	nf := opts.NamespaceFn
	if nf == nil {
		nf = o.Factory.Namespace
	}
	ns, _, err := nf()
	if err != nil {
		return nil, "", err
	}
	return c, ns, nil
}

// NewOptionalNameCommand is like NewSingleNameCommand but accepts an optional
// NAME argument (zero or one). When NAME is omitted, fn receives an empty name
// (list mode). The -o and color flags are registered on the command.
func NewOptionalNameCommand(o *cli.Options, opts *SingleNameOpts, use, short string, fn RunFn) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Printer.ResolveColor(cmd.Flags())
			c, ns, err := resolveClientNS(o, opts)
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return fn(cmd.Context(), c, o.Printer, ns, name)
		},
	}
	o.Printer.AddFlags(cmd.Flags())
	return cmd
}

// ConditionsGetter fetches the status fields a wait command polls for a single
// resource: its conditions, phase, generation, and observed generation.
type ConditionsGetter func(ctx context.Context, c *k8s.Client, ns, name string) (conds []metav1.Condition, phase string, generation, observedGeneration int64, err error)

// NewWaitCommand builds a `wait NAME --for=condition=<Type>|phase=<Phase>`
// command for the named resource (e.g. "Platform"). It resolves the
// client/namespace via the shared seams, parses --for, blocks via shared.Wait
// until the target is met, then prints "<resource>/<name> met <kind>=<value>".
func NewWaitCommand(o *cli.Options, opts *SingleNameOpts, resource string, getConds ConditionsGetter) *cobra.Command {
	var forExpr string
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "wait NAME --for=condition=<Type>|phase=<Phase>",
		Short: fmt.Sprintf("Block until a %s reaches a condition or phase", resource),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ns, err := resolveClientNS(o, opts)
			if err != nil {
				return err
			}
			target, err := shared.ParseWaitFor(forExpr)
			if err != nil {
				return cli.NewUsageError(err)
			}
			get := func() ([]metav1.Condition, string, int64, int64, error) {
				return getConds(cmd.Context(), c, ns, args[0])
			}
			if err := shared.Wait(cmd.Context(), get, target, timeout); err != nil {
				return err
			}
			_, err = fmt.Fprintf(o.Printer.Out, "%s/%s met %s=%s\n", strings.ToLower(resource), args[0], target.Kind, target.Value)
			return err
		},
	}
	cmd.Flags().StringVar(&forExpr, "for", "", "condition=<Type> or phase=<Phase>")
	cmd.Flags().DurationVar(&timeout, "timeout", shared.DefaultWaitTimeout, "maximum time to wait")
	_ = cmd.MarkFlagRequired("for")
	return cmd
}

// RenderConditions prints a resource's status conditions as a table. firstCol
// names the first column ("TYPE" or "CONDITION"). The table is printed even
// when there are no conditions (just the header row); callers that want to skip
// an empty section guard the call with len(conds) > 0.
func RenderConditions(p *render.Printer, firstCol string, conds []metav1.Condition) error {
	tbl := render.Table{Columns: []string{firstCol, "STATUS", "REASON", "MESSAGE"}}
	for _, cnd := range conds {
		tbl.Rows = append(tbl.Rows, []string{cnd.Type, string(cnd.Status), cnd.Reason, cnd.Message})
	}
	return p.PrintTable(tbl)
}

// LogsRunFn tails logs for a resolved client and request, writing to w.
type LogsRunFn func(ctx context.Context, c *k8s.Client, w io.Writer, req shared.LogsRequest) error

// NewLogsCommand builds a `logs NAME` command with the standard
// --follow/--since/--tail flags, resolves the client/namespace via the shared
// seams, and delegates the tail to run.
func NewLogsCommand(o *cli.Options, opts *SingleNameOpts, short string, run LogsRunFn) *cobra.Command {
	var (
		follow bool
		since  time.Duration
		tail   int64
	)
	cmd := &cobra.Command{
		Use:   "logs NAME",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, ns, err := resolveClientNS(o, opts)
			if err != nil {
				return err
			}
			return run(cmd.Context(), c, o.Printer.Out, shared.LogsRequest{
				Namespace: ns, Name: args[0], Follow: follow, Since: since, Tail: tail,
			})
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream logs")
	cmd.Flags().DurationVar(&since, "since", 0, "only return logs newer than this duration")
	cmd.Flags().Int64Var(&tail, "tail", 100, "lines of recent log to show (-1 = all)")
	return cmd
}

// TailPodLogs streams container logs from every pod matching selector to w.
// subject describes the workload for the not-found error (e.g. `connector "x"`).
func TailPodLogs(ctx context.Context, c *k8s.Client, w io.Writer, req shared.LogsRequest, subject, container string, selector map[string]string) error {
	pods, err := c.PodsFor(ctx, req.Namespace, selector)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return fmt.Errorf("no pods found for %s in namespace %q", subject, req.Namespace)
	}
	opts := podLogOptions(req, container)
	for i := range pods {
		rc, lerr := c.PodLogs(ctx, req.Namespace, pods[i].Name, container, opts)
		if lerr != nil {
			return lerr
		}
		if _, cerr := io.Copy(w, rc); cerr != nil {
			_ = rc.Close()
			return cerr
		}
		_ = rc.Close()
	}
	return nil
}

// podLogOptions builds a PodLogOptions from a LogsRequest for the given container.
func podLogOptions(req shared.LogsRequest, container string) *corev1.PodLogOptions {
	opts := &corev1.PodLogOptions{Follow: req.Follow, Container: container}
	if req.Tail >= 0 {
		t := req.Tail
		opts.TailLines = &t
	}
	if req.Since > 0 {
		secs := int64(req.Since.Seconds())
		opts.SinceSeconds = &secs
	}
	return opts
}

// RenderEventsTable prints resource events as a TYPE/REASON/OBJECT/MESSAGE
// table. When there are no events it prints a "no events for <kind>/<name> in
// <ns>" line instead.
func RenderEventsTable(p *render.Printer, evs []corev1.Event, kind, ns, name string) error {
	if len(evs) == 0 {
		_, err := fmt.Fprintf(p.Out, "no events for %s/%s in %s\n", kind, name, ns)
		return err
	}
	tbl := render.Table{Columns: []string{"TYPE", "REASON", "OBJECT", "MESSAGE"}}
	for i := range evs {
		e := evs[i]
		tbl.Rows = append(tbl.Rows, []string{
			e.Type, e.Reason,
			fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name), e.Message,
		})
	}
	return p.PrintTable(tbl)
}

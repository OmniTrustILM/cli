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

package platform

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/health"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// NewStatusCommand builds the `platform status` command.
func NewStatusCommand(o *cli.Options) *cobra.Command {
	return newStatusCommandFromOpts(o, &cmdutil.SingleNameOpts{})
}

func newStatusCommand(o *cli.Options) *cobra.Command { return NewStatusCommand(o) }

// newStatusCommandFromOpts builds the status cobra.Command from caller-supplied opts.
// Tests pre-populate opts.ClientFn to exercise RunE hermetically.
func newStatusCommandFromOpts(o *cli.Options, opts *cmdutil.SingleNameOpts) *cobra.Command {
	return cmdutil.NewSingleNameCommand(o, opts,
		"status NAME",
		"Show one Platform's phase, state and condition summary",
		runStatus,
	)
}

func runStatus(ctx context.Context, c *k8s.Client, p *render.Printer, ns, name string) error {
	pl, err := c.GetPlatform(ctx, ns, name)
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		pl.SetGroupVersionKind(cmdutil.OtilmGVK("Platform"))
		return p.PrintObject(pl)
	}
	state := health.PlatformState(pl)
	if _, err := fmt.Fprintf(p.Out, "%s/%s  phase=%s  state=%s  version=%s\n",
		ns, name, cmdutil.OrNone(string(pl.Status.Phase)), state, cmdutil.OrNone(pl.Status.ObservedVersion)); err != nil {
		return err
	}
	return cmdutil.RenderConditions(p, "CONDITION", pl.Status.Conditions)
}

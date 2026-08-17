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

package infra

import (
	"context"

	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// newReporter is the single place in this package that constructs a
// capabilities.Reporter from a connected client.
func newReporter(c *k8s.Client) *capabilities.Reporter {
	return capabilities.NewReporter(opcap.New(c.Mapper))
}

// modeManaged is the "managed" string literal used in Platform mode flags.
const modeManaged = "managed"

// checkCommand is the subcommand's name; synthesised findings reuse it as their Rule.
const checkCommand = "check"

type checkOptions struct {
	Pre           bool
	Namespaces    []string
	AllNamespaces bool
	IntendedModes capabilities.Modes // --pre: the modes the user intends to install

	// clientFn overrides o.Factory.Client() when set; used in tests to inject a
	// fake client without a live apiserver.
	clientFn func() (*k8s.Client, error)
}

// NewCheckCommand builds the `check` command (alias `doctor`).
func NewCheckCommand(o *cli.Options) *cobra.Command {
	return newCheckCommandFromOpts(o, &checkOptions{})
}

// newCheckCommandFromOpts builds the check cobra.Command from caller-supplied opts.
// The opts pointer is captured by the RunE closure so tests can pre-populate
// opts.clientFn before executing RunE.
func newCheckCommandFromOpts(o *cli.Options, opts *checkOptions) *cobra.Command {
	var dbMode, msgMode, kcMode, edge, tls string
	cmd := &cobra.Command{
		Use:     checkCommand,
		Aliases: []string{"doctor"},
		Short:   "Diagnose prerequisites (--pre) or running health via the analyzer engine",
		GroupID: string(cli.GroupInfrastructure),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o.Printer.ResolveColor(cmd.Flags())
			cf := opts.clientFn
			if cf == nil {
				cf = o.Factory.Client
			}
			c, err := cf()
			if err != nil {
				return err
			}
			rep := newReporter(c)
			opts.IntendedModes = capabilities.Modes{
				DBManaged:        dbMode == modeManaged,
				MessagingManaged: msgMode == modeManaged,
				KeycloakManaged:  kcMode == modeManaged,
				Edge:             edge,
				TLSSource:        tls,
			}
			nss, err := shared.ResolveNamespace(o.Factory, opts.AllNamespaces, nil)
			if err != nil {
				return err
			}
			opts.Namespaces = nss
			worst, err := runCheck(cmd.Context(), c, rep, o.Printer, *opts)
			if err != nil {
				return err
			}
			if worst == analyze.SeverityFail {
				return cli.ErrFailure
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.Pre, "pre", false, "check install prerequisites for the intended modes")
	cmd.Flags().BoolVarP(&opts.AllNamespaces, "all-namespaces", "A", false, "check across all namespaces")
	cmd.Flags().StringVar(&dbMode, "db-mode", "", "intended database mode for --pre (external|managed)")
	cmd.Flags().StringVar(&msgMode, "messaging-mode", "", "intended messaging mode for --pre (external|managed)")
	cmd.Flags().StringVar(&kcMode, "keycloak-mode", "", "intended keycloak mode for --pre (none|external|managed)")
	cmd.Flags().StringVar(&edge, "edge", "", "intended edge for --pre (ingress|gatewayAPI)")
	cmd.Flags().StringVar(&tls, "tls-source", "", "intended TLS source for --pre (internal|letsEncrypt|issuerRef|secret)")
	o.Printer.AddFlags(cmd.Flags())
	return cmd
}

// runCheck produces findings (pre or live), prints them, and returns the worst severity.
func runCheck(ctx context.Context, c *k8s.Client, rep *capabilities.Reporter, p *render.Printer, opts checkOptions) (analyze.Severity, error) {
	var findings []analyze.Finding
	if opts.Pre {
		findings = preFindings(rep, opts.IntendedModes)
	} else {
		snap, err := analyze.BuildLive(ctx, c, rep, analyze.BuildOptions{
			Namespaces: opts.Namespaces, AllNamespaces: opts.AllNamespaces,
		})
		if err != nil {
			return analyze.SeverityOK, err
		}
		findings = analyze.DefaultRegistry().Run(snap)
	}
	if len(findings) == 0 {
		findings = []analyze.Finding{{Severity: analyze.SeverityOK, Title: "no issues found", Rule: checkCommand}}
	}
	if err := p.PrintFindings(findings); err != nil {
		return analyze.SeverityOK, err
	}
	return analyze.Worst(findings), nil
}

// preFindings checks install prerequisites: each upstream operator the intended modes
// need must be present.
func preFindings(rep *capabilities.Reporter, modes capabilities.Modes) []analyze.Finding {
	required := capabilities.RequiredFor(modes)
	if len(required) == 0 {
		return []analyze.Finding{{
			Severity: analyze.SeverityOK, Rule: checkCommand,
			Title: "no upstream operators required for the intended modes",
		}}
	}
	out := make([]analyze.Finding, 0, len(required))
	for _, dep := range required {
		present, err := rep.Present(dep)
		switch {
		case err != nil:
			out = append(out, analyze.Finding{
				Severity: analyze.SeverityWarn, Rule: checkCommand,
				Title:    "could not detect " + string(dep),
				Evidence: err.Error(),
			})
		case !present:
			out = append(out, analyze.Finding{
				Severity:    analyze.SeverityFail,
				Rule:        checkCommand,
				Title:       "required upstream operator missing: " + string(dep),
				Remediation: "ilmctl deps install --only " + string(dep),
			})
		default:
			out = append(out, analyze.Finding{
				Severity: analyze.SeverityOK, Rule: checkCommand,
				Title: string(dep) + " present",
			})
		}
	}
	return out
}

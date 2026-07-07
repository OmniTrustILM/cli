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

// Package infra holds the cluster-scope infrastructure commands (status, check).
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/health"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

type statusOptions struct {
	Namespaces    []string
	AllNamespaces bool
	Verbose       int
	Watch         bool

	// clientFn overrides o.Factory.Client() when set; used in tests to inject a
	// fake client without a live apiserver.
	clientFn func() (*k8s.Client, error)
}

// NewStatusCommand builds the cluster/namespace-wide status command.
func NewStatusCommand(o *cli.Options) *cobra.Command {
	return newStatusCommandFromOpts(o, &statusOptions{})
}

// newStatusCommandFromOpts builds the status cobra.Command from caller-supplied opts.
// The opts pointer is captured by the RunE closure so tests can pre-populate
// opts.clientFn before executing RunE.
func newStatusCommandFromOpts(o *cli.Options, opts *statusOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Cluster/namespace-wide health: operator, platforms, managed infra, connectors, proxies",
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
			nss, err := shared.ResolveNamespace(o.Factory, opts.AllNamespaces, nil)
			if err != nil {
				return err
			}
			opts.Namespaces = nss
			rep := newReporter(c)
			if opts.Watch {
				return watchStatus(cmd.Context(), c, rep, o.Printer, *opts)
			}
			return runStatus(cmd.Context(), c, rep, o.Printer, *opts)
		},
	}
	cmd.Flags().CountVarP(&opts.Verbose, "verbose", "v", "increase detail (connectors/proxies, then managed-infra)")
	cmd.Flags().BoolVarP(&opts.AllNamespaces, "all-namespaces", "A", false, "report across all namespaces")
	cmd.Flags().BoolVar(&opts.Watch, "watch", false, "refresh the status table on an interval")
	o.Printer.AddFlags(cmd.Flags())
	return cmd
}

// runStatus builds the live Snapshot, enriches managed-infra phases, then prints.
func runStatus(ctx context.Context, c *k8s.Client, rep *capabilities.Reporter, p *render.Printer, opts statusOptions) error {
	snap, err := analyze.BuildLive(ctx, c, rep, analyze.BuildOptions{
		Namespaces: opts.Namespaces, AllNamespaces: opts.AllNamespaces,
	})
	if err != nil {
		return err
	}
	if shared.OResolved(p) {
		return printSnapshot(p, snap)
	}
	if opts.Verbose >= 2 {
		enrichManagedInfra(ctx, c, snap)
	}
	return renderStatusTables(p, snap, opts.Verbose)
}

// printSnapshot serialises the Snapshot for -o json/yaml.  Snapshot is not a
// Kubernetes runtime.Object so it cannot go through PrintFlags; we marshal it
// directly, matching the same approach used by PrintFindings.
func printSnapshot(p *render.Printer, snap *analyze.Snapshot) error {
	switch p.Format() {
	case "yaml":
		b, err := yaml.Marshal(snap)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(p.Out, string(b))
		return err
	default: // json and everything else
		b, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.Out, string(b))
		return err
	}
}

// enrichManagedInfra augments managed-mode platforms with foreign managed-infra
// phases (status-only, via the dynamic client). Best-effort: read errors
// (RBAC/absent CRD) are silently skipped — they surface in the capability
// analyzer instead.
func enrichManagedInfra(ctx context.Context, c *k8s.Client, snap *analyze.Snapshot) {
	for i := range snap.Platforms {
		enrichPlatformInfra(ctx, c, &snap.Platforms[i])
	}
}

// enrichPlatformInfra augments a single ResourceSnapshot with managed-infra phases.
func enrichPlatformInfra(ctx context.Context, c *k8s.Client, ps *analyze.ResourceSnapshot) {
	tryForeignPhase := func(gvr schema.GroupVersionResource, key string) {
		if st, err := c.ForeignStatus(ctx, gvr, ps.Namespace, ps.Name); err == nil {
			ps.Logs = appendInfra(ps.Logs, key, phaseString(st))
		}
	}
	if ps.SpecModes.DBManaged {
		tryForeignPhase(k8s.GVRCNPGCluster, "cnpg")
	}
	if ps.SpecModes.MessagingManaged {
		tryForeignPhase(k8s.GVRRabbitmqCluster, "rabbitmq")
	}
	if ps.SpecModes.KeycloakManaged {
		tryForeignPhase(k8s.GVRKeycloak, "keycloak")
	}
}

func appendInfra(m map[string]string, k, v string) map[string]string {
	if m == nil {
		m = map[string]string{}
	}
	m["infra:"+k] = v
	return m
}

func phaseString(status map[string]any) string {
	if p, ok := status["phase"].(string); ok && p != "" {
		return p
	}
	return "(status read)"
}

// renderStatusTables prints the operator line, platforms, and (verbose) connectors/proxies.
func renderStatusTables(p *render.Printer, snap *analyze.Snapshot, verbose int) error {
	opReady := "NotReady"
	if snap.OperatorReady {
		opReady = "Ready"
	}
	if _, err := fmt.Fprintf(p.Out, "Operator: %s (version %s)\n\n", opReady, orNone(snap.OperatorVersion)); err != nil {
		return err
	}
	if len(snap.Platforms) > 0 {
		tbl := render.Table{Columns: []string{"NAMESPACE", "PLATFORM", "PHASE", "VERSION", "READY"}}
		for _, ps := range snap.Platforms {
			ready, total := health.Summarize(ps.Conditions)
			tbl.Rows = append(tbl.Rows, []string{
				ps.Namespace, ps.Name, orNone(ps.Phase), orNone(ps.ObservedVersion),
				fmt.Sprintf("%d/%d", ready, total),
			})
		}
		if err := p.PrintTable(tbl); err != nil {
			return err
		}
	}
	if verbose >= 1 {
		if err := renderResourceTable(p, "CONNECTORS", snap.Connectors); err != nil {
			return err
		}
		if err := renderResourceTable(p, "PROXIES", snap.Proxies); err != nil {
			return err
		}
	}
	return nil
}

func renderResourceTable(p *render.Printer, header string, rs []analyze.ResourceSnapshot) error {
	if len(rs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(p.Out, "\n%s\n", header); err != nil {
		return err
	}
	tbl := render.Table{Columns: []string{"NAMESPACE", "NAME", "PHASE"}}
	for _, r := range rs {
		tbl.Rows = append(tbl.Rows, []string{r.Namespace, r.Name, orNone(r.Phase)})
	}
	return p.PrintTable(tbl)
}

func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// watchStatus refreshes the status table on an interval until the context is
// cancelled.
func watchStatus(ctx context.Context, c *k8s.Client, rep *capabilities.Reporter, p *render.Printer, opts statusOptions) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if _, err := fmt.Fprint(p.Out, "\033[H\033[2J"); err != nil {
			return err
		}
		if err := runStatus(ctx, c, rep, p, opts); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

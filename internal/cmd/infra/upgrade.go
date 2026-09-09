/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package infra

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

// NewUpgradeCommand builds the `upgrade` command that re-applies a newer operator manifest.
func NewUpgradeCommand(o *cli.Options) *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrade the ILM operator to a newer manifest",
		GroupID: string(cli.GroupInfrastructure),
		Long: "Re-apply a newer operator manifest. This upgrades the OPERATOR, not a Platform " +
			"instance (use `platform upgrade NAME --to vX` for that). Reports CRD/RBAC deltas before applying. " +
			"With no source flag the latest published operator release is used; release manifests are " +
			"verified against the release checksums before anything is applied.",
		RunE: func(cmd *cobra.Command, _ []string) error { return runUpgrade(cmd, o, f) },
	}
	fs := cmd.Flags()
	fs.StringVar(&f.version, "version", "", "operator release tag to upgrade to (default: the latest published release)")
	fs.StringVar(&f.ref, "ref", "", "git commit, tag, or branch")
	fs.StringVar(&f.manifestPath, "manifest", "", "explicit manifest file or URL")
	fs.StringVar(&f.fromSource, "from-source", "", "local operator checkout path (development only)")
	fs.StringVarP(&f.namespace, "namespace", "n", "ilm-operator-system", "operator namespace")
	fs.StringVar(&f.dryRun, "dry-run", "", "dry-run mode: client|server")
	fs.BoolVar(&f.forceConflict, "force-conflicts", false, "force server-side apply on field-ownership conflict")
	fs.BoolVar(&f.wait, "wait", false, "wait for the applied Deployments to become Available (ignored with --dry-run)")
	fs.DurationVar(&f.timeout, "timeout", 5*time.Minute, "wait timeout (used with --wait)")
	return cmd
}

func runUpgrade(cmd *cobra.Command, o *cli.Options, f *initFlags) error {
	dryRun, err := parseDryRun(f.dryRun)
	if err != nil {
		return err
	}
	c, err := clientFor(o)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	src := manifest.Source{
		Manifest: f.manifestPath, FromSource: f.fromSource, Ref: f.ref, Version: f.version,
	}
	crds, controller, err := resolveManifestObjects(ctx, o, src)
	if err != nil {
		return err
	}
	printDeltas(o, crds, controller)

	a := &manifest.Applier{Client: c, FieldManager: "ilmctl", ForceConflicts: f.forceConflict, DryRun: dryRun}
	res, err := a.ApplyOrdered(ctx, crds, controller)
	printApplyResult(o, res)
	if err != nil {
		return err
	}

	return waitForDeployments(ctx, o, a, controller, f, dryRun)
}

// printDeltas summarises the kinds of objects about to be applied.
func printDeltas(o *cli.Options, crds, controller []*unstructured.Unstructured) {
	counts := map[string]int{}
	for _, set := range [][]*unstructured.Unstructured{crds, controller} {
		for _, obj := range set {
			counts[obj.GetKind()]++
		}
	}
	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	t := render.Table{Columns: []string{"KIND", "COUNT"}}
	for _, k := range kinds {
		t.Rows = append(t.Rows, []string{k, fmt.Sprint(counts[k])})
	}
	_, _ = fmt.Fprintln(o.Out, "Objects to apply:")
	if err := o.Printer.PrintTable(t); err != nil {
		_, _ = fmt.Fprintln(o.ErrOut, "render:", err)
	}
}

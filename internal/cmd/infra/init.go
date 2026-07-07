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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/manifest"
)

// clientFor is overridable in tests; it returns the k8s.Client the command
// uses, so unit tests can inject a fake without a live apiserver.
var clientFor = func(o *cli.Options) (*k8s.Client, error) { return o.Factory.Client() }

type initFlags struct {
	version       string
	ref           string
	manifestPath  string
	fromSource    string
	method        string
	namespace     string
	createNS      bool
	withDeps      bool
	wait          bool
	timeout       time.Duration
	dryRun        string
	forceConflict bool
}

// NewInitCommand builds the `init` subcommand that installs the ILM operator.
func NewInitCommand(o *cli.Options) *cobra.Command {
	f := &initFlags{}
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Install the ILM operator",
		GroupID: string(cli.GroupInfrastructure),
		Long: "Install the ILM operator using the manifest method (CRDs-first ordered server-side apply) " +
			"or the OLM method (CatalogSource, OperatorGroup, Subscription). " +
			"Use --dry-run=client|server to render the resolved objects without applying (GitOps).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd, o, f)
		},
	}

	fs := cmd.Flags()
	fs.StringVar(&f.version, "version", "", "operator release tag (default install source)")
	fs.StringVar(&f.ref, "ref", "", "git commit, tag, or branch to read committed manifests from")
	fs.StringVar(&f.manifestPath, "manifest", "", "explicit manifest file or URL")
	fs.StringVar(&f.fromSource, "from-source", "", "local operator checkout path (development only)")
	fs.StringVar(&f.method, "method", "manifest", "install method: manifest|olm")
	fs.StringVarP(&f.namespace, "namespace", "n", "ilm-operator-system", "operator namespace")
	fs.BoolVar(&f.createNS, "create-namespace", false, "create the operator namespace if absent")
	fs.BoolVar(&f.withDeps, "with-deps", false, "also install pinned upstream dependencies")
	fs.BoolVar(&f.wait, "wait", false, "wait for the applied Deployments to become Available (ignored with --dry-run)")
	fs.DurationVar(&f.timeout, "timeout", 5*time.Minute, "wait timeout (used with --wait)")
	fs.StringVar(&f.dryRun, "dry-run", "", "dry-run mode: client|server (omit to apply)")
	fs.BoolVar(&f.forceConflict, "force-conflicts", false, "force server-side apply on field-ownership conflict")
	// OLM-specific flags.
	fs.String("channel", "stable", "OLM subscription channel (method=olm)")
	fs.String("catalog-image", "", "OLM catalog image override (method=olm)")

	return cmd
}

// parseDryRun converts the string value of --dry-run to a DryRunMode.
func parseDryRun(s string) (manifest.DryRunMode, error) {
	switch strings.ToLower(s) {
	case "":
		return manifest.DryRunNone, nil
	case "client":
		return manifest.DryRunClient, nil
	case "server":
		return manifest.DryRunServer, nil
	default:
		return manifest.DryRunNone, fmt.Errorf("invalid --dry-run %q (want client|server)", s)
	}
}

// runInit is the top-level RunE handler; it dispatches to the manifest or OLM path.
func runInit(cmd *cobra.Command, o *cli.Options, f *initFlags) error {
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

	// Explicit dispatch so unrecognised --method values fail fast rather than
	// silently falling through to the manifest path.
	switch f.method {
	case "manifest":
		return runInitManifest(ctx, o, c, f, dryRun)
	case "olm":
		return runInitOLM(ctx, cmd, o, c, f, dryRun)
	default:
		return fmt.Errorf("invalid --method %q: must be \"manifest\" or \"olm\"", f.method)
	}
}

// runInitManifest implements the manifest install path:
// Resolve → Fetch → Split → (optional InstallDeps) → ApplyOrdered.
func runInitManifest(ctx context.Context, o *cli.Options, c *k8s.Client, f *initFlags, dryRun manifest.DryRunMode) error {
	src := manifest.Source{
		Manifest:   f.manifestPath,
		FromSource: f.fromSource,
		Ref:        f.ref,
		Version:    f.version,
	}
	crdObjs, ctrlObjs, err := resolveManifestObjects(ctx, o, src, "Install")
	if err != nil {
		return err
	}

	if f.createNS {
		ctrlObjs = append([]*unstructured.Unstructured{namespaceObj(f.namespace)}, ctrlObjs...)
	}

	a := &manifest.Applier{
		Client:         c,
		FieldManager:   "ilmctl",
		ForceConflicts: f.forceConflict,
		DryRun:         dryRun,
	}

	var res manifest.ApplyResult
	if f.withDeps {
		if derr := applyDeps(ctx, o, a, &res); derr != nil {
			return derr
		}
	}

	ordered, err := a.ApplyOrdered(ctx, crdObjs, ctrlObjs)
	res.Applied = append(res.Applied, ordered.Applied...)
	res.Unchanged = append(res.Unchanged, ordered.Unchanged...)
	res.Conflicts = append(res.Conflicts, ordered.Conflicts...)
	printApplyResult(o, res)
	if err != nil {
		return err
	}

	return waitForDeployments(ctx, o, a, ctrlObjs, f, dryRun)
}

// resolveManifestObjects resolves a manifest source and returns the CRD and
// controller object sets. verb ("Install"/"Upgrade") tailors the guidance
// printed when the operator has no published release yet.
func resolveManifestObjects(ctx context.Context, o *cli.Options, src manifest.Source, verb string) (crdObjs, ctrlObjs []*unstructured.Unstructured, err error) {
	resolved, err := manifest.Resolve(src)
	if err != nil {
		if errors.Is(err, manifest.ErrUnreleased) {
			_, _ = fmt.Fprintln(o.ErrOut, "The operator has no published release yet.")
			_, _ = fmt.Fprintf(o.ErrOut, "%s from a specific commit with --ref <commit-sha>, or from a local checkout with --from-source <path>.\n", verb)
		}
		return nil, nil, err
	}

	crdObjs, err = fetchSplit(ctx, resolved.CRDsRef)
	if err != nil {
		return nil, nil, err
	}
	ctrlObjs, err = fetchSplit(ctx, resolved.ControllerRef)
	if err != nil {
		return nil, nil, err
	}

	// When a single --manifest file bundles both CRDs and controller objects,
	// the CRDs ref and controller ref resolve to the same path. Split the flat
	// object list by kind so the ordered apply can apply CRDs first.
	if resolved.Kind == manifest.SourceManifest {
		crdObjs, ctrlObjs = splitByCRD(ctrlObjs)
	}
	return crdObjs, ctrlObjs, nil
}

// fetchSplit fetches a manifest ref and splits it into individual objects.
func fetchSplit(ctx context.Context, ref string) ([]*unstructured.Unstructured, error) {
	raw, err := manifest.Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}
	return manifest.Split(raw)
}

// applyDeps installs pinned upstream dependencies, folding their apply results
// into res. On error it prints the accumulated result before returning.
func applyDeps(ctx context.Context, o *cli.Options, a *manifest.Applier, res *manifest.ApplyResult) error {
	depRes, derr := manifest.InstallDeps(ctx, a, nil)
	res.Applied = append(res.Applied, depRes.Applied...)
	res.Unchanged = append(res.Unchanged, depRes.Unchanged...)
	res.Conflicts = append(res.Conflicts, depRes.Conflicts...)
	if derr != nil {
		printApplyResult(o, *res)
		return derr
	}
	return nil
}

// waitForDeployments blocks until the controller Deployments become Available
// when --wait is set and the run is not a dry-run; otherwise it is a no-op.
func waitForDeployments(ctx context.Context, o *cli.Options, a *manifest.Applier, ctrlObjs []*unstructured.Unstructured, f *initFlags, dryRun manifest.DryRunMode) error {
	if !f.wait || dryRun != manifest.DryRunNone {
		return nil
	}
	deps := deploymentsOf(ctrlObjs)
	if len(deps) > 0 {
		_, _ = fmt.Fprintf(o.ErrOut, "Waiting for %d Deployment(s) to become Available (timeout %s)...\n", len(deps), f.timeout)
	}
	return a.WaitDeploymentsAvailable(ctx, deps, f.timeout)
}

// runInitOLM implements the OLM install path.
// It detects OLM first and fails fast with an actionable message if OLM is absent.
func runInitOLM(ctx context.Context, cmd *cobra.Command, o *cli.Options, c *k8s.Client, f *initFlags, dryRun manifest.DryRunMode) error {
	catalogImg, _ := cmd.Flags().GetString("catalog-image")
	if catalogImg == "" {
		return cli.NewUsageError(fmt.Errorf(
			"--method=olm requires --catalog-image: no default OLM catalog is published yet; " +
				"pass the operator's catalog image, or use --method=manifest"))
	}

	present, err := manifest.DetectOLM(ctx, c)
	if err != nil {
		return fmt.Errorf("detect OLM: %w", err)
	}
	if !present {
		return fmt.Errorf(
			"OLM is not installed in this cluster; " +
				"install OLM first (ilmctl never installs OLM itself) or switch to --method=manifest",
		)
	}

	channel, _ := cmd.Flags().GetString("channel")

	res, err := manifest.ApplyOLM(ctx, c, manifest.OLMOptions{
		Namespace:    f.namespace,
		Channel:      channel,
		CatalogImage: catalogImg,
		DryRun:       dryRun,
	})
	printApplyResult(o, res)
	if err == nil && f.wait && dryRun == manifest.DryRunNone {
		_, _ = fmt.Fprintln(o.ErrOut, "note: --wait does not await OLM CSV/Subscription readiness; check the operator's install status with `ilmctl infra status`")
	}
	return err
}

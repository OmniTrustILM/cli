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

package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/manifest"
)

// ChangedFlags returns the set of flag names the user explicitly provided on
// the command line. Generate commands use this map so explicitly-set flags
// always override profile seeds and the effective-value source is accurate.
func ChangedFlags(cmd *cobra.Command) map[string]bool {
	set := map[string]bool{}
	cmd.Flags().Visit(func(fl *pflag.Flag) { set[fl.Name] = true })
	return set
}

// ParseDryRun converts the string --dry-run value to a manifest.DryRunMode.
func ParseDryRun(s string) (manifest.DryRunMode, error) {
	switch strings.ToLower(s) {
	case "", "none":
		return manifest.DryRunNone, nil
	case "client":
		return manifest.DryRunClient, nil
	case "server":
		return manifest.DryRunServer, nil
	default:
		return manifest.DryRunNone, fmt.Errorf("invalid --dry-run %q (want none|client|server)", s)
	}
}

// ToUnstructured converts a typed object to unstructured for server-side apply.
func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("convert to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: m}, nil
}

// ReportApply emits a summary of the apply result to o.Out.
func ReportApply(o *cli.Options, res manifest.ApplyResult, mode manifest.DryRunMode) {
	prefix := ""
	if mode != manifest.DryRunNone {
		prefix = "(dry-run) "
	}
	for _, name := range res.Applied {
		_, _ = fmt.Fprintf(o.Out, "%s%s applied\n", prefix, name)
	}
	for _, name := range res.Unchanged {
		_, _ = fmt.Fprintf(o.Out, "%s%s unchanged\n", prefix, name)
	}
}

// ApplyObject server-side applies a typed CR via the manifest.Applier,
// honouring the --dry-run mode and --force-conflicts.
// The caller resolves *k8s.Client via its own per-package seam and passes it here;
// this keeps the injection seam local to each command package while keeping the
// apply logic in one place.
func ApplyObject(ctx context.Context, o *cli.Options, client *k8s.Client, obj runtime.Object, dryRun string, force bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	mode, err := ParseDryRun(dryRun)
	if err != nil {
		return err
	}
	u, err := ToUnstructured(obj)
	if err != nil {
		return err
	}
	applier := &manifest.Applier{
		Client:         client,
		FieldManager:   "ilmctl",
		ForceConflicts: force,
		DryRun:         mode,
	}
	res, err := applier.Apply(ctx, []*unstructured.Unstructured{u})
	if err != nil {
		return err
	}
	ReportApply(o, res, mode)
	return nil
}

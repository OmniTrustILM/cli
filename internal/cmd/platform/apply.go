/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/manifest"
)

// NewApplyCommand builds `platform apply`, server-side applying CR documents
// from a file (or stdin) with field manager "ilmctl". Any otilm.com/v1alpha1
// resource kind found in the file is applied; this is a generic SSA path.
func NewApplyCommand(o *cli.Options) *cobra.Command {
	var (
		filename string
		dryRun   string
		force    bool
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a Platform CR from a file (server-side apply)",
		Long: "Server-side apply Platform/Connector/Proxy documents from a file or stdin " +
			"with field manager ilmctl. Use --dry-run=server for CEL validation.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filename == "" {
				return fmt.Errorf("-f/--filename is required")
			}
			raw, err := readManifest(filename, o.In)
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			return applyRaw(cmd.Context(), o, raw, dryRun, force)
		},
	}
	cmd.Flags().StringVarP(&filename, "filename", "f", "", "Path to the CR manifest or - for stdin (required)")
	cmd.Flags().StringVar(&dryRun, "dry-run", "none", "Dry-run mode: none|client|server")
	cmd.Flags().BoolVar(&force, "force-conflicts", false, "Force-apply on field-manager conflicts")
	return cmd
}

// readManifest reads the raw YAML bytes from a file path or from stdin when
// path is "-".
func readManifest(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path) //nolint:gosec // user-supplied manifest path is intentional
}

// applyRaw splits a multi-doc YAML buffer and server-side applies every object.
func applyRaw(ctx context.Context, o *cli.Options, raw []byte, dryRun string, force bool) error {
	objs, err := manifest.Split(raw)
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return fmt.Errorf("no documents found in manifest")
	}
	mode, err := shared.ParseDryRun(dryRun)
	if err != nil {
		return err
	}
	c, err := o.Factory.Client()
	if err != nil {
		return err
	}
	applier := &manifest.Applier{
		Client:         c,
		FieldManager:   "ilmctl",
		ForceConflicts: force,
		DryRun:         mode,
	}
	res, err := applier.Apply(ctx, objs)
	if err != nil {
		return err
	}
	shared.ReportApply(o, res, mode)
	return nil
}

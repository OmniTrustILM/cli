/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/generate"
)

// NewMigrateCommand builds the `platform migrate` subcommand, which converts a
// Helm values.yaml into a Platform CR scaffold via the operator's pkg/convert.
func NewMigrateCommand(o *cli.Options) *cobra.Command {
	var values, name, namespace string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert a Helm values.yaml into a Platform CR scaffold",
		Long: "Convert a legacy Helm values.yaml into a Platform custom-resource scaffold. " +
			"Secret-bearing fields are surfaced as TODO comments; review the output before applying.",
		RunE: func(_ *cobra.Command, _ []string) error {
			if values == "" {
				return fmt.Errorf("--values is required")
			}
			if name == "" {
				return fmt.Errorf("--name is required")
			}
			raw, err := os.ReadFile(values) //nolint:gosec // user-supplied values file path is intentional
			if err != nil {
				return fmt.Errorf("read values file: %w", err)
			}
			out, err := generate.Migrate(raw, name, namespace)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprint(o.Out, out)
			return nil
		},
	}
	cmd.Flags().StringVar(&values, "values", "", "Path to the Helm values.yaml (required)")
	cmd.Flags().StringVar(&name, "name", "", "Platform resource name (required)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Target namespace")
	return cmd
}

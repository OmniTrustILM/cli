/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package deps

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/manifest"
)

type installFlags struct {
	only   []string
	dryRun bool
}

// installableDepNames maps --only tokens to capabilities.Dep values.
var installableDepNames = map[string]capabilities.Dep{
	"cert-manager": capabilities.DepCertManager,
	"cnpg":         capabilities.DepCNPG,
	"rabbitmq":     capabilities.DepRabbitMQ,
	"keycloak":     capabilities.DepKeycloak,
}

// NewInstallCommand builds `deps install`.
func NewInstallCommand(o *cli.Options) *cobra.Command {
	f := &installFlags{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install pinned upstream operators",
		Long:  "Install selected upstream operators at pinned versions. The CLI never installs OLM.",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runDepsInstall(cmd, o, f) },
	}
	fs := cmd.Flags()
	fs.StringSliceVar(&f.only, "only", nil, "subset to install: cert-manager,cnpg,rabbitmq,keycloak (default: all)")
	fs.BoolVar(&f.dryRun, "dry-run", false, "do not contact the server")
	return cmd
}

func runDepsInstall(cmd *cobra.Command, o *cli.Options, f *installFlags) error {
	only := make([]capabilities.Dep, 0, len(f.only))
	for _, name := range f.only {
		d, ok := installableDepNames[name]
		if !ok {
			return fmt.Errorf("unknown dependency %q (want cert-manager|cnpg|rabbitmq|keycloak)", name)
		}
		only = append(only, d)
	}
	c, err := depsClientFor(o)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	dryRun := manifest.DryRunNone
	if f.dryRun {
		dryRun = manifest.DryRunClient
	}
	a := &manifest.Applier{Client: c, FieldManager: "ilmctl", DryRun: dryRun}
	res, err := manifest.InstallDeps(ctx, a, only)
	printApplyResult(o, res)
	return err
}

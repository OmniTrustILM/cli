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

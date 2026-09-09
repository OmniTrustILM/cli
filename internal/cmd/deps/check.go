/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package deps holds the upstream-dependency check and install commands.
package deps

import (
	"fmt"
	"sort"

	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// depsClientFor is overridable in tests.
var depsClientFor = func(o *cli.Options) (*k8s.Client, error) { return o.Factory.Client() }

// NewCheckCommand builds `deps check`.
func NewCheckCommand(o *cli.Options) *cobra.Command {
	var modes capabilities.Modes
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Report which upstream operators are present and required",
		RunE:  func(_ *cobra.Command, _ []string) error { return runDepsCheck(o, modes) },
	}
	fs := cmd.Flags()
	fs.BoolVar(&modes.DBManaged, "db-managed", false, "intended Platform uses managed database")
	fs.BoolVar(&modes.MessagingManaged, "messaging-managed", false, "intended Platform uses managed messaging")
	fs.BoolVar(&modes.KeycloakManaged, "keycloak-managed", false, "intended Platform uses managed Keycloak")
	fs.StringVar(&modes.Edge, "edge", "", "intended Platform edge: ingress|gatewayAPI")
	fs.StringVar(&modes.TLSSource, "tls-source", "", "intended Platform TLS source")
	return cmd
}

func runDepsCheck(o *cli.Options, modes capabilities.Modes) error {
	c, err := depsClientFor(o)
	if err != nil {
		return err
	}
	reporter := capabilities.NewReporter(opcap.New(c.Mapper))
	results := reporter.Detect()

	required := map[capabilities.Dep]bool{}
	for _, d := range capabilities.RequiredFor(modes) {
		required[d] = true
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Dep < results[j].Dep })
	t := render.Table{Columns: []string{"DEPENDENCY", "PRESENT", "REQUIRED"}}
	for _, r := range results {
		present := "no"
		if r.Present {
			present = "yes"
		}
		req := "no"
		if required[r.Dep] {
			req = "yes"
		}
		t.Rows = append(t.Rows, []string{string(r.Dep), present, req})
	}
	if err := o.Printer.PrintTable(t); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	return nil
}

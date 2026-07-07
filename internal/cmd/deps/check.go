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

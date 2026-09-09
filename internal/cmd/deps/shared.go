/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package deps

import (
	"fmt"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

func printApplyResult(o *cli.Options, res manifest.ApplyResult) {
	t := render.Table{Columns: []string{"ACTION", "OBJECT"}}
	for _, id := range res.Applied {
		t.Rows = append(t.Rows, []string{"applied", id})
	}
	for _, id := range res.Unchanged {
		t.Rows = append(t.Rows, []string{"unchanged", id})
	}
	for _, id := range res.Conflicts {
		t.Rows = append(t.Rows, []string{"conflict", id})
	}
	if err := o.Printer.PrintTable(t); err != nil {
		_, _ = fmt.Fprintln(o.ErrOut, "render:", err)
	}
}

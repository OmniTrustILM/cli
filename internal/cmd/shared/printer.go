/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package shared

import "github.com/OmniTrustILM/cli/internal/render"

// OResolved reports whether a structured -o format (json/yaml/name/...) is
// requested. Commands branch to render.Printer.PrintObject when true,
// otherwise to a human table.
func OResolved(p *render.Printer) bool {
	return p.Format() != ""
}

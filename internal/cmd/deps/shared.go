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

//go:build tools

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

// Command gen-docs renders the cobra command tree to docs/commands/*.md and the
// combined single-page reference to docs/site/commands.md. Run via `make docs`; the
// build tag keeps this entry point out of ordinary builds. The rendering itself
// lives in internal/gendocs, where its drift test runs under `go test ./...`.
package main

import (
	"fmt"
	"os"

	"github.com/OmniTrustILM/cli/internal/gendocs"
)

func main() {
	treeDir := "docs/commands"
	combined := "docs/site/commands.md"
	if len(os.Args) > 1 {
		treeDir = os.Args[1]
	}
	if len(os.Args) > 2 {
		combined = os.Args[2]
	}

	if err := gendocs.Tree(treeDir); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}

	page, err := gendocs.Combined()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(combined, page, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-docs:", err)
		os.Exit(1)
	}
}

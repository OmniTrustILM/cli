//go:build tools

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

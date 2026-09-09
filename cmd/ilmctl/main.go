/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Command ilmctl is the ILM CLI. The same binary is published as kubectl-ilm;
// cli.Run dispatches on os.Args[0].
package main

import (
	"os"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/rootcmd"
)

func main() {
	os.Exit(cli.RunWithCommands(os.Args, rootcmd.Register))
}

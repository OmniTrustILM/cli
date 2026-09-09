/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/buildinfo"
)

// Shell names accepted by the completion command.
const (
	shellBash       = "bash"
	shellZsh        = "zsh"
	shellFish       = "fish"
	shellPowershell = "powershell"
)

// newCompletionCommand generates shell completion scripts. The script's binary
// name adapts to how the CLI was invoked (ilmctl vs kubectl-ilm).
func newCompletionCommand(o *Options) *cobra.Command {
	binName := buildinfo.BinaryName
	if o.InvokedAs == buildinfo.PluginBinaryName {
		binName = buildinfo.PluginBinaryName
	}

	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: fmt.Sprintf(`Generate the autocompletion script for %s for the specified shell.

See each sub-command's help for details on how to use the generated script.

When installed as a kubectl plugin, source the script for %s.`, binName, buildinfo.PluginBinaryName),
		GroupID:               string(GroupOther),
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{shellBash, shellZsh, shellFish, shellPowershell},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build a throwaway root with the correct binary name so the generated
			// script references the right program name (cobra derives it from root.Name()).
			root := cmd.Root()
			origUse := root.Use
			root.Use = binName
			defer func() { root.Use = origUse }()

			w := cmd.OutOrStdout()
			switch args[0] {
			case shellBash:
				return root.GenBashCompletionV2(w, true)
			case shellZsh:
				return root.GenZshCompletion(w)
			case shellFish:
				return root.GenFishCompletion(w, true)
			case shellPowershell:
				return root.GenPowerShellCompletionWithDesc(w)
			default:
				return UsageError{fmt.Errorf("unsupported shell %q", args[0])}
			}
		},
	}
	return cmd
}

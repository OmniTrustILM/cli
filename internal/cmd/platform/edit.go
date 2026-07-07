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

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/generate"
)

// runEditor is the injectable editor seam. Tests override this variable with a
// function that writes a canned file, so no real $EDITOR is spawned in hermetic
// test runs.
var runEditor = func(path string, o *cli.Options) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path) //nolint:gosec // editor from env is intentional
	cmd.Stdin = o.In
	cmd.Stdout = o.Out
	cmd.Stderr = o.ErrOut
	return cmd.Run()
}

// NewEditCommand builds `platform edit NAME`, fetching the live Platform CR,
// opening it in $EDITOR, and server-side applying the edited buffer.
func NewEditCommand(o *cli.Options) *cobra.Command {
	var (
		namespace string
		dryRun    string
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "edit NAME",
		Short: "Edit a Platform CR in $EDITOR and apply the result",
		Long: "Fetch the live Platform CR, open it in $EDITOR (fallback: vi), " +
			"and server-side apply the edited buffer with field manager ilmctl.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ns := resolveNamespace(namespace, o)
			c, err := o.Factory.Client()
			if err != nil {
				return err
			}
			p, err := c.GetPlatform(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			p.SetGroupVersionKind(cmdutil.OtilmGVK("Platform"))
			body, err := generate.Render(p, nil)
			if err != nil {
				return err
			}
			edited, changed, err := openInEditor(body, o)
			if err != nil {
				return err
			}
			if !changed {
				_, _ = fmt.Fprintln(o.Out, "Edit cancelled, no changes made.")
				return nil
			}
			return applyRaw(cmd.Context(), o, edited, dryRun, force)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the Platform (default from context)")
	cmd.Flags().StringVar(&dryRun, "dry-run", "none", "Dry-run mode: none|client|server")
	cmd.Flags().BoolVar(&force, "force-conflicts", false, "Force-apply on field-manager conflicts")
	return cmd
}

// openInEditor writes body to a temp file, launches the editor, reads the result
// back, and returns the edited bytes alongside a flag indicating whether the
// content actually changed. The temp directory is removed on return.
func openInEditor(body string, o *cli.Options) (edited []byte, changed bool, err error) {
	dir, err := os.MkdirTemp("", "ilmctl-edit")
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tmpPath := filepath.Join(dir, "platform.yaml")
	if err := os.WriteFile(tmpPath, []byte(body), 0o600); err != nil {
		return nil, false, err
	}
	if err := runEditor(tmpPath, o); err != nil {
		return nil, false, fmt.Errorf("editor exited with error: %w", err)
	}
	result, err := os.ReadFile(tmpPath) //nolint:gosec // tmpPath is created by MkdirTemp in a controlled directory
	if err != nil {
		return nil, false, err
	}
	return result, strings.TrimSpace(string(result)) != strings.TrimSpace(body), nil
}

// resolveNamespace returns the explicitly-given namespace flag value, falling
// back to Factory.Namespace() when the flag is empty.
func resolveNamespace(flag string, o *cli.Options) string {
	if flag != "" {
		return flag
	}
	ns, _, _ := o.Factory.Namespace()
	return ns
}

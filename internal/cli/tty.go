/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

// IsTerminal reports whether r is an interactive terminal. Only an *os.File on a
// TTY qualifies; pipes, buffers and redirected files return false. Gates
// interactive prompts and confirmations.
func IsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // fd is always a small non-negative value; overflow impossible
}

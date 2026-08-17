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

package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCompletion(t *testing.T, invokedAs string, args ...string) (string, error) {
	t.Helper()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := NewDefaultOptions(out, errOut)
	o.InvokedAs = invokedAs
	root := NewRootCommand(o)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(append([]string{"completion"}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestCompletion_Shells(t *testing.T) {
	for _, shell := range []string{shellBash, shellZsh, shellFish, shellPowershell} {
		t.Run(shell, func(t *testing.T) {
			out, err := runCompletion(t, cliILMCtl, shell)
			require.NoError(t, err)
			assert.NotEmpty(t, out)
		})
	}
}

func TestCompletion_UnknownShellIsError(t *testing.T) {
	_, err := runCompletion(t, cliILMCtl, "tcsh")
	require.Error(t, err)
}

func TestCompletion_RequiresOneArg(t *testing.T) {
	_, err := runCompletion(t, cliILMCtl)
	require.Error(t, err)
}

func TestCompletion_BashMentionsInvocationName(t *testing.T) {
	tests := []struct {
		invokedAs string
		wantToken string
	}{
		{cliILMCtl, cliILMCtl},
		{cliKubectlILM, cliKubectlILM},
	}
	for _, tt := range tests {
		t.Run(tt.invokedAs, func(t *testing.T) {
			out, err := runCompletion(t, tt.invokedAs, shellBash)
			require.NoError(t, err)
			assert.Contains(t, out, tt.wantToken)
		})
	}
}

func TestCompletion_InGroupOther(t *testing.T) {
	o := NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	cmd := newCompletionCommand(o)
	assert.Equal(t, string(GroupOther), cmd.GroupID)
}

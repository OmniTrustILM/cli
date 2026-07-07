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
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOptions() (*Options, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := NewDefaultOptions(out, errOut)
	o.In = &bytes.Buffer{}
	return o, out, errOut
}

func TestNewRootCommand_HasGroups(t *testing.T) {
	o, _, _ := newTestOptions()
	root := NewRootCommand(o)
	ids := map[string]bool{}
	for _, g := range root.Groups() {
		ids[g.ID] = true
	}
	assert.True(t, ids[string(GroupInfrastructure)])
	assert.True(t, ids[string(GroupResources)])
	assert.True(t, ids[string(GroupDiagnostics)])
	assert.True(t, ids[string(GroupOther)])
}

func TestNewRootCommand_GlobalFlagsRegistered(t *testing.T) {
	o, _, _ := newTestOptions()
	root := NewRootCommand(o)
	for _, name := range []string{"kubeconfig", "context", "namespace", "ilmconfig"} {
		assert.NotNil(t, root.PersistentFlags().Lookup(name), "expected persistent flag --%s", name)
	}
	assert.NotNil(t, root.PersistentFlags().ShorthandLookup("n"), "expected -n shorthand")
}

func TestNewRootCommand_UseReflectsInvokedAs(t *testing.T) {
	tests := []struct {
		invokedAs string
		wantUse   string
	}{
		{cliILMCtl, cliILMCtl},
		{cliKubectlILM, "kubectl ilm"},
	}
	for _, tt := range tests {
		t.Run(tt.invokedAs, func(t *testing.T) {
			o, _, _ := newTestOptions()
			o.InvokedAs = tt.invokedAs
			root := NewRootCommand(o)
			assert.Equal(t, tt.wantUse, root.Use)
		})
	}
}

func TestRun_UnknownCommandIsUsageExit(t *testing.T) {
	code := Run([]string{cliILMCtl, "no-such-command"})
	assert.Equal(t, ExitUsage, code)
}

func TestRun_HelpIsOK(t *testing.T) {
	assert.Equal(t, ExitOK, Run([]string{cliILMCtl, cliFlagHelp}))
}

func TestResolveInvokedAs(t *testing.T) {
	tests := []struct{ arg0, want string }{
		{"/usr/local/bin/ilmctl", cliILMCtl},
		{cliKubectlILM, cliKubectlILM},
		{"/opt/kube/kubectl-ilm", cliKubectlILM},
		{"weird", cliILMCtl},
	}
	for _, tt := range tests {
		t.Run(tt.arg0, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveInvokedAs(tt.arg0))
		})
	}
}

func TestExecute_WritesToConfiguredErrOut(t *testing.T) {
	o, _, errOut := newTestOptions()
	root := NewRootCommand(o)
	root.SetArgs([]string{"definitely-not-a-command"})
	err := root.Execute()
	require.Error(t, err)
	assert.NotEmpty(t, errOut.String())
}

func TestNewUsageError(t *testing.T) {
	err := NewUsageError(errors.New("bad flag"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad flag")
	assert.True(t, isUsageError(err))
}

func TestIsUsageError_WrappedError(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", UsageError{errors.New("inner")})
	assert.True(t, isUsageError(wrapped))
}

func TestIsUsageError_RegularError(t *testing.T) {
	assert.False(t, isUsageError(errors.New("regular")))
}

func TestRunWithCommands_RegisterCallbackInvoked(t *testing.T) {
	called := false
	code := RunWithCommands([]string{cliILMCtl, cliFlagHelp}, func(_ *cobra.Command, _ *Options) {
		called = true
	})
	assert.Equal(t, ExitOK, code)
	assert.True(t, called, "register callback was not called")
}

func TestRunWithCommands_NilRegisterOK(t *testing.T) {
	code := RunWithCommands([]string{cliILMCtl, cliFlagHelp}, nil)
	assert.Equal(t, ExitOK, code)
}

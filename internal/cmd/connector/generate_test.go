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

package connector

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newTestOptions() (*cli.Options, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.Options{In: bytes.NewReader(nil), Out: out, ErrOut: errOut, Printer: render.NewPrinter(out, errOut)}, out
}

func TestConnectorGenerate_FullRegistration(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{
		connectorFlagName, "cryptosense", "--namespace", "ilm",
		connectorFlagImage, "harbor.example.com/ilm/connector-cryptosense:1.4.0",
		"--platform-url", "https://ilm.example.com",
		"--auth-type", "apiKey", "--replicas", "2",
	})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "kind: Connector")
	assert.Contains(t, s, "repository: harbor.example.com/ilm")
	assert.Contains(t, s, "tag: 1.4.0")
	assert.Contains(t, s, "platformUrl: https://ilm.example.com")
	assert.Contains(t, s, "authType: apiKey")
}

func TestConnectorGenerate_NoRegistration(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{connectorFlagName, "demo", connectorFlagImage, "connector-demo:2.0.0"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, out.String(), "registration:")
}

func TestConnectorGenerate_RegistrationCaveatInHelp(t *testing.T) {
	o, _ := newTestOptions()
	cmd := NewGenerateCommand(o)
	assert.Contains(t, cmd.Long, "running platform")
}

func TestConnectorGenerate_Errors(t *testing.T) {
	tests := [][]string{
		{connectorFlagImage, "x:1"},                           // missing name
		{connectorFlagName, "c"},                              // missing image
		{connectorFlagName, "c", connectorFlagImage, "noTag"}, // image without tag
		{connectorFlagName, "c", connectorFlagImage, "x:1", "--auth-type", "kerberos"},
		{connectorFlagName, "c", connectorFlagImage, "x:1", "--name-reg", "r"}, // unknown flag (sanity)
	}
	for _, args := range tests {
		o, out := newTestOptions()
		cmd := NewGenerateCommand(o)
		cmd.SetArgs(args)
		cmd.SetOut(out)
		cmd.SetErr(out)
		assert.Error(t, cmd.Execute())
	}
}

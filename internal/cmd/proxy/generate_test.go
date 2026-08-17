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

package proxy

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

func TestProxyGenerate_Minimal(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{proxyFlagName, proxyEgress, proxyFlagNS, proxyNamespace, "--config-token-secret", "egress-config-token"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "kind: Proxy")
	assert.Contains(t, s, "name: egress-config-token")
}

func TestProxyGenerate_ImageAndReplicas(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{
		proxyFlagName, proxyEgress, "--config-token-secret", "egress-config-token",
		"--image", "harbor.example.com/ilm/proxy:2.18.0", "--replicas", "3",
	})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "tag: 2.18.0")
}

func TestProxyGenerate_RequiresConfigTokenSecret(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{proxyFlagName, proxyEgress})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

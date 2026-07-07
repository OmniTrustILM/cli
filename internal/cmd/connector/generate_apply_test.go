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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

func TestConnectorGenerate_Apply_DryRunClient(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	old := connectorClientFor
	connectorClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { connectorClientFor = old }()

	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{
		connectorFlagName, "demo", "--namespace", "ilm",
		connectorFlagImage, "harbor.example.com/ilm/connector-demo:1.0.0",
		"--apply", "--dry-run=client",
	})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	// Client dry-run records the apply line but never contacts the server.
	assert.Contains(t, out.String(), "applied")
}

func TestConnectorGenerate_Apply_DryRunServer(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	old := connectorClientFor
	connectorClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { connectorClientFor = old }()

	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{
		connectorFlagName, "demo", "--namespace", "ilm",
		connectorFlagImage, "harbor.example.com/ilm/connector-demo:1.0.0",
		"--apply", "--dry-run=server",
	})
	cmd.SetOut(out)
	cmd.SetErr(out)
	// The fake client does not run admission webhooks so server dry-run succeeds.
	require.NoError(t, cmd.Execute())
}

func TestConnectorGenerate_NoApply_NoClusterContact(t *testing.T) {
	called := false
	old := connectorClientFor
	connectorClientFor = func(*cli.Options) (*k8s.Client, error) {
		called = true
		return nil, nil
	}
	defer func() { connectorClientFor = old }()

	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{connectorFlagName, "demo", connectorFlagImage, "connector-demo:1.0.0"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	assert.False(t, called, "client seam must not be invoked without --apply")
	assert.Contains(t, out.String(), "kind: Connector")
}

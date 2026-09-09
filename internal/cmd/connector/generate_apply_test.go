/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
		connectorFlagName, connectorDemoName, connectorFlagNS, connectorNamespace,
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
		connectorFlagName, connectorDemoName, connectorFlagNS, connectorNamespace,
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
	cmd.SetArgs([]string{connectorFlagName, connectorDemoName, connectorFlagImage, "connector-demo:1.0.0"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	assert.False(t, called, "client seam must not be invoked without --apply")
	assert.Contains(t, out.String(), "kind: Connector")
}

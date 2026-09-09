/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

// minimalImage is the smallest well-formed image reference the flag tests accept.
const minimalImage = "x:1"

func newTestOptions() (*cli.Options, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.Options{In: bytes.NewReader(nil), Out: out, ErrOut: errOut, Printer: render.NewPrinter(out, errOut)}, out
}

func TestConnectorGenerate_FullRegistration(t *testing.T) {
	o, out := newTestOptions()
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{
		connectorFlagName, "cryptosense", connectorFlagNS, connectorNamespace,
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
	cmd.SetArgs([]string{connectorFlagName, connectorDemoName, connectorFlagImage, "connector-demo:2.0.0"})
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
		{connectorFlagImage, minimalImage},                    // missing name
		{connectorFlagName, "c"},                              // missing image
		{connectorFlagName, "c", connectorFlagImage, "noTag"}, // image without tag
		{connectorFlagName, "c", connectorFlagImage, minimalImage, "--auth-type", "kerberos"},
		{connectorFlagName, "c", connectorFlagImage, minimalImage, "--name-reg", "r"}, // unknown flag (sanity)
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

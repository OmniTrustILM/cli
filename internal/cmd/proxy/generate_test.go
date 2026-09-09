/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

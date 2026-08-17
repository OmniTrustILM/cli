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
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newTestOptions(in []byte) (*cli.Options, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.Options{
		In:      bytes.NewReader(in),
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
	}, out, errOut
}

func TestGenerate_DefaultProfileEmitsYAML(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagNS, platformName, platformFlagProf, platformExternal})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "kind: Platform")
	assert.Contains(t, s, "name: ilm")
	assert.Contains(t, s, "mode: external")
	assert.Contains(t, s, "# Effective values")
}

func TestGenerate_FlagOverridesEchoed(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagProf, "managed-ha", "--db-mode", platformExternal})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "# database.mode = external (flag)")
	assert.Contains(t, s, "# messaging.mode = managed (profile)")
}

func TestGenerate_KeycloakNoneOmitsBlock(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagProf, platformExternal, "--keycloak-mode", "none"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, out.String(), "keycloak:")
}

func TestGenerate_InvalidFlagIsError(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, "--db-mode", "bogus"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestGenerate_MissingNameIsError(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagProf, platformExternal})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestGenerate_Apply_DryRunServer(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	old := generateClientFor
	generateClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { generateClientFor = old }()

	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagNS, platformName, platformFlagProf, platformExternal, "--apply", "--dry-run=server"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	// The fake client does not run CEL webhooks, so the apply should succeed.
	require.NoError(t, cmd.Execute())
}

func TestGenerate_NoApply_NoClusterContact(t *testing.T) {
	// The client seam must never be called when --apply is absent.
	called := false
	old := generateClientFor
	generateClientFor = func(*cli.Options) (*k8s.Client, error) {
		called = true
		return nil, nil
	}
	defer func() { generateClientFor = old }()

	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagProf, platformExternal})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.False(t, called, "client seam must not be invoked without --apply")
}

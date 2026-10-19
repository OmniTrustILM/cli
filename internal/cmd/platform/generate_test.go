/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"bytes"
	"context"
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

// completeApplyArgs returns the arguments that generate and apply a Platform with no
// placeholders left, followed by extra.
func completeApplyArgs(extra ...string) []string {
	return append([]string{
		platformFlagName, platformName, platformFlagProf, "managed-ha",
		"--provisioning-mode", platformExternal, "--host", platformILMDomain, "--apply",
	}, extra...)
}

func TestGenerate_Apply_DryRunServer(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	old := generateClientFor
	generateClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { generateClientFor = old }()

	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs(completeApplyArgs(platformFlagNS, platformName, "--dry-run=server"))
	cmd.SetOut(out)
	cmd.SetErr(out)
	// The fake client does not run CEL webhooks, so the apply should succeed.
	require.NoError(t, cmd.Execute())
}

func TestGenerate_Apply_UsesContextNamespace(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	old := generateClientFor
	generateClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { generateClientFor = old }()

	o, out, _ := newTestOptions(nil)
	o.Factory = k8s.NewFactoryWithClient(c)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs(completeApplyArgs())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	ns, _, err := o.Factory.Namespace()
	require.NoError(t, err)
	_, err = c.GetPlatform(context.Background(), ns, platformName)
	require.NoError(t, err, "without -n the Platform is applied to the context namespace")
}

func TestGenerate_Apply_RefusesPlaceholders(t *testing.T) {
	called := false
	old := generateClientFor
	generateClientFor = func(*cli.Options) (*k8s.Client, error) {
		called = true
		return nil, nil
	}
	defer func() { generateClientFor = old }()

	o, out, _ := newTestOptions(nil)
	cmd := NewGenerateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName, platformFlagProf, platformExternal, "--apply"})
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.host", "the refusal names the fields to replace")
	assert.False(t, called, "a Platform with placeholders never reaches the cluster")
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

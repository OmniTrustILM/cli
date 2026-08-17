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

package rootcmd_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/rootcmd"
)

const (
	cmdDescribe    = "describe"
	cmdDiagnostics = "diagnostics"
	cmdEvents      = "events"
	cmdGenerate    = "generate"
	cmdGet         = "get"
	cmdWait        = "wait"
	cmdPlatform    = "platform"
	cmdStatus      = "status"
)

// cmdMap returns a map of command name → cobra.Command for all direct
// subcommands of the root, plus a depth-2 walk of platform/connector/proxy/diag.
func cmdMap(t *testing.T) map[string]bool {
	t.Helper()
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)

	names := make(map[string]bool)
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	return names
}

func subCmds(t *testing.T, parent string) map[string]bool {
	t.Helper()
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)
	cmd, _, err := root.Find([]string{parent})
	require.NoErrorf(t, err, "Find(%q)", parent)
	require.NotNilf(t, cmd, "command %q must exist", parent)

	names := make(map[string]bool)
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	return names
}

// TestRegister_TopLevelSubcommands verifies that Register wires every expected
// top-level subcommand to the root cobra.Command.
func TestRegister_TopLevelSubcommands(t *testing.T) {
	t.Parallel()
	names := cmdMap(t)

	required := []string{
		"init", "upgrade", "uninstall", cmdStatus, "check",
		"version", "deps", cmdPlatform, "connector", "proxy", cmdDiagnostics,
		"completion",
	}
	for _, want := range required {
		assert.Truef(t, names[want], "root must have subcommand %q", want)
	}
}

// TestRegister_PlatformSubcommands verifies the platform sub-tree.
func TestRegister_PlatformSubcommands(t *testing.T) {
	t.Parallel()
	names := subCmds(t, cmdPlatform)

	required := []string{
		cmdGet, cmdDescribe, cmdStatus, cmdEvents, cmdWait, "logs",
		cmdGenerate, "migrate", "apply", "edit", "delete", "upgrade", "credentials",
	}
	for _, want := range required {
		assert.Truef(t, names[want], "platform must have subcommand %q", want)
	}
}

// TestRegister_ConnectorSubcommands verifies the connector sub-tree.
func TestRegister_ConnectorSubcommands(t *testing.T) {
	t.Parallel()
	names := subCmds(t, "connector")

	for _, want := range []string{cmdGet, cmdDescribe, cmdStatus, cmdEvents, cmdWait, cmdGenerate} {
		assert.Truef(t, names[want], "connector must have subcommand %q", want)
	}
}

// TestRegister_ProxySubcommands verifies the proxy sub-tree.
func TestRegister_ProxySubcommands(t *testing.T) {
	t.Parallel()
	names := subCmds(t, "proxy")

	for _, want := range []string{cmdGet, cmdDescribe, cmdStatus, cmdEvents, cmdWait, cmdGenerate} {
		assert.Truef(t, names[want], "proxy must have subcommand %q", want)
	}
}

// TestRegister_DiagnosticsSubcommands verifies the diagnostics sub-tree.
func TestRegister_DiagnosticsSubcommands(t *testing.T) {
	t.Parallel()
	names := subCmds(t, cmdDiagnostics)
	assert.Truef(t, names["analyze"], "diagnostics must have subcommand \"analyze\"")
}

// TestRegister_HelpGroups verifies that representative commands carry the expected GroupID.
func TestRegister_HelpGroups(t *testing.T) {
	t.Parallel()
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)

	tests := []struct {
		cmd   string
		group cli.GroupID
	}{
		{"init", cli.GroupInfrastructure},
		{cmdPlatform, cli.GroupResources},
		{cmdDiagnostics, cli.GroupDiagnostics},
	}
	for _, tt := range tests {
		cmd, _, err := root.Find([]string{tt.cmd})
		require.NoErrorf(t, err, "Find(%q)", tt.cmd)
		require.NotNilf(t, cmd, "command %q must exist", tt.cmd)
		assert.Equalf(t, string(tt.group), cmd.GroupID,
			"command %q must be in group %q", tt.cmd, tt.group)
	}
}

// TestNew_ReturnsFullyWiredRoot verifies that New builds a root command that is
// immediately executable (cobra can walk the tree without panicking) and that
// the root Use field reflects the binary name.
func TestNew_ReturnsFullyWiredRoot(t *testing.T) {
	t.Parallel()
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)
	assert.NotNil(t, root)
	assert.NotEmpty(t, root.Use)
	// Cobra's Commands() traverses the tree; a panic here means wiring is broken.
	assert.NotPanics(t, func() {
		_ = root.Commands()
	})
}

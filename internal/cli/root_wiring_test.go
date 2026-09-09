/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/rootcmd"
)

func TestRootHasReadCommands(t *testing.T) {
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"status", "check", "platform", "connector", "proxy"} {
		assert.Truef(t, names[want], "root must register %q", want)
	}
}

func TestRootHasDiagnostics(t *testing.T) {
	t.Parallel()
	o := cli.NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	root := rootcmd.New(o)
	diag, _, err := root.Find([]string{"diagnostics"})
	assert.NoError(t, err)
	assert.NotNil(t, diag)
	assert.Equal(t, string(cli.GroupDiagnostics), diag.GroupID)
	_, _, err = root.Find([]string{"diagnostics", "analyze"})
	assert.NoError(t, err)
}

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

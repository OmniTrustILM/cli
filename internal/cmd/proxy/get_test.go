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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunGet_ListWide(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "p1")
	assert.Contains(t, s, "Running")
	assert.Contains(t, s, "2.18.0")
	assert.Contains(t, s, "NAMESPACE")
	assert.Contains(t, s, "PHASE")
	assert.Contains(t, s, "VERSION")
	assert.Contains(t, s, "READY")
	assert.Contains(t, s, "1") // ReadyReplicas from newProxy fixture
}

func TestRunGet_SingleNotFound(t *testing.T) {
	c := newProxyClient(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	assert.Error(t, runGet(context.Background(), c, p, testNS, "absent"))
}

func TestRunGet_Multiple(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS), newProxy("p2", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "p1")
	assert.Contains(t, s, "p2")
}

func TestRunGet_JSONSingle(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, "p1"))
	assert.Contains(t, out.String(), "p1")
}

func TestRunGet_JSONList(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	assert.Contains(t, out.String(), "ProxyList")
}

func TestNewGetCommandFromOpts_RunE_ListWide(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newGetCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{}))
	assert.Contains(t, out.String(), "p1")
}

func TestNewProxyCommand_SubcommandRegistration(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	cmd := NewProxyCommand(o)
	// Literal on purpose: the command sets Use from proxyKind, so asserting against
	// that same constant would pass whatever value it held. This pins the invocation
	// name users type.
	assert.Equal(t, "proxy", cmd.Use)
	assert.Equal(t, string(cli.GroupResources), cmd.GroupID)
	assert.Contains(t, cmd.Aliases, "prx")
	assert.Contains(t, cmd.Aliases, "proxies")
	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"get", "status", "describe", "events", "wait", "logs"} {
		assert.True(t, names[want], "expected subcommand %q", want)
	}
}

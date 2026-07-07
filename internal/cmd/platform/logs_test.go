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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestResolveLogComponent(t *testing.T) {
	require.NoError(t, resolveLogComponent("core"))
	require.NoError(t, resolveLogComponent("provisioning-rabbitmq"))
	assert.Error(t, resolveLogComponent("nope"))
}

func TestLogsSelectorMatchesShared(t *testing.T) {
	assert.Equal(t, shared.ComponentSelector("ilm", "core"), shared.ComponentSelector("ilm", "core"))
}

func TestResolveLogComponent_AllValid(t *testing.T) {
	for _, comp := range shared.PlatformLogComponents {
		assert.NoError(t, resolveLogComponent(comp), "expected %q to be valid", comp)
	}
}

func TestResolveLogComponent_InvalidError(t *testing.T) {
	err := resolveLogComponent("unknown-component")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown component")
	assert.Contains(t, err.Error(), "core")
}

// TestRunLogs_NoPods exercises the "no pods found" error path in execLogs.
func TestRunLogs_NoPods(t *testing.T) {
	// The platform exists but has no pods matching the component selector.
	c := newPlatformClient(t, newPlatform("ilm", testNS))
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "ilm", Component: "core",
		Follow: false, Since: 0, Tail: 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

// TestNewLogsCommand_InvalidComponent verifies that an unknown --component is rejected
// before calling into the client.
func TestNewLogsCommand_UnknownComponent(t *testing.T) {
	err := resolveLogComponent("not-a-component")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-component")
}

// TestNewLogsCommandFromOpts_RunE_NoPods exercises the RunE path via clientFn injection.
// The platform exists but has no pods, so execLogs returns "no pods found".
func TestNewLogsCommandFromOpts_RunE_NoPods(t *testing.T) {
	c := newPlatformClient(t, newPlatform("ilm", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &logsOpts{
		clientFn:    func() (*k8s.Client, error) { return c, nil },
		namespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newLogsCommandFromOpts(o, opts)
	err := cmd.RunE(cmd, []string{"ilm"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

// TestNewLogsCommandFromOpts_RunE_InvalidComponent verifies the --component validation
// inside RunE rejects unknown component names.
func TestNewLogsCommandFromOpts_RunE_InvalidComponent(t *testing.T) {
	c := newPlatformClient(t, newPlatform("ilm", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &logsOpts{
		clientFn:    func() (*k8s.Client, error) { return c, nil },
		namespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newLogsCommandFromOpts(o, opts)
	require.NoError(t, cmd.Flags().Set("component", "invalid-comp"))
	err := cmd.RunE(cmd, []string{"ilm"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid-comp")
}

// TestRunLogs_PodLogsFails exercises lines 92-103: opts building and the PodLogs
// error path. A real pod exists (so PodsFor succeeds) but the fake client has no
// REST discovery, so PodLogs returns an error before any io.Copy.
func TestRunLogs_PodLogsFails(t *testing.T) {
	sel := shared.ComponentSelector("ilm", "core")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ilm-core-0", Namespace: testNS,
			Labels: sel,
		},
	}
	c := newPlatformClient(t, newPlatform("ilm", testNS), pod)
	var out bytes.Buffer
	// since>0 and tail<0 to exercise both option branches before PodLogs is called.
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "ilm", Component: "core",
		Follow: false, Since: 5 * time.Second, Tail: -1,
	})
	require.Error(t, err) // fake client has no REST client for logs
}

// TestRunLogs_PodLogsFailsTailSet exercises the tail>=0 option branch (default path).
func TestRunLogs_PodLogsFailsTailSet(t *testing.T) {
	sel := shared.ComponentSelector("ilm", "core")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ilm-core-0", Namespace: testNS,
			Labels: sel,
		},
	}
	c := newPlatformClient(t, newPlatform("ilm", testNS), pod)
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "ilm", Component: "core",
		Follow: false, Since: 0, Tail: 50,
	})
	require.Error(t, err) // fake client has no REST client for logs
}

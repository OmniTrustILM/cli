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

package cmdutil_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func okSeams() *cmdutil.SingleNameOpts {
	return &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return &k8s.Client{}, nil },
		NamespaceFn: func() (string, bool, error) { return "ns1", true, nil },
	}
}

func TestNewOptionalNameCommand_ListAndGet(t *testing.T) {
	var gotName string
	fn := cmdutil.RunFn(func(_ context.Context, _ *k8s.Client, _ *render.Printer, _, name string) error {
		gotName = name
		return nil
	})
	o := &cli.Options{Printer: render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})}

	cmd := cmdutil.NewOptionalNameCommand(o, okSeams(), "get [NAME]", "get", fn)
	require.NoError(t, cmd.RunE(cmd, nil))
	assert.Equal(t, "", gotName, "no arg means list mode with empty name")

	cmd2 := cmdutil.NewOptionalNameCommand(o, okSeams(), "get [NAME]", "get", fn)
	require.NoError(t, cmd2.RunE(cmd2, []string{"foo"}))
	assert.Equal(t, "foo", gotName)
}

func TestNewOptionalNameCommand_ResolutionErrors(t *testing.T) {
	fn := cmdutil.RunFn(func(context.Context, *k8s.Client, *render.Printer, string, string) error { return nil })
	o := &cli.Options{Printer: render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})}

	clientErr := &cmdutil.SingleNameOpts{ClientFn: func() (*k8s.Client, error) { return nil, assert.AnError }}
	cmd := cmdutil.NewOptionalNameCommand(o, clientErr, "get", "get", fn)
	require.ErrorIs(t, cmd.RunE(cmd, nil), assert.AnError)

	nsErr := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return &k8s.Client{}, nil },
		NamespaceFn: func() (string, bool, error) { return "", false, assert.AnError },
	}
	cmd2 := cmdutil.NewOptionalNameCommand(o, nsErr, "get", "get", fn)
	require.ErrorIs(t, cmd2.RunE(cmd2, nil), assert.AnError)
}

func TestNewWaitCommand_Success(t *testing.T) {
	getConds := func(context.Context, *k8s.Client, string, string) ([]metav1.Condition, string, int64, int64, error) {
		return []metav1.Condition{{Type: "Available", Status: metav1.ConditionTrue}}, "Running", 1, 1, nil
	}
	out := &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, &bytes.Buffer{})}
	cmd := cmdutil.NewWaitCommand(o, okSeams(), "Platform", getConds)
	require.NoError(t, cmd.Flags().Set("for", "condition=Available"))
	require.NoError(t, cmd.Flags().Set("timeout", "2s"))
	require.NoError(t, cmd.RunE(cmd, []string{"ilm"}))
	assert.Contains(t, out.String(), "platform/ilm met condition=Available")
}

func TestNewWaitCommand_InvalidForIsUsageError(t *testing.T) {
	getConds := func(context.Context, *k8s.Client, string, string) ([]metav1.Condition, string, int64, int64, error) {
		return nil, "", 0, 0, nil
	}
	o := &cli.Options{Printer: render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})}
	cmd := cmdutil.NewWaitCommand(o, okSeams(), "Platform", getConds)
	require.NoError(t, cmd.Flags().Set("for", "bogus=x"))
	err := cmd.RunE(cmd, []string{"ilm"})
	require.Error(t, err)
	var ue cli.UsageError
	assert.True(t, errors.As(err, &ue), "invalid --for should be a usage error")
}

func TestNewWaitCommand_ClientError(t *testing.T) {
	getConds := func(context.Context, *k8s.Client, string, string) ([]metav1.Condition, string, int64, int64, error) {
		return nil, "", 0, 0, nil
	}
	o := &cli.Options{Printer: render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})}
	opts := &cmdutil.SingleNameOpts{ClientFn: func() (*k8s.Client, error) { return nil, assert.AnError }}
	cmd := cmdutil.NewWaitCommand(o, opts, "Platform", getConds)
	require.NoError(t, cmd.Flags().Set("for", "condition=Available"))
	require.ErrorIs(t, cmd.RunE(cmd, []string{"ilm"}), assert.AnError)
}

func TestRenderConditions(t *testing.T) {
	out := &bytes.Buffer{}
	p := render.NewPrinter(out, &bytes.Buffer{})
	p.Color = render.ColorNever
	conds := []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: "OK", Message: "all good"}}
	require.NoError(t, cmdutil.RenderConditions(p, "CONDITION", conds))
	s := out.String()
	assert.Contains(t, s, "CONDITION")
	assert.Contains(t, s, "Ready")

	out.Reset()
	require.NoError(t, cmdutil.RenderConditions(p, "TYPE", nil))
	assert.Contains(t, out.String(), "TYPE")
}

func fakeClientWithPods(t *testing.T, pods ...*corev1.Pod) *k8s.Client {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	b := ctrlfake.NewClientBuilder().WithScheme(s)
	for _, p := range pods {
		b = b.WithObjects(p)
	}
	return &k8s.Client{Typed: b.Build(), Scheme: s}
}

func TestNewLogsCommand_Delegates(t *testing.T) {
	var got shared.LogsRequest
	run := func(_ context.Context, _ *k8s.Client, _ io.Writer, req shared.LogsRequest) error {
		got = req
		return nil
	}
	o := &cli.Options{Printer: render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})}
	cmd := cmdutil.NewLogsCommand(o, okSeams(), "Tail logs", run)
	require.NoError(t, cmd.Flags().Set("tail", "42"))
	require.NoError(t, cmd.Flags().Set("since", "5s"))
	require.NoError(t, cmd.Flags().Set("follow", "true"))
	require.NoError(t, cmd.RunE(cmd, []string{"x"}))
	assert.Equal(t, "x", got.Name)
	assert.Equal(t, int64(42), got.Tail)
	assert.Equal(t, 5*time.Second, got.Since)
	assert.True(t, got.Follow)
}

func TestTailPodLogs_NoPods(t *testing.T) {
	c := fakeClientWithPods(t)
	err := cmdutil.TailPodLogs(context.Background(), c, &bytes.Buffer{},
		shared.LogsRequest{Namespace: "ns1", Name: "x", Tail: 100},
		`proxy "x"`, "proxy", map[string]string{"app": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no pods found for proxy "x" in namespace "ns1"`)
}

func TestTailPodLogs_OptionsBranches(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "x-0", Namespace: "ns1", Labels: map[string]string{"app": "x"},
	}}
	c := fakeClientWithPods(t, pod)
	// The fake client has no logs REST client, so PodLogs errors — but podLogOptions
	// (both the --since and the --tail branches) is exercised on the way there.
	for _, req := range []shared.LogsRequest{
		{Namespace: "ns1", Name: "x", Since: 5 * time.Second, Tail: -1},
		{Namespace: "ns1", Name: "x", Tail: 50},
	} {
		err := cmdutil.TailPodLogs(context.Background(), c, &bytes.Buffer{}, req,
			`proxy "x"`, "proxy", map[string]string{"app": "x"})
		require.Error(t, err)
	}
}

func TestRenderEventsTable(t *testing.T) {
	out := &bytes.Buffer{}
	p := render.NewPrinter(out, &bytes.Buffer{})
	p.Color = render.ColorNever

	require.NoError(t, cmdutil.RenderEventsTable(p, nil, "platform", "ns1", "ilm"))
	assert.Contains(t, out.String(), "no events for platform/ilm in ns1")

	out.Reset()
	evs := []corev1.Event{{
		Type:           "Normal",
		Reason:         "Created",
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "p1"},
		Message:        "created pod",
	}}
	require.NoError(t, cmdutil.RenderEventsTable(p, evs, "platform", "ns1", "ilm"))
	s := out.String()
	assert.Contains(t, s, "Created")
	assert.Contains(t, s, "Pod/p1")
}

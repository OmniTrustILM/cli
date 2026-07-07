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

package connector

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newConnectorClient(t *testing.T, objs ...interface {
	metav1.Object
	runtimeObject
}) *k8s.Client {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	b := ctrlfake.NewClientBuilder().WithScheme(s)
	for _, o := range objs {
		b = b.WithObjects(o)
	}
	return &k8s.Client{Typed: b.Build(), Scheme: s}
}

func newConnector(name, ns string) *otilmv1alpha1.Connector {
	return &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning, ReadyReplicas: 1, Replicas: 1,
			Conditions: []metav1.Condition{{Type: connectorAvailable, Status: metav1.ConditionTrue}},
		},
	}
}

func TestRunStatus_ShowsRegistration(t *testing.T) {
	ts := metav1.NewTime(time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC))
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning, ReadyReplicas: 1, Replicas: 1,
			Registration: &otilmv1alpha1.RegistrationStatus{
				UUID: "uuid-123", Status: otilmv1alpha1.RegistrationStatusWaitingForApproval,
				RegisteredAt: &ts,
			},
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "uuid-123")
	assert.Contains(t, s, "waitingForApproval")
	assert.Contains(t, s, "Running")
	assert.Contains(t, s, "2025-06-01T12:00:00Z")
}

func TestRunStatus_RegisteredAt_Nil(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning,
			Registration: &otilmv1alpha1.RegistrationStatus{
				UUID: "uuid-456", Status: otilmv1alpha1.RegistrationStatusConnected,
				// RegisteredAt intentionally nil
			},
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "uuid-456")
	assert.Contains(t, s, "<unknown>")
}

func TestRunStatus_NoRegistration(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhasePending},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "c1"))
	assert.Contains(t, out.String(), "<not registered>")
}

func TestRunStatus_WithConditions(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning,
			Conditions: []metav1.Condition{
				{Type: connectorAvailable, Status: metav1.ConditionTrue, Reason: "AllReady", Message: "ok"},
			},
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, connectorAvailable)
	assert.Contains(t, s, "AllReady")
}

func TestRunStatus_JSONStructured(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "c1"))
	assert.Contains(t, out.String(), "c1")
}

// TestNewStatusCommandFromOpts_RunE exercises the status RunE via ClientFn injection.
func TestNewStatusCommandFromOpts_RunE(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newStatusCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"c1"}))
	assert.Contains(t, out.String(), "Running")
}

// TestNewConnectorCommand_SubcommandRegistration verifies the parent command registers
// all expected subcommands.
func TestNewConnectorCommand_SubcommandRegistration(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	cmd := NewConnectorCommand(o)
	assert.Equal(t, "connector", cmd.Use)
	assert.Equal(t, string(cli.GroupResources), cmd.GroupID)
	assert.Contains(t, cmd.Aliases, "conn")
	assert.Contains(t, cmd.Aliases, "connectors")

	names := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"get", "status", "describe", "events", "wait", "logs"} {
		assert.True(t, names[want], "expected subcommand %q", want)
	}
}

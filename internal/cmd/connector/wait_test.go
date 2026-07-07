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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// newTestPod creates a minimal Pod with the given labels for use in hermetic tests.
func newTestPod(name, ns string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
	}
}

func TestRunWait_ConditionMet(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	target, err := shared.ParseWaitFor("condition=Available")
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, "c1", target, time.Second))
}

func TestRunWait_PhaseTimeout(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhasePending},
	}
	c := newConnectorClient(t, conn)
	target, _ := shared.ParseWaitFor("phase=Running")
	err := runWait(context.Background(), c, testNS, "c1", target, 150*time.Millisecond)
	assert.ErrorIs(t, err, shared.ErrWaitTimeout)
}

func TestRunWait_PhaseMet(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	c := newConnectorClient(t, conn)
	target, err := shared.ParseWaitFor("phase=Running")
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, "c1", target, time.Second))
}

// TestNewWaitCommandFromOpts_RunE exercises the wait RunE via clientFn injection.
func TestNewWaitCommandFromOpts_RunE(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newWaitCommandFromOpts(o, opts)
	require.NoError(t, cmd.Flags().Set("for", "condition=Available"))
	require.NoError(t, cmd.Flags().Set("timeout", "2s"))
	require.NoError(t, cmd.RunE(cmd, []string{"c1"}))
	assert.Contains(t, out.String(), "met condition=Available")
}

// TestNewEventsCommandFromOpts_RunE exercises the events RunE via ClientFn injection.
func TestNewEventsCommandFromOpts_RunE(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newEventsCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"c1"}))
	assert.Contains(t, out.String(), "no events")
}

// TestNewDescribeCommandFromOpts_RunE exercises the describe RunE via ClientFn injection.
func TestNewDescribeCommandFromOpts_RunE(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Spec: otilmv1alpha1.ConnectorSpec{
			Image: otilmv1alpha1.ImageSpec{Repository: "example.com/connector", Tag: "1.0.0"},
			Registration: &otilmv1alpha1.RegistrationSpec{
				PlatformURL: "https://ilm.example.com", Name: "my-connector", AuthType: otilmv1alpha1.AuthTypeCertificate,
			},
		},
		Status: otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	c := newConnectorClient(t, conn)
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newDescribeCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"c1"}))
	s := out.String()
	assert.Contains(t, s, "Running")
	assert.Contains(t, s, "example.com/connector")
	assert.Contains(t, s, "https://ilm.example.com")
}

// TestRunLogs_NoPods exercises the "no pods found" path.
func TestRunLogs_NoPods(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "c1", Follow: false, Since: 0, Tail: 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

// TestNewLogsCommandFromOpts_RunE_NoPods exercises the RunE path via clientFn injection.
func TestNewLogsCommandFromOpts_RunE_NoPods(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newLogsCommandFromOpts(o, opts)
	err := cmd.RunE(cmd, []string{"c1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

// TestRunLogs_PodLogsFails exercises the PodLogs error path.
func TestRunLogs_PodLogsFails(t *testing.T) {
	sel := connectorPodSelector("c1")
	pod := newTestPod("c1-pod-0", testNS, sel)
	c := newConnectorClient(t, newConnector("c1", testNS), pod)
	var out bytes.Buffer
	// since>0 and tail<0 to hit both option branches.
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "c1", Follow: false, Since: 5 * time.Second, Tail: -1,
	})
	require.Error(t, err) // fake client has no REST client for logs
}

// TestRunLogs_PodLogsFailsTailSet exercises the tail>=0 option branch.
func TestRunLogs_PodLogsFailsTailSet(t *testing.T) {
	sel := connectorPodSelector("c1")
	pod := newTestPod("c1-pod-0", testNS, sel)
	c := newConnectorClient(t, newConnector("c1", testNS), pod)
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "c1", Follow: false, Since: 0, Tail: 50,
	})
	require.Error(t, err) // fake client has no REST client for logs
}

// TestAgeFunction covers all branches of the age helper via cmdutil.
func TestAgeFunction(t *testing.T) {
	assert.Equal(t, "<unknown>", cmdutil.Age(metav1.Time{}.Time))
	s := cmdutil.Age(time.Now().Add(-10 * time.Second))
	assert.Contains(t, s, "s")
	m := cmdutil.Age(time.Now().Add(-5 * time.Minute))
	assert.Contains(t, m, "m")
	h := cmdutil.Age(time.Now().Add(-3 * time.Hour))
	assert.Contains(t, h, "h")
	d := cmdutil.Age(time.Now().Add(-48 * time.Hour))
	assert.Contains(t, d, "d")
}

// TestOrNone covers the orNone helper via cmdutil.
func TestOrNone(t *testing.T) {
	assert.Equal(t, "<none>", cmdutil.OrNone(""))
	assert.Equal(t, "foo", cmdutil.OrNone("foo"))
}

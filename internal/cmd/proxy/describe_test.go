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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunDescribe_WithConditions(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: "my-config-token"},
		},
		Status: otilmv1alpha1.ProxyStatus{
			Phase: otilmv1alpha1.ProxyPhaseRunning,
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue, Reason: "AllReady", Message: "ok"},
			},
		},
	}
	c := newProxyClient(t, prx)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "p1"))
	s := out.String()
	assert.Contains(t, s, "my-config-token")
	assert.Contains(t, s, "Available")
	assert.Contains(t, s, "AllReady")
}

// TestProxyPodSelector verifies the selector matches the operator's real proxy
// pod label (otilm.com/proxy=<name>), not the app.kubernetes.io labels that
// matched nothing on a live cluster.
func TestProxyPodSelector(t *testing.T) {
	assert.Equal(t, map[string]string{proxyPodLabel: "p1"}, proxyPodSelector("p1"))
}

func TestRunDescribe_WithPods(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: proxyCfgTok},
		},
		Status: otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseRunning},
	}
	// Seed with the labels the operator actually applies to proxy pods.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p1-pod-xyz", Namespace: testNS, Labels: map[string]string{
			proxyPodLabel:                 "p1",
			"app.kubernetes.io/name":      "p1",
			"app.kubernetes.io/component": proxyKind,
		}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := newProxyClient(t, prx, pod)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "p1"))
	assert.Contains(t, out.String(), "p1-pod-xyz")
}

func TestRunDescribe_WithEvents(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: proxyCfgTok},
		},
		Status: otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseRunning},
	}
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "p1-evt-1", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Proxy", Name: "p1", Namespace: testNS,
		},
		Type:    corev1.EventTypeNormal,
		Reason:  "Deployed",
		Message: "proxy deployed successfully",
	}
	c := newProxyClient(t, prx, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "p1"))
	s := out.String()
	assert.Contains(t, s, "Deployed")
	assert.Contains(t, s, "proxy deployed successfully")
}

func TestRunDescribe_JSONStructured(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: proxyCfgTok},
		},
	}
	c := newProxyClient(t, prx)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "p1"))
	assert.Contains(t, out.String(), "p1")
}

func TestNewDescribeCommandFromOpts_RunE(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: "my-token"},
		},
		Status: otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseRunning},
	}
	c := newProxyClient(t, prx)
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newDescribeCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"p1"}))
	assert.Contains(t, out.String(), "my-token")
}

func TestRunEvents_NoEvents(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runEvents(context.Background(), c, p, testNS, "p1"))
	assert.Contains(t, out.String(), "no events")
}

func TestRunEvents_WithEvents(t *testing.T) {
	prx := newProxy("p1", testNS)
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "p1-evt-2", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Proxy", Name: "p1", Namespace: testNS,
		},
		Type:    corev1.EventTypeWarning,
		Reason:  "ProvisionFailed",
		Message: "config token missing",
	}
	c := newProxyClient(t, prx, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runEvents(context.Background(), c, p, testNS, "p1"))
	s := out.String()
	assert.Contains(t, s, "ProvisionFailed")
	assert.Contains(t, s, "config token missing")
}

func TestNewEventsCommandFromOpts_RunE(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newEventsCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"p1"}))
	assert.Contains(t, out.String(), "no events")
}

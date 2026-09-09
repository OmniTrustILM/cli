/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newProxy(name, ns string) *otilmv1alpha1.Proxy {
	return &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status: otilmv1alpha1.ProxyStatus{
			Phase:           otilmv1alpha1.ProxyPhaseRunning,
			ObservedVersion: proxyVer2180,
			ReadyReplicas:   1,
			Conditions:      []metav1.Condition{{Type: proxyAvailable, Status: metav1.ConditionTrue}},
		},
	}
}

func TestRunStatus_ScaledDownAndChecksum(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Status: otilmv1alpha1.ProxyStatus{
			Phase:           otilmv1alpha1.ProxyPhaseScaledDown,
			ObservedVersion: proxyVer2180,
			ConfigChecksum:  "sha256:abc",
			Conditions: []metav1.Condition{
				{Type: proxyAvailable, Status: metav1.ConditionFalse, Reason: "ScaledDown"},
			},
		},
	}
	c := newProxyClient(t, prx)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "p1"))
	s := out.String()
	assert.Contains(t, s, "ScaledDown")
	assert.Contains(t, s, "sha256:abc")
	assert.Contains(t, s, proxyVer2180)
}

func TestRunStatus_JSONStructured(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "p1"))
	assert.Contains(t, out.String(), "p1")
}

func TestRunStatus_WithConditions(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Status: otilmv1alpha1.ProxyStatus{
			Phase: otilmv1alpha1.ProxyPhaseRunning,
			Conditions: []metav1.Condition{
				{Type: proxyAvailable, Status: metav1.ConditionTrue, Reason: "AllReady", Message: "ok"},
				{Type: "Degraded", Status: metav1.ConditionFalse, Reason: "NotDegraded", Message: ""},
			},
		},
	}
	c := newProxyClient(t, prx)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, p, testNS, "p1"))
	s := out.String()
	assert.Contains(t, s, proxyAvailable)
	assert.Contains(t, s, "AllReady")
}

func TestNewStatusCommandFromOpts_RunE(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newStatusCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{"p1"}))
	assert.Contains(t, out.String(), "Running")
}

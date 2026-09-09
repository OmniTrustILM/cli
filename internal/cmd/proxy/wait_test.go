/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package proxy

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunWait_PhaseMet(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Status:     otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseRunning},
	}
	c := newProxyClient(t, prx)
	target, err := shared.ParseWaitFor(proxyPhaseRunning)
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, "p1", target, time.Second))
}

func TestRunWait_PhaseTimeout(t *testing.T) {
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: testNS},
		Status:     otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhasePending},
	}
	c := newProxyClient(t, prx)
	target, _ := shared.ParseWaitFor(proxyPhaseRunning)
	err := runWait(context.Background(), c, testNS, "p1", target, 150*time.Millisecond)
	assert.ErrorIs(t, err, shared.ErrWaitTimeout)
}

func TestRunWait_ConditionMet(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	target, err := shared.ParseWaitFor("condition=Available")
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, "p1", target, time.Second))
}

func TestNewWaitCommandFromOpts_RunE(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newWaitCommandFromOpts(o, opts)
	require.NoError(t, cmd.Flags().Set("for", proxyPhaseRunning))
	require.NoError(t, cmd.Flags().Set("timeout", "2s"))
	require.NoError(t, cmd.RunE(cmd, []string{"p1"}))
	assert.Contains(t, out.String(), "met phase=Running")
}

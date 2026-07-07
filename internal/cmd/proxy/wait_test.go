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

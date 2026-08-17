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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newTestPod(name, ns string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
	}
}

func TestRunLogs_NoPods(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "p1", Follow: false, Since: 0, Tail: 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

// realProxyPodLabels are the labels the operator actually applies to a proxy's
// pods, used so the logs tests exercise the same selector a live cluster hits.
func realProxyPodLabels(name string) map[string]string {
	return map[string]string{
		proxyPodLabel:                 name,
		"app.kubernetes.io/name":      name,
		"app.kubernetes.io/component": proxyKind,
	}
}

func TestRunLogs_PodLogsFails_SinceAndTailNeg(t *testing.T) {
	pod := newTestPod("p1-pod-0", testNS, realProxyPodLabels("p1"))
	c := newProxyClient(t, newProxy("p1", testNS), pod)
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "p1", Follow: false, Since: 5 * time.Second, Tail: -1,
	})
	require.Error(t, err) // fake client has no REST client for logs
}

func TestRunLogs_PodLogsFails_TailSet(t *testing.T) {
	pod := newTestPod("p1-pod-0", testNS, realProxyPodLabels("p1"))
	c := newProxyClient(t, newProxy("p1", testNS), pod)
	var out bytes.Buffer
	err := execLogs(context.Background(), c, &out, shared.LogsRequest{
		Namespace: testNS, Name: "p1", Follow: false, Since: 0, Tail: 50,
	})
	require.Error(t, err)
}

func TestNewLogsCommandFromOpts_RunE_NoPods(t *testing.T) {
	c := newProxyClient(t, newProxy("p1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newLogsCommandFromOpts(o, opts)
	err := cmd.RunE(cmd, []string{"p1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

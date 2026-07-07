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
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

// runtimeObject lets the per-package fake-client helper accept typed CRs.
type runtimeObject = runtime.Object

// testNS is the namespace used across hermetic proxy tests.
const testNS = "ns1"

// fmtJSON is the -o flag value for JSON structured output.
const fmtJSON = "json"

const (
	proxyFlagName        = "--name"
	proxyEgress          = "egress"
	proxyEgressToken     = "egress-config-token"
	proxyFlagConfigToken = "--config-token-secret"
	proxyCfgTok          = "cfg-tok"
	proxyVer2180         = "2.18.0"
	proxyAvailable       = "Available"
	proxyPhaseRunning    = "phase=Running"
)

func newProxyClient(t *testing.T, objs ...interface {
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

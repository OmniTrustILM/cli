/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
	proxyFlagNS          = "--namespace"
	proxyNamespace       = "ilm"
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

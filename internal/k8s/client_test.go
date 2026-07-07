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

package k8s

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func TestNewFactory_DefaultsSchemeAndConfigFlags(t *testing.T) {
	cf := genericclioptions.NewConfigFlags(true)
	f, err := NewFactory(cf)
	require.NoError(t, err)
	assert.Same(t, cf, f.ConfigFlags)
	require.NotNil(t, f.Scheme)
	assert.True(t, f.Scheme.Recognizes(otilmv1alpha1.GroupVersion.WithKind("Platform")))
}

func TestNewFactory_NilConfigFlagsIsError(t *testing.T) {
	_, err := NewFactory(nil)
	require.Error(t, err)
}

func TestFactory_Namespace(t *testing.T) {
	// "explicit namespace" case: drive via ConfigFlags.Namespace field; no kubeconfig needed.
	t.Run("explicit namespace", func(t *testing.T) {
		cf := genericclioptions.NewConfigFlags(true)
		ns := "ilm-prod"
		cf.Namespace = &ns
		f, err := NewFactory(cf)
		require.NoError(t, err)
		got, explicit, err := f.Namespace()
		require.NoError(t, err)
		assert.Equal(t, "ilm-prod", got)
		assert.True(t, explicit)
	})

	// "unset → default" case: hermetic — point kubeconfig at /dev/null so the loader
	// never reads the developer's ambient context namespace.
	t.Run("unset falls back to default", func(t *testing.T) {
		devNull := "/dev/null"
		cf := genericclioptions.NewConfigFlags(true)
		cf.KubeConfig = &devNull
		f, err := NewFactory(cf)
		require.NoError(t, err)
		got, explicit, err := f.Namespace()
		require.NoError(t, err)
		assert.Equal(t, "default", got)
		assert.False(t, explicit)
	})
}

func platform(ns, name string) *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning},
	}
}

func TestClient_PlatformGetAndList(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{
		Objects: []ctrlclient.Object{platform("ilm", k8sAlpha), platform("ilm", "beta")},
	})
	got, err := c.GetPlatform(context.Background(), "ilm", k8sAlpha)
	require.NoError(t, err)
	assert.Equal(t, otilmv1alpha1.PlatformPhaseRunning, got.Status.Phase)

	list, err := c.ListPlatforms(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Len(t, list.Items, 2)
}

func TestClient_ConnectorAndProxyGet(t *testing.T) {
	conn := &otilmv1alpha1.Connector{ObjectMeta: metav1.ObjectMeta{Namespace: "ilm", Name: "c1"}}
	prx := &otilmv1alpha1.Proxy{ObjectMeta: metav1.ObjectMeta{Namespace: "ilm", Name: "p1"}}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{conn, prx}})

	gotConn, err := c.GetConnector(context.Background(), "ilm", "c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", gotConn.Name)

	gotPrx, err := c.GetProxy(context.Background(), "ilm", "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", gotPrx.Name)

	connList, err := c.ListConnectors(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Len(t, connList.Items, 1)

	prxList, err := c.ListProxies(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Len(t, prxList.Items, 1)
}

func TestClient_OperatorDeployment(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: k8sOperatorSys, Name: "ilm-operator-controller-manager"},
	}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{dep}})
	got, err := c.OperatorDeployment(context.Background(), k8sOperatorSys)
	require.NoError(t, err)
	assert.Equal(t, "ilm-operator-controller-manager", got.Name)
}

func TestClient_OperatorDeployment_NotFound(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	_, err := c.OperatorDeployment(context.Background(), k8sOperatorSys)
	require.Error(t, err)
}

func TestClient_DeploymentsForPlatform(t *testing.T) {
	mine := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ilm", Name: "core",
		Labels: map[string]string{"app.kubernetes.io/managed-by": "ilm-operator", "otilm.com/platform": "ilm"},
	}}
	other := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ilm", Name: "unrelated",
		Labels: map[string]string{"app.kubernetes.io/managed-by": "someone-else"},
	}}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{mine, other}})
	deps, err := c.DeploymentsForPlatform(context.Background(), "ilm", "ilm")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "core", deps[0].Name)
}

func TestClient_PodsFor(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ilm", Name: k8sCoreZero, Labels: map[string]string{"app": "core"},
	}}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{pod}})
	pods, err := c.PodsFor(context.Background(), "ilm", map[string]string{"app": "core"})
	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, k8sCoreZero, pods[0].Name)
}

func TestClient_Events(t *testing.T) {
	plat := platform("ilm", k8sAlpha)
	plat.UID = "uid-1"
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "ilm", Name: "ev1"},
		InvolvedObject: corev1.ObjectReference{Namespace: "ilm", Name: k8sAlpha, UID: "uid-1"},
		Reason:         "Reconciled",
	}
	noise := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "ilm", Name: "ev2"},
		InvolvedObject: corev1.ObjectReference{Namespace: "ilm", Name: "other", UID: "uid-2"},
	}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{plat, ev, noise}})
	events, err := c.Events(context.Background(), "ilm", plat)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "Reconciled", events[0].Reason)
}

func TestClient_GetPlatform_NotFound(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	_, err := c.GetPlatform(context.Background(), "ilm", k8sNonexistent)
	require.Error(t, err)
}

func TestClient_ListPlatforms_Empty(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	list, err := c.ListPlatforms(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestClient_GetConnector_NotFound(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	_, err := c.GetConnector(context.Background(), "ilm", k8sNonexistent)
	require.Error(t, err)
}

func TestClient_GetProxy_NotFound(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	_, err := c.GetProxy(context.Background(), "ilm", k8sNonexistent)
	require.Error(t, err)
}

func TestClient_ListConnectors_Empty(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	list, err := c.ListConnectors(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestClient_ListProxies_Empty(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	list, err := c.ListProxies(context.Background(), "ilm")
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestClient_PodsFor_Empty(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	pods, err := c.PodsFor(context.Background(), "ilm", map[string]string{"app": "gone"})
	require.NoError(t, err)
	assert.Empty(t, pods)
}

func TestClient_DeploymentsForPlatform_Empty(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	deps, err := c.DeploymentsForPlatform(context.Background(), "ilm", "absent")
	require.NoError(t, err)
	assert.Empty(t, deps)
}

// TestClient_PodLogs_FakeDiscoError verifies PodLogs returns an error when the
// Discovery client does not expose a REST client (as the fake does not).
// PodLogs must stream through the clientset's core/v1 REST path (GetLogs), not
// the discovery client — the latter is scoped to the discovery group and cannot
// encode PodLogOptions (the "not suitable for converting to meta.k8s.io/v1"
// failure seen against a live cluster).
func TestClient_PodLogs_StreamsViaClientset(t *testing.T) {
	c := NewFakeClient(t, FakeClientOptions{})
	rc, err := c.PodLogs(context.Background(), "ilm", k8sCoreZero, "core", nil)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	_, err = io.ReadAll(rc)
	require.NoError(t, err)
}

func TestClient_PodLogs_NilClientsetErrors(t *testing.T) {
	c := &Client{} // no clientset wired
	_, err := c.PodLogs(context.Background(), "ilm", k8sCoreZero, "core", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clientset")
}

// TestNewFakeClient_CustomScheme verifies that a caller-supplied scheme is honoured.
func TestNewFakeClient_CustomScheme(t *testing.T) {
	s, err := NewScheme()
	require.NoError(t, err)
	c := NewFakeClient(t, FakeClientOptions{Scheme: s})
	assert.Same(t, s, c.Scheme)
}

// TestFactory_RESTConfig_Hermetic verifies RESTConfig returns the injected config
// via the test seam without touching the ambient kubeconfig.
func TestFactory_RESTConfig_Hermetic(t *testing.T) {
	old := restConfigForTest
	restConfigForTest = func(_ *Factory) (*rest.Config, error) {
		return &rest.Config{Host: k8sFakeServer}, nil
	}
	t.Cleanup(func() { restConfigForTest = old })

	cf := genericclioptions.NewConfigFlags(true)
	f, err := NewFactory(cf)
	require.NoError(t, err)
	cfg, err := f.RESTConfig()
	require.NoError(t, err)
	assert.Equal(t, k8sFakeServer, cfg.Host)
}

// TestFactory_Client_Hermetic covers the Factory.Client() construction path
// (ctrlclient.New, dynamic.NewForConfig, kubernetes.NewForConfig) without a real
// cluster or kubeconfig. The test seam injects a synthetic *rest.Config and a
// no-op mapper so neither the config-loader nor the discovery endpoint is called.
func TestFactory_Client_Hermetic(t *testing.T) {
	oldCfg := restConfigForTest
	restConfigForTest = func(_ *Factory) (*rest.Config, error) {
		return &rest.Config{Host: k8sFakeServer}, nil
	}
	t.Cleanup(func() { restConfigForTest = oldCfg })

	oldMapper := restMapperForTest
	restMapperForTest = func(_ *Factory) (meta.RESTMapper, error) {
		// NewDefaultRESTMapper returns a non-nil mapper with no registered types;
		// it satisfies the interface without any network calls.
		return meta.NewDefaultRESTMapper(nil), nil
	}
	t.Cleanup(func() { restMapperForTest = oldMapper })

	cf := genericclioptions.NewConfigFlags(true)
	f, err := NewFactory(cf)
	require.NoError(t, err)

	c, err := f.Client()
	require.NoError(t, err)
	assert.NotNil(t, c.Typed)
	assert.NotNil(t, c.Dynamic)
	assert.NotNil(t, c.Discovery)
	assert.NotNil(t, c.Mapper)
	assert.Same(t, f.Scheme, c.Scheme)
}

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakedisco "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// TestingT is the subset of *testing.T the fake helpers use; accepting an
// interface keeps internal/k8s free of a hard testing import in non-test builds.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// FakeClientOptions configures NewFakeClient. Objects seed the typed client;
// Dynamic seeds the dynamic client (unstructured foreign CRs).
type FakeClientOptions struct {
	Scheme  *runtime.Scheme
	Objects []ctrlclient.Object
	Dynamic []runtime.Object
}

// foreignListKinds maps the managed-infra GVRs to their list kinds so the fake
// dynamic client can serve list requests without needing the real schemes.
var foreignListKinds = map[schema.GroupVersionResource]string{
	GVRCNPGCluster:     "ClusterList",
	GVRRabbitmqCluster: "RabbitmqClusterList",
	GVRKeycloak:        "KeycloakList",
}

// NewFactoryWithClient returns a Factory whose Client() method returns c directly,
// bypassing any cluster connection. Namespace() returns ("default", false, nil)
// because ConfigFlags is nil; commands that need a specific namespace must accept
// it as a local -n flag and pass it explicitly.
// This is the hermetic test seam used by apply/edit/delete tests.
func NewFactoryWithClient(c *Client) *Factory {
	return &Factory{fixedClient: c, Scheme: c.Scheme}
}

// NewFakeClient builds a Client backed by the controller-runtime fake client, a
// fake dynamic client, and a fake discovery client. It is the single test seam
// shared by health/analyze/render/bundle/cmd packages.
func NewFakeClient(t TestingT, o FakeClientOptions) *Client {
	t.Helper()
	s := o.Scheme
	if s == nil {
		var err error
		s, err = NewScheme()
		if err != nil {
			t.Fatalf("k8s: NewScheme: %v", err)
		}
	}

	typed := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(o.Objects...).
		WithStatusSubresource(
			&otilmv1alpha1.Platform{},
			&otilmv1alpha1.Connector{},
			&otilmv1alpha1.Proxy{},
		).
		Build()

	dynScheme := runtime.NewScheme()
	gvrToListKind := make(map[schema.GroupVersionResource]string, len(foreignListKinds))
	for gvr, listKind := range foreignListKinds {
		gvrToListKind[gvr] = listKind
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(dynScheme, gvrToListKind, o.Dynamic...)

	disco := &fakedisco.FakeDiscovery{Fake: &clienttesting.Fake{}}

	return &Client{
		Typed:     typed,
		Dynamic:   dyn,
		Discovery: discovery.DiscoveryInterface(disco),
		Clientset: kubefake.NewClientset(),
		Mapper:    nil,
		Scheme:    s,
	}
}

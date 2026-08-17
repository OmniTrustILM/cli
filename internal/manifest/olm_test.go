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

package manifest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedisco "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

func TestDetectOLM_Present(t *testing.T) {
	t.Parallel()
	// Set the group list explicitly so ServerGroups() deterministically returns
	// the operators.coreos.com group without relying on GroupVersion string parsing.
	disco := &fakedisco.FakeDiscovery{Fake: &clienttesting.Fake{
		Resources: []*metav1.APIResourceList{
			{GroupVersion: "operators.coreos.com/v1alpha1", APIResources: []metav1.APIResource{
				{Name: "subscriptions", Kind: "Subscription"},
				{Name: "catalogsources", Kind: kindCatalogSource},
			}},
			{GroupVersion: "operators.coreos.com/v1", APIResources: []metav1.APIResource{
				{Name: "operatorgroups", Kind: "OperatorGroup"},
			}},
		},
	}}
	c := &k8s.Client{Discovery: disco}

	got, err := DetectOLM(context.Background(), c)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestDetectOLM_Absent(t *testing.T) {
	t.Parallel()
	disco := &fakedisco.FakeDiscovery{Fake: &clienttesting.Fake{
		Resources: []*metav1.APIResourceList{
			{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{{Name: "deployments", Kind: kindDeployment}}},
		},
	}}
	c := &k8s.Client{Discovery: disco}

	got, err := DetectOLM(context.Background(), c)
	require.NoError(t, err)
	assert.False(t, got)
}

func olmDynamicClient(t *testing.T) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	gvrCatalog := schema.GroupVersionResource{Group: manifestOLMGroup, Version: manifestOLMV1alpha1, Resource: "catalogsources"}
	gvrGroup := schema.GroupVersionResource{Group: manifestOLMGroup, Version: "v1", Resource: "operatorgroups"}
	gvrSub := schema.GroupVersionResource{Group: manifestOLMGroup, Version: manifestOLMV1alpha1, Resource: "subscriptions"}
	listKinds := map[schema.GroupVersionResource]string{
		gvrCatalog: "CatalogSourceList",
		gvrGroup:   "OperatorGroupList",
		gvrSub:     "SubscriptionList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	typed := newSSAFakeClient(t, scheme)
	return &k8s.Client{Typed: typed, Dynamic: dyn, Scheme: scheme}
}

// catalogSourceImage retrieves the applied CatalogSource from the typed fake
// client and returns the value of spec.image. The typed fake stores every
// Apply call regardless of whether the GVK is registered in the scheme, so
// this is the correct seam for verifying what was actually written.
func catalogSourceImage(t *testing.T, c *k8s.Client, ns string) string {
	t.Helper()
	var got unstructured.Unstructured
	got.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   manifestOLMGroup,
		Version: manifestOLMV1alpha1,
		Kind:    kindCatalogSource,
	})
	require.NoError(t, c.Typed.Get(
		context.Background(),
		ctrlclient.ObjectKey{Namespace: ns, Name: "ilm-operator-catalog"},
		&got,
	))
	img, _, err := unstructured.NestedString(got.Object, keySpec, "image")
	require.NoError(t, err)
	return img
}

// TestApplyOLM_RequiresCatalogImage confirms ApplyOLM errors (applies nothing)
// when no catalog image is supplied — no default catalog is published yet.
func TestApplyOLM_RequiresCatalogImage(t *testing.T) {
	t.Parallel()
	c := olmDynamicClient(t)

	_, err := ApplyOLM(context.Background(), c, OLMOptions{Namespace: manifestOperatorSys, Channel: manifestOLMChannel})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog image")
}

func TestApplyOLM_CustomCatalogImage(t *testing.T) {
	t.Parallel()
	c := olmDynamicClient(t)

	res, err := ApplyOLM(context.Background(), c, OLMOptions{
		Namespace:    manifestOperatorSys,
		Channel:      manifestOLMChannel,
		CatalogImage: manifestCatalogImage,
	})
	require.NoError(t, err)
	assert.Contains(t, res.Applied, "CatalogSource/ilm-operator-system/ilm-operator-catalog")
	assert.Contains(t, res.Applied, "OperatorGroup/ilm-operator-system/ilm-operator-group")
	assert.Contains(t, res.Applied, "Subscription/ilm-operator-system/ilm-operator")
	// Verify the custom --catalog-image value was embedded in the applied CatalogSource.
	// This assertion fails if the CatalogImage override were ignored (e.g., always defaulting).
	assert.Equal(t, manifestCatalogImage, catalogSourceImage(t, c, manifestOperatorSys))
}

func TestApplyOLM_DryRunClient(t *testing.T) {
	t.Parallel()
	c := olmDynamicClient(t)

	res, err := ApplyOLM(context.Background(), c, OLMOptions{
		Namespace: manifestOperatorSys, Channel: manifestOLMChannel, CatalogImage: manifestCatalogImage, DryRun: DryRunClient,
	})
	require.NoError(t, err)
	for _, id := range res.Applied {
		assert.Contains(t, id, "(dry-run)")
	}
}

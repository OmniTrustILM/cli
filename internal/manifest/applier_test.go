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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

// newSSAFakeClient returns a controller-runtime fake client built with the given
// scheme. It is shared by tests that need a typed client supporting SSA.
func newSSAFakeClient(t *testing.T, scheme *runtime.Scheme) ctrlclient.Client {
	t.Helper()
	return ctrlfake.NewClientBuilder().WithScheme(scheme).Build()
}

func crd(name string, established bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": name},
	}}
	if established {
		_ = unstructured.SetNestedSlice(u.Object, []any{
			map[string]any{"type": "Established", "status": "True"},
		}, "status", "conditions")
	}
	return u
}

func deployObj(name, ns string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": name, "namespace": ns},
	}}
}

// testClient builds a k8s.Client whose Typed client supports SSA and whose
// Dynamic client serves the supplied CRD objects.
func testClient(t *testing.T, crds ...*unstructured.Unstructured) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	require.NoError(t, apiextv1.AddToScheme(scheme))

	typed := ctrlfake.NewClientBuilder().WithScheme(scheme).Build()

	dynScheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	listKinds := map[schema.GroupVersionResource]string{gvr: "CustomResourceDefinitionList"}
	objs := make([]runtime.Object, 0, len(crds))
	for _, c := range crds {
		objs = append(objs, c)
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(dynScheme, listKinds, objs...)

	return &k8s.Client{Typed: typed, Dynamic: dyn, Scheme: scheme}
}

func TestApply_CreatesAndRecords(t *testing.T) {
	t.Parallel()
	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	res, err := a.Apply(context.Background(), []*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)})
	require.NoError(t, err)
	assert.Equal(t, []string{manifestDeploymentID}, res.Applied)
	assert.Empty(t, res.Conflicts)

	var got unstructured.Unstructured
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	require.NoError(t, c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Namespace: manifestOperatorSys, Name: "mgr"}, &got))
	assert.Equal(t, "mgr", got.GetName())
}

func TestApply_ClientDryRunDoesNotPersist(t *testing.T) {
	t.Parallel()
	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	res, err := a.Apply(context.Background(), []*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)})
	require.NoError(t, err)
	assert.Equal(t, []string{"Deployment/ilm-operator-system/mgr (dry-run)"}, res.Applied)

	var got unstructured.Unstructured
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	err = c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Namespace: manifestOperatorSys, Name: "mgr"}, &got)
	require.Error(t, err, "client dry-run must not create the object")
}

func TestWaitCRDsEstablished(t *testing.T) {
	t.Parallel()
	established := crd(manifestPlatformsCRD, true)
	c := testClient(t, established)
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	err := a.WaitCRDsEstablished(context.Background(), []*unstructured.Unstructured{established}, 2*time.Second)
	require.NoError(t, err)
}

func TestWaitCRDsEstablished_TimesOut(t *testing.T) {
	t.Parallel()
	notReady := crd(manifestPlatformsCRD, false)
	c := testClient(t, notReady)
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	err := a.WaitCRDsEstablished(context.Background(), []*unstructured.Unstructured{notReady}, 500*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), manifestPlatformsCRD)
}

func TestApplyOrdered_AppliesCRDsThenController(t *testing.T) {
	t.Parallel()
	established := crd(manifestPlatformsCRD, true)
	c := testClient(t, established)
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	res, err := a.ApplyOrdered(context.Background(),
		[]*unstructured.Unstructured{established},
		[]*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)},
	)
	require.NoError(t, err)
	assert.Contains(t, res.Applied, "CustomResourceDefinition//platforms.otilm.com")
	assert.Contains(t, res.Applied, manifestDeploymentID)

	crdIdx := slices.Index(res.Applied, "CustomResourceDefinition//platforms.otilm.com")
	depIdx := slices.Index(res.Applied, manifestDeploymentID)
	require.GreaterOrEqual(t, crdIdx, 0)
	require.GreaterOrEqual(t, depIdx, 0)
	assert.Less(t, crdIdx, depIdx, "CRDs must be applied before controller objects")
}

func TestApplyOrdered_DryRunSkipsWait(t *testing.T) {
	t.Parallel()

	modes := []struct {
		name   string
		dryRun DryRunMode
	}{
		{"DryRunServer", DryRunServer},
		{"DryRunClient", DryRunClient},
	}
	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// CRD is NOT established; dry-run must still succeed because the wait is skipped.
			notReady := crd(manifestPlatformsCRD, false)
			c := testClient(t, notReady)
			a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: tc.dryRun}

			_, err := a.ApplyOrdered(context.Background(),
				[]*unstructured.Unstructured{notReady},
				[]*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)},
			)
			require.NoError(t, err)
		})
	}
}

var _ = metav1.Now // keep metav1 import used if helper edits drop it

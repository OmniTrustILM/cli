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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

// clientWithDeployments builds a k8s.Client whose typed client is seeded with
// the given appsv1 Deployments (including their status).
func clientWithDeployments(t *testing.T, deps ...*appsv1.Deployment) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)

	b := ctrlfake.NewClientBuilder().WithScheme(scheme)
	for _, d := range deps {
		b = b.WithObjects(d)
	}
	return &k8s.Client{Typed: b.Build(), Scheme: scheme}
}

// availableDeployment returns a Deployment whose status reports it Available:
// observedGeneration matches, and updated/available replicas match the desired
// replica count.
func availableDeployment(name, ns string) *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &one},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    1,
			AvailableReplicas:  1,
			ReadyReplicas:      1,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: conditionTrue},
			},
		},
	}
}

// pendingDeployment returns a Deployment whose status reports it NOT available
// (zero available replicas, Available condition false).
func pendingDeployment(name, ns string) *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Generation: 1},
		Spec:       appsv1.DeploymentSpec{Replicas: &one},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 1,
			UpdatedReplicas:    0,
			AvailableReplicas:  0,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentAvailable, Status: "False"},
			},
		},
	}
}

func TestWaitDeploymentsAvailable_Ready(t *testing.T) {
	t.Parallel()
	c := clientWithDeployments(t, availableDeployment("mgr", manifestOperatorSys))
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	err := a.WaitDeploymentsAvailable(context.Background(),
		[]*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)}, 2*time.Second)
	require.NoError(t, err)
}

func TestWaitDeploymentsAvailable_TimesOut(t *testing.T) {
	t.Parallel()
	c := clientWithDeployments(t, pendingDeployment("mgr", manifestOperatorSys))
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	err := a.WaitDeploymentsAvailable(context.Background(),
		[]*unstructured.Unstructured{deployObj("mgr", manifestOperatorSys)}, 300*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mgr")
}

// TestWaitDeploymentsAvailable_NoDeployments confirms waiting is a no-op success
// when the applied set contains no Deployments.
func TestWaitDeploymentsAvailable_NoDeployments(t *testing.T) {
	t.Parallel()
	c := clientWithDeployments(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl}

	err := a.WaitDeploymentsAvailable(context.Background(),
		[]*unstructured.Unstructured{crd(manifestPlatformsCRD, true)}, 300*time.Millisecond)
	require.NoError(t, err)
}

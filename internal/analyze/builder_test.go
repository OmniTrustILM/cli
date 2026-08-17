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

package analyze

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/version"
	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func newClient(t *testing.T, objs ...client.Object) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &k8s.Client{Typed: fc, Scheme: scheme}
}

func platformFixture() *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: analyzePlatformName, Namespace: analyzePlatformName, Generation: 2},
		Spec: otilmv1alpha1.PlatformSpec{
			Version:   analyzeVer2180,
			Database:  otilmv1alpha1.DatabaseSpec{Mode: "external", Credentials: &otilmv1alpha1.CredentialsRef{SecretRef: analyzeDBCreds}},
			Messaging: otilmv1alpha1.MessagingSpec{Mode: "managed"},
		},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:              otilmv1alpha1.PlatformPhaseRunning,
			ObservedGeneration: 2,
			ObservedVersion:    analyzeVer2180,
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue, Reason: "AllComponentsReady"},
			},
		},
	}
}

func TestBuilderBuildsSnapshotShape(t *testing.T) {
	t.Parallel()
	plat := platformFixture()
	cl := newClient(t, plat)
	b := NewBuilder(cl, nil)

	snap, err := b.Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)

	require.Len(t, snap.Platforms, 1)
	rs := snap.Platforms[0]
	assert.Equal(t, analyzeAPIGroup, rs.GVK)
	assert.Equal(t, analyzePlatformName, rs.Name)
	assert.Equal(t, analyzePlatformName, rs.Namespace)
	assert.Equal(t, "Running", rs.Phase)
	assert.Equal(t, int64(2), rs.Generation)
	assert.Equal(t, int64(2), rs.ObservedGen)
	assert.True(t, rs.SpecModes.MessagingManaged)
	assert.False(t, rs.SpecModes.DBManaged)
	assert.Contains(t, rs.SecretRefs, analyzeDBCreds)

	// Shape parity with the bundle reader: version fields populated.
	assert.NotEmpty(t, snap.SupportedVersions)
	assert.Equal(t, clientVersionForTest(t), snap.ClientVersion)
}

func TestBuilderMissingRefsPreResolution(t *testing.T) {
	t.Parallel()
	plat := platformFixture() // references Secret analyzeDBCreds which is NOT seeded
	cl := newClient(t, plat)
	b := NewBuilder(cl, nil)

	snap, err := b.Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)
	assert.Contains(t, snap.MissingRefs, "Secret/ilm/db-creds")
}

func TestBuilderRefPresentNotFlagged(t *testing.T) {
	t.Parallel()
	plat := platformFixture()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: analyzeDBCreds, Namespace: analyzePlatformName}}
	cl := newClient(t, plat, secret)
	b := NewBuilder(cl, nil)

	snap, err := b.Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)
	assert.NotContains(t, snap.MissingRefs, "Secret/ilm/db-creds")
}

func TestBuilderOperatorReadiness(t *testing.T) {
	t.Parallel()
	one := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm-operator-controller-manager", Namespace: "ilm-operator-system"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "manager", Image: "ghcr.io/omnitrustilm/operator:2.18.0"}},
			}},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1, Replicas: 1},
	}
	cl := newClient(t, platformFixture(), dep)
	b := NewBuilder(cl, nil)

	snap, err := b.Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)
	assert.True(t, snap.OperatorReady)
	assert.Equal(t, analyzeVer2180, snap.OperatorVersion)
}

func TestBuilderNoOperatorIsNotFatal(t *testing.T) {
	t.Parallel()
	cl := newClient(t, platformFixture())
	b := NewBuilder(cl, nil)

	snap, err := b.Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)
	assert.False(t, snap.OperatorReady)
	assert.Equal(t, "", snap.OperatorVersion)
}

// TestBuilderAndDefaultRegistryIdenticalFindingsShape proves the live snapshot
// feeds DefaultRegistry exactly like a bundle would.
func TestBuilderAndDefaultRegistryIdenticalFindingsShape(t *testing.T) {
	t.Parallel()
	degraded := platformFixture()
	degraded.Status.Phase = otilmv1alpha1.PlatformPhaseDegraded
	degraded.Status.Conditions = []metav1.Condition{
		{Type: analyzeDatabaseReady, Status: metav1.ConditionFalse, Reason: "CNPGDown", Message: "down"},
	}
	cl := newClient(t, degraded)
	snap, err := NewBuilder(cl, nil).Build(context.Background(), BuildOptions{AllNamespaces: true})
	require.NoError(t, err)

	findings := DefaultRegistry().Run(snap)
	assert.Equal(t, SeverityFail, Worst(findings))
}

func clientVersionForTest(t *testing.T) string {
	t.Helper()
	return version.Client().ClientVersion
}

// TestBuildLive verifies the convenience wrapper delegates correctly to Builder.Build.
func TestBuildLive(t *testing.T) {
	t.Parallel()
	plat := platformFixture()
	cl := newClient(t, plat)
	snap, err := BuildLive(context.Background(), cl, nil, BuildOptions{AllNamespaces: true})
	require.NoError(t, err)
	require.Len(t, snap.Platforms, 1)
	assert.Equal(t, "Running", snap.Platforms[0].Phase)
	assert.Equal(t, analyzeVer2180, snap.Platforms[0].ObservedVersion)
}

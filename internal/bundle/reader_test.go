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

package bundle

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/k8s"
	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
)

// readerStubMapper is a minimal meta.RESTMapper used to build a
// capabilities.Reporter in bundle reader tests. It returns a mapping for every
// GroupKind in present, and a NoKindMatchError for everything else.
type readerStubMapper struct {
	present map[schema.GroupKind]bool
}

func (m *readerStubMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	if m.present[gk] {
		return &meta.RESTMapping{}, nil
	}
	return nil, &meta.NoKindMatchError{GroupKind: gk}
}
func (m *readerStubMapper) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic("unused")
}
func (m *readerStubMapper) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic("unused")
}
func (m *readerStubMapper) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic("unused")
}
func (m *readerStubMapper) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic("unused")
}
func (m *readerStubMapper) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	panic("unused")
}
func (m *readerStubMapper) ResourceSingularizer(string) (string, error) { panic("unused") }

// newStubReporter builds a capabilities.Reporter whose Detect() returns all
// six deps with Present values dictated by present.
func newStubReporter(present map[schema.GroupKind]bool) *capabilities.Reporter {
	return capabilities.NewReporter(opcap.New(&readerStubMapper{present: present}))
}

// collectFixtureBundle produces a real bundle on disk and returns its path.
func collectFixtureBundle(t *testing.T, format Format, ext string) string {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm", Generation: 3},
		Spec:       otilmv1alpha1.PlatformSpec{Version: bundleVer2180},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:              otilmv1alpha1.PlatformPhaseDegraded,
			ObservedGeneration: 2,
			ObservedVersion:    bundleVer2180,
			Conditions: []metav1.Condition{
				{Type: bundleDatabaseReady, Status: metav1.ConditionFalse, Reason: "ClusterNotReady", Message: "CNPG cluster not ready"},
			},
		},
	}
	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(plat).Build()
	c := NewCollector(&k8s.Client{Typed: fc, Scheme: scheme}, nil)

	var buf bytes.Buffer
	_, err = c.Collect(context.Background(), CollectOptions{AllNamespaces: true, Redact: true, Format: format}, &buf)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "bundle"+ext)
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))
	return path
}

func TestRead_ReconstructsSnapshotFromZip(t *testing.T) {
	t.Parallel()
	path := collectFixtureBundle(t, FormatZip, ".zip")

	snap, m, err := Read(path)
	require.NoError(t, err)
	assert.Equal(t, SchemaVersion, m.SchemaVersion)

	require.Len(t, snap.Platforms, 1)
	rs := snap.Platforms[0]
	assert.Equal(t, "ilm", rs.Name)
	assert.Equal(t, "ilm", rs.Namespace)
	assert.Equal(t, "Degraded", rs.Phase)
	assert.EqualValues(t, 3, rs.Generation)
	assert.EqualValues(t, 2, rs.ObservedGen)
	require.Len(t, rs.Conditions, 1)
	assert.Equal(t, bundleDatabaseReady, rs.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, rs.Conditions[0].Status)

	// SupportedVersions is populated so the version analyzer behaves identically.
	assert.NotEmpty(t, snap.SupportedVersions)
}

func TestRead_TGZBundle(t *testing.T) {
	t.Parallel()
	path := collectFixtureBundle(t, FormatTGZ, ".tgz")
	snap, _, err := Read(path)
	require.NoError(t, err)
	require.Len(t, snap.Platforms, 1)
	assert.Equal(t, "Degraded", snap.Platforms[0].Phase)
}

// TestRead_FindingsMatchLiveBuilder is the gold-standard determinism test:
// from identical fixture objects, findings produced by DefaultRegistry over a
// live snapshot must equal those produced over the offline bundle snapshot,
// finding-for-finding.
//
// Divergence probes seeded in this fixture (all must fire offline AND live):
//  1. Operator Deployment with ReadyReplicas=0 — proves OperatorReady is derived
//     from cluster/operator.yaml (ReadyReplicas>=1), not from versions.json.
//     A bug there would produce OperatorReady=true offline, suppressing findings.
//  2. A child Deployment of the Platform with spec.Replicas=2, status.ReadyReplicas=0 —
//     proves rs.Deployments is populated offline from
//     state/workloads/platform_<ns>_<name>.json, so the workload analyzer fires
//     its under-replication finding on both sides.
//  3. A capabilities Reporter with CNPG absent while the Platform uses managed DB
//     mode — proves snap.Capabilities is reconstructed from capabilities.json so the
//     capability analyzer fires on both sides.
//
// Design constraints that keep the comparison fair:
//   - No Secret refs: MissingRefs nil on both sides (bundles leave it nil by design).
//   - Generation == ObservedGeneration: reconcile analyzer silent on both.
//   - No Events seeded: event analyzer silent on both.
//   - Pods not seeded: live builder never populates rs.Pods; both nil.
func TestRead_FindingsMatchLiveBuilder(t *testing.T) {
	t.Parallel()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)

	// Divergence probe 3: Platform with managed DB mode requires CNPG.
	// The Reporter has CNPG absent, so capabilityAnalyzer must emit a finding.
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm", Generation: 1},
		Spec: otilmv1alpha1.PlatformSpec{
			Version:  bundleVer2180,
			Database: otilmv1alpha1.DatabaseSpec{Mode: "managed"},
		},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:              otilmv1alpha1.PlatformPhaseDegraded,
			ObservedGeneration: 1,
			ObservedVersion:    bundleVer2180,
			Conditions: []metav1.Condition{
				{Type: bundleDatabaseReady, Status: metav1.ConditionFalse, Reason: "ClusterNotReady", Message: "CNPG cluster not ready"},
			},
		},
	}

	// Connector Running; no refs.
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ilm", Generation: 1},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase:              otilmv1alpha1.ConnectorPhaseRunning,
			ObservedGeneration: 1,
		},
	}

	// Proxy Running; no SecretRef so both sides have nil SecretRefs.
	prxy := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ilm", Generation: 1},
		Status: otilmv1alpha1.ProxyStatus{
			Phase:              otilmv1alpha1.ProxyPhaseRunning,
			ObservedGeneration: 1,
		},
	}

	// Divergence probe 1: operator Deployment with 0 ready replicas.
	// OperatorReady must be false on both sides; versions.json must not override this.
	opDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8s.OperatorDeploymentName,
			Namespace: "ilm-operator-system",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "manager", Image: "ghcr.io/omnitrust/ilm-operator:v0.5.0"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 0},
	}

	// Divergence probe 2: under-replicated Deployment owned by the Platform.
	// Labels match DeploymentsForPlatform's selector so the live builder picks it up.
	replicas := int32(2)
	childDep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ilm-core",
			Namespace: "ilm",
			Labels: map[string]string{
				k8s.OperatorManagedByLabel: k8s.OperatorManagedByValue,
				k8s.OperatorPlatformLabel:  "ilm",
			},
		},
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 0},
	}

	cl := k8s.NewFakeClient(t, k8s.FakeClientOptions{
		Scheme:  scheme,
		Objects: []ctrlclient.Object{plat, conn, prxy, opDep, childDep},
	})

	// Divergence probe 3: Reporter with no deps present — CNPG absent triggers the
	// capability finding for the managed-DB Platform.
	reporter := newStubReporter(nil)

	// Live snapshot via the same Builder that the check command uses.
	liveSnap, err := analyze.BuildLive(context.Background(), cl, reporter, analyze.BuildOptions{AllNamespaces: true})
	require.NoError(t, err)

	// Sanity: live side must see the under-replicated Deployment.
	require.Len(t, liveSnap.Platforms, 1)
	assert.NotEmpty(t, liveSnap.Platforms[0].Deployments,
		"live builder must populate rs.Deployments for the Platform")
	assert.False(t, liveSnap.OperatorReady,
		"live snapshot must report OperatorReady=false for 0 ready replicas")
	assert.NotEmpty(t, liveSnap.Capabilities,
		"live snapshot must have Capabilities populated by the Reporter")

	liveFindings := analyze.DefaultRegistry().Run(liveSnap)

	// Offline snapshot: collect a bundle from the same objects and Reporter, then Read it.
	var buf bytes.Buffer
	_, err = NewCollector(cl, reporter).Collect(context.Background(), CollectOptions{AllNamespaces: true, Format: FormatZip}, &buf)
	require.NoError(t, err)
	bundlePath := filepath.Join(t.TempDir(), "bundle.zip")
	require.NoError(t, os.WriteFile(bundlePath, buf.Bytes(), 0o600))

	offlineSnap, _, err := Read(bundlePath)
	require.NoError(t, err)

	// Divergence 1 assertion: OperatorReady must match between snapshots.
	assert.Equal(t, liveSnap.OperatorReady, offlineSnap.OperatorReady,
		"OperatorReady must match between live and offline snapshots")

	// Divergence 2 assertion: offline Platform must have Deployments populated.
	require.Len(t, offlineSnap.Platforms, 1)
	assert.NotEmpty(t, offlineSnap.Platforms[0].Deployments,
		"offline reader must populate rs.Deployments for the Platform")

	// Divergence 3 assertion: Capabilities must be reconstructed from capabilities.json.
	assert.ElementsMatch(t, liveSnap.Capabilities, offlineSnap.Capabilities,
		"offline Capabilities must equal live Capabilities")

	offlineFindings := analyze.DefaultRegistry().Run(offlineSnap)

	// Gold-standard: the complete finding set must be identical.
	assert.ElementsMatch(t, liveFindings, offlineFindings,
		"offline bundle findings must equal live snapshot findings over the same fixtures")

	// Confirm the fixture exercises multiple analyzers on both sides.
	var sawPhase, sawCondition, sawWorkload, sawCapability bool
	for _, f := range liveFindings {
		switch {
		case f.Rule == "phase" && f.Severity == analyze.SeverityFail:
			sawPhase = true
		case f.Rule == "condition" && f.Severity == analyze.SeverityFail:
			sawCondition = true
		case f.Rule == "workload" && f.Severity == analyze.SeverityFail:
			sawWorkload = true
		case f.Rule == "capability" && f.Severity == analyze.SeverityFail:
			sawCapability = true
		}
	}
	assert.True(t, sawPhase, "Degraded phase must produce a fail finding")
	assert.True(t, sawCondition, "DatabaseReady=False must produce a fail finding")
	assert.True(t, sawWorkload, "under-replicated Deployment must produce a workload fail finding")
	assert.True(t, sawCapability, "absent CNPG with managed-DB Platform must produce a capability fail finding")
}

// TestRead_ConnectorAndProxyLogsAttached proves the offline reader attaches the
// connector/proxy pod logs the collector wrote (logs/<ns>_<kind>_<name>.log) to
// the reconstructed ResourceSnapshot.Logs, so the logsig analyzer scans them
// offline exactly as it would live.
func TestRead_ConnectorAndProxyLogsAttached(t *testing.T) {
	t.Parallel()
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ilm", Generation: 1},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning, ObservedGeneration: 1},
	}
	prxy := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ilm", Generation: 1},
		Status:     otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseRunning, ObservedGeneration: 1},
	}
	connPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c1-pod", Namespace: "ilm",
			Labels: map[string]string{"otilm.com/connector": "c1"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "connector"}}},
	}
	proxyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "p1-pod", Namespace: "ilm",
			Labels: map[string]string{"otilm.com/proxy": "p1"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "proxy"}}},
	}
	// NewFakeClient wires the fake clientset whose GetLogs returns canned content.
	cl := k8s.NewFakeClient(t, k8s.FakeClientOptions{
		Objects: []ctrlclient.Object{conn, prxy, connPod, proxyPod},
	})

	var buf bytes.Buffer
	_, err := NewCollector(cl, nil).Collect(context.Background(), CollectOptions{
		AllNamespaces: true, Format: FormatZip, IncludeLogs: true,
	}, &buf)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "bundle.zip")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	snap, _, err := Read(path)
	require.NoError(t, err)

	require.Len(t, snap.Connectors, 1)
	assert.NotEmpty(t, snap.Connectors[0].Logs, "connector snapshot must carry collected logs")
	require.Len(t, snap.Proxies, 1)
	assert.NotEmpty(t, snap.Proxies[0].Logs, "proxy snapshot must carry collected logs")
}

func TestRead_RejectsUnknownSchema(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.zip")
	writeMinimalZip(t, path, `{"schemaVersion":"999"}`)

	_, _, err := Read(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema")
}

func TestRead_MissingManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.zip")
	writeEmptyZip(t, path)
	_, _, err := Read(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manifest.json")
}

// collectFullBundle produces a bundle with a Platform, Connector, and Proxy.
func collectFullBundle(t *testing.T, format Format, ext string) string {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm", Generation: 1},
		Spec:       otilmv1alpha1.PlatformSpec{Version: bundleVer2180},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:              otilmv1alpha1.PlatformPhaseRunning,
			ObservedGeneration: 1,
			ObservedVersion:    bundleVer2180,
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue, Reason: "AllComponentsReady"},
			},
		},
	}
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "ilm", Generation: 1},
		Spec:       otilmv1alpha1.ConnectorSpec{},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase:              otilmv1alpha1.ConnectorPhaseRunning,
			ObservedGeneration: 1,
		},
	}
	prxy := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ilm", Generation: 1},
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: "proxy-token"},
		},
		Status: otilmv1alpha1.ProxyStatus{
			Phase:              otilmv1alpha1.ProxyPhaseRunning,
			ObservedGeneration: 1,
		},
	}
	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(plat, conn, prxy).Build()
	c := NewCollector(&k8s.Client{Typed: fc, Scheme: scheme}, nil)

	var buf bytes.Buffer
	_, err = c.Collect(context.Background(), CollectOptions{AllNamespaces: true, Format: format}, &buf)
	require.NoError(t, err)

	p := filepath.Join(t.TempDir(), "bundle"+ext)
	require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o600))
	return p
}

func TestRead_ConnectorAndProxy(t *testing.T) {
	t.Parallel()
	path := collectFullBundle(t, FormatZip, ".zip")

	snap, _, err := Read(path)
	require.NoError(t, err)

	require.Len(t, snap.Platforms, 1)
	assert.Equal(t, bundleRunning, snap.Platforms[0].Phase)

	require.Len(t, snap.Connectors, 1)
	assert.Equal(t, "c1", snap.Connectors[0].Name)
	assert.Equal(t, bundleRunning, snap.Connectors[0].Phase)
	assert.Equal(t, analyze.GVKConnector, snap.Connectors[0].GVK)

	require.Len(t, snap.Proxies, 1)
	assert.Equal(t, "p1", snap.Proxies[0].Name)
	assert.Equal(t, bundleRunning, snap.Proxies[0].Phase)
	assert.Equal(t, analyze.GVKProxy, snap.Proxies[0].GVK)
	// proxy-token secret ref must be preserved
	assert.Equal(t, []string{"proxy-token"}, snap.Proxies[0].SecretRefs)
}

func TestRead_TGZWithConnectorAndProxy(t *testing.T) {
	t.Parallel()
	path := collectFullBundle(t, FormatTGZ, ".tgz")

	snap, _, err := Read(path)
	require.NoError(t, err)
	require.Len(t, snap.Connectors, 1)
	require.Len(t, snap.Proxies, 1)
}

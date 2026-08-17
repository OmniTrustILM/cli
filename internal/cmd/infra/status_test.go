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

package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func newStatusFixtures(t *testing.T, objs ...ctrlclient.Object) (*k8s.Client, *capabilities.Reporter) {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	mapper := testrestmapper.TestOnlyStaticRESTMapper(s)
	c := &k8s.Client{
		Typed:  ctrlfake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build(),
		Scheme: s,
		Mapper: mapper,
	}
	return c, capabilities.NewReporter(opcap.New(mapper))
}

func TestRunStatus_TableShowsPlatformPhaseAndReadySummary(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status: otilmv1alpha1.PlatformStatus{
			Phase: otilmv1alpha1.PlatformPhaseRunning, ObservedVersion: infraVer2180,
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue},
				{Type: "Progressing", Status: metav1.ConditionFalse},
			},
		},
	}
	c, rep := newStatusFixtures(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}}))
	s := out.String()
	assert.Contains(t, s, infraPlatformName)
	assert.Contains(t, s, infraRunning)
	assert.Contains(t, s, infraVer2180)
	// Available=True and Progressing=False are BOTH healthy (Progressing is a
	// negative-polarity condition), so a reconciled platform reads 2/2.
	assert.Contains(t, s, "2/2")
}

func TestRunStatus_VerboseListsConnectorsAndProxies(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace}}
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: infraNamespace},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	prx := &otilmv1alpha1.Proxy{
		ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: infraNamespace},
		Status:     otilmv1alpha1.ProxyStatus{Phase: otilmv1alpha1.ProxyPhaseScaledDown},
	}
	c, rep := newStatusFixtures(t, plat, conn, prx)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}, Verbose: 1}))
	s := out.String()
	assert.Contains(t, s, "c1")
	assert.Contains(t, s, "p1")
	assert.Contains(t, s, "ScaledDown")
}

func TestRunStatus_JSONEmitsSnapshot(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning},
	}
	c, rep := newStatusFixtures(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = "json"
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}}))
	var snap analyze.Snapshot
	require.NoError(t, json.Unmarshal(out.Bytes(), &snap))
	require.Len(t, snap.Platforms, 1)
	assert.Equal(t, infraRunning, snap.Platforms[0].Phase)
}

func TestRunStatus_YAMLEmitsSnapshot(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning},
	}
	c, rep := newStatusFixtures(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = "yaml"
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}}))
	assert.Contains(t, out.String(), infraRunning)
}

func TestRunStatus_OperatorReadyLine(t *testing.T) {
	// No Platform objects; just verify the operator header line is present.
	c, rep := newStatusFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}}))
	assert.Contains(t, out.String(), "Operator:")
}

func TestNewStatusCommand_FlagsAndGroup(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	cmd := NewStatusCommand(o)
	assert.Equal(t, "status", cmd.Use)
	assert.Equal(t, string(cli.GroupInfrastructure), cmd.GroupID)
	assert.NotNil(t, cmd.Flags().Lookup("verbose"))
	assert.NotNil(t, cmd.Flags().Lookup("all-namespaces"))
	assert.NotNil(t, cmd.Flags().Lookup("watch"))
}

func TestAppendInfra(t *testing.T) {
	m := appendInfra(nil, "cnpg", infraHealthy)
	assert.Equal(t, infraHealthy, m[infraCNPG])
	m2 := appendInfra(m, "rabbitmq", infraReady)
	assert.Equal(t, infraHealthy, m2[infraCNPG])
	assert.Equal(t, infraReady, m2["infra:rabbitmq"])
}

func TestPhaseString(t *testing.T) {
	assert.Equal(t, infraHealthy, phaseString(map[string]any{infraPhaseKey: infraHealthy}))
	assert.Equal(t, "(status read)", phaseString(map[string]any{}))
	assert.Equal(t, "(status read)", phaseString(map[string]any{infraPhaseKey: ""}))
}

func TestRenderStatusTables_OperatorReady(t *testing.T) {
	snap := &analyze.Snapshot{OperatorReady: true, OperatorVersion: "v1.2.3"}
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, renderStatusTables(p, snap, 0))
	assert.Contains(t, out.String(), infraReady)
	assert.Contains(t, out.String(), "v1.2.3")
}

func TestRunStatus_VerboseTwoEnrichesManagedInfra(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning},
	}
	c, rep := newStatusFixtures(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	// Verbose=2 exercises the enrichManagedInfra branch inside runStatus.
	require.NoError(t, runStatus(context.Background(), c, rep, p, statusOptions{Namespaces: []string{infraNamespace}, Verbose: 2}))
	assert.Contains(t, out.String(), infraPlatformName)
}

func TestRenderStatusTables_NotReadyNoVersion(t *testing.T) {
	snap := &analyze.Snapshot{OperatorReady: false, OperatorVersion: ""}
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, renderStatusTables(p, snap, 0))
	assert.Contains(t, out.String(), "NotReady")
	assert.Contains(t, out.String(), "<none>")
}

func TestRenderResourceTable_Empty(t *testing.T) {
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, renderResourceTable(p, "CONNECTORS", nil))
	assert.Empty(t, out.String())
}

// foreignCRStatus builds a minimal unstructured object suitable for the dynamic
// fake client.  apiVersion must match the GVR (e.g. "postgresql.cnpg.io/v1").
func foreignCRStatus(apiVersion, kind, ns, name string, status map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"namespace": ns, "name": name},
		"status":     status,
	}}
	return u
}

func TestEnrichManagedInfra_DBManagedBranch(t *testing.T) {
	const ns, name = infraNamespace, infraPlatformName

	cnpg := foreignCRStatus(
		"postgresql.cnpg.io/v1", "Cluster", ns, name,
		map[string]any{infraPhaseKey: infraHealthy},
	)
	rmq := foreignCRStatus(
		"rabbitmq.com/v1beta1", "RabbitmqCluster", ns, name,
		map[string]any{infraPhaseKey: "AllReplicasReady"},
	)

	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{
		Dynamic: []runtime.Object{cnpg, rmq},
	})

	snap := &analyze.Snapshot{
		Platforms: []analyze.ResourceSnapshot{
			{
				Namespace: ns,
				Name:      name,
				SpecModes: capabilities.Modes{DBManaged: true, MessagingManaged: true},
			},
		},
	}

	enrichManagedInfra(context.Background(), c, snap)

	logs := snap.Platforms[0].Logs
	assert.Equal(t, infraHealthy, logs[infraCNPG])
	assert.Equal(t, "AllReplicasReady", logs["infra:rabbitmq"])
	// Keycloak was not managed, so its key must be absent.
	_, hasKeycloak := logs["infra:keycloak"]
	assert.False(t, hasKeycloak)
}

func TestEnrichManagedInfra_KeycloakBranch(t *testing.T) {
	const ns, name = "ns2", infraPlatformName

	kc := foreignCRStatus(
		"k8s.keycloak.org/v2alpha1", "Keycloak", ns, name,
		map[string]any{infraPhaseKey: infraReady},
	)

	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{
		Dynamic: []runtime.Object{kc},
	})

	snap := &analyze.Snapshot{
		Platforms: []analyze.ResourceSnapshot{
			{
				Namespace: ns,
				Name:      name,
				SpecModes: capabilities.Modes{KeycloakManaged: true},
			},
		},
	}

	enrichManagedInfra(context.Background(), c, snap)

	assert.Equal(t, infraReady, snap.Platforms[0].Logs["infra:keycloak"])
}

func TestWatchStatus_CancelReturnsNil(t *testing.T) {
	c, rep := newStatusFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// watchStatus uses a 2 s ticker internally; with a 200 ms context deadline it
	// runs one iteration and then returns nil on ctx.Done — never reaches the ticker.
	err := watchStatus(ctx, c, rep, p, statusOptions{Namespaces: []string{infraNamespace}})
	assert.NoError(t, err)
}

// TestNewStatusCommand_RunE_RendersFakeClient exercises the RunE closure via
// clientFn injection, confirming that runStatus is reached and renders output
// from the fake-backed client without error.
func TestNewStatusCommand_RunE_RendersFakeClient(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning, ObservedVersion: infraVer2180},
	}

	s, err := k8s.NewScheme()
	require.NoError(t, err)
	mapper := testrestmapper.TestOnlyStaticRESTMapper(s)
	fakeC := &k8s.Client{
		Typed:  ctrlfake.NewClientBuilder().WithScheme(s).WithObjects(plat).Build(),
		Scheme: s,
		Mapper: mapper,
	}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	opts := &statusOptions{clientFn: func() (*k8s.Client, error) { return fakeC, nil }}
	cmd := newStatusCommandFromOpts(o, opts)
	// --all-namespaces avoids calling Factory.Namespace() (Factory is nil in this test).
	require.NoError(t, cmd.Flags().Set("all-namespaces", "true"))

	require.NoError(t, cmd.RunE(cmd, []string{}))
	s2 := out.String()
	assert.Contains(t, s2, infraPlatformName)
	assert.Contains(t, s2, infraRunning)
}

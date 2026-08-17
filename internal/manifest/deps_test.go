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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// cannedManifests maps each pinned URL to a minimal-but-valid YAML manifest.
// These stand in for the real upstream releases so the tests run without a
// network and can assert exactly which URLs each dep fetches.
var cannedManifests = map[string]string{
	urlCertManager: "" +
		"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n" +
		"---\n" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: certificates.cert-manager.io\n" +
		"---\n" +
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cert-manager\n  namespace: cert-manager\n",

	urlCNPG: "" +
		"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cnpg-system\n" +
		"---\n" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: clusters.postgresql.cnpg.io\n" +
		"---\n" +
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cnpg-controller-manager\n  namespace: cnpg-system\n",

	urlRabbitMQCluster: "" +
		"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: rabbitmq-system\n" +
		"---\n" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: rabbitmqclusters.rabbitmq.com\n" +
		"---\n" +
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: rabbitmq-cluster-operator\n  namespace: rabbitmq-system\n",

	urlRabbitMQTopology: "" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: queues.rabbitmq.com\n" +
		"---\n" +
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: messaging-topology-operator\n  namespace: rabbitmq-system\n",

	urlKeycloakCRDKeycloak: "" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: keycloaks.k8s.keycloak.org\n",

	urlKeycloakCRDRealm: "" +
		"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: keycloakrealmimports.k8s.keycloak.org\n",

	// The resources file omits namespaces (published for `kubectl -n keycloak`)
	// and carries the operator Deployment with a single container.
	urlKeycloakResources: "" +
		"apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: keycloak-operator\n" +
		"---\n" +
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: keycloak-operator\nspec:\n  template:\n    spec:\n      containers:\n      - name: keycloak-operator\n        image: quay.io/keycloak/keycloak-operator:26.6.3\n",
}

// recordingFetch returns a depFetch stand-in that serves cannedManifests and
// records the URLs requested, in order.
func recordingFetch(t *testing.T) (fetch func(context.Context, string) ([]byte, error), fetched *[]string) {
	t.Helper()
	var urls []string
	f := func(_ context.Context, url string) ([]byte, error) {
		urls = append(urls, url)
		body, ok := cannedManifests[url]
		if !ok {
			return nil, fmt.Errorf("unexpected fetch URL: %s", url)
		}
		return []byte(body), nil
	}
	return f, &urls
}

// withFetch swaps depFetch for the duration of a test and restores it after.
func withFetch(t *testing.T, f func(context.Context, string) ([]byte, error)) {
	t.Helper()
	orig := depFetch
	depFetch = f
	t.Cleanup(func() { depFetch = orig })
}

func TestInstallDeps_All_FetchesPinnedURLs(t *testing.T) {
	fetch, fetched := recordingFetch(t)
	withFetch(t, fetch)

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	res, err := InstallDeps(context.Background(), a, nil)
	require.NoError(t, err)

	// Every pinned URL is fetched; RabbitMQ fetches BOTH of its URLs and
	// Keycloak fetches all three of its files.
	want := []string{
		urlCertManager,
		urlCNPG,
		urlRabbitMQCluster, urlRabbitMQTopology,
		urlKeycloakCRDKeycloak, urlKeycloakCRDRealm, urlKeycloakResources,
	}
	assert.Equal(t, want, *fetched, "deps must fetch their exact pinned URLs in order")

	// Each dep contributed its namespace/identifying objects.
	assert.Contains(t, res.Applied, "Namespace//cert-manager (dry-run)")
	assert.Contains(t, res.Applied, "Namespace//cnpg-system (dry-run)")
	assert.Contains(t, res.Applied, "Namespace//rabbitmq-system (dry-run)")
	assert.Contains(t, res.Applied, "Namespace//keycloak (dry-run)")

	// No object is applied twice.
	seen := map[string]int{}
	for _, id := range res.Applied {
		seen[id]++
	}
	for id, n := range seen {
		assert.Equalf(t, 1, n, "object %q applied %d times (want 1)", id, n)
	}
}

func TestInstallDeps_Only_CNPG(t *testing.T) {
	fetch, fetched := recordingFetch(t)
	withFetch(t, fetch)

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	res, err := InstallDeps(context.Background(), a, []capabilities.Dep{capabilities.DepCNPG})
	require.NoError(t, err)

	// Only the CNPG URL is fetched.
	assert.Equal(t, []string{urlCNPG}, *fetched)

	assert.Contains(t, res.Applied, "Namespace//cnpg-system (dry-run)")
	for _, id := range res.Applied {
		assert.NotContains(t, []string{
			"Namespace//cert-manager (dry-run)",
			"Namespace//rabbitmq-system (dry-run)",
			"Namespace//keycloak (dry-run)",
		}, id, "object from non-selected dep leaked into Applied: %s", id)
	}
}

func TestInstallDeps_DetectionOnly_FetchesNothing(t *testing.T) {
	var called bool
	withFetch(t, func(context.Context, string) ([]byte, error) {
		called = true
		return nil, nil
	})

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	_, err := InstallDeps(context.Background(), a, []capabilities.Dep{capabilities.DepGatewayAPI})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gateway-api")
	assert.False(t, called, "detection-only dep must error before any fetch")
}

func TestInstallDeps_UnknownDep_Errors(t *testing.T) {
	var called bool
	withFetch(t, func(context.Context, string) ([]byte, error) {
		called = true
		return nil, nil
	})

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	_, err := InstallDeps(context.Background(), a, []capabilities.Dep{capabilities.Dep("bogus")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
	assert.False(t, called, "unknown dep must error before any fetch")
}

func TestInstallDeps_FetchError_NamesDepAndURL(t *testing.T) {
	withFetch(t, func(_ context.Context, url string) ([]byte, error) {
		if url == urlCNPG {
			return nil, fmt.Errorf("boom")
		}
		return []byte(cannedManifests[url]), nil
	})

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	_, err := InstallDeps(context.Background(), a, []capabilities.Dep{capabilities.DepCNPG})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cnpg", "error must name the dep")
	assert.Contains(t, err.Error(), urlCNPG, "error must name the offending URL")
}

// TestInstallDeps_Keycloak_NamespaceEnvRBAC asserts the full Keycloak handling
// hermetically: it captures every object applied (via a DryRunClient Applier
// and a recording fetch), then inspects the actual unstructured objects the
// builder produced.
func TestInstallDeps_Keycloak_NamespaceEnvRBAC(t *testing.T) {
	fetch, fetched := recordingFetch(t)
	withFetch(t, fetch)

	crds, controller := keycloakObjects(t)

	// All three Keycloak files are fetched.
	assert.Equal(t, []string{urlKeycloakCRDKeycloak, urlKeycloakCRDRealm, urlKeycloakResources}, *fetched)

	// (1) The keycloak Namespace is applied first, ahead of the CRDs.
	require.NotEmpty(t, crds)
	assert.Equal(t, kindNamespace, crds[0].GetKind())
	assert.Equal(t, keycloakNamespace, crds[0].GetName())

	// (2) The operator Deployment has both all-namespaces env vars on its
	// container after processing.
	dep := findObj(controller, kindDeployment, keycloakOperatorSA)
	require.NotNil(t, dep, "keycloak-operator Deployment must be present")
	envKeys := operatorEnvKeys(t, dep)
	assert.Contains(t, envKeys, envKeycloakControllerNS)
	assert.Contains(t, envKeys, envKeycloakRealmImportNS)
	assert.Equal(t, envAllNamespaces, envKeys[envKeycloakControllerNS])
	assert.Equal(t, envAllNamespaces, envKeys[envKeycloakRealmImportNS])

	// (3) The 1 ClusterRole + 3 ClusterRoleBindings are present with the right
	// names, roleRefs and subject.
	role := findObj(controller, kindClusterRole, keycloakAllNsRole)
	require.NotNil(t, role, "keycloak-operator-allns-role ClusterRole must be present")

	assertBinding(t, controller, "keycloak-operator-allns-controller", keycloakControllerRoleRef)
	assertBinding(t, controller, "keycloak-operator-allns-realmimport", keycloakRealmImportRoleRef)
	assertBinding(t, controller, keycloakAllNsRole, keycloakAllNsRole)
}

// keycloakObjects runs the Keycloak builder and returns its (crds, controller)
// split. Kept separate so the assertions read cleanly.
func keycloakObjects(t *testing.T) (crds, controller []*unstructured.Unstructured) {
	t.Helper()
	crds, controller, err := buildKeycloak(context.Background())
	require.NoError(t, err)
	return crds, controller
}

// findObj returns the first object of the given kind/name, or nil.
func findObj(objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	for _, o := range objs {
		if o.GetKind() == kind && o.GetName() == name {
			return o
		}
	}
	return nil
}

// operatorEnvKeys reads the env of the first container of dep into a name→value map.
func operatorEnvKeys(t *testing.T, dep *unstructured.Unstructured) map[string]string {
	t.Helper()
	containers, found, err := unstructured.NestedSlice(dep.Object, keySpec, "template", keySpec, "containers")
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, containers)

	out := map[string]string{}
	for _, cAny := range containers {
		c, ok := cAny.(map[string]any)
		require.True(t, ok)
		env, _ := c["env"].([]any)
		for _, eAny := range env {
			e, ok := eAny.(map[string]any)
			require.True(t, ok)
			out[fmt.Sprint(e["name"])] = fmt.Sprint(e["value"])
		}
	}
	return out
}

// assertBinding verifies a ClusterRoleBinding by name: correct roleRef and the
// keycloak-operator ServiceAccount subject in the keycloak namespace.
func assertBinding(t *testing.T, objs []*unstructured.Unstructured, name, roleRef string) {
	t.Helper()
	b := findObj(objs, kindClusterRoleBinding, name)
	require.NotNilf(t, b, "ClusterRoleBinding %s must be present", name)

	gotRoleRef, _, _ := unstructured.NestedString(b.Object, "roleRef", "name")
	assert.Equalf(t, roleRef, gotRoleRef, "%s roleRef", name)
	gotKind, _, _ := unstructured.NestedString(b.Object, "roleRef", keyKind)
	assert.Equal(t, kindClusterRole, gotKind)

	subjects, found, _ := unstructured.NestedSlice(b.Object, "subjects")
	require.Truef(t, found, "%s must have subjects", name)
	require.Len(t, subjects, 1)
	subj, ok := subjects[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ServiceAccount", subj[keyKind])
	assert.Equal(t, keycloakOperatorSA, subj["name"])
	assert.Equal(t, keycloakNamespace, subj[keyNamespace])
}

// TestInstallDeps_DeterministicOrder asserts the deps are applied in the fixed
// cert-manager → cnpg → rabbitmq → keycloak order regardless of the order the
// caller lists them in --only.
func TestInstallDeps_DeterministicOrder(t *testing.T) {
	fetch, fetched := recordingFetch(t)
	withFetch(t, fetch)

	c := testClient(t)
	a := &Applier{Client: c, FieldManager: manifestILMCtl, DryRun: DryRunClient}

	// Caller lists them out of order.
	_, err := InstallDeps(context.Background(), a, []capabilities.Dep{
		capabilities.DepKeycloak, capabilities.DepCertManager, capabilities.DepCNPG,
	})
	require.NoError(t, err)

	// cert-manager fetched before cnpg before keycloak's files.
	certIdx := indexOf(*fetched, urlCertManager)
	cnpgIdx := indexOf(*fetched, urlCNPG)
	keycloakIdx := indexOf(*fetched, urlKeycloakCRDKeycloak)
	require.NotEqual(t, -1, certIdx)
	require.NotEqual(t, -1, cnpgIdx)
	require.NotEqual(t, -1, keycloakIdx)
	assert.True(t, certIdx < cnpgIdx && cnpgIdx < keycloakIdx,
		"deterministic order violated: %v", *fetched)
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// TestInstallDeps_DefaultsKeycloakNamespace confirms namespaced objects lacking
// a namespace are defaulted to keycloak while cluster-scoped objects are left
// untouched.
func TestInstallDeps_DefaultsKeycloakNamespace(t *testing.T) {
	fetch, _ := recordingFetch(t)
	withFetch(t, fetch)

	crds, controller := keycloakObjects(t)

	// The ServiceAccount (namespaced, published without a namespace) is defaulted.
	sa := findObj(controller, "ServiceAccount", keycloakOperatorSA)
	require.NotNil(t, sa)
	assert.Equal(t, keycloakNamespace, sa.GetNamespace())

	// CRDs stay cluster-scoped (no namespace).
	for _, c := range crds {
		if c.GetKind() == kindCRD {
			assert.Empty(t, c.GetNamespace(), "CRD %s must remain cluster-scoped", c.GetName())
		}
	}
}

// fetchWithKeycloakResources serves cannedManifests but overrides the Keycloak
// resources file with the supplied YAML, so a test can vary the operator layout.
func fetchWithKeycloakResources(resourcesYAML string) func(context.Context, string) ([]byte, error) {
	return func(_ context.Context, url string) ([]byte, error) {
		if url == urlKeycloakResources {
			return []byte(resourcesYAML), nil
		}
		body, ok := cannedManifests[url]
		if !ok {
			return nil, fmt.Errorf("unexpected fetch URL: %s", url)
		}
		return []byte(body), nil
	}
}

// operatorEnvCount counts how many times an env var name appears across the
// Deployment's containers (raw, not deduped) — used to prove idempotency.
func operatorEnvCount(t *testing.T, dep *unstructured.Unstructured, name string) int {
	t.Helper()
	containers, found, err := unstructured.NestedSlice(dep.Object, keySpec, "template", keySpec, "containers")
	require.NoError(t, err)
	require.True(t, found)
	n := 0
	for _, cAny := range containers {
		c, ok := cAny.(map[string]any)
		require.True(t, ok)
		env, _ := c["env"].([]any)
		for _, eAny := range env {
			e, ok := eAny.(map[string]any)
			require.True(t, ok)
			if fmt.Sprint(e["name"]) == name {
				n++
			}
		}
	}
	return n
}

// A fetched Keycloak manifest missing the operator Deployment means the pinned
// upstream layout changed; the builder must fail loudly rather than ship an
// unconfigured (namespace-scoped) operator.
func TestBuildKeycloak_MissingOperatorDeployment_Errors(t *testing.T) {
	withFetch(t, fetchWithKeycloakResources(
		"apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: keycloak-operator\n"))

	_, _, err := buildKeycloak(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), versionKeycloak)
}

// The env mutation is idempotent: a var already set upstream is not duplicated,
// and the missing companion var is still added.
func TestBuildKeycloak_EnvIdempotent(t *testing.T) {
	resources := fmt.Sprintf(
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: keycloak-operator\n"+
			"spec:\n  template:\n    spec:\n      containers:\n      - name: keycloak-operator\n"+
			"        image: quay.io/keycloak/keycloak-operator:26.6.3\n"+
			"        env:\n        - name: %s\n          value: %s\n",
		envKeycloakControllerNS, envAllNamespaces)
	withFetch(t, fetchWithKeycloakResources(resources))

	_, controller, err := buildKeycloak(context.Background())
	require.NoError(t, err)

	dep := findObj(controller, kindDeployment, keycloakOperatorSA)
	require.NotNil(t, dep)
	assert.Equal(t, 1, operatorEnvCount(t, dep, envKeycloakControllerNS),
		"a pre-existing env var must not be duplicated")
	keys := operatorEnvKeys(t, dep)
	assert.Equal(t, envAllNamespaces, keys[envKeycloakRealmImportNS],
		"the missing companion env var must still be added")
}

// Regression for the real upstream 26.6.3 layout: the operator Deployment ships
// both namespace env vars set to JOSDK_WATCH_CURRENT (watch own namespace only).
// The builder must overwrite them to JOSDK_ALL_NAMESPACES, not leave the
// upstream default in place, so the operator watches every namespace.
func TestBuildKeycloak_EnvOverwritesWatchCurrent(t *testing.T) {
	const watchCurrent = "JOSDK_WATCH_CURRENT"
	resources := fmt.Sprintf(
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: keycloak-operator\n"+
			"spec:\n  template:\n    spec:\n      containers:\n      - name: keycloak-operator\n"+
			"        image: quay.io/keycloak/keycloak-operator:26.6.3\n"+
			"        env:\n"+
			"        - name: %s\n          value: %s\n"+
			"        - name: %s\n          value: %s\n",
		envKeycloakControllerNS, watchCurrent,
		envKeycloakRealmImportNS, watchCurrent)
	withFetch(t, fetchWithKeycloakResources(resources))

	_, controller, err := buildKeycloak(context.Background())
	require.NoError(t, err)

	dep := findObj(controller, kindDeployment, keycloakOperatorSA)
	require.NotNil(t, dep)
	keys := operatorEnvKeys(t, dep)
	assert.Equal(t, envAllNamespaces, keys[envKeycloakControllerNS],
		"the upstream JOSDK_WATCH_CURRENT value must be overwritten to JOSDK_ALL_NAMESPACES")
	assert.Equal(t, envAllNamespaces, keys[envKeycloakRealmImportNS],
		"the upstream JOSDK_WATCH_CURRENT value must be overwritten to JOSDK_ALL_NAMESPACES")
	assert.Equal(t, 1, operatorEnvCount(t, dep, envKeycloakControllerNS),
		"overwriting must not duplicate the key")
	assert.Equal(t, 1, operatorEnvCount(t, dep, envKeycloakRealmImportNS),
		"overwriting must not duplicate the key")
}

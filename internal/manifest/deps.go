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

// Package manifest — deps.go fetches upstream-operator install manifests from
// pinned release URLs at install time and applies them through the Applier.
// Nothing is vendored: a version bump is a one-line URL/constant change here.
package manifest

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// Pinned upstream versions. Bumping a dependency is a one-line change here plus
// the matching URL below; no manifests are vendored, so nothing goes stale.
const (
	versionCertManager      = "v1.20.2"
	versionCNPG             = "1.29.1"
	versionRabbitMQ         = "v2.21.0"
	versionRabbitMQTopology = "v1.19.2"
	versionKeycloak         = "26.6.3"
)

// Pinned upstream source URLs, kept next to the versions they encode.
const (
	urlCertManager = "https://github.com/cert-manager/cert-manager/releases/download/v1.20.2/cert-manager.yaml"

	urlCNPG = "https://github.com/cloudnative-pg/cloudnative-pg/releases/download/v1.29.1/cnpg-1.29.1.yaml"

	urlRabbitMQCluster  = "https://github.com/rabbitmq/cluster-operator/releases/download/v2.21.0/cluster-operator.yml"
	urlRabbitMQTopology = "https://github.com/rabbitmq/messaging-topology-operator/releases/download/v1.19.2/messaging-topology-operator-with-certmanager.yaml"

	keycloakBase           = "https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.6.3/kubernetes"
	urlKeycloakCRDKeycloak = keycloakBase + "/keycloaks.k8s.keycloak.org-v1.yml"
	urlKeycloakCRDRealm    = keycloakBase + "/keycloakrealmimports.k8s.keycloak.org-v1.yml"
	urlKeycloakResources   = keycloakBase + "/kubernetes.yml"
)

// keycloakNamespace is where the keycloak-operator and its objects live; the
// upstream manifests are published for `kubectl apply -n keycloak`.
const keycloakNamespace = "keycloak"

// Object kinds referenced across builders (deduped for consistency and to keep
// string literals in one place).
const (
	kindDeployment         = "Deployment"
	kindClusterRole        = "ClusterRole"
	kindClusterRoleBinding = "ClusterRoleBinding"
	kindNamespace          = "Namespace"
	kindCRD                = "CustomResourceDefinition"
	kindCatalogSource      = "CatalogSource"

	apiRBAC = "rbac.authorization.k8s.io/v1"
	apiCore = "v1"

	rbacGroup = "rbac.authorization.k8s.io"
	groupApps = "apps"

	// Unstructured object keys the builders assemble manifests from.
	keyAPIVersion = "apiVersion"
	keyName       = "name"
	keyKind       = "kind"
	keyMetadata   = "metadata"
	keyNamespace  = "namespace"
	keySpec       = "spec"

	keycloakOperatorSA         = "keycloak-operator"
	keycloakAllNsRole          = "keycloak-operator-allns-role"
	verbGet                    = "get"
	verbList                   = "list"
	verbWatch                  = "watch"
	verbCreate                 = "create"
	verbDelete                 = "delete"
	verbPatch                  = "patch"
	verbUpdate                 = "update"
	envKeycloakControllerNS    = "QUARKUS_OPERATOR_SDK_CONTROLLERS_KEYCLOAKCONTROLLER_NAMESPACES"
	envKeycloakRealmImportNS   = "QUARKUS_OPERATOR_SDK_CONTROLLERS_KEYCLOAKREALMIMPORTCONTROLLER_NAMESPACES"
	envAllNamespaces           = "JOSDK_ALL_NAMESPACES"
	keycloakControllerRoleRef  = "keycloakcontroller-cluster-role"
	keycloakRealmImportRoleRef = "keycloakrealmimportcontroller-cluster-role"
)

// depFetch is the fetch seam. It defaults to the package-level Fetch and is
// overridden in tests to serve canned manifests without network access.
var depFetch = Fetch

// SetDepFetchForTest overrides the dependency-manifest fetch seam with fn and
// returns a restore function. It exists so tests in dependent packages can
// exercise InstallDeps hermetically without network access; production code
// must not call it.
func SetDepFetchForTest(fn func(ctx context.Context, ref string) ([]byte, error)) (restore func()) {
	prev := depFetch
	depFetch = fn
	return func() { depFetch = prev }
}

// depSpec describes one installable dependency: its enum value, pinned version,
// the URLs it reads, and the builder that turns fetched bytes into an ordered
// (crds, controller) object split for ApplyOrdered.
type depSpec struct {
	dep     capabilities.Dep
	version string
	urls    []string
	build   func(ctx context.Context) (crds, controller []*unstructured.Unstructured, err error)
}

// installableDeps lists the deps the CLI can install, in deterministic order.
// DepGatewayAPI and DepServiceMonitor are detection-only and are absent here;
// InstallDeps returns an error if asked to install either.
var installableDeps = []depSpec{
	{capabilities.DepCertManager, versionCertManager, []string{urlCertManager}, buildCertManager},
	{capabilities.DepCNPG, versionCNPG, []string{urlCNPG}, buildCNPG},
	{capabilities.DepRabbitMQ, versionRabbitMQ, []string{urlRabbitMQCluster, urlRabbitMQTopology}, buildRabbitMQ},
	{capabilities.DepKeycloak, versionKeycloak, []string{urlKeycloakCRDKeycloak, urlKeycloakCRDRealm, urlKeycloakResources}, buildKeycloak},
}

// fetchSplit fetches one URL and splits it into unstructured objects, wrapping
// any error with the URL for actionable diagnostics.
func fetchSplit(ctx context.Context, url string) ([]*unstructured.Unstructured, error) {
	raw, err := depFetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	objs, err := Split(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return objs, nil
}

// buildCertManager fetches the single cert-manager release manifest. It carries
// its own CRDs, so ApplyOrdered separates them from the rest.
func buildCertManager(ctx context.Context) (crds, controller []*unstructured.Unstructured, err error) {
	objs, err := fetchSplit(ctx, urlCertManager)
	if err != nil {
		return nil, nil, err
	}
	crds, controller = partitionCRDs(objs)
	return crds, controller, nil
}

// buildCNPG fetches the single CNPG release manifest. Its CRDs are large and
// rely on server-side apply, which the Applier already uses.
func buildCNPG(ctx context.Context) (crds, controller []*unstructured.Unstructured, err error) {
	objs, err := fetchSplit(ctx, urlCNPG)
	if err != nil {
		return nil, nil, err
	}
	crds, controller = partitionCRDs(objs)
	return crds, controller, nil
}

// buildRabbitMQ fetches the cluster-operator manifest first, then the
// messaging-topology-operator manifest, and applies the combined set. The
// topology operator depends on cert-manager, which the user installs first.
func buildRabbitMQ(ctx context.Context) (crds, controller []*unstructured.Unstructured, err error) {
	cluster, err := fetchSplit(ctx, urlRabbitMQCluster)
	if err != nil {
		return nil, nil, err
	}
	topology, err := fetchSplit(ctx, urlRabbitMQTopology)
	if err != nil {
		return nil, nil, err
	}
	crds, controller = partitionCRDs(append(cluster, topology...))
	return crds, controller, nil
}

// partitionCRDs splits objects into CustomResourceDefinitions and everything
// else, preserving order within each group so ApplyOrdered can install CRDs
// first and wait for them to become Established.
func partitionCRDs(objs []*unstructured.Unstructured) (crds, rest []*unstructured.Unstructured) {
	for _, o := range objs {
		if o.GetKind() == kindCRD {
			crds = append(crds, o)
			continue
		}
		rest = append(rest, o)
	}
	return crds, rest
}

// buildKeycloak assembles the Keycloak install set. Keycloak is the complex
// dependency: its manifests are published for `kubectl apply -n keycloak`, so
// the CLI reconstructs that context offline. Order returned to ApplyOrdered:
// the Namespace and CRDs go in the crds slice (applied first, before namespaced
// objects and before CRs), and the operator/RBAC objects go in controller.
//
// Steps:
//  1. Create the keycloak Namespace.
//  2. Fetch the two CRD files and the resources file; default the namespace on
//     namespaced objects that lack one.
//  3. Flip the keycloak-operator Deployment to watch all namespaces by adding
//     two env vars to each container.
//  4. Append the cluster-wide RBAC that all-namespaces operation needs.
func buildKeycloak(ctx context.Context) (crds, controller []*unstructured.Unstructured, err error) {
	ns := newKeycloakNamespace()

	crdKeycloak, err := fetchSplit(ctx, urlKeycloakCRDKeycloak)
	if err != nil {
		return nil, nil, err
	}
	crdRealm, err := fetchSplit(ctx, urlKeycloakCRDRealm)
	if err != nil {
		return nil, nil, err
	}
	resources, err := fetchSplit(ctx, urlKeycloakResources)
	if err != nil {
		return nil, nil, err
	}

	all := append(append(crdKeycloak, crdRealm...), resources...)
	defaultKeycloakNamespace(all)
	if err := mutateKeycloakOperatorEnv(all); err != nil {
		return nil, nil, err
	}

	crdObjs, rest := partitionCRDs(all)
	// Namespace before CRDs so it exists ahead of any namespaced object; the
	// RBAC follows the operator objects.
	crds = append([]*unstructured.Unstructured{ns}, crdObjs...)
	controller = append(rest, keycloakAllNamespacesRBAC()...)
	return crds, controller, nil
}

// newKeycloakNamespace constructs the keycloak Namespace object.
func newKeycloakNamespace() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion: apiCore,
		keyKind:       kindNamespace,
		keyMetadata:   map[string]any{keyName: keycloakNamespace},
	}}
}

// defaultKeycloakNamespace sets metadata.namespace on namespaced objects that
// lack one. Cluster-scoped kinds (CRDs, ClusterRole/Binding) are left alone.
func defaultKeycloakNamespace(objs []*unstructured.Unstructured) {
	for _, o := range objs {
		if isClusterScoped(o.GetKind()) {
			continue
		}
		if o.GetNamespace() == "" {
			o.SetNamespace(keycloakNamespace)
		}
	}
}

// isClusterScoped reports whether a kind is applied without a namespace.
func isClusterScoped(kind string) bool {
	switch kind {
	case kindCRD, kindClusterRole, kindClusterRoleBinding, kindNamespace:
		return true
	default:
		return false
	}
}

// mutateKeycloakOperatorEnv adds the all-namespaces env vars to every container
// of the keycloak-operator Deployment, appending only keys not already present.
// It returns an error if the expected Deployment is absent: a missing operator
// Deployment means the pinned upstream manifest changed its layout, and silently
// shipping an unconfigured (namespace-scoped) operator would break managed
// Keycloak in every other namespace — fail loudly instead.
func mutateKeycloakOperatorEnv(objs []*unstructured.Unstructured) error {
	for _, o := range objs {
		if o.GetKind() == kindDeployment && o.GetName() == keycloakOperatorSA {
			return addAllNamespacesEnv(o)
		}
	}
	return fmt.Errorf("keycloak: expected Deployment %q not found in the fetched manifest; the pinned Keycloak version (%s) may have changed its layout",
		keycloakOperatorSA, versionKeycloak)
}

// addAllNamespacesEnv forces the two watch-all-namespaces env vars on each
// container in the Deployment's pod template. The upstream Keycloak manifest
// ships these keys set to JOSDK_WATCH_CURRENT (watch only the operator's own
// namespace); a plain append-if-absent would leave that default in place and
// silently ship a namespace-scoped operator. This upserts instead: an existing
// key has its value overwritten to JOSDK_ALL_NAMESPACES, and a missing key is
// appended.
func addAllNamespacesEnv(dep *unstructured.Unstructured) error {
	containers, found, err := unstructured.NestedSlice(dep.Object, keySpec, "template", keySpec, "containers")
	if err != nil {
		return fmt.Errorf("keycloak: read operator containers: %w", err)
	}
	if !found || len(containers) == 0 {
		return fmt.Errorf("keycloak: operator Deployment %q has no containers to configure", keycloakOperatorSA)
	}
	for i := range containers {
		c, ok := containers[i].(map[string]any)
		if !ok {
			continue
		}
		env, _ := c["env"].([]any)
		env = upsertEnv(env, envKeycloakControllerNS, envAllNamespaces)
		env = upsertEnv(env, envKeycloakRealmImportNS, envAllNamespaces)
		c["env"] = env
		containers[i] = c
	}
	if err := unstructured.SetNestedSlice(dep.Object, containers, keySpec, "template", keySpec, "containers"); err != nil {
		return fmt.Errorf("keycloak: set operator containers: %w", err)
	}
	return nil
}

// upsertEnv sets name=value in env: if name already exists its value is
// overwritten, otherwise a new entry is appended. It never duplicates a key.
func upsertEnv(env []any, name, value string) []any {
	for _, e := range env {
		if m, ok := e.(map[string]any); ok && m[keyName] == name {
			m["value"] = value
			return env
		}
	}
	return append(env, map[string]any{keyName: name, "value": value})
}

// keycloakAllNamespacesRBAC synthesizes the cluster-wide RBAC that the
// keycloak-operator needs when watching all namespaces.
//
// The ClusterRole rules mirror the published keycloak-operator operational role
// for the pinned Keycloak version (26.6.3) and should be re-verified against the
// upstream role on any Keycloak version bump.
func keycloakAllNamespacesRBAC() []*unstructured.Unstructured {
	role := &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion: apiRBAC,
		keyKind:       kindClusterRole,
		keyMetadata:   map[string]any{keyName: keycloakAllNsRole},
		"rules": []any{
			policyRule([]string{groupApps}, []string{"statefulsets"}, verbGet, verbList, verbWatch, verbCreate, verbDelete, verbPatch, verbUpdate),
			policyRule([]string{""}, []string{"configmaps"}, verbGet, verbList, verbWatch),
			policyRule([]string{""}, []string{"secrets", "services"}, verbGet, verbList, verbWatch, verbCreate, verbDelete, verbPatch, verbUpdate),
			policyRule([]string{""}, []string{"pods"}, verbList),
			policyRule([]string{""}, []string{"pods/log"}, verbGet),
			policyRule([]string{"batch"}, []string{"jobs"}, verbGet, verbList, verbWatch, verbCreate, verbDelete, verbPatch, verbUpdate),
			policyRule([]string{"networking.k8s.io"}, []string{"ingresses"}, verbGet, verbList, verbWatch, verbCreate, verbDelete, verbPatch, verbUpdate),
			policyRule([]string{"monitoring.coreos.com"}, []string{"servicemonitors"}, verbGet, verbList, verbWatch, verbCreate, verbDelete, verbPatch, verbUpdate),
		},
	}}
	return []*unstructured.Unstructured{
		role,
		clusterRoleBinding("keycloak-operator-allns-controller", keycloakControllerRoleRef),
		clusterRoleBinding("keycloak-operator-allns-realmimport", keycloakRealmImportRoleRef),
		clusterRoleBinding(keycloakAllNsRole, keycloakAllNsRole),
	}
}

// policyRule builds one rbac PolicyRule as an unstructured map.
func policyRule(apiGroups, resources []string, verbs ...string) map[string]any {
	return map[string]any{
		"apiGroups": toAnySlice(apiGroups),
		"resources": toAnySlice(resources),
		"verbs":     toAnySlice(verbs),
	}
}

// clusterRoleBinding builds a ClusterRoleBinding named name that binds the
// keycloak-operator ServiceAccount (ns keycloak) to the ClusterRole roleRef.
func clusterRoleBinding(name, roleRef string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion: apiRBAC,
		keyKind:       kindClusterRoleBinding,
		keyMetadata:   map[string]any{keyName: name},
		"roleRef": map[string]any{
			"apiGroup": rbacGroup,
			keyKind:    kindClusterRole,
			keyName:    roleRef,
		},
		"subjects": []any{
			map[string]any{
				keyKind:      "ServiceAccount",
				"apiGroup":   "",
				keyName:      keycloakOperatorSA,
				keyNamespace: keycloakNamespace,
			},
		},
	}}
}

// toAnySlice converts a []string into the []any shape unstructured requires.
func toAnySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// InstallDeps applies the selected upstream operators through the supplied
// Applier, fetching each dependency's pinned manifests at install time. When
// only is empty every installable dep is applied, in deterministic order
// (cert-manager, cnpg, rabbitmq, keycloak). When only is non-empty, only the
// named subset is applied (still in deterministic order, deduped). An unknown
// or detection-only dep in only returns an error before any fetch or apply is
// attempted.
func InstallDeps(ctx context.Context, a *Applier, only []capabilities.Dep) (ApplyResult, error) {
	selected, err := selectDeps(only)
	if err != nil {
		return ApplyResult{}, err
	}

	var res ApplyResult
	for _, s := range selected {
		crds, controller, berr := s.build(ctx)
		if berr != nil {
			return res, fmt.Errorf("install %s: %w", s.dep, berr)
		}
		r, aerr := a.ApplyOrdered(ctx, crds, controller)
		res.merge(r)
		if aerr != nil {
			return res, fmt.Errorf("install %s: apply failed: %w", s.dep, aerr)
		}
	}
	return res, nil
}

// selectDeps resolves the requested subset against installableDeps, returning
// an error for an unknown or detection-only dep before any fetch/apply happens.
func selectDeps(only []capabilities.Dep) ([]depSpec, error) {
	if len(only) == 0 {
		return installableDeps, nil
	}
	known := make(map[capabilities.Dep]bool, len(installableDeps))
	for _, s := range installableDeps {
		known[s.dep] = true
	}
	wanted := make(map[capabilities.Dep]bool, len(only))
	for _, d := range only {
		if !known[d] {
			return nil, fmt.Errorf("dependency %q is detection-only and cannot be installed by the CLI", d)
		}
		wanted[d] = true
	}
	var selected []depSpec
	for _, s := range installableDeps {
		if wanted[s.dep] {
			selected = append(selected, s)
		}
	}
	return selected, nil
}

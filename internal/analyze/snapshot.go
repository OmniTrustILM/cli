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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// ResourceSnapshot bundles one ILM custom resource with its workload, event and
// log context. It is the unit every rule iterates over. JSON tags are the bundle
// on-disk contract; Logs is excluded from JSON (logs are stored as bundle files).
type ResourceSnapshot struct {
	// GVK is the canonical "Kind.group/version" string, e.g. "Platform.otilm.com/v1alpha1".
	GVK             string             `json:"gvk"`
	Namespace       string             `json:"namespace"`
	Name            string             `json:"name"`
	Phase           string             `json:"phase"`
	ObservedVersion string             `json:"observedVersion,omitempty"`
	ObservedGen     int64              `json:"observedGeneration"`
	Generation      int64              `json:"generation"`
	Conditions      []metav1.Condition `json:"conditions"`
	// SpecModes carries the db/messaging/keycloak/edge/tls choices extracted from
	// the CR spec, so the capability rule can demand the right upstream operators.
	SpecModes   capabilities.Modes  `json:"specModes"`
	SecretRefs  []string            `json:"secretRefs,omitempty"`
	IssuerRefs  []string            `json:"issuerRefs,omitempty"`
	Deployments []appsv1.Deployment `json:"deployments,omitempty"`
	Pods        []corev1.Pod        `json:"pods,omitempty"`
	Events      []corev1.Event      `json:"events,omitempty"`
	// Logs maps a component name to a log tail; populated only when logs were
	// collected. Not serialized (bundle logs live as separate files).
	Logs map[string]string `json:"-"`
}

// ResourceRef returns the canonical "Kind/namespace/name" string used as the
// Finding.Resource field, derived from the GVK and object coordinates.
func (r ResourceSnapshot) ResourceRef() string {
	kind := r.GVK
	if i := indexByte(kind, '.'); i > 0 {
		kind = kind[:i]
	}
	return kind + "/" + r.Namespace + "/" + r.Name
}

// indexByte is a dependency-free bytes.IndexByte equivalent to keep this
// file's import set minimal.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// Snapshot is the single input both the live check and the offline bundle
// analyze feed. The live builder and the bundle reader must produce identical
// shapes so DefaultRegistry yields identical findings.
type Snapshot struct {
	ClientVersion   string                `json:"clientVersion"`
	OperatorVersion string                `json:"operatorVersion"`
	OperatorReady   bool                  `json:"operatorReady"`
	Capabilities    []capabilities.Result `json:"capabilities"`
	Platforms       []ResourceSnapshot    `json:"platforms"`
	Connectors      []ResourceSnapshot    `json:"connectors"`
	Proxies         []ResourceSnapshot    `json:"proxies"`
	// MissingRefs lists pre-resolved absent secret/issuer/configmap references
	// (kind/namespace/name). The live builder pre-resolves these; a bundle leaves
	// them nil.
	MissingRefs       []string `json:"missingRefs,omitempty"`
	SupportedVersions []string `json:"supportedVersions"`
}

// Resources returns every ResourceSnapshot in a stable order
// (Platforms, then Connectors, then Proxies) so rules iterate deterministically.
func (s *Snapshot) Resources() []ResourceSnapshot {
	out := make([]ResourceSnapshot, 0, len(s.Platforms)+len(s.Connectors)+len(s.Proxies))
	out = append(out, s.Platforms...)
	out = append(out, s.Connectors...)
	out = append(out, s.Proxies...)
	return out
}

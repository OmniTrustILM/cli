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

// Package capabilities reports which upstream operators the cluster serves. It
// wraps the operator's generic pkg/capabilities.Detector (a GroupKind probe over a
// RESTMapper) and supplies the CLI-side catalog of the deps ILM's managed modes
// and edge/TLS choices depend on.
package capabilities

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
)

// Dep identifies one upstream operator the CLI detects or installs.
type Dep string

// Upstream operator identifiers (also the --only deps install tokens).
const (
	DepCertManager    Dep = "cert-manager"
	DepCNPG           Dep = "cnpg"
	DepRabbitMQ       Dep = "rabbitmq"
	DepKeycloak       Dep = "keycloak"
	DepGatewayAPI     Dep = "gateway-api"
	DepServiceMonitor Dep = "servicemonitor"
)

// depOrder is the stable iteration order for Detect() with no explicit deps.
var depOrder = []Dep{
	DepCertManager, DepCNPG, DepRabbitMQ, DepKeycloak, DepGatewayAPI, DepServiceMonitor,
}

// ProbeGroupKinds is the canonical GroupKind probed per Dep. The probe is by
// GroupKind (any served version), matching the operator detector's contract.
var ProbeGroupKinds = map[Dep]schema.GroupKind{
	DepCertManager:    {Group: "cert-manager.io", Kind: "Certificate"},
	DepCNPG:           {Group: "postgresql.cnpg.io", Kind: "Cluster"},
	DepRabbitMQ:       {Group: "rabbitmq.com", Kind: "RabbitmqCluster"},
	DepKeycloak:       {Group: "k8s.keycloak.org", Kind: "Keycloak"},
	DepGatewayAPI:     {Group: "gateway.networking.k8s.io", Kind: "Gateway"},
	DepServiceMonitor: {Group: "monitoring.coreos.com", Kind: "ServiceMonitor"},
}

// Result is one Dep's detection outcome. Err is non-nil only for transient
// discovery failures; a not-installed Dep is Present=false with Err=nil.
type Result struct {
	Dep     Dep   `json:"dep"`
	Present bool  `json:"present"`
	Err     error `json:"-"`
}

// Reporter detects upstream-operator presence via the operator's Detector.
type Reporter struct {
	d *opcap.Detector
}

// NewReporter wraps an operator capability Detector.
func NewReporter(d *opcap.Detector) *Reporter { return &Reporter{d: d} }

// Present reports whether a single Dep is served. An unknown Dep is an error.
func (r *Reporter) Present(dep Dep) (bool, error) {
	gk, ok := ProbeGroupKinds[dep]
	if !ok {
		return false, fmt.Errorf("capabilities: unknown dependency %q", dep)
	}
	return r.d.Available(gk)
}

// Detect probes the given deps (empty => all, in catalog order) and returns one
// Result each. An unknown Dep yields a Result whose Err describes it.
func (r *Reporter) Detect(deps ...Dep) []Result {
	if len(deps) == 0 {
		deps = depOrder
	}
	out := make([]Result, 0, len(deps))
	for _, dep := range deps {
		present, err := r.Present(dep)
		out = append(out, Result{Dep: dep, Present: present, Err: err})
	}
	return out
}

// Modes captures the Platform spec choices that imply upstream-operator deps.
type Modes struct {
	DBManaged        bool   `json:"dbManaged"`
	MessagingManaged bool   `json:"messagingManaged"`
	KeycloakManaged  bool   `json:"keycloakManaged"`
	Edge             string `json:"edge"`      // ingress|gatewayAPI
	TLSSource        string `json:"tlsSource"` // internal|letsEncrypt|issuerRef|secret
}

// RequiredFor returns the deps a chosen set of Platform modes needs, in a stable
// order (db, messaging, keycloak, edge, tls). cert-manager is required for the
// cert-managed edge TLS sources (letsEncrypt, issuerRef).
func RequiredFor(modes Modes) []Dep {
	var out []Dep
	if modes.DBManaged {
		out = append(out, DepCNPG)
	}
	if modes.MessagingManaged {
		out = append(out, DepRabbitMQ)
	}
	if modes.KeycloakManaged {
		out = append(out, DepKeycloak)
	}
	if modes.Edge == "gatewayAPI" {
		out = append(out, DepGatewayAPI)
	}
	switch modes.TLSSource {
	case "letsEncrypt", "issuerRef":
		out = append(out, DepCertManager)
	}
	return out
}

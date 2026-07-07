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
	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// modeManaged is the operator's managed-mode literal shared across all three
// infrastructure mode checks (database, messaging, keycloak).
const modeManaged = "managed"

// PlatformModes extracts the db/messaging/keycloak/edge/tls choices from a
// Platform spec. Exported so the bundle reader extracts identically offline.
func PlatformModes(p *otilmv1alpha1.Platform) capabilities.Modes {
	m := capabilities.Modes{
		DBManaged:        p.Spec.Database.Mode == modeManaged,
		MessagingManaged: p.Spec.Messaging.Mode == modeManaged,
	}
	if p.Spec.Keycloak != nil && p.Spec.Keycloak.Mode == modeManaged {
		m.KeycloakManaged = true
	}
	if e := p.Spec.Edge; e != nil && e.Enabled {
		m.Edge = e.Type
		if e.TLS != nil {
			m.TLSSource = e.TLS.Source
		}
	}
	return m
}

// refCollector accumulates Secret and Issuer references from a Platform spec.
type refCollector struct {
	secrets []string
	issuers []string
}

func (rc *refCollector) addSecret(v string) {
	if v != "" {
		rc.secrets = append(rc.secrets, v)
	}
}

func (rc *refCollector) addSecretPtr(v *string) {
	if v != nil {
		rc.addSecret(*v)
	}
}

func (rc *refCollector) addIssuer(v string) {
	if v != "" {
		rc.issuers = append(rc.issuers, v)
	}
}

func (rc *refCollector) collectInfraRefs(p *otilmv1alpha1.Platform) {
	if c := p.Spec.Database.Credentials; c != nil {
		rc.addSecret(c.SecretRef)
	}
	if c := p.Spec.Messaging.Credentials; c != nil {
		rc.addSecret(c.SecretRef)
	}
	if pr := p.Spec.Provisioning; pr != nil {
		rc.addSecret(pr.APIKeySecretRef)
	}
}

func (rc *refCollector) collectEdgeRefs(p *otilmv1alpha1.Platform) {
	e := p.Spec.Edge
	if e == nil || !e.Enabled || e.TLS == nil {
		return
	}
	rc.addSecretPtr(e.TLS.SecretRef)
	if e.TLS.IssuerRef != nil {
		rc.addIssuer(e.TLS.IssuerRef.Name)
	}
}

func (rc *refCollector) collectRegisterAdminRefs(p *otilmv1alpha1.Platform) {
	ra := p.Spec.RegisterAdmin
	if ra == nil {
		return
	}
	if cert := ra.Certificate; cert != nil {
		rc.addSecretPtr(cert.SecretRef)
		if cert.IssuerRef != nil {
			rc.addIssuer(cert.IssuerRef.Name)
		}
	}
	if pw := ra.Password; pw != nil {
		rc.addSecret(pw.SecretRef)
	}
}

// PlatformRefs returns the non-empty Secret and Issuer references a Platform
// spec carries, so the reference rule can flag absent ones.
func PlatformRefs(p *otilmv1alpha1.Platform) (secretRefs, issuerRefs []string) {
	rc := &refCollector{}
	rc.collectInfraRefs(p)
	rc.collectEdgeRefs(p)
	rc.collectRegisterAdminRefs(p)
	return rc.secrets, rc.issuers
}

// ConnectorRefs returns the Secret references a Connector spec carries.
// A Connector spec carries no embedded Secret references; returns nil (kept for
// shape parity and forward extension).
func ConnectorRefs(_ *otilmv1alpha1.Connector) []string { return nil }

// ProxyRefs returns the Secret references a Proxy spec carries (its
// config-token Secret).
func ProxyRefs(p *otilmv1alpha1.Proxy) []string {
	if p.Spec.ConfigTokenSecretRef.Name == "" {
		return nil
	}
	return []string{p.Spec.ConfigTokenSecretRef.Name}
}

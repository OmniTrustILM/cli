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
	"testing"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func strptr(s string) *string { return &s }

func TestPlatformModes(t *testing.T) {
	t.Parallel()
	p := &otilmv1alpha1.Platform{
		Spec: otilmv1alpha1.PlatformSpec{
			Database:  otilmv1alpha1.DatabaseSpec{Mode: analyzeManaged},
			Messaging: otilmv1alpha1.MessagingSpec{Mode: analyzeExternal},
			Keycloak:  &otilmv1alpha1.KeycloakSpec{Mode: analyzeManaged},
			Edge: &otilmv1alpha1.EdgeSpec{
				Enabled: true,
				Type:    "gatewayAPI",
				TLS:     &otilmv1alpha1.EdgeTLSSpec{Source: "letsEncrypt"},
			},
		},
	}
	m := PlatformModes(p)
	assert.True(t, m.DBManaged)
	assert.False(t, m.MessagingManaged)
	assert.True(t, m.KeycloakManaged)
	assert.Equal(t, "gatewayAPI", m.Edge)
	assert.Equal(t, "letsEncrypt", m.TLSSource)
}

func TestPlatformModesDisabledEdgeAndNilKeycloak(t *testing.T) {
	t.Parallel()
	p := &otilmv1alpha1.Platform{
		Spec: otilmv1alpha1.PlatformSpec{
			Database:  otilmv1alpha1.DatabaseSpec{Mode: analyzeExternal},
			Messaging: otilmv1alpha1.MessagingSpec{Mode: analyzeManaged},
			Edge:      &otilmv1alpha1.EdgeSpec{Enabled: false, Type: "ingress"},
		},
	}
	m := PlatformModes(p)
	assert.False(t, m.DBManaged)
	assert.True(t, m.MessagingManaged)
	assert.False(t, m.KeycloakManaged)
	assert.Equal(t, "", m.Edge, "disabled edge contributes no mode")
}

func TestPlatformRefs(t *testing.T) {
	t.Parallel()
	enabled := true
	p := &otilmv1alpha1.Platform{
		Spec: otilmv1alpha1.PlatformSpec{
			Database:  otilmv1alpha1.DatabaseSpec{Credentials: &otilmv1alpha1.CredentialsRef{SecretRef: "db-creds"}},
			Messaging: otilmv1alpha1.MessagingSpec{Credentials: &otilmv1alpha1.CredentialsRef{SecretRef: "mq-creds"}},
			Edge: &otilmv1alpha1.EdgeSpec{
				Enabled: true,
				TLS: &otilmv1alpha1.EdgeTLSSpec{
					Source:    "issuerRef",
					SecretRef: strptr("edge-tls"),
					IssuerRef: &otilmv1alpha1.CertManagerIssuerRef{Name: "edge-issuer"},
				},
			},
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled: true,
				Certificate: &otilmv1alpha1.AdminCertificateSpec{
					Enabled:   &enabled,
					Source:    "provided",
					SecretRef: strptr("admin-cert"),
				},
				Password: &otilmv1alpha1.AdminPasswordSpec{Enabled: true, SecretRef: "admin-pw"},
			},
		},
	}
	secrets, issuers := PlatformRefs(p)
	assert.ElementsMatch(t, []string{"db-creds", "mq-creds", "edge-tls", "admin-cert", "admin-pw"}, secrets)
	assert.ElementsMatch(t, []string{"edge-issuer"}, issuers)
}

func TestPlatformRefsDisabledEdgeExcluded(t *testing.T) {
	t.Parallel()
	p := &otilmv1alpha1.Platform{
		Spec: otilmv1alpha1.PlatformSpec{
			Database:  otilmv1alpha1.DatabaseSpec{Mode: analyzeExternal},
			Messaging: otilmv1alpha1.MessagingSpec{Mode: analyzeExternal},
			Edge: &otilmv1alpha1.EdgeSpec{
				Enabled: false,
				TLS: &otilmv1alpha1.EdgeTLSSpec{
					Source:    "issuerRef",
					SecretRef: strptr("leftover-tls"),
					IssuerRef: &otilmv1alpha1.CertManagerIssuerRef{Name: "leftover-issuer"},
				},
			},
		},
	}
	secrets, issuers := PlatformRefs(p)
	assert.NotContains(t, secrets, "leftover-tls", "disabled edge TLS secret must not be extracted")
	assert.NotContains(t, issuers, "leftover-issuer", "disabled edge issuer must not be extracted")
}

func TestPlatformRefsAdminIssuerRef(t *testing.T) {
	t.Parallel()
	p := &otilmv1alpha1.Platform{
		Spec: otilmv1alpha1.PlatformSpec{
			Database:  otilmv1alpha1.DatabaseSpec{Mode: analyzeExternal},
			Messaging: otilmv1alpha1.MessagingSpec{Mode: analyzeExternal},
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled: true,
				Certificate: &otilmv1alpha1.AdminCertificateSpec{
					Source:    "generated",
					IssuerRef: &otilmv1alpha1.CertManagerIssuerRef{Name: "admin-ca"},
				},
			},
		},
	}
	secrets, issuers := PlatformRefs(p)
	assert.Empty(t, secrets)
	assert.Contains(t, issuers, "admin-ca")
}

func TestProxyRefs(t *testing.T) {
	t.Parallel()
	p := &otilmv1alpha1.Proxy{
		Spec: otilmv1alpha1.ProxySpec{
			ConfigTokenSecretRef: otilmv1alpha1.ConfigTokenRef{Name: "proxy-token"},
		},
	}
	assert.Equal(t, []string{"proxy-token"}, ProxyRefs(p))
}

func TestConnectorRefsEmpty(t *testing.T) {
	t.Parallel()
	c := &otilmv1alpha1.Connector{}
	assert.Empty(t, ConnectorRefs(c))
}

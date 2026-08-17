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

package capabilities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	opcap "github.com/OmniTrustILM/operator/pkg/capabilities"
)

const capUnused = "unused"

// stubMapper feeds opcap.Detector: RESTMapping returns a mapping for the GKs in
// present, a NoKindMatchError for everything else, or a sentinel error per-GK.
type stubMapper struct {
	present map[schema.GroupKind]bool
	errs    map[schema.GroupKind]error
}

func (m *stubMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	if err, ok := m.errs[gk]; ok {
		return nil, err
	}
	if m.present[gk] {
		return &meta.RESTMapping{}, nil
	}
	return nil, &meta.NoKindMatchError{GroupKind: gk}
}
func (m *stubMapper) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic(capUnused)
}
func (m *stubMapper) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic(capUnused)
}
func (m *stubMapper) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic(capUnused)
}
func (m *stubMapper) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic(capUnused)
}
func (m *stubMapper) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	panic(capUnused)
}
func (m *stubMapper) ResourceSingularizer(string) (string, error) { panic(capUnused) }

func reporterWith(present map[schema.GroupKind]bool, errs map[schema.GroupKind]error) *Reporter {
	return NewReporter(opcap.New(&stubMapper{present: present, errs: errs}))
}

func TestProbeGroupKinds_Catalog(t *testing.T) {
	want := map[Dep]schema.GroupKind{
		DepCertManager:    {Group: "cert-manager.io", Kind: "Certificate"},
		DepCNPG:           {Group: "postgresql.cnpg.io", Kind: "Cluster"},
		DepRabbitMQ:       {Group: "rabbitmq.com", Kind: "RabbitmqCluster"},
		DepKeycloak:       {Group: "k8s.keycloak.org", Kind: "Keycloak"},
		DepGatewayAPI:     {Group: "gateway.networking.k8s.io", Kind: "Gateway"},
		DepServiceMonitor: {Group: "monitoring.coreos.com", Kind: "ServiceMonitor"},
	}
	assert.Equal(t, want, ProbeGroupKinds)
}

func TestReporter_Present(t *testing.T) {
	r := reporterWith(map[schema.GroupKind]bool{
		ProbeGroupKinds[DepCertManager]: true,
	}, nil)

	ok, err := r.Present(DepCertManager)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = r.Present(DepCNPG)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestReporter_Present_UnknownDepIsError(t *testing.T) {
	r := reporterWith(nil, nil)
	_, err := r.Present(Dep("bogus"))
	require.Error(t, err)
}

func TestReporter_Detect_AllWhenEmpty(t *testing.T) {
	r := reporterWith(map[schema.GroupKind]bool{
		ProbeGroupKinds[DepCNPG]:     true,
		ProbeGroupKinds[DepRabbitMQ]: true,
	}, nil)
	results := r.Detect()
	require.Len(t, results, len(ProbeGroupKinds))
	byDep := map[Dep]Result{}
	for _, res := range results {
		byDep[res.Dep] = res
	}
	assert.True(t, byDep[DepCNPG].Present)
	assert.True(t, byDep[DepRabbitMQ].Present)
	assert.False(t, byDep[DepCertManager].Present)
	for _, res := range results {
		assert.NoError(t, res.Err)
	}
}

func TestReporter_Detect_Subset(t *testing.T) {
	r := reporterWith(map[schema.GroupKind]bool{ProbeGroupKinds[DepKeycloak]: true}, nil)
	results := r.Detect(DepKeycloak, DepCertManager)
	require.Len(t, results, 2)
	assert.Equal(t, DepKeycloak, results[0].Dep)
	assert.True(t, results[0].Present)
	assert.Equal(t, DepCertManager, results[1].Dep)
	assert.False(t, results[1].Present)
}

func TestReporter_Detect_PropagatesTransientError(t *testing.T) {
	sentinel := errors.New("discovery down")
	r := reporterWith(nil, map[schema.GroupKind]error{ProbeGroupKinds[DepCNPG]: sentinel})
	results := r.Detect(DepCNPG)
	require.Len(t, results, 1)
	assert.False(t, results[0].Present)
	assert.ErrorIs(t, results[0].Err, sentinel)
}

func TestRequiredFor(t *testing.T) {
	tests := []struct {
		name  string
		modes Modes
		want  []Dep
	}{
		{"all external, ingress, internal tls", Modes{Edge: "ingress", TLSSource: "internal"}, nil},
		{"managed db", Modes{DBManaged: true}, []Dep{DepCNPG}},
		{"managed messaging", Modes{MessagingManaged: true}, []Dep{DepRabbitMQ}},
		{"managed keycloak", Modes{KeycloakManaged: true}, []Dep{DepKeycloak}},
		{"gateway edge", Modes{Edge: edgeGatewayAPI}, []Dep{DepGatewayAPI}},
		{"letsEncrypt tls needs cert-manager", Modes{TLSSource: tlsLetsEncrypt}, []Dep{DepCertManager}},
		{"issuerRef tls needs cert-manager", Modes{TLSSource: "issuerRef"}, []Dep{DepCertManager}},
		{"secret tls needs nothing", Modes{TLSSource: "secret"}, nil},
		{
			"managed-ha everything",
			Modes{DBManaged: true, MessagingManaged: true, KeycloakManaged: true, Edge: edgeGatewayAPI, TLSSource: tlsLetsEncrypt},
			[]Dep{DepCNPG, DepRabbitMQ, DepKeycloak, DepGatewayAPI, DepCertManager},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RequiredFor(tt.modes))
		})
	}
}

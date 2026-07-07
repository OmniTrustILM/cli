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

package deps

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// stubMapper satisfies meta.RESTMapper: RESTMapping returns a mapping for GKs
// listed in present, and a NoKindMatchError for everything else.
type stubMapper struct {
	present map[schema.GroupKind]bool
}

func (m *stubMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	if m.present[gk] {
		return &meta.RESTMapping{}, nil
	}
	return nil, &meta.NoKindMatchError{GroupKind: gk}
}
func (m *stubMapper) KindFor(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	panic(stubUnused)
}
func (m *stubMapper) KindsFor(schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	panic(stubUnused)
}
func (m *stubMapper) ResourceFor(schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	panic(stubUnused)
}
func (m *stubMapper) ResourcesFor(schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	panic(stubUnused)
}
func (m *stubMapper) RESTMappings(schema.GroupKind, ...string) ([]*meta.RESTMapping, error) {
	panic(stubUnused)
}
func (m *stubMapper) ResourceSingularizer(string) (string, error) { panic(stubUnused) }

func clientWithEmptyMapper(t *testing.T) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	require.NoError(t, apiextv1.AddToScheme(scheme))
	// Empty mapper => nothing present; detector reports all absent.
	mapper := restmapper.NewDiscoveryRESTMapper(nil)
	return &k8s.Client{Typed: ctrlfake.NewClientBuilder().WithScheme(scheme).Build(), Mapper: mapper, Scheme: scheme}
}

func clientWithPresentDeps(t *testing.T, present map[schema.GroupKind]bool) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	require.NoError(t, apiextv1.AddToScheme(scheme))
	return &k8s.Client{
		Typed:  ctrlfake.NewClientBuilder().WithScheme(scheme).Build(),
		Mapper: &stubMapper{present: present},
		Scheme: scheme,
	}
}

// TestDepsCheck_ReportsPresence verifies the table lists all known dep names.
func TestDepsCheck_ReportsPresence(t *testing.T) {
	c := clientWithEmptyMapper(t)
	old := depsClientFor
	depsClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { depsClientFor = old }()

	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	cmd := NewCheckCommand(o)
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), depCertManager)
	assert.Contains(t, out.String(), "cnpg")
}

// TestDepsCheck_PresentVsAbsent exercises the present/absent distinction and
// mode-based REQUIRED column via --tls-source=letsEncrypt (which requires cert-manager).
func TestDepsCheck_PresentVsAbsent(t *testing.T) {
	// Mark cert-manager present; all others absent.
	c := clientWithPresentDeps(t, map[schema.GroupKind]bool{
		capabilities.ProbeGroupKinds[capabilities.DepCertManager]: true,
	})
	old := depsClientFor
	depsClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { depsClientFor = old }()

	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	cmd := NewCheckCommand(o)
	// --tls-source=letsEncrypt makes cert-manager required; cnpg remains not required.
	cmd.SetArgs([]string{"--tls-source=letsEncrypt"})
	require.NoError(t, cmd.Execute())

	outStr := out.String()
	assert.Contains(t, outStr, depCertManager)
	assert.Contains(t, outStr, "cnpg")

	// cert-manager row: present=yes, required=yes.
	// cnpg row: present=no, required=no.
	// The table has columns: DEPENDENCY | PRESENT | REQUIRED.
	// We verify the presence and required values by checking substrings near the dep name.
	// A simple approach: split by newline and check the cert-manager and cnpg lines.
	for _, line := range strings.Split(outStr, "\n") {
		switch {
		case strings.Contains(line, depCertManager):
			assert.Contains(t, line, "yes", "cert-manager must be present=yes")
		case strings.Contains(line, "cnpg"):
			assert.Contains(t, line, "no", "cnpg must report no (not present, not required)")
		}
	}
}

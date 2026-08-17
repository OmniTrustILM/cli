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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func newCheckFixtures(t *testing.T, objs ...ctrlclient.Object) (*k8s.Client, *capabilities.Reporter) {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	mapper := testrestmapper.TestOnlyStaticRESTMapper(s)
	b := ctrlfake.NewClientBuilder().WithScheme(s)
	for _, o := range objs {
		b = b.WithObjects(o)
	}
	c := &k8s.Client{Typed: b.Build(), Scheme: s, Mapper: mapper}
	return c, capabilities.NewReporter(opcap.New(mapper))
}

func TestRunCheck_LiveFailReturnsWorstFail(t *testing.T) {
	// Managed-DB platform with no CNPG CRD served -> the capability analyzer fails.
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseProgressing},
	}
	plat.Spec.Database.Mode = "managed"
	c, rep := newCheckFixtures(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{Namespaces: []string{infraNamespace}})
	require.NoError(t, err)
	assert.Equal(t, analyze.SeverityFail, worst)
	assert.Contains(t, out.String(), "✗")
}

func TestRunCheck_PreModeMissingDepFails(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{DBManaged: true},
	})
	require.NoError(t, err)
	assert.Equal(t, analyze.SeverityFail, worst)
	assert.Contains(t, out.String(), "cnpg")
	assert.Contains(t, out.String(), "ilmctl deps install --only cnpg")
}

func TestRunCheck_PreModeNoDepsIsOK(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{}, // all-external: no upstream operators required
	})
	require.NoError(t, err)
	assert.NotEqual(t, analyze.SeverityFail, worst)
	assert.Contains(t, out.String(), "no upstream operators required")
}

func TestRunCheck_CleanLiveIsOK(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{Namespaces: []string{infraNamespace}})
	require.NoError(t, err)
	assert.NotEqual(t, analyze.SeverityFail, worst)
}

func TestRunCheck_JSONFindings(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = "json"
	_, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{DBManaged: true},
	})
	require.NoError(t, err)
	var findings []analyze.Finding
	require.NoError(t, json.Unmarshal(out.Bytes(), &findings))
	require.NotEmpty(t, findings)
}

func TestNewCheckCommand_FlagsAndGroup(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	cmd := NewCheckCommand(o)
	// Literal on purpose: the command sets Use from checkCommand, so asserting against
	// that same constant would pass whatever value it held. This pins the invocation
	// name users type.
	assert.Equal(t, "check", cmd.Use)
	assert.Equal(t, string(cli.GroupInfrastructure), cmd.GroupID)
	assert.Contains(t, cmd.Aliases, "doctor")
	assert.NotNil(t, cmd.Flags().Lookup("pre"))
	assert.NotNil(t, cmd.Flags().Lookup(infraAllNamespaces))
	assert.NotNil(t, cmd.Flags().Lookup("db-mode"))
	assert.NotNil(t, cmd.Flags().Lookup("messaging-mode"))
	assert.NotNil(t, cmd.Flags().Lookup("keycloak-mode"))
	assert.NotNil(t, cmd.Flags().Lookup("edge"))
	assert.NotNil(t, cmd.Flags().Lookup("tls-source"))
}

func TestRunCheck_PreModeMultipleDepsFail(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{
			DBManaged: true, MessagingManaged: true, KeycloakManaged: true,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, analyze.SeverityFail, worst)
	assert.Contains(t, out.String(), "cnpg")
	assert.Contains(t, out.String(), "rabbitmq")
	assert.Contains(t, out.String(), "keycloak")
}

func TestRunCheck_PreModeCertManagerRequired(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{TLSSource: "letsEncrypt"},
	})
	require.NoError(t, err)
	assert.Equal(t, analyze.SeverityFail, worst)
	assert.Contains(t, out.String(), "cert-manager")
}

func TestRunCheck_WorstFailReturnsFailSeverity(t *testing.T) {
	c, rep := newCheckFixtures(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	worst, err := runCheck(context.Background(), c, rep, p, checkOptions{
		Pre: true, IntendedModes: capabilities.Modes{Edge: "gatewayAPI"},
	})
	require.NoError(t, err)
	assert.Equal(t, analyze.SeverityFail, worst)
}

// TestNewCheckCommand_RunE_DegradedReturnsErrFailure exercises the RunE closure
// end-to-end via clientFn injection.  A degraded fixture (managed-DB platform,
// no CNPG CRD served) causes worst==SeverityFail; the RunE must return cli.ErrFailure.
func TestNewCheckCommand_RunE_DegradedReturnsErrFailure(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: infraPlatformName, Namespace: infraNamespace},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseProgressing},
	}
	plat.Spec.Database.Mode = "managed"

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
	opts := &checkOptions{clientFn: func() (*k8s.Client, error) { return fakeC, nil }}
	cmd := newCheckCommandFromOpts(o, opts)
	// --all-namespaces avoids calling Factory.Namespace() (Factory is nil in this test).
	require.NoError(t, cmd.Flags().Set(infraAllNamespaces, "true"))

	runErr := cmd.RunE(cmd, []string{})
	assert.True(t, errors.Is(runErr, cli.ErrFailure), "want cli.ErrFailure, got %v", runErr)
}

// TestNewCheckCommand_RunE_HealthyReturnsNil confirms the RunE returns nil (exit 0)
// when no findings are SeverityFail.
func TestNewCheckCommand_RunE_HealthyReturnsNil(t *testing.T) {
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	mapper := testrestmapper.TestOnlyStaticRESTMapper(s)
	fakeC := &k8s.Client{
		Typed:  ctrlfake.NewClientBuilder().WithScheme(s).Build(),
		Scheme: s,
		Mapper: mapper,
	}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := &cli.Options{Printer: render.NewPrinter(out, errOut)}
	opts := &checkOptions{clientFn: func() (*k8s.Client, error) { return fakeC, nil }}
	cmd := newCheckCommandFromOpts(o, opts)
	require.NoError(t, cmd.Flags().Set(infraAllNamespaces, "true"))

	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

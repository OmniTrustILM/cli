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

package shared

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

const (
	applyConnectorRef = "Connector/ilm/demo"
	applyFlagAlpha    = "alpha"
)

// newSharedTestOptions constructs a cli.Options wired to in-memory buffers.
func newSharedTestOptions() (*cli.Options, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &cli.Options{
		In:      bytes.NewReader(nil),
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
	}, out
}

// minimalConnector returns a typed Connector with GVK set, suitable for
// round-trip tests via ToUnstructured.
func minimalConnector(name, ns string) *otilmv1alpha1.Connector {
	return &otilmv1alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: otilmv1alpha1.GroupVersion.String(),
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: otilmv1alpha1.ConnectorSpec{
			Image: otilmv1alpha1.ImageSpec{
				Repository: "harbor.example.com/ilm",
				Tag:        "1.0.0",
			},
		},
	}
}

// ── ParseDryRun ──────────────────────────────────────────────────────────────

func TestParseDryRun(t *testing.T) {
	tests := []struct {
		input   string
		want    manifest.DryRunMode
		wantErr bool
	}{
		{"", manifest.DryRunNone, false},
		{"none", manifest.DryRunNone, false},
		{"None", manifest.DryRunNone, false},
		{"NONE", manifest.DryRunNone, false},
		{"client", manifest.DryRunClient, false},
		{"Client", manifest.DryRunClient, false},
		{"server", manifest.DryRunServer, false},
		{"Server", manifest.DryRunServer, false},
		{"bogus", manifest.DryRunNone, true},
		{"all", manifest.DryRunNone, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseDryRun(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ── ChangedFlags ─────────────────────────────────────────────────────────────

func TestChangedFlags_OnlyExplicitFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String(applyFlagAlpha, "default-a", "")
	cmd.Flags().String("beta", "default-b", "")
	cmd.Flags().String("gamma", "default-g", "")

	// Simulate the user setting only --beta on the command line.
	require.NoError(t, cmd.Flags().Set("beta", "override"))

	got := ChangedFlags(cmd)
	assert.True(t, got["beta"], "beta was explicitly set")
	assert.False(t, got[applyFlagAlpha], "alpha was not set")
	assert.False(t, got["gamma"], "gamma was not set")
}

func TestChangedFlags_EmptyWhenNoneSet(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String(applyFlagAlpha, "default", "")
	assert.Empty(t, ChangedFlags(cmd))
}

// ── ToUnstructured ────────────────────────────────────────────────────────────

func TestToUnstructured_PreservesGVK(t *testing.T) {
	obj := minimalConnector("demo", "ilm")
	u, err := ToUnstructured(obj)
	require.NoError(t, err)

	assert.Equal(t, otilmv1alpha1.GroupVersion.String(), u.GetAPIVersion())
	assert.Equal(t, "Connector", u.GetKind())
	assert.Equal(t, "demo", u.GetName())
	assert.Equal(t, "ilm", u.GetNamespace())
}

func TestToUnstructured_FieldsRoundTrip(t *testing.T) {
	obj := minimalConnector("round-trip", "ns")
	u, err := ToUnstructured(obj)
	require.NoError(t, err)

	spec, ok, err := unstructured.NestedMap(u.Object, "spec")
	require.NoError(t, err)
	require.True(t, ok, "spec field must be present")
	assert.NotEmpty(t, spec)
}

// ── ApplyObject + ReportApply ─────────────────────────────────────────────────

func TestApplyObject_ClientDryRun_ReportsApplied(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	o, out := newSharedTestOptions()

	obj := minimalConnector("demo", "ilm")
	require.NoError(t, ApplyObject(context.Background(), o, c, obj, "client", false))

	// Client dry-run must print the (dry-run) suffix.
	assert.Contains(t, out.String(), "applied")
	assert.Contains(t, out.String(), "dry-run")
}

func TestApplyObject_ServerDryRun_Succeeds(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	o, out := newSharedTestOptions()

	obj := minimalConnector("demo", "ilm")
	require.NoError(t, ApplyObject(context.Background(), o, c, obj, "server", false))
	// Server dry-run with a fake client succeeds; result line printed.
	assert.NotEmpty(t, out.String())
}

func TestApplyObject_RealApply_ReportsApplied(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	o, out := newSharedTestOptions()

	obj := minimalConnector("demo", "ilm")
	require.NoError(t, ApplyObject(context.Background(), o, c, obj, "none", false))

	s := out.String()
	assert.Contains(t, s, applyConnectorRef)
	assert.Contains(t, s, "applied")
}

func TestApplyObject_InvalidDryRun_ReturnsError(t *testing.T) {
	c := k8s.NewFakeClient(t, k8s.FakeClientOptions{})
	o, _ := newSharedTestOptions()

	obj := minimalConnector("demo", "ilm")
	err := ApplyObject(context.Background(), o, c, obj, "invalid", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --dry-run")
}

// ── ReportApply ───────────────────────────────────────────────────────────────

func TestReportApply_Applied(t *testing.T) {
	o, out := newSharedTestOptions()
	ReportApply(o, manifest.ApplyResult{Applied: []string{applyConnectorRef}}, manifest.DryRunNone)
	assert.Contains(t, out.String(), "Connector/ilm/demo applied")
}

func TestReportApply_Unchanged(t *testing.T) {
	o, out := newSharedTestOptions()
	ReportApply(o, manifest.ApplyResult{Unchanged: []string{applyConnectorRef}}, manifest.DryRunNone)
	assert.Contains(t, out.String(), "Connector/ilm/demo unchanged")
}

func TestReportApply_DryRunPrefix(t *testing.T) {
	o, out := newSharedTestOptions()
	ReportApply(o, manifest.ApplyResult{Applied: []string{applyConnectorRef}}, manifest.DryRunServer)
	assert.Contains(t, out.String(), "(dry-run) Connector/ilm/demo applied")
}

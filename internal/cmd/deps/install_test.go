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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

// cnpgManifest is a minimal-but-valid stand-in for the CNPG release manifest:
// a Namespace, a CRD, and the controller Deployment. It lets the install tests
// run hermetically (no network) via manifest.SetDepFetchForTest.
const cnpgManifest = "" +
	"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cnpg-system\n" +
	"---\n" +
	"apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: clusters.postgresql.cnpg.io\n" +
	"---\n" +
	"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cnpg-controller-manager\n  namespace: cnpg-system\n"

// certManagerManifest is a minimal stand-in for the cert-manager release.
const certManagerManifest = "" +
	"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: cert-manager\n"

// stubDepFetch serves canned manifests for the install tests, keyed by a
// substring of the requested URL, and returns a restore function.
func stubDepFetch(t *testing.T) func() {
	t.Helper()
	return manifest.SetDepFetchForTest(func(_ context.Context, ref string) ([]byte, error) {
		switch {
		case contains(ref, "cloudnative-pg"):
			return []byte(cnpgManifest), nil
		case contains(ref, depCertManager):
			return []byte(certManagerManifest), nil
		default:
			return []byte(certManagerManifest), nil
		}
	})
}

func contains(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }

// establishedCNPGCRD is the CRD the CNPG install waits on, marked Established so
// ApplyOrdered's readiness poll returns immediately.
func establishedCNPGCRD() *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "clusters.postgresql.cnpg.io"},
	}}
	_ = unstructured.SetNestedSlice(u.Object, []any{
		map[string]any{"type": "Established", "status": "True"},
	}, "status", "conditions")
	return u
}

func installClient(t *testing.T) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	require.NoError(t, apiextv1.AddToScheme(scheme))

	dynScheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	listKinds := map[schema.GroupVersionResource]string{gvr: "CustomResourceDefinitionList"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(dynScheme, listKinds, establishedCNPGCRD())

	return &k8s.Client{
		Typed:   ctrlfake.NewClientBuilder().WithScheme(scheme).Build(),
		Dynamic: dyn,
		Scheme:  scheme,
	}
}

func TestDepsInstall_Only(t *testing.T) {
	defer stubDepFetch(t)()
	c := installClient(t)
	old := depsClientFor
	depsClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { depsClientFor = old }()

	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	cmd := NewInstallCommand(o)
	cmd.SetArgs([]string{installFlagOnly, "cnpg"})
	require.NoError(t, cmd.Execute())

	var ns corev1.Namespace
	require.NoError(t, c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Name: "cnpg-system"}, &ns))
	assert.Contains(t, out.String(), "cnpg-system")
}

func TestDepsInstall_DryRunDoesNotPersist(t *testing.T) {
	defer stubDepFetch(t)()
	c := installClient(t)
	old := depsClientFor
	depsClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { depsClientFor = old }()

	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	cmd := NewInstallCommand(o)
	cmd.SetArgs([]string{installFlagOnly, depCertManager, "--dry-run"})
	require.NoError(t, cmd.Execute())

	var ns corev1.Namespace
	err := c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Name: depCertManager}, &ns)
	require.Error(t, err)
}

func TestDepsInstall_UnknownOnly(t *testing.T) {
	c := installClient(t)
	old := depsClientFor
	depsClientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { depsClientFor = old }()

	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	cmd := NewInstallCommand(o)
	cmd.SetArgs([]string{installFlagOnly, "bogus"})
	require.Error(t, cmd.Execute())
}

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakedisco "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

// buildInitClient builds a Client backed by fake typed, dynamic, and discovery
// implementations. When olmPresent is true the discovery fake advertises the
// operators.coreos.com API group so DetectOLM returns true.
func buildInitClient(t *testing.T, olmPresent bool) *k8s.Client {
	t.Helper()
	s, err := k8s.NewScheme()
	require.NoError(t, err)

	crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	listKinds := map[schema.GroupVersionResource]string{
		crdGVR: "CustomResourceDefinitionList",
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	disco := &fakedisco.FakeDiscovery{Fake: &clienttesting.Fake{}}
	if olmPresent {
		disco.Resources = []*metav1.APIResourceList{
			{GroupVersion: "operators.coreos.com/v1alpha1"},
		}
	}
	return &k8s.Client{
		Typed:     ctrlfake.NewClientBuilder().WithScheme(s).Build(),
		Dynamic:   dyn,
		Discovery: disco,
		Scheme:    s,
	}
}

// operatorPath writes a self-contained fake operator source checkout to a temp
// dir laid out like the real operator (deploy/manifests/*.yaml) and returns its
// root. Tests point --from-source at it, so they never depend on a sibling
// operator checkout being present on disk — it is absent in CI.
func operatorPath(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifests := filepath.Join(root, "deploy", "manifests")
	require.NoError(t, os.MkdirAll(manifests, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(manifests, "ilm-operator.crds.yaml"), []byte(fakeOperatorCRDs), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(manifests, "ilm-operator.yaml"), []byte(fakeOperatorController), 0o600))
	return root
}

// fakeOperatorCRDs and fakeOperatorController are minimal, valid manifests
// shaped like the real operator's deploy/manifests/*.yaml: enough for the
// --from-source resolver to split them and for dry-run rendering to report the
// object kinds/names the tests assert on (a CustomResourceDefinition, a
// ClusterRole, and the ilm-operator-controller-manager Deployment).
const fakeOperatorCRDs = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: platforms.otilm.com
spec:
  group: otilm.com
  names:
    kind: Platform
    listKind: PlatformList
    plural: platforms
    singular: platform
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`

const fakeOperatorController = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ilm-operator-manager-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ilm-operator-controller-manager
  namespace: ilm-operator-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
        - name: manager
          image: example.com/ilm-operator:latest
`

// newInitOptions builds a minimal cli.Options with a Printer wired to the
// supplied buffers. The Factory is intentionally nil; tests inject via clientFor.
func newInitOptions(out, errOut *bytes.Buffer) *cli.Options {
	return &cli.Options{
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
	}
}

// TestInit_FlagsAndGroup verifies the command is registered in the
// Infrastructure group with all expected flags present.
func TestInit_FlagsAndGroup(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))

	assert.Equal(t, "init", cmd.Use)
	assert.Equal(t, string(cli.GroupInfrastructure), cmd.GroupID)

	for _, name := range []string{
		"version", "ref", "manifest", "from-source", "method",
		"namespace", "create-namespace", "with-deps", "wait", "timeout",
		infraDryRun, "force-conflicts",
		"channel", "catalog-image",
	} {
		assert.NotNil(t, cmd.Flags().Lookup(name), "missing flag --%s", name)
	}
	// Short flag -n must be registered.
	assert.NotNil(t, cmd.Flags().ShorthandLookup("n"))
}

// TestInit_DefaultVersionFailsFast confirms that the unreleased default path
// (no --version/--ref/--from-source/--manifest) prints guidance to ErrOut and
// returns a non-nil error.
func TestInit_DefaultVersionFailsFast(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{}) // no source flags → ErrUnreleased
	err := cmd.Execute()
	require.Error(t, err)
	// Guidance must steer the user toward both --ref and --from-source.
	assert.Contains(t, errOut.String(), "--ref")
	assert.Contains(t, errOut.String(), infraFromSource)
}

// TestInit_FromSource_DryRunClient verifies that --from-source + --dry-run=client
// renders objects without touching the cluster (no Deployment created).
func TestInit_FromSource_DryRunClient(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraFromSource, operatorPath(t),
		infraDryRunClient,
		"-n", infraOperatorSys,
	})
	require.NoError(t, cmd.Execute())
	// dry-run output must mention the mode.
	assert.Contains(t, out.String(), infraDryRun)
}

// TestInit_FromSource_CreateNamespace verifies that --create-namespace with
// --dry-run=client includes a Namespace object in the rendered output.
func TestInit_FromSource_CreateNamespace(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraFromSource, operatorPath(t),
		infraDryRunClient,
		"-n", "my-ns",
		"--create-namespace",
	})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Namespace")
}

// TestInit_OLMAbsentFailsFast confirms that --method=olm with OLM not installed
// on the cluster returns the "not installed" error (a catalog image is supplied
// so the check reaches DetectOLM rather than the required-flag guard).
func TestInit_OLMAbsentFailsFast(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{infraMethodOLM, "--ref", "abc123", infraChannelFlag, infraChannelStable, infraCatalogFlag, infraCatalogImage})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OLM is not installed")
}

// TestInit_OLMRequiresCatalogImage confirms --method=olm without --catalog-image
// is a usage error (no default catalog is published).
func TestInit_OLMRequiresCatalogImage(t *testing.T) {
	c := buildInitClient(t, true)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{infraMethodOLM, infraChannelFlag, infraChannelStable, "-n", infraOperatorSys})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), infraCatalogFlag)
}

// TestInit_OLMPresentApplies verifies that when OLM is present and dry-run=client
// is requested the output mentions the Subscription object (proving ApplyOLM ran).
func TestInit_OLMPresentApplies(t *testing.T) {
	c := buildInitClient(t, true)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{infraMethodOLM, infraChannelFlag, infraChannelStable, "-n", infraOperatorSys, infraCatalogFlag, infraCatalogImage, infraDryRunClient})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Subscription")
}

// TestInit_WithDeps_DryRunClient verifies that --with-deps runs InstallDeps in
// dry-run mode before the operator manifests, and that dep objects appear in the
// output before operator objects.
func TestInit_WithDeps_DryRunClient(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraFromSource, operatorPath(t),
		infraDryRunClient,
		"--with-deps",
		"-n", infraOperatorSys,
	})
	require.NoError(t, cmd.Execute())
	outStr := out.String()
	// Both dep objects and operator objects must appear in dry-run output.
	assert.Contains(t, outStr, infraDryRun)
	// Dep objects (cert-manager) must appear before operator objects (ilm-operator-controller-manager).
	depIdx := strings.Index(outStr, "cert-manager")
	opIdx := strings.Index(outStr, "ilm-operator-controller-manager")
	assert.True(t, depIdx >= 0, "expected cert-manager dep object in output")
	assert.True(t, opIdx >= 0, "expected ilm-operator-controller-manager operator object in output")
	assert.Less(t, depIdx, opIdx, "dep objects must appear before operator objects in output")
	// Verify errOut is empty (--wait not passed, no unexpected notices).
	assert.Empty(t, errOut.String())
}

// TestInit_InvalidDryRun verifies that an unrecognised --dry-run value returns
// a usage error without panicking.
func TestInit_InvalidDryRun(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{infraFromSource, operatorPath(t), "--dry-run=bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), infraDryRun)
}

// TestInit_InvalidMethod verifies that an unrecognised --method value returns
// a descriptive error rather than silently falling through to the manifest path.
func TestInit_InvalidMethod(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{infraFromSource, operatorPath(t), "--method=bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --method")
	assert.Contains(t, err.Error(), "bogus")
	_ = errOut // no ErrOut output expected
}

// TestInit_Manifest_DryRunClient exercises the --manifest install path including
// splitByCRD ordering. It writes a combined multi-doc YAML (one CRD doc + one
// Deployment doc) to a temp file, runs with --dry-run=client -o yaml, and asserts
// that the CRD appears before the controller object in the output (CRDs-first).
func TestInit_Manifest_DryRunClient(t *testing.T) {
	// Combined manifest: CRD doc first in file order, then a Deployment.
	// splitByCRD must partition them and ApplyOrdered must apply CRDs first.
	combined := `---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.example.com
spec:
  group: example.com
  names:
    kind: Foo
    plural: foos
  scope: Cluster
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
  namespace: ilm-operator-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-controller
  template:
    metadata:
      labels:
        app: my-controller
    spec:
      containers:
      - name: manager
        image: example.com/manager:latest
`
	f, err := os.CreateTemp(t.TempDir(), "combined-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(combined)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraManifestFlag, f.Name(),
		infraDryRunClient,
		"-n", infraOperatorSys,
	})
	require.NoError(t, cmd.Execute())

	outStr := out.String()
	// Both objects must appear in the dry-run output.
	assert.Contains(t, outStr, "foos.example.com", "CRD must appear in output")
	assert.Contains(t, outStr, infraController, "Deployment must appear in output")
	// CRDs-first: the CRD row must appear before the Deployment row.
	crdIdx := strings.Index(outStr, "foos.example.com")
	deployIdx := strings.Index(outStr, infraController)
	assert.Less(t, crdIdx, deployIdx, "CRD must appear before controller Deployment in output")
	// Dry-run mode must be indicated.
	assert.Contains(t, outStr, infraDryRun)
	// Nothing should be written to ErrOut.
	assert.Empty(t, errOut.String())
}

// deploymentManifest is a single-doc Deployment manifest used by the --wait tests.
// It is applied to the fake cluster; the readiness of the seeded Deployment of
// the same name/namespace determines whether --wait succeeds.
const deploymentManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-controller
  namespace: ilm-operator-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-controller
  template:
    metadata:
      labels:
        app: my-controller
    spec:
      containers:
      - name: manager
        image: example.com/manager:latest
`

// writeManifest writes body to a temp file and returns its path.
func writeManifest(t *testing.T, body string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "m-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(body)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// seedDeployment adds a Deployment (with the given status) to the fake client so
// the readiness waiter can observe it after the apply.
func seedDeployment(t *testing.T, c *k8s.Client, name, ns string, status appsv1.DeploymentStatus) {
	t.Helper()
	one := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Generation: 1},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
		},
		Status: status,
	}
	require.NoError(t, c.Typed.Create(context.Background(), dep))
}

// TestInit_Wait_Succeeds verifies that --wait returns success when the applied
// Deployment is already Available on the (fake) cluster.
func TestInit_Wait_Succeeds(t *testing.T) {
	c := buildInitClient(t, false)
	seedDeployment(t, c, infraController, infraOperatorSys, appsv1.DeploymentStatus{
		ObservedGeneration: 1, UpdatedReplicas: 1, AvailableReplicas: 1, ReadyReplicas: 1,
		Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: "True"}},
	})
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraManifestFlag, writeManifest(t, deploymentManifest),
		"-n", infraOperatorSys,
		infraWaitFlag, infraTimeoutFlag, "2s",
	})
	require.NoError(t, cmd.Execute())
	// The "not yet implemented" notice must be gone.
	assert.NotContains(t, errOut.String(), "not yet implemented")
}

// TestInit_Wait_TimesOut verifies that --wait fails with a clear, Deployment-naming
// error when the applied Deployment never becomes Available.
func TestInit_Wait_TimesOut(t *testing.T) {
	c := buildInitClient(t, false)
	seedDeployment(t, c, infraController, infraOperatorSys, appsv1.DeploymentStatus{
		ObservedGeneration: 1, UpdatedReplicas: 0, AvailableReplicas: 0,
		Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: "False"}},
	})
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraManifestFlag, writeManifest(t, deploymentManifest),
		"-n", infraOperatorSys,
		infraWaitFlag, infraTimeoutFlag, "300ms",
	})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), infraController)
}

// TestInit_Wait_DryRunNoOp verifies that --dry-run=client with --wait applies and
// waits for nothing: no client mutation, no timeout error even though no
// Deployment is present to observe.
func TestInit_Wait_DryRunNoOp(t *testing.T) {
	c := buildInitClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewInitCommand(newInitOptions(out, errOut))
	cmd.SetArgs([]string{
		infraManifestFlag, writeManifest(t, deploymentManifest),
		"-n", infraOperatorSys,
		infraDryRunClient,
		infraWaitFlag, infraTimeoutFlag, "300ms",
	})
	require.NoError(t, cmd.Execute())

	// No Deployment must have been created by the client dry-run apply.
	var got appsv1.Deployment
	err := c.Typed.Get(context.Background(),
		ctrlclient.ObjectKey{Namespace: infraOperatorSys, Name: infraController}, &got)
	require.Error(t, err, "client dry-run must not create the Deployment")
}

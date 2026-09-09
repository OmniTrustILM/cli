/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package infra

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

func clientWithOperatorObjects(t *testing.T) *k8s.Client {
	t.Helper()
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	require.NoError(t, apiextv1.AddToScheme(scheme))

	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: operatorDeployment, Namespace: "ilm-operator-system",
	}}
	crd := &apiextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: infraPlatformsCRD}}
	typed := ctrlfake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, crd).Build()
	return &k8s.Client{Typed: typed, Scheme: scheme}
}

func TestUninstall_RequiresConfirmation(t *testing.T) {
	c := clientWithOperatorObjects(t)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUninstallCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{}) // no -y, not a TTY
	require.Error(t, cmd.Execute())
}

func TestUninstall_KeepsCRDsByDefault(t *testing.T) {
	// Seed: 1 of 2 workloads present (Deployment yes, Service no).
	// Expected: deleted count == 1, not 2.
	c := clientWithOperatorObjects(t)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUninstallCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{"-y"})
	require.NoError(t, cmd.Execute())

	// Only the seeded Deployment was deleted; the absent Service is not counted.
	assert.Contains(t, out.String(), "Removed 1 operator object(s).")

	// Controller deployment gone, CRD retained.
	var dep appsv1.Deployment
	err := c.Typed.Get(context.Background(),
		ctrlclient.ObjectKey{Namespace: "ilm-operator-system", Name: operatorDeployment}, &dep)
	require.Error(t, err)

	var crd apiextv1.CustomResourceDefinition
	require.NoError(t, c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Name: infraPlatformsCRD}, &crd))
}

func TestUninstall_DeletesCRDsWhenAsked(t *testing.T) {
	// Seed: 1 workload (Deployment) + 1 CRD (platforms.otilm.com). The other 2 CRDs are absent.
	// With --keep-crds=false: deleted count == 1 workload + 1 CRD = 2, NOT 1+3=4.
	c := clientWithOperatorObjects(t)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUninstallCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{"-y", "--keep-crds=false"})
	require.NoError(t, cmd.Execute())

	// 1 workload actually deleted + 1 CRD actually deleted = 2, not the full candidate set (2+3=5).
	assert.Contains(t, out.String(), "Removed 2 operator object(s).")

	var crd apiextv1.CustomResourceDefinition
	err := c.Typed.Get(context.Background(), ctrlclient.ObjectKey{Name: infraPlatformsCRD}, &crd)
	require.Error(t, err)
}

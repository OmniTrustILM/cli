/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/version"
)

func runVersion(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	o := NewDefaultOptions(out, errOut)
	cmd := newVersionCommand(o)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestVersionCommand_HumanDefault(t *testing.T) {
	out, _, err := runVersion(t)
	require.NoError(t, err)
	assert.Contains(t, out, "Client Version:")
	assert.Contains(t, out, version.Client().ClientVersion)
	assert.Contains(t, out, "Go Version:")
}

func TestVersionCommand_Short(t *testing.T) {
	out, _, err := runVersion(t, "--short")
	require.NoError(t, err)
	assert.Equal(t, version.Client().ClientVersion+"\n", out)
}

func TestVersionCommand_JSON(t *testing.T) {
	out, _, err := runVersion(t, "-o", "json")
	require.NoError(t, err)
	var got version.Info
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, version.Client().ClientVersion, got.ClientVersion)
	assert.Equal(t, version.Client().Platform, got.Platform)
}

func TestVersionCommand_YAML(t *testing.T) {
	out, _, err := runVersion(t, "-o", "yaml")
	require.NoError(t, err)
	var got version.Info
	require.NoError(t, yaml.Unmarshal([]byte(out), &got))
	assert.Equal(t, version.Client().ClientVersion, got.ClientVersion)
	assert.Equal(t, version.Client().Platform, got.Platform)
}

func TestVersionCommand_InGroupOther(t *testing.T) {
	o := NewDefaultOptions(&bytes.Buffer{}, &bytes.Buffer{})
	cmd := newVersionCommand(o)
	assert.Equal(t, string(GroupOther), cmd.GroupID)
}

func TestFillClusterVersions(t *testing.T) {
	s, err := k8s.NewScheme()
	require.NoError(t, err)
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: cliPlatformName, Namespace: "ns1"},
		Status:     otilmv1alpha1.PlatformStatus{ObservedVersion: cliVer2180},
	}
	// Assert the data path fillClusterVersions relies on (list Platforms + read
	// observedVersion) against the fake client; factory-level rest.Config wiring
	// is e2e-only.
	c := &k8s.Client{Typed: ctrlfake.NewClientBuilder().WithScheme(s).WithObjects(plat).Build(), Scheme: s}
	list, err := c.ListPlatforms(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, cliVer2180, list.Items[0].Status.ObservedVersion)
}

func TestFillClusterVersions_PopulatesInfo(t *testing.T) {
	// Exercises the real applyPlatformVersions logic: mixed empty/non-empty
	// ObservedVersions → correct OperatorVersion (first non-empty) + PlatformVersions.
	plat1 := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: cliPlatformName, Namespace: "ns1"},
		Status:     otilmv1alpha1.PlatformStatus{ObservedVersion: cliVer2180},
	}
	plat2 := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm2", Namespace: "ns2"},
		Status:     otilmv1alpha1.PlatformStatus{ObservedVersion: "2.19.0"},
	}
	platEmpty := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: "ilm3", Namespace: "ns3"},
		Status:     otilmv1alpha1.PlatformStatus{ObservedVersion: ""},
	}
	list := &otilmv1alpha1.PlatformList{
		Items: []otilmv1alpha1.Platform{*plat1, *platEmpty, *plat2},
	}
	info := version.Info{}
	applyPlatformVersions(list, &info)
	// First non-empty ObservedVersion (plat1) sets OperatorVersion.
	assert.Equal(t, cliVer2180, info.OperatorVersion)
	// Both non-empty versions are appended; the empty one is skipped.
	assert.Equal(t, []string{cliVer2180, "2.19.0"}, info.PlatformVersions)
}

// TestFillClusterVersions_FactoryError verifies that fillClusterVersions is a
// no-op when the factory cannot build a client (expected for offline/no-kubeconfig).
func TestFillClusterVersions_FactoryError(t *testing.T) {
	// Point ConfigFlags at a path that does not exist — Client() returns an error
	// deterministically and never panics.
	cf := genericclioptions.NewConfigFlags(true)
	noSuchKubeconfig := t.TempDir() + "/no-such-kubeconfig.yaml"
	cf.KubeConfig = &noSuchKubeconfig
	f, err := k8s.NewFactory(cf)
	require.NoError(t, err)
	info := version.Info{}
	// fillClusterVersions must swallow the Client() error and leave info unchanged.
	fillClusterVersions(context.Background(), f, &info)
	assert.Empty(t, info.OperatorVersion)
	assert.Empty(t, info.PlatformVersions)
}

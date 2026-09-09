/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package infra

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

func TestUpgrade_FromSourceDryRunReportsDeltas(t *testing.T) {
	c := establishedCRDClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUpgradeCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{"--from-source", operatorPath(t), "--dry-run=client"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "CustomResourceDefinition")
	assert.Contains(t, out.String(), "ClusterRole")
}

// TestUpgrade_VersionUsesTheVerifiedRelease confirms that upgrade resolves a
// pinned release, verifies it, and reports the release it is about to apply.
func TestUpgrade_VersionUsesTheVerifiedRelease(t *testing.T) {
	serveRelease(t, infraReleaseTag, releaseAssets())
	c := establishedCRDClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUpgradeCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{infraVersionFlag, infraReleaseTag, infraDryRunClient})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, errOut.String(), infraReleaseTag)
	assert.Contains(t, out.String(), "CustomResourceDefinition")
}

// TestUpgrade_UnknownVersionIsActionable confirms that a release tag with no
// published assets names the tag and where to look, instead of a bare 404.
func TestUpgrade_UnknownVersionIsActionable(t *testing.T) {
	serveRelease(t, infraReleaseTag, releaseAssets())
	c := establishedCRDClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUpgradeCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{infraVersionFlag, "v9.9.9"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v9.9.9")
	assert.Contains(t, err.Error(), infraCRDsAsset)
	assert.Empty(t, out.String())
}

// TestUpgrade_Wait_TimesOut verifies that upgrade --wait fails with a clear,
// Deployment-naming error when the applied Deployment never becomes Available.
func TestUpgrade_Wait_TimesOut(t *testing.T) {
	c := establishedCRDClient(t, false)
	seedDeployment(t, c, "my-controller", infraOperatorSys, appsv1.DeploymentStatus{
		ObservedGeneration: 1, UpdatedReplicas: 0, AvailableReplicas: 0,
		Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: "False"}},
	})
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUpgradeCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{
		"--manifest", writeManifest(t, deploymentManifest),
		"-n", infraOperatorSys,
		"--wait", "--timeout", infraShortTimeout,
	})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-controller")
}

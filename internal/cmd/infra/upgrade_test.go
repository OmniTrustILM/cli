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

func TestUpgrade_DefaultVersionFailsFast(t *testing.T) {
	c := establishedCRDClient(t, false)
	old := clientFor
	clientFor = func(*cli.Options) (*k8s.Client, error) { return c, nil }
	defer func() { clientFor = old }()

	var out, errOut bytes.Buffer
	cmd := NewUpgradeCommand(newTestOptions(&out, &errOut))
	cmd.SetArgs([]string{"--version", "v2.18.0"})
	require.Error(t, cmd.Execute())
	assert.Contains(t, errOut.String(), "--ref")
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
		"--wait", "--timeout", "300ms",
	})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-controller")
}

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

package platform

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func platformWithVersion(observed, spec string) *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec: otilmv1alpha1.PlatformSpec{
			Version:        spec,
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
		},
		Status: otilmv1alpha1.PlatformStatus{ObservedVersion: observed},
	}
}

func TestUpgrade_ForwardSucceeds(t *testing.T) {
	p := platformWithVersion(platformVer2170, platformVer2170)
	o, out := newFakeOptions(t, p)

	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2180})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	cl, err := o.Factory.Client()
	require.NoError(t, err)
	got, err := cl.GetPlatform(context.Background(), platformName, platformName)
	require.NoError(t, err)
	assert.Equal(t, platformVer2180, got.Spec.Version)
}

func TestUpgrade_RejectsDowngrade(t *testing.T) {
	p := platformWithVersion(platformVer2180, platformVer2180)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2170})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoDowngrade)
}

func TestUpgrade_AckFlagsSetManagedAcknowledged(t *testing.T) {
	p := platformWithVersion(platformVer2170, platformVer2170)
	p.Spec.Database.Mode = modeManaged
	p.Spec.Database.Managed = &otilmv1alpha1.ManagedDatabaseSpec{}
	p.Spec.Messaging.Mode = modeManaged
	p.Spec.Messaging.Managed = &otilmv1alpha1.ManagedMessagingSpec{}
	p.Spec.Keycloak = &otilmv1alpha1.KeycloakSpec{Mode: modeManaged, Managed: &otilmv1alpha1.ManagedKeycloakSpec{}}
	o, out := newFakeOptions(t, p)

	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2180, "--ack-database", "--ack-messaging", "--ack-keycloak"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	cl, _ := o.Factory.Client()
	got, _ := cl.GetPlatform(context.Background(), platformName, platformName)
	assert.True(t, got.Spec.Database.Managed.UpgradeAcknowledged)
	assert.True(t, got.Spec.Messaging.Managed.UpgradeAcknowledged)
	assert.True(t, got.Spec.Keycloak.Managed.UpgradeAcknowledged)
}

func TestUpgrade_RequiresTo(t *testing.T) {
	p := platformWithVersion(platformVer2170, platformVer2170)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestUpgrade_DryRunDoesNotPatch(t *testing.T) {
	p := platformWithVersion(platformVer2170, platformVer2170)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2180, "--dry-run"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	cl, _ := o.Factory.Client()
	got, _ := cl.GetPlatform(context.Background(), platformName, platformName)
	assert.Equal(t, platformVer2170, got.Spec.Version, "dry-run must not patch")
	assert.Contains(t, out.String(), "would upgrade")
}

func TestUpgrade_RejectsSameVersion(t *testing.T) {
	p := platformWithVersion(platformVer2180, platformVer2180)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2180})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoDowngrade)
}

// unsupportedVersion is a dotted-numeric version that is not in the BOM's
// supported set. It must be numerically ahead of the fixture current version
// (2.18.0) so it does not trigger the no-downgrade guard before reaching the
// compat check.
const unsupportedVersion = "99.0.0"

func TestUpgrade_RejectsUnsupportedWithoutForce(t *testing.T) {
	p := platformWithVersion(platformVer2180, platformVer2180)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, unsupportedVersion})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "supported")

	// Spec.Version must NOT have been mutated.
	cl, _ := o.Factory.Client()
	got, _ := cl.GetPlatform(context.Background(), platformName, platformName)
	assert.Equal(t, platformVer2180, got.Spec.Version, "unsupported target without --force must not patch")
}

func TestUpgrade_UnsupportedWithForceSucceeds(t *testing.T) {
	p := platformWithVersion(platformVer2180, platformVer2180)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, unsupportedVersion, "--force"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	// Version must have been patched.
	cl, _ := o.Factory.Client()
	got, _ := cl.GetPlatform(context.Background(), platformName, platformName)
	assert.Equal(t, unsupportedVersion, got.Spec.Version)

	// A warning must have been written to ErrOut.
	errBuf, ok := o.ErrOut.(*bytes.Buffer)
	require.True(t, ok, "ErrOut must be *bytes.Buffer in tests")
	assert.Contains(t, errBuf.String(), "warning")
}

func TestUpgrade_DowngradeWithForceStillRejected(t *testing.T) {
	p := platformWithVersion(platformVer2180, platformVer2180)
	o, out := newFakeOptions(t, p)
	cmd := NewUpgradeCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, platformFlagTo, platformVer2170, "--force"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), errNoDowngrade)

	// Spec.Version must NOT have been mutated.
	cl, _ := o.Factory.Client()
	got, _ := cl.GetPlatform(context.Background(), platformName, platformName)
	assert.Equal(t, platformVer2180, got.Spec.Version, "--force must not override the no-downgrade guard")
}

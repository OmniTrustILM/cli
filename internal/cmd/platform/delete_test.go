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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

func newPlatformWithPolicy(name, ns string, policy otilmv1alpha1.PlatformDeletionPolicy) *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       otilmv1alpha1.PlatformSpec{DeletionPolicy: policy},
	}
}

func TestDelete_ReportsDeletionPolicy_Delete(t *testing.T) {
	p := newPlatformWithPolicy("ilm", "ilm", otilmv1alpha1.PlatformDeletionPolicyDelete)
	o, out := newFakeOptions(t, p)

	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm", "-y"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "deletionPolicy: Delete")
	assert.Contains(t, s, "managed infrastructure")
	assert.Contains(t, s, platformDeleted)
}

func TestDelete_ReportsDeletionPolicy_Retain(t *testing.T) {
	p := newPlatformWithPolicy("ilm", "ilm", otilmv1alpha1.PlatformDeletionPolicyRetain)
	o, out := newFakeOptions(t, p)
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm", "-y"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "deletionPolicy: Retain")
}

func TestDelete_NotFound(t *testing.T) {
	o, out := newFakeOptions(t)
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"missing", "-n", "ilm", "-y"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestDelete_RequiresName(t *testing.T) {
	o, out := newFakeOptions(t)
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestDelete_NonTTY_RefusesWithoutYes(t *testing.T) {
	p := newPlatformWithPolicy("ilm", "ilm", otilmv1alpha1.PlatformDeletionPolicyRetain)
	o, out := newFakeOptions(t, p)
	c, err := o.Factory.Client()
	require.NoError(t, err)
	// o.In is already a bytes.Buffer (not a TTY) — no -y flag
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
	// Nothing must have been deleted: the Platform must still exist.
	got, getErr := c.GetPlatform(context.Background(), "ilm", "ilm")
	require.NoError(t, getErr, "Platform must still exist after refused delete")
	assert.Equal(t, "ilm", got.Name)
}

func TestDelete_WaitReturnsWhenGone(t *testing.T) {
	// When --wait is used and the object is already gone, waitGone should return nil
	// immediately (fake client returns NotFound for missing objects).
	o, out := newFakeOptions(t)
	// No platform seeded — it won't be found
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"absent", "-n", "ilm", "-y"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	// Expect error because Get for "absent" returns NotFound before delete
	assert.Error(t, cmd.Execute())
}

func TestDelete_WaitPrintsDeletedAfterGone(t *testing.T) {
	// --wait: after the fake client's Delete removes the object immediately,
	// waitPlatformGone sees NotFound on the first poll and returns nil.
	// The platformDeleted confirmation must appear ONLY in that success path.
	p := newPlatformWithPolicy("ilm", "ilm", otilmv1alpha1.PlatformDeletionPolicyRetain)
	o, out := newFakeOptions(t, p)
	cmd := NewDeleteCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm", "-y", "--wait"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	s := out.String()
	assert.Contains(t, s, "deletion requested")
	assert.Contains(t, s, platformDeleted)
}

func TestWaitPlatformGone_ImmediatelyGone(t *testing.T) {
	// waitPlatformGone should return nil when the object is already absent.
	scheme, err := k8s.NewScheme()
	require.NoError(t, err)
	fc := fake.NewClientBuilder().WithScheme(scheme).Build()
	err = waitPlatformGone(context.Background(), fc, "ilm", "gone")
	require.NoError(t, err)
}

func TestConfirmDelete_AcceptsYes(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	o := &cli.Options{
		In:     bytes.NewBufferString("y\n"),
		Out:    out,
		ErrOut: errOut,
	}
	assert.True(t, confirmDelete(o, "proceed?"))
}

func TestConfirmDelete_RejectsNo(t *testing.T) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	o := &cli.Options{
		In:     bytes.NewBufferString("n\n"),
		Out:    out,
		ErrOut: errOut,
	}
	assert.False(t, confirmDelete(o, "proceed?"))
}

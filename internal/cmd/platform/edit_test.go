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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// fakeEditorScript writes a shell script that rewrites the buffer file to flip db mode.
func fakeEditorScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "editor.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsed -i.bak 's/mode: external/mode: managed/' \"$1\"\n"), 0o755))
	return script
}

func TestEdit_AppliesEditedBuffer(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec: otilmv1alpha1.PlatformSpec{
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
			Database:       otilmv1alpha1.DatabaseSpec{Mode: modeExternal},
			Messaging:      otilmv1alpha1.MessagingSpec{Mode: modeExternal},
		},
	}
	o, out := newFakeOptions(t, p)

	t.Setenv("EDITOR", fakeEditorScript(t))
	cmd := NewEditCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName, "--force-conflicts"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Platform/ilm")
}

func TestEdit_RequiresName(t *testing.T) {
	o, out := newFakeOptions(t)
	cmd := NewEditCommand(o)
	cmd.SetArgs([]string{})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestEdit_NoopWhenUnchanged(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec:       otilmv1alpha1.PlatformSpec{DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain},
	}
	o, out := newFakeOptions(t, p)

	// Editor that makes no change (just reads the file and exits).
	dir := t.TempDir()
	script := filepath.Join(dir, "noop.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("EDITOR", script)

	cmd := NewEditCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	// Should report no changes
	assert.Contains(t, out.String(), "no changes")
}

func TestResolveNamespace_FallsBackToFactory(t *testing.T) {
	o, _ := newFakeOptions(t)
	// flag is empty, Factory.Namespace() returns "default" for our test factory
	ns := resolveNamespace("", o)
	assert.Equal(t, "default", ns)
}

func TestResolveNamespace_UsesFlag(t *testing.T) {
	o, _ := newFakeOptions(t)
	ns := resolveNamespace("custom-ns", o)
	assert.Equal(t, "custom-ns", ns)
}

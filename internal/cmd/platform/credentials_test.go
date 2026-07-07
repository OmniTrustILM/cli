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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func TestCredentials_ResolvesCertAndPasswordRefs(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm"},
		Spec: otilmv1alpha1.PlatformSpec{
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled:  true,
				Username: "admin",
				Certificate: &otilmv1alpha1.AdminCertificateSpec{
					Enabled: boolPtr(true), Source: "provided", SecretRef: strPtr(platformAdminCert),
				},
				Password: &otilmv1alpha1.AdminPasswordSpec{
					Enabled: true, SecretRef: platformAdminPwd, PasswordKey: platformPassword,
				},
			},
		},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: platformAdminCert, Namespace: "ilm"},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{"tls.crt": []byte("PEM"), "tls.key": []byte("KEY")},
	}
	pwSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: platformAdminPwd, Namespace: "ilm"},
		Data:       map[string][]byte{platformPassword: []byte("s3cr3t")},
	}
	o, out := newFakeOptions(t, p, certSecret, pwSecret)

	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())

	s := out.String()
	assert.Contains(t, s, "certificate")
	assert.Contains(t, s, platformAdminCert)
	assert.Contains(t, s, platformPassword)
	assert.Contains(t, s, platformAdminPwd)
	// redacted by default: never print the secret value
	assert.NotContains(t, s, "s3cr3t")
	assert.Contains(t, s, "***")
}

func TestCredentials_NoRegisterAdmin(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm"},
		Spec:       otilmv1alpha1.PlatformSpec{DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain},
	}
	o, out := newFakeOptions(t, p)
	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "no registerAdmin")
}

func TestCredentials_MissingSecretReported(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: "ilm"},
		Spec: otilmv1alpha1.PlatformSpec{
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled:     true,
				Certificate: &otilmv1alpha1.AdminCertificateSpec{Enabled: boolPtr(true), Source: "provided", SecretRef: strPtr("absent")},
			},
		},
	}
	o, out := newFakeOptions(t, p)
	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{"ilm", "-n", "ilm"})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "not found")
}

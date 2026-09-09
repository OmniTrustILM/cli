/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec: otilmv1alpha1.PlatformSpec{
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled:  true,
				Username: "admin",
				Certificate: &otilmv1alpha1.AdminCertificateSpec{
					Enabled: boolPtr(true), Source: certSourceProvided, SecretRef: strPtr(platformAdminCert),
				},
				Password: &otilmv1alpha1.AdminPasswordSpec{
					Enabled: true, SecretRef: platformAdminPwd, PasswordKey: platformPassword,
				},
			},
		},
	}
	certSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: platformAdminCert, Namespace: platformName},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{"tls.crt": []byte("PEM"), "tls.key": []byte("KEY")},
	}
	pwSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: platformAdminPwd, Namespace: platformName},
		Data:       map[string][]byte{platformPassword: []byte("s3cr3t")},
	}
	o, out := newFakeOptions(t, p, certSecret, pwSecret)

	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName})
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
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec:       otilmv1alpha1.PlatformSpec{DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain},
	}
	o, out := newFakeOptions(t, p)
	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "no registerAdmin")
}

func TestCredentials_MissingSecretReported(t *testing.T) {
	p := &otilmv1alpha1.Platform{
		TypeMeta:   metav1.TypeMeta{APIVersion: platformAPIGroup, Kind: platformKind},
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: platformName},
		Spec: otilmv1alpha1.PlatformSpec{
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicyRetain,
			RegisterAdmin: &otilmv1alpha1.RegisterAdminSpec{
				Enabled:     true,
				Certificate: &otilmv1alpha1.AdminCertificateSpec{Enabled: boolPtr(true), Source: certSourceProvided, SecretRef: strPtr("absent")},
			},
		},
	}
	o, out := newFakeOptions(t, p)
	cmd := NewCredentialsCommand(o)
	cmd.SetArgs([]string{platformName, "-n", platformName})
	cmd.SetContext(context.Background())
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "not found")
}

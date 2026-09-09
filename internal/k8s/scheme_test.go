/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func TestNewScheme_RegistersAllGroups(t *testing.T) {
	s, err := NewScheme()
	require.NoError(t, err)

	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
	}{
		{"platform", otilmv1alpha1.GroupVersion.WithKind("Platform")},
		{"connector", otilmv1alpha1.GroupVersion.WithKind("Connector")},
		{"proxy", otilmv1alpha1.GroupVersion.WithKind("Proxy")},
		{"pod", corev1.SchemeGroupVersion.WithKind("Pod")},
		{"deployment", appsv1.SchemeGroupVersion.WithKind("Deployment")},
		{"poddisruptionbudget", policyv1.SchemeGroupVersion.WithKind("PodDisruptionBudget")},
		{"networkpolicy", networkingv1.SchemeGroupVersion.WithKind("NetworkPolicy")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, s.Recognizes(tt.gvk), "scheme should recognize %s", tt.gvk)
		})
	}
}

func TestNewScheme_RoundTripPlatform(t *testing.T) {
	s, err := NewScheme()
	require.NoError(t, err)
	obj, err := s.New(otilmv1alpha1.GroupVersion.WithKind("Platform"))
	require.NoError(t, err)
	_, ok := obj.(*otilmv1alpha1.Platform)
	assert.True(t, ok, "expected *v1alpha1.Platform")
}

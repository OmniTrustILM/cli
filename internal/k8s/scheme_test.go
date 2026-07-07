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

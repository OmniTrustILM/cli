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

// Package k8s provides the Kubernetes client surface (scheme, factory, typed +
// dynamic clients) the CLI's read/inspect/install code consumes.
package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewScheme builds a runtime.Scheme registering the otilm.com/v1alpha1 CRDs plus
// the core/apps/policy/networking groups the CLI reads operator-owned workloads from.
func NewScheme() (*runtime.Scheme, error) {
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme, // corev1, appsv1, policy/v1, networking/v1, ...
		otilmv1alpha1.AddToScheme,
		apiextv1.AddToScheme, // CustomResourceDefinition management
	} {
		if err := add(s); err != nil {
			return nil, err
		}
	}
	// Defensive: clientgoscheme already covers these, but assert membership so a
	// future client-go trim is caught by tests rather than at runtime.
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(appsv1.AddToScheme(s))
	utilruntime.Must(policyv1.AddToScheme(s))
	utilruntime.Must(networkingv1.AddToScheme(s))
	return s, nil
}

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

package bundle

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// forbiddenCRDClient wraps a controller-runtime client and returns a Forbidden
// error whenever a CustomResourceDefinitionList is listed, so the collector's
// graceful-degradation path can be exercised deterministically.
type forbiddenCRDClient struct {
	ctrlclient.Client
}

func (f forbiddenCRDClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	if _, ok := list.(*apiextv1.CustomResourceDefinitionList); ok {
		return apierrors.NewForbidden(
			schema.GroupResource{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
			"", nil,
		)
	}
	return f.Client.List(ctx, list, opts...)
}

// fatalCRDClient wraps a controller-runtime client and returns a non-RBAC
// error whenever a CustomResourceDefinitionList is listed, so the collector's
// fatal-error path can be exercised deterministically.
type fatalCRDClient struct {
	ctrlclient.Client
}

func (f fatalCRDClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	if _, ok := list.(*apiextv1.CustomResourceDefinitionList); ok {
		return fmt.Errorf("internal error: storage unavailable")
	}
	return f.Client.List(ctx, list, opts...)
}

// fatalNodeClient returns a non-RBAC error for NodeList so the
// collectClusterInfo fatal-propagation path can be exercised.
type fatalNodeClient struct {
	ctrlclient.Client
}

func (f fatalNodeClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	if _, ok := list.(*corev1.NodeList); ok {
		return fmt.Errorf("internal error: nodes unavailable")
	}
	return f.Client.List(ctx, list, opts...)
}

// fatalPlatformClient returns a non-RBAC error for PlatformList so the
// collectConfigAndState fatal-propagation path can be exercised.
type fatalPlatformClient struct {
	ctrlclient.Client
}

func (f fatalPlatformClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	if _, ok := list.(*otilmv1alpha1.PlatformList); ok {
		return fmt.Errorf("internal error: platform store unavailable")
	}
	return f.Client.List(ctx, list, opts...)
}

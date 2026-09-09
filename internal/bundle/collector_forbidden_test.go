/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

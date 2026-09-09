/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package k8s

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Factory builds clients from genericclioptions; the single seam fakes inject in
// tests. The Factory struct is defined here so it remains the canonical type
// declaration; methods (NewFactory, RESTConfig, Client, Namespace) live in client.go.
type Factory struct {
	ConfigFlags *genericclioptions.ConfigFlags
	Scheme      *runtime.Scheme
	// fixedClient, when non-nil, is returned directly by Client() without any
	// cluster connection. Set only via NewFactoryWithClient for hermetic tests.
	fixedClient *Client
}

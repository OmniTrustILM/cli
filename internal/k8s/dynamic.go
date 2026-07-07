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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Managed-infra GroupVersionResources the CLI reports on. They are read via the
// dynamic client (status only) so the CLI takes no compile dependency on the CNPG,
// RabbitMQ, or Keycloak Go modules.
var (
	GVRCNPGCluster     = schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}
	GVRRabbitmqCluster = schema.GroupVersionResource{Group: "rabbitmq.com", Version: "v1beta1", Resource: "rabbitmqclusters"}
	GVRKeycloak        = schema.GroupVersionResource{Group: "k8s.keycloak.org", Version: "v2alpha1", Resource: "keycloaks"}
)

// ForeignStatus reads only the .status of a foreign CR as a generic map. A missing
// .status returns an empty (non-nil) map and no error. Foreign CRs are read via the
// dynamic client, so the CLI carries no compile-time dependency on CNPG, RabbitMQ,
// or Keycloak Go modules.
func (c *Client) ForeignStatus(ctx context.Context, gvr schema.GroupVersionResource, ns, name string) (map[string]any, error) {
	u, err := c.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	status, found := nestedMap(u.Object, "status")
	if !found || status == nil {
		return map[string]any{}, nil
	}
	return status, nil
}

// nestedMap retrieves a nested map[string]any from an unstructured object without
// importing the unstructured helpers at call sites. Returns (nil, false) when the
// field is absent or not a map.
func nestedMap(obj map[string]any, field string) (map[string]any, bool) {
	v, ok := obj[field]
	if !ok {
		return nil, false
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	return m, true
}

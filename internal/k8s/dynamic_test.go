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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGVRConstants(t *testing.T) {
	assert.Equal(t, schema.GroupVersionResource{Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters"}, GVRCNPGCluster)
	assert.Equal(t, schema.GroupVersionResource{Group: "rabbitmq.com", Version: "v1beta1", Resource: "rabbitmqclusters"}, GVRRabbitmqCluster)
	assert.Equal(t, schema.GroupVersionResource{Group: "k8s.keycloak.org", Version: "v2alpha1", Resource: "keycloaks"}, GVRKeycloak)
}

func foreignCR(apiVersion, kind, ns, name string, status map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"namespace": ns, "name": name},
	}}
	if status != nil {
		u.Object["status"] = status
	}
	return u
}

func TestForeignStatus(t *testing.T) {
	tests := []struct {
		name    string
		gvr     schema.GroupVersionResource
		seed    *unstructured.Unstructured
		ns      string
		objName string
		want    map[string]any
		wantErr bool
	}{
		{
			name:    "cnpg cluster status",
			gvr:     GVRCNPGCluster,
			seed:    foreignCR("postgresql.cnpg.io/v1", "Cluster", k8sILM, "ilm-db", map[string]any{"phase": "Cluster in healthy state", "readyInstances": int64(3)}),
			ns:      k8sILM,
			objName: "ilm-db",
			want:    map[string]any{"phase": "Cluster in healthy state", "readyInstances": int64(3)},
		},
		{
			name:    "rabbitmq cluster missing status returns empty map",
			gvr:     GVRRabbitmqCluster,
			seed:    foreignCR("rabbitmq.com/v1beta1", "RabbitmqCluster", k8sILM, "ilm-mq", nil),
			ns:      k8sILM,
			objName: "ilm-mq",
			want:    map[string]any{},
		},
		{
			name:    "keycloak not found",
			gvr:     GVRKeycloak,
			seed:    foreignCR("k8s.keycloak.org/v2alpha1", "Keycloak", k8sILM, "present", map[string]any{"ready": true}),
			ns:      k8sILM,
			objName: "absent",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewFakeClient(t, FakeClientOptions{Dynamic: []runtime.Object{tt.seed}})
			got, err := c.ForeignStatus(context.Background(), tt.gvr, tt.ns, tt.objName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

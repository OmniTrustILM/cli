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

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScaffoldProxy_Defaults(t *testing.T) {
	p, notes, err := ScaffoldProxy(ProxyOptions{
		Name: genEgress, Namespace: genILM, ConfigTokenSecret: genEgressConfigToken,
	})
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "Proxy", p.Kind)
	assert.Equal(t, "otilm.com/v1alpha1", p.APIVersion)
	assert.Equal(t, genEgress, p.Name)
	assert.Equal(t, genEgressConfigToken, p.Spec.ConfigTokenSecretRef.Name)
	assert.Nil(t, p.Spec.Image)
	assert.NotEmpty(t, notes)
}

func TestScaffoldProxy_ImageAndReplicas(t *testing.T) {
	p, _, err := ScaffoldProxy(ProxyOptions{
		Name: genEgress, Namespace: genILM, ConfigTokenSecret: genEgressConfigToken,
		Image: "harbor.example.com/ilm/proxy:2.18.0", Replicas: int32Ptr(3),
	})
	require.NoError(t, err)
	require.NotNil(t, p.Spec.Image)
	assert.Equal(t, "harbor.example.com/ilm", p.Spec.Image.Repository)
	assert.Equal(t, "proxy", p.Spec.Image.Name)
	assert.Equal(t, "2.18.0", p.Spec.Image.Tag)
	require.NotNil(t, p.Spec.Replicas)
	assert.Equal(t, int32(3), *p.Spec.Replicas)
}

func TestScaffoldProxy_Validation(t *testing.T) {
	tests := []struct {
		name string
		o    ProxyOptions
	}{
		{genEmptyName, ProxyOptions{Name: "", Namespace: genILM, ConfigTokenSecret: "t"}},
		{"empty config-token-secret", ProxyOptions{Name: "p", Namespace: genILM, ConfigTokenSecret: ""}},
		{"bad image", ProxyOptions{Name: "p", Namespace: genILM, ConfigTokenSecret: "t", Image: "noTag"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ScaffoldProxy(tc.o)
			assert.Error(t, err)
		})
	}
}

func TestScaffoldProxy_TypeMeta(t *testing.T) {
	p, _, err := ScaffoldProxy(ProxyOptions{
		Name: "prx", Namespace: "ns", ConfigTokenSecret: "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "Proxy", p.Kind)
	assert.Equal(t, "otilm.com/v1alpha1", p.APIVersion)
	assert.Equal(t, "prx", p.Name)
	assert.Equal(t, "ns", p.Namespace)
}

func TestScaffoldProxy_NoImage(t *testing.T) {
	p, notes, err := ScaffoldProxy(ProxyOptions{
		Name: "p", Namespace: genILM, ConfigTokenSecret: "tok",
	})
	require.NoError(t, err)
	assert.Nil(t, p.Spec.Image)
	assert.Nil(t, p.Spec.Replicas)
	assert.NotEmpty(t, notes)
}

func TestScaffoldProxy_ConfigTokenSecretRefNotes(t *testing.T) {
	_, notes, err := ScaffoldProxy(ProxyOptions{
		Name: "p", Namespace: genILM, ConfigTokenSecret: "my-secret",
	})
	require.NoError(t, err)
	var found bool
	for _, n := range notes {
		if n.Field == "configTokenSecretRef.name" && n.Value == "my-secret" {
			found = true
		}
	}
	assert.True(t, found, "expected note for configTokenSecretRef.name")
}

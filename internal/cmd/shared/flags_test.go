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

package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentSelector(t *testing.T) {
	sel := ComponentSelector("ilm", componentCore)
	assert.Equal(t, map[string]string{
		"app.kubernetes.io/name":     componentCore,
		"app.kubernetes.io/instance": "ilm",
	}, sel)
}

func TestPlatformLogComponents_RealNames(t *testing.T) {
	// Must match the operator's real Deployment component names exactly.
	assert.Equal(t, []string{
		componentCore, "auth", "auth-opa-policies", "scheduler",
		"fe-administrator", "utils", "api-gateway", "provisioning-rabbitmq",
	}, PlatformLogComponents)
}

func TestResolveNamespace_AllNamespaces(t *testing.T) {
	// allNamespaces=true short-circuits before touching the factory; nil is safe.
	got, err := ResolveNamespace(nil, true, nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{""}, got)
}

func TestResolveNamespace_Explicit(t *testing.T) {
	// Explicit list short-circuits before touching the factory; nil is safe.
	got, err := ResolveNamespace(nil, false, []string{"ns-a", "ns-b"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"ns-a", "ns-b"}, got)
}

func TestResolveNamespace_ExplicitTakesPrecedenceOverNil(t *testing.T) {
	// Even with allNamespaces=false and a nil factory, an explicit list is
	// returned without a nil-pointer dereference.
	got, err := ResolveNamespace(nil, false, []string{"only-ns"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"only-ns"}, got)
}

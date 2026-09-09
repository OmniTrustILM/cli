/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

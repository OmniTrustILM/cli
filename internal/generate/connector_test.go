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

func int32Ptr(i int32) *int32 { return &i }

func TestScaffoldConnector_FullImageAndRegistration(t *testing.T) {
	c, notes, err := ScaffoldConnector(ConnectorOptions{
		Name:        genCryptosense,
		Namespace:   "ilm",
		Image:       "harbor.example.com/ilm/connector-cryptosense:1.4.0",
		PlatformURL: genILMURL,
		RegName:     genCryptosense,
		AuthType:    genAPIKey,
		Replicas:    int32Ptr(2),
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "Connector", c.Kind)
	assert.Equal(t, "otilm.com/v1alpha1", c.APIVersion)
	assert.Equal(t, genCryptosense, c.Name)
	assert.Equal(t, "harbor.example.com/ilm", c.Spec.Image.Repository)
	assert.Equal(t, "connector-cryptosense", c.Spec.Image.Name)
	assert.Equal(t, "1.4.0", c.Spec.Image.Tag)
	require.NotNil(t, c.Spec.Replicas)
	assert.Equal(t, int32(2), *c.Spec.Replicas)
	require.NotNil(t, c.Spec.Registration)
	assert.Equal(t, genILMURL, c.Spec.Registration.PlatformURL)
	assert.Equal(t, genCryptosense, c.Spec.Registration.Name)
	assert.Equal(t, genAPIKey, string(c.Spec.Registration.AuthType))
	assert.NotEmpty(t, notes)
}

func TestScaffoldConnector_SimpleRepoTag(t *testing.T) {
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "demo", Namespace: "ilm", Image: "connector-demo:2.0.0", AuthType: "none",
	})
	require.NoError(t, err)
	assert.Equal(t, genConnectorDemo, c.Spec.Image.Repository)
	assert.Equal(t, "2.0.0", c.Spec.Image.Tag)
	// no registration flags => no registration block
	assert.Nil(t, c.Spec.Registration)
}

func TestScaffoldConnector_NoRegistrationWhenAuthTypeOnlyNone(t *testing.T) {
	// AuthType "none" with no PlatformURL/RegName => registration nil (not an error)
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "none",
	})
	require.NoError(t, err)
	assert.Nil(t, c.Spec.Registration)
}

func TestScaffoldConnector_Validation(t *testing.T) {
	tests := []struct {
		name string
		o    ConnectorOptions
	}{
		{"empty name", ConnectorOptions{Name: "", Namespace: "ilm", Image: "x:1", AuthType: "none"}},
		{"empty image", ConnectorOptions{Name: "c", Namespace: "ilm", Image: "", AuthType: "none"}},
		{"image without tag", ConnectorOptions{Name: "c", Namespace: "ilm", Image: genConnectorDemo, AuthType: "none"}},
		{"bad auth-type", ConnectorOptions{Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "kerberos"}},
		{"registration name without platform-url", ConnectorOptions{Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "none", RegName: "c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ScaffoldConnector(tc.o)
			assert.Error(t, err)
		})
	}
}

func TestScaffoldConnector_AllAuthTypes(t *testing.T) {
	validTypes := []string{"none", "basic", "certificate", genAPIKey, "jwt"}
	for _, at := range validTypes {
		t.Run(at, func(t *testing.T) {
			_, _, err := ScaffoldConnector(ConnectorOptions{
				Name: "c", Namespace: "ilm", Image: "img:1",
				PlatformURL: genILMURL, RegName: "c", AuthType: at,
			})
			require.NoError(t, err)
		})
	}
}

func TestScaffoldConnector_TypeMeta(t *testing.T) {
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "demo", Namespace: "test", Image: "img:1", AuthType: "none",
	})
	require.NoError(t, err)
	assert.Equal(t, "Connector", c.Kind)
	assert.Equal(t, "otilm.com/v1alpha1", c.APIVersion)
	assert.Equal(t, "demo", c.Name)
	assert.Equal(t, "test", c.Namespace)
}

func TestScaffoldConnector_ReplicasPointer(t *testing.T) {
	// no replicas => nil pointer
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "none",
	})
	require.NoError(t, err)
	assert.Nil(t, c.Spec.Replicas)

	// with replicas
	c2, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "none", Replicas: int32Ptr(3),
	})
	require.NoError(t, err)
	require.NotNil(t, c2.Spec.Replicas)
	assert.Equal(t, int32(3), *c2.Spec.Replicas)
}

func TestScaffoldConnector_FullRegistrationWithPlatformURL(t *testing.T) {
	// PlatformURL without RegName should error
	_, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "c", Namespace: "ilm", Image: "x:1", AuthType: "none",
		PlatformURL: genILMURL,
	})
	assert.Error(t, err)
}

func TestScaffoldConnector_DigestWithTag_NoSlash(t *testing.T) {
	// "name:tag@sha256:<hex>" — no registry prefix
	const digest = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	c, notes, err := ScaffoldConnector(ConnectorOptions{
		Name: "foo", Namespace: "ilm",
		Image:    "connector-foo:1.0@" + digest,
		AuthType: "none",
	})
	require.NoError(t, err)
	assert.Equal(t, "connector-foo", c.Spec.Image.Repository)
	assert.Equal(t, "", c.Spec.Image.Name)
	assert.Equal(t, "1.0", c.Spec.Image.Tag)
	assert.Equal(t, digest, c.Spec.Image.Digest)
	var digestNote bool
	for _, n := range notes {
		if n.Field == "image.digest" {
			digestNote = true
			assert.Equal(t, digest, n.Value)
		}
	}
	assert.True(t, digestNote, "expected image.digest note")
}

func TestScaffoldConnector_DigestWithTag_FullRegistry(t *testing.T) {
	// "harbor.example.com/ilm/connector-foo:tag@sha256:<hex>"
	const digest = "sha256:deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "foo", Namespace: "ilm",
		Image:    "harbor.example.com/ilm/connector-foo:2.3.1@" + digest,
		AuthType: "none",
	})
	require.NoError(t, err)
	assert.Equal(t, "harbor.example.com/ilm", c.Spec.Image.Repository)
	assert.Equal(t, "connector-foo", c.Spec.Image.Name)
	assert.Equal(t, "2.3.1", c.Spec.Image.Tag)
	assert.Equal(t, digest, c.Spec.Image.Digest)
}

func TestScaffoldConnector_DigestWithTag_RegistryPort(t *testing.T) {
	// "host:5000/repo/name:tag@sha256:<hex>" — registry port must not be mistaken for tag
	const digest = "sha256:cafecafecafecafecafecafecafecafecafecafecafecafecafecafecafecafe"
	c, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "foo", Namespace: "ilm",
		Image:    "host:5000/repo/name:stable@" + digest,
		AuthType: "none",
	})
	require.NoError(t, err)
	assert.Equal(t, "host:5000/repo", c.Spec.Image.Repository)
	assert.Equal(t, "name", c.Spec.Image.Name)
	assert.Equal(t, "stable", c.Spec.Image.Tag)
	assert.Equal(t, digest, c.Spec.Image.Digest)
}

func TestScaffoldConnector_DigestOnly_Rejected(t *testing.T) {
	// A digest-only ref (no tag) must error because the Connector CRD
	// XValidation requires has(self.tag). Caller must also supply a tag.
	const digest = "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	_, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "foo", Namespace: "ilm",
		Image:    "connector-foo@" + digest,
		AuthType: "none",
	})
	assert.Error(t, err)
}

func TestScaffoldConnector_NoTagNoDigest_Rejected(t *testing.T) {
	// Neither tag nor digest should still error.
	_, _, err := ScaffoldConnector(ConnectorOptions{
		Name: "c", Namespace: "ilm", Image: genConnectorDemo, AuthType: "none",
	})
	assert.Error(t, err)
}

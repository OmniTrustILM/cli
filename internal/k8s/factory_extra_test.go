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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// TestNewFactoryWithClient_ReturnsFakeClientDirectly verifies that
// NewFactoryWithClient bypasses cluster connection and returns the provided
// Client unchanged via Client().
func TestNewFactoryWithClient_ReturnsFakeClientDirectly(t *testing.T) {
	t.Parallel()
	fake := NewFakeClient(t, FakeClientOptions{})
	f := NewFactoryWithClient(fake)
	require.NotNil(t, f)

	got, err := f.Client()
	require.NoError(t, err)
	assert.Same(t, fake, got, "NewFactoryWithClient must return the provided Client via Client()")
}

// TestNewFactoryWithClient_SchemeMatchesClient verifies that the Factory's
// Scheme field is set to the Client's Scheme.
func TestNewFactoryWithClient_SchemeMatchesClient(t *testing.T) {
	t.Parallel()
	fake := NewFakeClient(t, FakeClientOptions{})
	f := NewFactoryWithClient(fake)
	assert.Same(t, fake.Scheme, f.Scheme)
}

// TestNewFactoryWithClient_NamespaceDefaultsToDefault verifies that Namespace()
// returns "default" when ConfigFlags is nil (NewFactoryWithClient does not set it).
func TestNewFactoryWithClient_NamespaceDefaultsToDefault(t *testing.T) {
	t.Parallel()
	fake := NewFakeClient(t, FakeClientOptions{})
	f := NewFactoryWithClient(fake)
	ns, explicit, err := f.Namespace()
	require.NoError(t, err)
	assert.Equal(t, "default", ns)
	assert.False(t, explicit)
}

// TestFactory_RESTConfig_ErrorPath verifies that RESTConfig surfaces errors
// returned by the injected test seam.
func TestFactory_RESTConfig_ErrorPath(t *testing.T) {
	old := restConfigForTest
	restConfigForTest = func(_ *Factory) (*rest.Config, error) {
		return nil, errors.New("synthetic config error")
	}
	t.Cleanup(func() { restConfigForTest = old })

	s, err := NewScheme()
	require.NoError(t, err)
	f := &Factory{Scheme: s}
	_, err = f.RESTConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "synthetic config error")
}

// TestFactory_Client_RESTConfigError verifies that Client() propagates an error
// from RESTConfig.
func TestFactory_Client_RESTConfigError(t *testing.T) {
	old := restConfigForTest
	restConfigForTest = func(_ *Factory) (*rest.Config, error) {
		return nil, errors.New("config error")
	}
	t.Cleanup(func() { restConfigForTest = old })

	s, err := NewScheme()
	require.NoError(t, err)
	f := &Factory{Scheme: s}
	_, err = f.Client()
	require.Error(t, err)
}

// TestFactory_Client_MapperError verifies that Client() propagates an error
// from the mapper construction seam.
func TestFactory_Client_MapperError(t *testing.T) {
	oldCfg := restConfigForTest
	restConfigForTest = func(_ *Factory) (*rest.Config, error) {
		return &rest.Config{Host: "https://fake.local:6443"}, nil
	}
	t.Cleanup(func() { restConfigForTest = oldCfg })

	oldMapper := restMapperForTest
	restMapperForTest = func(_ *Factory) (meta.RESTMapper, error) {
		return nil, errors.New("mapper error")
	}
	t.Cleanup(func() { restMapperForTest = oldMapper })

	s, err := NewScheme()
	require.NoError(t, err)
	f := &Factory{Scheme: s}
	_, err = f.Client()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mapper error")
}

// TestClient_Events_EmptyUID verifies that an event whose InvolvedObject UID is
// empty still matches when the name matches, exercising the "ref.UID == """ branch.
func TestClient_Events_EmptyUID(t *testing.T) {
	t.Parallel()
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ilm", Name: "alpha"},
	}
	plat.UID = "uid-1"

	// Event with no UID set on the InvolvedObject — must match by name alone.
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "ilm", Name: "ev-name-only"},
		InvolvedObject: corev1.ObjectReference{Namespace: "ilm", Name: "alpha", UID: ""},
		Reason:         "NoUIDMatch",
	}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{plat, ev}})
	events, err := c.Events(context.Background(), "ilm", plat)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "NoUIDMatch", events[0].Reason)
}

// TestClient_Events_WrongUID verifies that an event with a different
// InvolvedObject UID is excluded even when the name matches.
func TestClient_Events_WrongUID(t *testing.T) {
	t.Parallel()
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ilm", Name: "alpha"},
	}
	plat.UID = "uid-correct"

	// Event name matches but UID differs — must be excluded.
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "ilm", Name: "ev-wrong-uid"},
		InvolvedObject: corev1.ObjectReference{Namespace: "ilm", Name: "alpha", UID: "uid-wrong"},
	}
	c := NewFakeClient(t, FakeClientOptions{Objects: []ctrlclient.Object{plat, ev}})
	events, err := c.Events(context.Background(), "ilm", plat)
	require.NoError(t, err)
	assert.Empty(t, events, "event with wrong UID must be excluded")
}

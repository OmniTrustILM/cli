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

package platform

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunDescribe_ResolvedEndpointsAndMQ(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Enabled: true, Host: platformILMDomain}
	plat.Spec.Messaging.Mode = modeManaged
	plat.Spec.Messaging.BrokerType = "rabbitmq"
	plat.Spec.Messaging.Management.Expose = true
	plat.Spec.Keycloak = &otilmv1alpha1.KeycloakSpec{Mode: modeManaged}
	plat.Status.Phase = otilmv1alpha1.PlatformPhaseRunning
	plat.Status.Conditions = []metav1.Condition{{Type: "Available", Status: metav1.ConditionTrue, Message: "all ready"}}

	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	s := out.String()
	assert.Contains(t, s, "https://ilm.example.com")
	assert.Contains(t, s, "https://ilm.example.com/mq") // managed RabbitMQ management UI
	assert.Contains(t, s, "Keycloak:  mode=managed")    // third managed backing service
	assert.Contains(t, s, "Available")
	assert.Contains(t, s, "all ready")
}

// TestRunDescribe_KeycloakNoneWhenAbsent verifies the header shows mode=none when
// spec.keycloak is nil (no Keycloak configured), consistent with Database/Messaging.
func TestRunDescribe_KeycloakNoneWhenAbsent(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	// no spec.keycloak set
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	assert.Contains(t, out.String(), "Keycloak:  mode=none")
}

func TestRunDescribe_ExternalMessagingNoMQ(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Enabled: true, Host: platformILMDomain}
	plat.Spec.Messaging.Mode = "external" // no /mq route for an external broker
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	assert.NotContains(t, out.String(), "/mq")
}

func TestRunDescribe_CommonHostNameFallback(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Common.HostName = "fallback.example.com"
	// no edge block set → should fall back to spec.common.hostName
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	assert.Contains(t, out.String(), "https://fallback.example.com")
}

func TestRunDescribe_NoHostConfigured(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	assert.Contains(t, out.String(), "<no public host configured>")
}

func TestResolveEndpoints_ManagedNotExposed(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Enabled: true, Host: platformILMDomain}
	plat.Spec.Messaging.Mode = modeManaged
	plat.Spec.Messaging.BrokerType = "rabbitmq"
	plat.Spec.Messaging.Management.Expose = false // management not exposed

	eps := resolveEndpoints(plat)
	require.Len(t, eps, 1)
	assert.Equal(t, "admin-ui", eps[0].label)
	assert.NotContains(t, eps[0].url, "/mq")
}

// TestRunDescribe_JSONStructured covers the -o json path of describe.
func TestRunDescribe_JSONStructured(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Host: platformILMDomain}
	c := newPlatformClient(t, plat)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	assert.Contains(t, out.String(), "ilm")
}

// TestRunEvents_WithEvents covers the events table rendering path.
func TestRunEvents_WithEvents(t *testing.T) {
	plat := newPlatform("ilm", testNS)
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev1", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{Kind: "Platform", Name: "ilm"},
		Type:           "Warning",
		Reason:         "Degraded",
		Message:        "something went wrong",
	}
	c := newPlatformClient(t, plat, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runEvents(context.Background(), c, p, testNS, "ilm"))
	s := out.String()
	assert.Contains(t, s, "Warning")
	assert.Contains(t, s, "Degraded")
	assert.Contains(t, s, "something went wrong")
}

// TestRunDescribe_WithChildDeployments covers the child-deployments section.
func TestRunDescribe_WithChildDeployments(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	plat.Spec.Edge = &otilmv1alpha1.EdgeSpec{Host: platformILMDomain}
	plat.Status.Phase = otilmv1alpha1.PlatformPhaseRunning

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ilm-core", Namespace: testNS,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ilm-operator",
				"otilm.com/platform":           "ilm",
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 1, ReadyReplicas: 1, AvailableReplicas: 1,
		},
	}
	c := newPlatformClient(t, plat, dep)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	s := out.String()
	assert.Contains(t, s, "ilm-core")
	assert.Contains(t, s, "1/1")
}

// TestRunDescribe_WithEvents covers the events section in describe.
func TestRunDescribe_WithEvents(t *testing.T) {
	plat := &otilmv1alpha1.Platform{ObjectMeta: metav1.ObjectMeta{Name: "ilm", Namespace: testNS}}
	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev1", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{Kind: "Platform", Name: "ilm"},
		Type:           "Normal",
		Reason:         "Created",
		Message:        "platform created",
	}
	c := newPlatformClient(t, plat, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "ilm"))
	s := out.String()
	assert.Contains(t, s, "platform created")
}

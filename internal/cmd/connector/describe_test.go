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

package connector

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/render"
)

// TestRunDescribe_WithConditions verifies that the conditions table is rendered
// when Status.Conditions is non-empty.
func TestRunDescribe_WithConditions(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning,
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "AllReady", Message: "everything ok"},
			},
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "Ready")
	assert.Contains(t, s, "AllReady")
}

// TestConnectorPodSelector verifies the selector matches the operator's real
// connector pod label (otilm.com/connector=<name>), not the app.kubernetes.io
// labels that matched nothing on a live cluster.
func TestConnectorPodSelector(t *testing.T) {
	assert.Equal(t, map[string]string{"otilm.com/connector": "c1"}, connectorPodSelector("c1"))
}

// TestRunDescribe_WithPods verifies that the pods table is rendered when pods
// carrying the operator's real connector labels exist. The pod is seeded with
// the labels the operator actually applies (otilm.com/connector plus the
// app.kubernetes.io labels), proving the selector finds real workload pods.
func TestRunDescribe_WithPods(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-pod-xyz", Namespace: testNS, Labels: map[string]string{
			"otilm.com/connector":         "c1",
			"app.kubernetes.io/name":      "c1",
			"app.kubernetes.io/component": "connector",
		}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	c := newConnectorClient(t, conn, pod)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "c1-pod-xyz")
}

// TestRunDescribe_WithEvents verifies the events table in describe is rendered
// when events for the connector exist. k8s.Client.Events does an in-process
// name-match (no field selector), so seeding a matching Event is sufficient.
func TestRunDescribe_WithEvents(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-evt-1", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Connector", Name: "c1", Namespace: testNS,
		},
		Type:    corev1.EventTypeNormal,
		Reason:  "Registered",
		Message: "connector registered successfully",
	}
	c := newConnectorClient(t, conn, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "Registered")
	assert.Contains(t, s, "connector registered successfully")
}

// TestRunDescribe_JSONStructured exercises the -o json path in runDescribe.
func TestRunDescribe_JSONStructured(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runDescribe(context.Background(), c, p, testNS, "c1"))
	assert.Contains(t, out.String(), "c1")
}

// TestRunEvents_WithEvents verifies the events table in the standalone events
// command is rendered when events for the connector exist.
func TestRunEvents_WithEvents(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status:     otilmv1alpha1.ConnectorStatus{Phase: otilmv1alpha1.ConnectorPhaseRunning},
	}
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "c1-evt-2", Namespace: testNS},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Connector", Name: "c1", Namespace: testNS,
		},
		Type:    corev1.EventTypeWarning,
		Reason:  "RegistrationFailed",
		Message: "platform unreachable",
	}
	c := newConnectorClient(t, conn, ev)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runEvents(context.Background(), c, p, testNS, "c1"))
	s := out.String()
	assert.Contains(t, s, "RegistrationFailed")
	assert.Contains(t, s, "platform unreachable")
}

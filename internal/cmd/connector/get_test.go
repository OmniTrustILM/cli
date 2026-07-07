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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunGet_ListWide(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning, ReadyReplicas: 1, Replicas: 1,
			Endpoint: "https://connector.example.com",
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "c1")
	assert.Contains(t, s, "Running")
	assert.Contains(t, s, "https://connector.example.com")
	assert.Contains(t, s, "NAMESPACE")
	assert.Contains(t, s, "PHASE")
}

func TestRunGet_ListWithRegistration(t *testing.T) {
	conn := &otilmv1alpha1.Connector{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: testNS},
		Status: otilmv1alpha1.ConnectorStatus{
			Phase: otilmv1alpha1.ConnectorPhaseRunning,
			Registration: &otilmv1alpha1.RegistrationStatus{
				UUID:   "uuid-abc",
				Status: otilmv1alpha1.RegistrationStatusConnected,
			},
		},
	}
	c := newConnectorClient(t, conn)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "connected")
}

func TestRunGet_ListNoRegistration(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	assert.Contains(t, out.String(), "<not registered>")
}

func TestRunGet_SingleNotFound(t *testing.T) {
	c := newConnectorClient(t)
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	assert.Error(t, runGet(context.Background(), c, p, testNS, "absent"))
}

func TestRunGet_Multiple(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS), newConnector("c2", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	s := out.String()
	assert.Contains(t, s, "c1")
	assert.Contains(t, s, "c2")
}

func TestRunGet_JSONSingle(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, "c1"))
	assert.Contains(t, out.String(), "c1")
}

func TestRunGet_JSONList(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	var out bytes.Buffer
	p := render.NewPrinter(&out, &bytes.Buffer{})
	p.Color = render.ColorNever
	*p.FormatPtrForTest() = fmtJSON
	require.NoError(t, runGet(context.Background(), c, p, testNS, ""))
	assert.Contains(t, out.String(), "ConnectorList")
}

// TestNewGetCommandFromOpts_RunE_ListWide exercises the RunE closure via clientFn injection.
func TestNewGetCommandFromOpts_RunE_ListWide(t *testing.T) {
	c := newConnectorClient(t, newConnector("c1", testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newGetCommandFromOpts(o, opts)
	require.NoError(t, cmd.RunE(cmd, []string{}))
	assert.Contains(t, out.String(), "c1")
}

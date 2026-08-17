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

package cmdutil_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestOrNone(t *testing.T) {
	assert.Equal(t, "<none>", cmdutil.OrNone(""))
	assert.Equal(t, "foo", cmdutil.OrNone("foo"))
}

func TestAge(t *testing.T) {
	assert.Equal(t, "<unknown>", cmdutil.Age(metav1.Time{}.Time))
	assert.Contains(t, cmdutil.Age(time.Now().Add(-10*time.Second)), "s")
	assert.Contains(t, cmdutil.Age(time.Now().Add(-5*time.Minute)), "m")
	assert.Contains(t, cmdutil.Age(time.Now().Add(-3*time.Hour)), "h")
	assert.Contains(t, cmdutil.Age(time.Now().Add(-48*time.Hour)), "d")
}

func TestOtilmGVK(t *testing.T) {
	gvk := cmdutil.OtilmGVK("Platform")
	assert.Equal(t, "otilm.com", gvk.Group)
	assert.Equal(t, "v1alpha1", gvk.Version)
	assert.Equal(t, "Platform", gvk.Kind)
}

func TestNewSingleNameCommand_RunE(t *testing.T) {
	var called bool
	fn := cmdutil.RunFn(func(_ context.Context, _ *k8s.Client, _ *render.Printer, _, _ string) error {
		called = true
		return nil
	})
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return &k8s.Client{}, nil },
		NamespaceFn: func() (string, bool, error) { return testNamespace, true, nil },
	}
	p := render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	o := &cli.Options{Printer: p}
	cmd := cmdutil.NewSingleNameCommand(o, opts, "test NAME", "test cmd", fn)
	require.NoError(t, cmd.RunE(cmd, []string{"myname"}))
	assert.True(t, called)
}

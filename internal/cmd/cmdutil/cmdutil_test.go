/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

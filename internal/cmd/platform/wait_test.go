/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/cmdutil"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestRunWait_ConditionMet(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	target, err := shared.ParseWaitFor("condition=Available")
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, platformName, target, time.Second))
}

func TestRunWait_PhaseTimeout(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: testNS},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseProgressing},
	}
	c := newPlatformClient(t, plat)
	target, _ := shared.ParseWaitFor("phase=Running")
	err := runWait(context.Background(), c, testNS, platformName, target, 150*time.Millisecond)
	assert.ErrorIs(t, err, shared.ErrWaitTimeout)
}

func TestRunWait_PhaseMet(t *testing.T) {
	plat := &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: platformName, Namespace: testNS},
		Status:     otilmv1alpha1.PlatformStatus{Phase: otilmv1alpha1.PlatformPhaseRunning},
	}
	c := newPlatformClient(t, plat)
	target, err := shared.ParseWaitFor("phase=Running")
	require.NoError(t, err)
	require.NoError(t, runWait(context.Background(), c, testNS, platformName, target, time.Second))
}

// TestNewWaitCommandFromOpts_RunE exercises the wait RunE via clientFn injection.
func TestNewWaitCommandFromOpts_RunE(t *testing.T) {
	c := newPlatformClient(t, newPlatform(platformName, testNS))
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := render.NewPrinter(out, errOut)
	p.Color = render.ColorNever
	o := &cli.Options{Printer: p}
	opts := &cmdutil.SingleNameOpts{
		ClientFn:    func() (*k8s.Client, error) { return c, nil },
		NamespaceFn: func() (string, bool, error) { return testNS, true, nil },
	}
	cmd := newWaitCommandFromOpts(o, opts)
	require.NoError(t, cmd.Flags().Set("for", "condition=Available"))
	require.NoError(t, cmd.Flags().Set("timeout", "2s"))
	require.NoError(t, cmd.RunE(cmd, []string{platformName}))
	assert.Contains(t, out.String(), "met condition=Available")
}

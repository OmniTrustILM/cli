/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package shared

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	phaseRunning      = "Running"
	condAvailable     = "Available"
	waitCondAvailable = "condition=Available"
	condKind          = "condition"
)

func TestParseWaitFor(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    WaitTarget
		wantErr bool
	}{
		{condKind, waitCondAvailable, WaitTarget{Kind: condKind, Value: condAvailable}, false},
		{waitKindPhase, "phase=Running", WaitTarget{Kind: waitKindPhase, Value: "Running"}, false},
		{"trim spaces", " condition = Available ", WaitTarget{Kind: condKind, Value: condAvailable}, false},
		{"empty", "", WaitTarget{}, true},
		{"no equals", condKind, WaitTarget{}, true},
		{"unknown kind", "ready=true", WaitTarget{}, true},
		{"empty value", "phase=", WaitTarget{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseWaitFor(tt.in)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWait_ConditionMetImmediately(t *testing.T) {
	get := func() ([]metav1.Condition, string, int64, int64, error) {
		return []metav1.Condition{{Type: condAvailable, Status: metav1.ConditionTrue}}, phaseRunning, 1, 1, nil
	}
	target, err := ParseWaitFor(waitCondAvailable)
	require.NoError(t, err)
	require.NoError(t, Wait(context.Background(), get, target, time.Second))
}

func TestWait_PhaseMet(t *testing.T) {
	get := func() ([]metav1.Condition, string, int64, int64, error) {
		return nil, phaseRunning, 2, 2, nil
	}
	target, _ := ParseWaitFor("phase=" + phaseRunning)
	require.NoError(t, Wait(context.Background(), get, target, time.Second))
}

func TestWait_ConditionFalseTimesOut(t *testing.T) {
	get := func() ([]metav1.Condition, string, int64, int64, error) {
		return []metav1.Condition{{Type: condAvailable, Status: metav1.ConditionFalse}}, "Progressing", 1, 1, nil
	}
	target, _ := ParseWaitFor(waitCondAvailable)
	err := Wait(context.Background(), get, target, 120*time.Millisecond)
	assert.ErrorIs(t, err, ErrWaitTimeout)
}

func TestWait_ReconcileLagBlocksConditionMatch(t *testing.T) {
	// observedGeneration < generation: the True condition is stale, so the predicate must NOT pass.
	get := func() ([]metav1.Condition, string, int64, int64, error) {
		return []metav1.Condition{{Type: condAvailable, Status: metav1.ConditionTrue}}, phaseRunning, 5, 4, nil
	}
	target, _ := ParseWaitFor(waitCondAvailable)
	err := Wait(context.Background(), get, target, 120*time.Millisecond)
	assert.ErrorIs(t, err, ErrWaitTimeout)
}

func TestWait_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	get := func() ([]metav1.Condition, string, int64, int64, error) {
		return nil, "Progressing", 1, 1, nil
	}
	target, _ := ParseWaitFor("phase=Running")
	assert.Error(t, Wait(ctx, get, target, time.Second))
}

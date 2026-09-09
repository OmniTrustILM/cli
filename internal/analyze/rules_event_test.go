/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func warnEvent(reason, msg string, ts time.Time, count int32) corev1.Event {
	return corev1.Event{
		Type:          corev1.EventTypeWarning,
		Reason:        reason,
		Message:       msg,
		Count:         count,
		LastTimestamp: metav1.NewTime(ts),
	}
}

func TestEventAnalyzer(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	a := newEventAnalyzer()
	a.now = func() time.Time { return base }

	t.Run("recent warning is a warn finding", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Events: []corev1.Event{warnEvent("FailedScheduling", "no nodes available", base.Add(-5*time.Minute), 3)},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityWarn, got[0].Severity)
		assert.Equal(t, "event", got[0].Rule)
		assert.Contains(t, got[0].Evidence, "FailedScheduling")
	})

	t.Run("stale warning outside window is ignored", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Events: []corev1.Event{warnEvent("FailedMount", "x", base.Add(-3*time.Hour), 1)},
		}}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("normal events are ignored", func(t *testing.T) {
		t.Parallel()
		ev := corev1.Event{Type: corev1.EventTypeNormal, Reason: "Pulled", LastTimestamp: metav1.NewTime(base)}
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Events: []corev1.Event{ev},
		}}}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestEventAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "event", newEventAnalyzer().Name())
}

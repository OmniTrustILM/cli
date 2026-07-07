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
			GVK: analyzeAPIGroup, Namespace: "ns", Name: "ilm",
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
			GVK: analyzeAPIGroup, Namespace: "ns", Name: "ilm",
			Events: []corev1.Event{warnEvent("FailedMount", "x", base.Add(-3*time.Hour), 1)},
		}}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("normal events are ignored", func(t *testing.T) {
		t.Parallel()
		ev := corev1.Event{Type: corev1.EventTypeNormal, Reason: "Pulled", LastTimestamp: metav1.NewTime(base)}
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: "ilm",
			Events: []corev1.Event{ev},
		}}}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestEventAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "event", newEventAnalyzer().Name())
}

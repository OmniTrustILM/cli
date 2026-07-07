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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// defaultEventWindow bounds how recent a Warning event must be to surface.
const defaultEventWindow = time.Hour

// eventAnalyzer surfaces recent Warning events on ILM-owned objects
// (FailedScheduling, FailedMount, BackOff, ...).
type eventAnalyzer struct {
	now    func() time.Time
	window time.Duration
}

func newEventAnalyzer() eventAnalyzer {
	return eventAnalyzer{now: time.Now, window: defaultEventWindow}
}

func (eventAnalyzer) Name() string { return "event" }

func (a eventAnalyzer) Analyze(s *Snapshot) []Finding {
	now := a.now
	if now == nil {
		now = time.Now
	}
	window := a.window
	if window <= 0 {
		window = defaultEventWindow
	}
	cutoff := now().Add(-window)

	var out []Finding
	for _, r := range s.Resources() {
		ref := r.ResourceRef()
		seen := map[string]bool{}
		for i := range r.Events {
			ev := r.Events[i]
			if ev.Type != corev1.EventTypeWarning {
				continue
			}
			if eventTime(ev).Before(cutoff) {
				continue
			}
			key := ev.Reason + "|" + ev.Message
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Finding{
				Severity: SeverityWarn,
				Rule:     a.Name(),
				Resource: ref,
				Title:    fmt.Sprintf("recent warning event: %s", ev.Reason),
				Evidence: fmt.Sprintf("%s: %s (x%d)", ev.Reason, ev.Message, ev.Count),
			})
		}
	}
	return out
}

// eventTime picks the most meaningful timestamp available on an Event.
func eventTime(ev corev1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	return ev.FirstTimestamp.Time
}

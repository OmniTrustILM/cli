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

// Package shared holds cross-command flag, wait and output helpers reused by the
// platform/connector/proxy read subcommands and the infra status/check commands.
package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/health"
)

// DefaultWaitTimeout is the default --timeout applied by every wait command.
const DefaultWaitTimeout = 5 * time.Minute

// waitPollInterval is how often Wait re-reads the resource while blocking.
const waitPollInterval = 100 * time.Millisecond

// ErrWaitTimeout is returned when the predicate is not satisfied before the deadline.
var ErrWaitTimeout = errors.New("timed out waiting for condition")

// WaitTarget is a parsed --for predicate.
type WaitTarget struct {
	Kind  string // "condition" | "phase"
	Value string // the condition Type, or the phase value
}

// ParseWaitFor parses --for=condition=<Type> or --for=phase=<Phase>.
func ParseWaitFor(s string) (WaitTarget, error) {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return WaitTarget{}, fmt.Errorf("invalid --for %q: expected condition=<Type> or phase=<Phase>", s)
	}
	kind := strings.TrimSpace(kv[0])
	value := strings.TrimSpace(kv[1])
	if kind != "condition" && kind != "phase" {
		return WaitTarget{}, fmt.Errorf("invalid --for kind %q: must be condition or phase", kind)
	}
	if value == "" {
		return WaitTarget{}, fmt.Errorf("invalid --for %q: value must not be empty", s)
	}
	return WaitTarget{Kind: kind, Value: value}, nil
}

// Wait polls get until the target predicate holds, the context is done, or the
// timeout elapses. A condition match also requires the status to be observed for
// the current generation (no reconcile lag), so a stale True is not mistaken for
// readiness.
func Wait(
	ctx context.Context,
	get func() (conds []metav1.Condition, phase string, gen, observed int64, err error),
	t WaitTarget,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(waitPollInterval)
	defer ticker.Stop()
	for {
		conds, phase, gen, observed, err := get()
		if err != nil {
			return err
		}
		if met(t, conds, phase, gen, observed) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %s=%s not met within %s", ErrWaitTimeout, t.Kind, t.Value, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// met reports whether the predicate holds for the latest observed status.
func met(t WaitTarget, conds []metav1.Condition, phase string, gen, observed int64) bool {
	switch t.Kind {
	case "phase":
		return phase == t.Value
	case "condition":
		if health.ReconcileLagged(observed, gen) {
			return false
		}
		c := health.Condition(conds, t.Value)
		return c != nil && c.Status == metav1.ConditionTrue
	default:
		return false
	}
}

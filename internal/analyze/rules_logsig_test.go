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
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsigAnalyzer(t *testing.T) {
	t.Parallel()
	sig := LogSignature{
		Name:        "panic",
		Pattern:     regexp.MustCompile(`panic:`),
		Severity:    SeverityFail,
		Title:       "component panicked",
		Remediation: "inspect the stack trace in component logs",
	}
	a := newLogsigAnalyzer(sig)

	t.Run("matching log line emits a finding", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Logs: map[string]string{analyzeCoreComponent: "starting...\npanic: nil pointer\n"},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityFail, got[0].Severity)
		assert.Equal(t, "logsig", got[0].Rule)
		assert.Contains(t, got[0].Evidence, analyzeCoreComponent)
	})

	t.Run("no match emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Logs: map[string]string{analyzeCoreComponent: "all good\n"},
		}}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("no logs collected emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
		}}}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestDefaultLogSignaturesNonEmpty(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, DefaultLogSignatures())
}

func TestLogsigAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "logsig", newLogsigAnalyzer().Name())
}

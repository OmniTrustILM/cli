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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

func TestCapabilityAnalyzer(t *testing.T) {
	t.Parallel()
	a := capabilityAnalyzer{}

	t.Run("managed db without CNPG is fail with deps install remediation", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{
			Capabilities: []capabilities.Result{{Dep: capabilities.DepCNPG, Present: false}},
			Platforms: []ResourceSnapshot{{
				GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
				SpecModes: capabilities.Modes{DBManaged: true},
			}},
		}
		got := a.Analyze(s)
		require.NotEmpty(t, got)
		var found bool
		for _, f := range got {
			if assertContains(f.Remediation, string(capabilities.DepCNPG)) {
				found = true
				assert.Equal(t, SeverityFail, f.Severity)
				assert.Equal(t, "capability", f.Rule)
				assert.Contains(t, f.Remediation, "deps install")
			}
		}
		assert.True(t, found, "expected a cnpg deps-install remediation")
	})

	t.Run("managed db with CNPG present emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{
			Capabilities: []capabilities.Result{{Dep: capabilities.DepCNPG, Present: true}},
			Platforms: []ResourceSnapshot{{
				GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
				SpecModes: capabilities.Modes{DBManaged: true},
			}},
		}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("no capabilities reported (offline) emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			SpecModes: capabilities.Modes{DBManaged: true},
		}}}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestCapabilityAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "capability", capabilityAnalyzer{}.Name())
}

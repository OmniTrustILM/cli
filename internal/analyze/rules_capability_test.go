/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

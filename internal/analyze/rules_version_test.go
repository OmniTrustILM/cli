/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionAnalyzer(t *testing.T) {
	t.Parallel()
	a := versionAnalyzer{}
	platform := func(observed string) ResourceSnapshot {
		return ResourceSnapshot{GVK: GVKPlatform, Namespace: analyzePlatformName, Name: analyzePlatformName, ObservedVersion: observed}
	}

	t.Run("platform version not in SupportedVersions is a warn (not fail)", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{platform(analyzeVer9900)}, SupportedVersions: []string{analyzeVer2180}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityWarn, got[0].Severity)
		assert.Equal(t, "version", got[0].Rule)
		assert.Equal(t, s.Platforms[0].ResourceRef(), got[0].Resource)
		assert.Contains(t, got[0].Evidence, analyzeVer9900)
		assert.Contains(t, got[0].Evidence, analyzeVer2180)
	})

	t.Run("operator version is not a platform version", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{OperatorVersion: "1.1.0", Platforms: []ResourceSnapshot{platform(analyzeVer2180)}, SupportedVersions: []string{analyzeVer2180}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("unreconciled platform emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{platform("")}, SupportedVersions: []string{analyzeVer2180}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("empty SupportedVersions emits nothing (offline or unknown)", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{platform(analyzeVer9900)}, SupportedVersions: nil}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestVersionAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "version", versionAnalyzer{}.Name())
}

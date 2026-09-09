/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package version

import (
	"runtime"
	"testing"

	"github.com/OmniTrustILM/operator/pkg/bom"
	"github.com/stretchr/testify/assert"
)

func TestClient_PopulatesRuntimeFields(t *testing.T) {
	i := Client()
	assert.Equal(t, runtime.Version(), i.GoVersion)
	assert.Equal(t, runtime.GOOS+"/"+runtime.GOARCH, i.Platform)
	assert.NotEmpty(t, i.ClientVersion) // "dev" when ldflags unset
}

func TestClient_UsesLdflagsWhenSet(t *testing.T) {
	// Must NOT run in parallel: mutates the GitVersion package-level var.
	old := GitVersion
	t.Cleanup(func() { GitVersion = old })
	GitVersion = "v1.2.3"
	assert.Equal(t, "v1.2.3", Client().ClientVersion)
}

func TestDefaultMatchesBOM(t *testing.T) {
	assert.Equal(t, bom.DefaultVersion, Default())
}

func TestSupportedVersionsMatchesBOM(t *testing.T) {
	assert.ElementsMatch(t, bom.SupportedVersions(), SupportedVersions())
}

func TestCheckOperator(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantOK    bool
		wantInMsg string
	}{
		{"supported default", bom.DefaultVersion, true, "supported"},
		{"empty treated as default", "", true, "supported"},
		{"out of range", "0.0.1-unknown", false, "outside the supported range"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CheckOperator(tt.version)
			assert.Equal(t, tt.wantOK, r.Supported)
			assert.Contains(t, r.Message, tt.wantInMsg)
			assert.ElementsMatch(t, bom.SupportedVersions(), r.SupportedVersions)
		})
	}
}

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// stubAnalyzer emits a fixed set of findings, ignoring the snapshot.
type stubAnalyzer struct {
	name string
	out  []Finding
}

func (s stubAnalyzer) Name() string                { return s.name }
func (s stubAnalyzer) Analyze(*Snapshot) []Finding { return s.out }

func TestSnapshotResources(t *testing.T) {
	t.Parallel()
	s := &Snapshot{
		Platforms:  []ResourceSnapshot{{Name: "p1"}},
		Connectors: []ResourceSnapshot{{Name: "c1"}, {Name: "c2"}},
		Proxies:    []ResourceSnapshot{{Name: "x1"}},
	}
	got := s.Resources()
	require.Len(t, got, 4)
	// Platforms first, then connectors, then proxies.
	assert.Equal(t, "p1", got[0].Name)
	assert.Equal(t, "c1", got[1].Name)
	assert.Equal(t, "c2", got[2].Name)
	assert.Equal(t, "x1", got[3].Name)
}

func TestNewRegistryRunOrdersFindings(t *testing.T) {
	t.Parallel()
	// Two analyzers contribute mixed-severity findings on different resources.
	a := stubAnalyzer{name: "a", out: []Finding{
		{Severity: SeverityWarn, Title: "warn-b", Resource: analyzePlatNSB, Rule: "a"},
		{Severity: SeverityFail, Title: "fail-a", Resource: analyzePlatNSA, Rule: "a"},
	}}
	b := stubAnalyzer{name: "b", out: []Finding{
		{Severity: SeverityInfo, Title: "info-a", Resource: analyzePlatNSA, Rule: "b"},
		{Severity: SeverityFail, Title: "fail-b", Resource: analyzePlatNSB, Rule: "b"},
	}}
	reg := NewRegistry(a, b)

	got := reg.Run(&Snapshot{})
	require.Len(t, got, 4)

	// Ordered: fail > warn > info; within a severity, by resource then title.
	assert.Equal(t, SeverityFail, got[0].Severity)
	assert.Equal(t, analyzePlatNSA, got[0].Resource)
	assert.Equal(t, SeverityFail, got[1].Severity)
	assert.Equal(t, analyzePlatNSB, got[1].Resource)
	assert.Equal(t, SeverityWarn, got[2].Severity)
	assert.Equal(t, SeverityInfo, got[3].Severity)

	// Worst over the ordered set is fail.
	assert.Equal(t, SeverityFail, Worst(got))
}

func TestRegistryAnalyzersExposed(t *testing.T) {
	t.Parallel()
	reg := NewRegistry(stubAnalyzer{name: "only"})
	names := make([]string, 0, len(reg.Analyzers()))
	for _, a := range reg.Analyzers() {
		names = append(names, a.Name())
	}
	assert.Equal(t, []string{"only"}, names)
}

func TestRunEmptyRegistryNoFindings(t *testing.T) {
	t.Parallel()
	assert.Empty(t, NewRegistry().Run(&Snapshot{}))
}

// TestSnapshotShapeCompiles pins the public field set so the live builder
// and bundle reader populate the same shape.
func TestSnapshotShapeCompiles(t *testing.T) {
	t.Parallel()
	s := &Snapshot{
		ClientVersion:     "v0.1.0",
		OperatorVersion:   analyzeVer2180,
		OperatorReady:     true,
		Capabilities:      []capabilities.Result{{Dep: capabilities.DepCNPG, Present: true}},
		SupportedVersions: []string{analyzeVer2180},
		MissingRefs:       []string{"Secret/ns/missing"},
		Platforms: []ResourceSnapshot{{
			GVK:         analyzeAPIGroup,
			Namespace:   "ns",
			Name:        analyzePlatformName,
			Phase:       "Running",
			ObservedGen: 2,
			Generation:  2,
			Conditions:  []metav1.Condition{{Type: "Available", Status: metav1.ConditionTrue}},
			SpecModes:   capabilities.Modes{DBManaged: true},
			SecretRefs:  []string{"ilm-db"},
			IssuerRefs:  []string{"ilm-issuer"},
			Logs:        map[string]string{analyzeCoreComponent: "ok"},
		}},
	}
	assert.Equal(t, analyzePlatformName, s.Platforms[0].Name)
}

func TestResourceRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rs   ResourceSnapshot
		want string
	}{
		{
			name: "platform with dot-qualified GVK",
			rs:   ResourceSnapshot{GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName},
			want: "Platform/ns/ilm",
		},
		{
			name: "connector with dot-qualified GVK",
			rs:   ResourceSnapshot{GVK: GVKConnector, Namespace: "prod", Name: "my-conn"},
			want: "Connector/prod/my-conn",
		},
		{
			name: "bare kind (no dot)",
			rs:   ResourceSnapshot{GVK: "Platform", Namespace: "ns", Name: analyzePlatformName},
			want: "Platform/ns/ilm",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.rs.ResourceRef())
		})
	}
}

func TestDefaultRegistryContainsBaseRules(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	names := make([]string, 0, len(reg.Analyzers()))
	for _, a := range reg.Analyzers() {
		names = append(names, a.Name())
	}
	assert.Contains(t, names, "condition")
	assert.Contains(t, names, "phase")
	assert.Contains(t, names, "reconcile")
}

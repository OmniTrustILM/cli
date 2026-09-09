/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

func TestDefaultRegistryWiresNineRules(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	names := make([]string, 0, len(reg.Analyzers()))
	for _, a := range reg.Analyzers() {
		names = append(names, a.Name())
	}
	assert.ElementsMatch(t, []string{
		"condition", "phase", "reconcile", "workload",
		"capability", "reference", "version", "event", "logsig",
	}, names)
}

func TestDefaultRegistryEndToEnd(t *testing.T) {
	t.Parallel()
	// A snapshot that trips multiple rules at once exercises ordering + dedup.
	s := &Snapshot{
		OperatorVersion:   "99.0.0",
		SupportedVersions: []string{analyzeVer2180},
		Capabilities:      []capabilities.Result{{Dep: capabilities.DepCNPG, Present: false}},
		MissingRefs:       []string{"Secret/ns/ilm-db"},
		Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Phase:      phaseDegraded,
			Generation: 5, ObservedGen: 4,
			SpecModes:  capabilities.Modes{DBManaged: true},
			Conditions: []metav1.Condition{{Type: analyzeDatabaseReady, Status: metav1.ConditionFalse, Reason: "CNPGDown"}},
			Deployments: []appsv1.Deployment{{
				ObjectMeta: metav1.ObjectMeta{Name: analyzeCoreComponent, Namespace: "ns"},
				Spec:       appsv1.DeploymentSpec{Replicas: ptrInt32(2)},
				Status:     appsv1.DeploymentStatus{ReadyReplicas: 0},
			}},
		}},
	}
	got := DefaultRegistry().Run(s)
	require.NotEmpty(t, got)
	// Fail findings sort to the front and the worst severity is fail.
	assert.Equal(t, SeverityFail, got[0].Severity)
	assert.Equal(t, SeverityFail, Worst(got))
}

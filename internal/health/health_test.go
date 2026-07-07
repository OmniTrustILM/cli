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

package health

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

const (
	healthAvailable = "Available"
	healthDBReady   = "DatabaseReady"
	healthEdgeReady = "EdgeReady"
)

func cond(typ string, status metav1.ConditionStatus) metav1.Condition {
	return metav1.Condition{Type: typ, Status: status}
}

func TestKnownConditions_Catalog(t *testing.T) {
	// The literal condition TYPE names the operator publishes (re-declared here
	// because the operator constants are unexported).
	wantTypes := []string{
		// Platform.
		healthAvailable, "Progressing", "Degraded", "OIDCConfigured", healthEdgeReady,
		healthDBReady, "MessagingReady", "KeycloakReady",
		"AdminCertReady", "AdminUserReady", "ServiceMonitorsReady",
		"DatabaseUpgradeBlocked", "MessagingUpgradeBlocked", "KeycloakUpgradeBlocked",
		// Proxy / Connector.
		"ServiceMonitorReady",
	}
	for _, typ := range wantTypes {
		t.Run(typ, func(t *testing.T) {
			meta, ok := KnownConditions[typ]
			require.True(t, ok, "KnownConditions must contain %q", typ)
			assert.Equal(t, typ, meta.Type, "ConditionMeta.Type must echo the key")
		})
	}
}

func TestKnownConditions_ReadyTypesHaveRemediation(t *testing.T) {
	// The *Ready/Degraded/UpgradeBlocked conditions carry curated remediation.
	for _, typ := range []string{
		healthDBReady, "MessagingReady", "KeycloakReady", healthEdgeReady,
		"AdminCertReady", "AdminUserReady", "Degraded", "DatabaseUpgradeBlocked",
	} {
		assert.NotEmpty(t, KnownConditions[typ].FailRemediation, "%s should have remediation", typ)
	}
}

func TestKnownConditions_UnknownConditionNotInCatalog(t *testing.T) {
	// Data-driven behavior: unknown condition types are absent from the catalog
	// but callers must still be able to surface them (the catalog only adds
	// curated severity/remediation for known ones).
	_, ok := KnownConditions["SomeFutureConditionType"]
	assert.False(t, ok, "unknown condition types must not be in KnownConditions")
}

func TestCondition(t *testing.T) {
	conds := []metav1.Condition{cond(healthAvailable, metav1.ConditionTrue), cond(healthEdgeReady, metav1.ConditionFalse)}
	got := Condition(conds, healthEdgeReady)
	require.NotNil(t, got)
	assert.Equal(t, metav1.ConditionFalse, got.Status)
	assert.Nil(t, Condition(conds, "Missing"))
	assert.Nil(t, Condition(nil, healthAvailable))
}

func TestSummarize(t *testing.T) {
	conds := []metav1.Condition{
		cond(healthAvailable, metav1.ConditionTrue),
		cond(healthEdgeReady, metav1.ConditionTrue),
		cond(healthDBReady, metav1.ConditionFalse),
		cond("Progressing", metav1.ConditionUnknown),
	}
	ready, total := Summarize(conds)
	assert.Equal(t, 2, ready)
	assert.Equal(t, 4, total)
	r, tot := Summarize(nil)
	assert.Equal(t, 0, r)
	assert.Equal(t, 0, tot)
}

// TestSummarize_ConditionPolarity locks the healthy-state polarity: a fully
// reconciled platform reports Progressing=False (and, when present, Degraded=
// False), which must count as healthy — otherwise a 100%-healthy platform shows
// N-1/N (the real-cluster "7/8" symptom).
func TestSummarize_ConditionPolarity(t *testing.T) {
	// A fully healthy platform: every positive condition True, Progressing False.
	healthy := []metav1.Condition{
		cond(CondDatabaseReady, metav1.ConditionTrue),
		cond(CondMessagingReady, metav1.ConditionTrue),
		cond(CondKeycloakReady, metav1.ConditionTrue),
		cond(CondEdgeReady, metav1.ConditionTrue),
		cond(CondOIDCConfigured, metav1.ConditionTrue),
		cond(CondAvailable, metav1.ConditionTrue),
		cond(CondProgressing, metav1.ConditionFalse), // reconcile complete == healthy
		cond(CondAdminUserReady, metav1.ConditionTrue),
	}
	ready, total := Summarize(healthy)
	assert.Equal(t, 8, total)
	assert.Equal(t, 8, ready, "Progressing=False must count as healthy → 8/8")

	// Progressing=True (still reconciling) is NOT ready; Degraded=True is NOT ready.
	unhealthy := []metav1.Condition{
		cond(CondAvailable, metav1.ConditionTrue),
		cond(CondProgressing, metav1.ConditionTrue),
		cond(CondDegraded, metav1.ConditionTrue),
	}
	ready, total = Summarize(unhealthy)
	assert.Equal(t, 3, total)
	assert.Equal(t, 1, ready, "only Available=True is healthy; Progressing/Degraded True are not")

	// Degraded=False (not degraded) counts as healthy.
	ready, _ = Summarize([]metav1.Condition{cond(CondDegraded, metav1.ConditionFalse)})
	assert.Equal(t, 1, ready, "Degraded=False must count as healthy")
}

func TestReconcileLagged(t *testing.T) {
	assert.True(t, ReconcileLagged(2, 3))
	assert.False(t, ReconcileLagged(3, 3))
	assert.False(t, ReconcileLagged(0, 0))
	assert.False(t, ReconcileLagged(4, 3))
}

func TestPlatformState(t *testing.T) {
	tests := []struct {
		name  string
		phase otilmv1alpha1.PlatformPhase
		conds []metav1.Condition
		want  State
	}{
		{"degraded phase", otilmv1alpha1.PlatformPhaseDegraded, nil, StateDegraded},
		{"running and available", otilmv1alpha1.PlatformPhaseRunning, []metav1.Condition{cond(healthAvailable, metav1.ConditionTrue)}, StateOK},
		{"running but available false", otilmv1alpha1.PlatformPhaseRunning, []metav1.Condition{cond(healthAvailable, metav1.ConditionFalse)}, StateProgressing},
		{"progressing phase", otilmv1alpha1.PlatformPhaseProgressing, nil, StateProgressing},
		{"empty phase", "", nil, StateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &otilmv1alpha1.Platform{Status: otilmv1alpha1.PlatformStatus{Phase: tt.phase, Conditions: tt.conds}}
			assert.Equal(t, tt.want, PlatformState(p))
		})
	}
}

func TestConnectorState(t *testing.T) {
	tests := []struct {
		phase otilmv1alpha1.ConnectorPhase
		want  State
	}{
		{otilmv1alpha1.ConnectorPhaseRunning, StateOK},
		{otilmv1alpha1.ConnectorPhaseFailed, StateDegraded},
		{otilmv1alpha1.ConnectorPhasePending, StateProgressing},
		{otilmv1alpha1.ConnectorPhaseDeploying, StateProgressing},
		{otilmv1alpha1.ConnectorPhaseUpdating, StateProgressing},
		{"", StateUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			c := &otilmv1alpha1.Connector{Status: otilmv1alpha1.ConnectorStatus{Phase: tt.phase}}
			assert.Equal(t, tt.want, ConnectorState(c))
		})
	}
}

func TestProxyState(t *testing.T) {
	tests := []struct {
		phase otilmv1alpha1.ProxyPhase
		want  State
	}{
		{otilmv1alpha1.ProxyPhaseRunning, StateOK},
		{otilmv1alpha1.ProxyPhaseScaledDown, StateOK},
		{otilmv1alpha1.ProxyPhaseFailed, StateDegraded},
		{otilmv1alpha1.ProxyPhasePending, StateProgressing},
		{otilmv1alpha1.ProxyPhaseDeploying, StateProgressing},
		{otilmv1alpha1.ProxyPhaseUpdating, StateProgressing},
		{"", StateUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			p := &otilmv1alpha1.Proxy{Status: otilmv1alpha1.ProxyStatus{Phase: tt.phase}}
			assert.Equal(t, tt.want, ProxyState(p))
		})
	}
}

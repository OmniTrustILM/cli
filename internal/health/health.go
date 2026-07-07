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

// Package health interprets the operator's published phases and conditions into a
// small, presentation-ready State, plus condition helpers and the curated
// KnownConditions catalog. The operator's condition TYPE strings live in its
// internal/controller package and are NOT importable, so they are re-declared
// here as literals; the analyzer remains data-driven over whatever conditions
// actually appear and uses this catalog only for curated severity/remediation.
package health

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// State is the presentation-level rollup of a resource's health.
type State string

// The four rollup states.
const (
	StateOK          State = "ok"
	StateProgressing State = "progressing"
	StateDegraded    State = "degraded"
	StateUnknown     State = "unknown"
)

// Condition TYPE literals. The operator defines these strings in its
// internal/controller package (unexported), so they are re-declared here as the
// CLI's own literals. They mirror the operator's published condition types.
const (
	// Shared across Platform, Proxy, and Connector.
	CondAvailable   = "Available"
	CondProgressing = "Progressing"
	CondDegraded    = "Degraded"

	// Platform-specific.
	CondOIDCConfigured          = "OIDCConfigured"
	CondEdgeReady               = "EdgeReady"
	CondDatabaseReady           = "DatabaseReady"
	CondMessagingReady          = "MessagingReady"
	CondKeycloakReady           = "KeycloakReady"
	CondAdminCertReady          = "AdminCertReady"
	CondAdminUserReady          = "AdminUserReady"
	CondServiceMonitorsReady    = "ServiceMonitorsReady"
	CondDatabaseUpgradeBlocked  = "DatabaseUpgradeBlocked"
	CondMessagingUpgradeBlocked = "MessagingUpgradeBlocked"
	CondKeycloakUpgradeBlocked  = "KeycloakUpgradeBlocked"

	// Proxy-specific (singular form; Platform uses ServiceMonitorsReady).
	CondServiceMonitorReady = "ServiceMonitorReady"
)

// ConditionMeta is the curated entry for a known condition type.
type ConditionMeta struct {
	// Type echoes the map key so callers don't need to track both.
	Type string `json:"type"`
	// DocsURL is an optional deep-link to documentation for this condition.
	DocsURL string `json:"docsURL,omitempty"`
	// FailRemediation is human-readable guidance when this condition is False or Unknown.
	FailRemediation string `json:"failRemediation,omitempty"`
}

// KnownConditions is the curated catalog of operator condition TYPE names.
// Absence from this map is fine: the analyzer still reports unknown conditions,
// it just won't have curated severity/remediation for them.
var KnownConditions = map[string]ConditionMeta{
	CondAvailable: {
		Type:            CondAvailable,
		FailRemediation: "Run `ilmctl platform describe` and inspect the failing component conditions and recent events.",
	},
	CondProgressing: {
		Type:            CondProgressing,
		FailRemediation: "Reconciliation is in progress; wait or run `ilmctl status` to track readiness.",
	},
	CondDegraded: {
		Type:            CondDegraded,
		FailRemediation: "A fatal error was reported; read the condition message and run `ilmctl platform events` for the cause.",
	},
	CondOIDCConfigured: {
		Type:            CondOIDCConfigured,
		FailRemediation: "Check the Keycloak/OIDC configuration in spec.keycloak and the referenced Secrets.",
	},
	CondEdgeReady: {
		Type:            CondEdgeReady,
		FailRemediation: "Verify the edge config (spec.edge): for gatewayAPI ensure the Gateway API CRDs are installed; for ingress ensure an ingress controller is present; check spec.edge.tls.",
	},
	CondDatabaseReady: {
		Type:            CondDatabaseReady,
		FailRemediation: "Inspect the CloudNativePG Cluster (`ilmctl status`); for db-mode=managed ensure CloudNativePG is installed (`ilmctl deps install --only cnpg`).",
	},
	CondMessagingReady: {
		Type:            CondMessagingReady,
		FailRemediation: "Inspect the RabbitmqCluster status; for messaging-mode=managed ensure the RabbitMQ Cluster Operator is installed (`ilmctl deps install --only rabbitmq`).",
	},
	CondKeycloakReady: {
		Type:            CondKeycloakReady,
		FailRemediation: "Inspect the Keycloak status; for keycloak-mode=managed ensure the Keycloak Operator is installed (`ilmctl deps install --only keycloak`).",
	},
	CondAdminCertReady: {
		Type:            CondAdminCertReady,
		FailRemediation: "Ensure the Secret referenced by spec.registerAdmin.certificate.secretRef exists and contains a valid TLS cert/key.",
	},
	CondAdminUserReady: {
		Type:            CondAdminUserReady,
		FailRemediation: "Ensure the Secret referenced by spec.registerAdmin.password.secretRef exists with the expected key.",
	},
	CondServiceMonitorsReady: {
		Type:            CondServiceMonitorsReady,
		FailRemediation: "ServiceMonitor objects require the Prometheus Operator CRDs (monitoring.coreos.com).",
	},
	CondServiceMonitorReady: {
		Type:            CondServiceMonitorReady,
		FailRemediation: "The ServiceMonitor requires the Prometheus Operator CRDs (monitoring.coreos.com).",
	},
	CondDatabaseUpgradeBlocked: {
		Type:            CondDatabaseUpgradeBlocked,
		FailRemediation: "A managed-database major upgrade awaits acknowledgement; re-run `ilmctl platform upgrade --ack-database`.",
	},
	CondMessagingUpgradeBlocked: {
		Type:            CondMessagingUpgradeBlocked,
		FailRemediation: "A managed-messaging major upgrade awaits acknowledgement; re-run `ilmctl platform upgrade --ack-messaging`.",
	},
	CondKeycloakUpgradeBlocked: {
		Type:            CondKeycloakUpgradeBlocked,
		FailRemediation: "A managed-Keycloak major upgrade awaits acknowledgement; re-run `ilmctl platform upgrade --ack-keycloak`.",
	},
}

// Condition returns the condition of the given type from the slice, or nil if absent.
// The slice is iterated linearly; callers may cache the result for hot paths.
func Condition(conds []metav1.Condition, typ string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == typ {
			return &conds[i]
		}
	}
	return nil
}

// negativePolarityConditions are condition types whose HEALTHY state is
// Status == False rather than True: Progressing (False == reconcile complete),
// Degraded (False == not degraded), and the *UpgradeBlocked gates (False ==
// not blocked). Summarize must not treat their healthy False as "not ready".
var negativePolarityConditions = map[string]bool{
	CondProgressing:             true,
	CondDegraded:                true,
	CondDatabaseUpgradeBlocked:  true,
	CondMessagingUpgradeBlocked: true,
	CondKeycloakUpgradeBlocked:  true,
}

// conditionHealthy reports whether a single condition is in its healthy state,
// accounting for polarity: positive conditions (Available, *Ready, …) are
// healthy when True; negative ones (Progressing, Degraded, *UpgradeBlocked) are
// healthy when False.
func conditionHealthy(c metav1.Condition) bool {
	if negativePolarityConditions[c.Type] {
		return c.Status == metav1.ConditionFalse
	}
	return c.Status == metav1.ConditionTrue
}

// Summarize counts how many conditions are in their HEALTHY state out of the
// total, accounting for condition polarity — so a fully-reconciled platform
// (which reports Progressing=False) counts as N/N, not N-1/N. Both values are
// zero for a nil or empty slice.
func Summarize(conds []metav1.Condition) (ready, total int) {
	for i := range conds {
		if conditionHealthy(conds[i]) {
			ready++
		}
	}
	return ready, len(conds)
}

// ReconcileLagged reports whether the operator has not yet reconciled the latest
// spec generation (observedGeneration < generation). Both values must be positive
// for a lag to be declared; a zero generation means the resource was never written.
func ReconcileLagged(observedGen, gen int64) bool {
	return gen > 0 && observedGen > 0 && observedGen < gen
}

// PlatformState rolls the Platform phase and Available condition into a State.
// If the phase is Running but Available is False or Unknown, StateProgressing is
// returned (the operator is still settling).
func PlatformState(p *otilmv1alpha1.Platform) State {
	switch p.Status.Phase {
	case otilmv1alpha1.PlatformPhaseDegraded:
		return StateDegraded
	case otilmv1alpha1.PlatformPhaseRunning:
		if c := Condition(p.Status.Conditions, CondAvailable); c != nil && c.Status != metav1.ConditionTrue {
			return StateProgressing
		}
		return StateOK
	case otilmv1alpha1.PlatformPhaseProgressing:
		return StateProgressing
	default:
		return StateUnknown
	}
}

// ConnectorState rolls the Connector phase into a State.
func ConnectorState(c *otilmv1alpha1.Connector) State {
	switch c.Status.Phase {
	case otilmv1alpha1.ConnectorPhaseRunning:
		return StateOK
	case otilmv1alpha1.ConnectorPhaseFailed:
		return StateDegraded
	case otilmv1alpha1.ConnectorPhasePending,
		otilmv1alpha1.ConnectorPhaseDeploying,
		otilmv1alpha1.ConnectorPhaseUpdating:
		return StateProgressing
	default:
		return StateUnknown
	}
}

// ProxyState rolls the Proxy phase into a State. ScaledDown is a settled (ok)
// state because the resource intentionally has zero replicas.
func ProxyState(p *otilmv1alpha1.Proxy) State {
	switch p.Status.Phase {
	case otilmv1alpha1.ProxyPhaseRunning, otilmv1alpha1.ProxyPhaseScaledDown:
		return StateOK
	case otilmv1alpha1.ProxyPhaseFailed:
		return StateDegraded
	case otilmv1alpha1.ProxyPhasePending,
		otilmv1alpha1.ProxyPhaseDeploying,
		otilmv1alpha1.ProxyPhaseUpdating:
		return StateProgressing
	default:
		return StateUnknown
	}
}

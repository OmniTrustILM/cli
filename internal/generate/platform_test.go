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

package generate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

func boolPtr(b bool) *bool { return &b }

func TestScaffoldPlatform_ProfileDefaults(t *testing.T) {
	tests := []struct {
		name             string
		profile          Profile
		wantDBMode       string
		wantMsgMode      string
		wantKeycloakNil  bool
		wantKeycloakMode string
		wantHA           bool
	}{
		{name: "minimal", profile: ProfileMinimal, wantDBMode: genExternal, wantMsgMode: genExternal, wantKeycloakNil: true, wantHA: false},
		{name: genExternal, profile: ProfileExternal, wantDBMode: genExternal, wantMsgMode: genExternal, wantKeycloakMode: genExternal, wantHA: false},
		{name: "managed-ha", profile: ProfileManagedHA, wantDBMode: genManaged, wantMsgMode: genManaged, wantKeycloakMode: genManaged, wantHA: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, notes, err := ScaffoldPlatform(PlatformOptions{
				Name:      genILM,
				Namespace: genILM,
				Profile:   tc.profile,
				Set:       map[string]bool{},
			})
			require.NoError(t, err)
			require.NotNil(t, p)
			assert.Equal(t, "Platform", p.Kind)
			assert.Equal(t, "otilm.com/v1alpha1", p.APIVersion)
			assert.Equal(t, genILM, p.Name)
			assert.Equal(t, genILM, p.Namespace)
			assert.Equal(t, tc.wantDBMode, p.Spec.Database.Mode)
			assert.Equal(t, tc.wantMsgMode, p.Spec.Messaging.Mode)
			assert.Equal(t, tc.wantHA, p.Spec.HighAvailability != nil && p.Spec.HighAvailability.Enabled)
			if tc.wantKeycloakNil {
				assert.Nil(t, p.Spec.Keycloak, "keycloak-mode none must omit spec.keycloak")
			} else {
				require.NotNil(t, p.Spec.Keycloak)
				assert.Equal(t, tc.wantKeycloakMode, p.Spec.Keycloak.Mode)
			}
			assert.NotEmpty(t, notes)
		})
	}
}

func TestScaffoldPlatform_FlagsOverrideProfile(t *testing.T) {
	// managed-ha profile defaults DB to managed; explicit --db-mode external must win.
	p, notes, err := ScaffoldPlatform(PlatformOptions{
		Name:      genILM,
		Namespace: genILM,
		Profile:   ProfileManagedHA,
		DBMode:    genExternal,
		Set:       map[string]bool{flagDBMode: true},
	})
	require.NoError(t, err)
	assert.Equal(t, genExternal, p.Spec.Database.Mode)

	var dbNote *EffectiveNote
	for i := range notes {
		if notes[i].Field == "database.mode" {
			dbNote = &notes[i]
		}
	}
	require.NotNil(t, dbNote)
	assert.Equal(t, genExternal, dbNote.Value)
	assert.Equal(t, "flag", dbNote.Source)
}

func TestScaffoldPlatform_KeycloakNoneOmits(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileExternal,
		KeycloakMode: modeNone, Set: map[string]bool{"keycloak-mode": true},
	})
	require.NoError(t, err)
	assert.Nil(t, p.Spec.Keycloak)
}

func TestScaffoldPlatform_NetworkPolicyDefaultTrue(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, Set: map[string]bool{}})
	require.NoError(t, err)
	require.NotNil(t, p.Spec.NetworkPolicy)
	require.NotNil(t, p.Spec.NetworkPolicy.Enabled)
	assert.True(t, *p.Spec.NetworkPolicy.Enabled)

	// explicit --network-policy=false wins
	p2, _, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileMinimal,
		NetworkPolicy: boolPtr(false), Set: map[string]bool{"network-policy": true},
	})
	require.NoError(t, err)
	require.NotNil(t, p2.Spec.NetworkPolicy.Enabled)
	assert.False(t, *p2.Spec.NetworkPolicy.Enabled)
}

func TestScaffoldPlatform_EdgeAndTLS(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileExternal,
		Edge: genGatewayAPI, TLSSource: genLetsEncrypt,
		Set: map[string]bool{flagEdge: true, genTLSSource: true},
	})
	require.NoError(t, err)
	require.NotNil(t, p.Spec.Edge)
	assert.True(t, p.Spec.Edge.Enabled)
	assert.Equal(t, genGatewayAPI, p.Spec.Edge.Type)
	require.NotNil(t, p.Spec.Edge.TLS)
	assert.Equal(t, genLetsEncrypt, p.Spec.Edge.TLS.Source)
}

func TestScaffoldPlatform_DeletionPolicyDefaultRetain(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, Set: map[string]bool{}})
	require.NoError(t, err)
	// Literal on purpose: the seed sets deletionPolicy from policyRetain, so asserting
	// against that same constant would pass whatever value it held. This pins the
	// CRD enum value the generated manifest must carry.
	assert.Equal(t, "Retain", string(p.Spec.DeletionPolicy))
}

func TestScaffoldPlatform_Validation(t *testing.T) {
	tests := []struct {
		name string
		o    PlatformOptions
	}{
		{"bad profile", PlatformOptions{Name: genILM, Namespace: genILM, Profile: Profile("bogus"), Set: map[string]bool{}}},
		{"bad db-mode", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, DBMode: "weird", Set: map[string]bool{flagDBMode: true}}},
		{"bad keycloak-mode", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, KeycloakMode: "auto", Set: map[string]bool{"keycloak-mode": true}}},
		{"bad tls-source", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, TLSSource: "acme", Set: map[string]bool{genTLSSource: true}}},
		{genEmptyName, PlatformOptions{Name: "", Namespace: genILM, Profile: ProfileMinimal, Set: map[string]bool{}}},
		// enum validation for flags the reviewer flagged as missing
		{"bad messaging-mode", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, MessagingMode: "kafka", Set: map[string]bool{"messaging-mode": true}}},
		{"bad broker-type", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, BrokerType: "kafka", Set: map[string]bool{"broker-type": true}}},
		{"bad provisioning-mode", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, ProvisioningMode: "auto", Set: map[string]bool{"provisioning-mode": true}}},
		{"bad edge", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, Edge: "traefik", Set: map[string]bool{flagEdge: true}}},
		{"bad deletion-policy", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, DeletionPolicy: "Keep", Set: map[string]bool{"deletion-policy": true}}},
		// deploy mode requires rabbitmq broker
		{"deploy+servicebus conflict", PlatformOptions{Name: genILM, Namespace: genILM, Profile: ProfileMinimal, ProvisioningMode: modeDeploy, BrokerType: "servicebus", Set: map[string]bool{"provisioning-mode": true, "broker-type": true}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ScaffoldPlatform(tc.o)
			assert.Error(t, err)
		})
	}
}

// TestScaffoldPlatform_CRDHostRule verifies that every profile's default scaffold
// satisfies the CRD rule "an enabled edge needs a public host: set edge.host or
// common.hostName" by setting common.hostName to a non-empty value.
func TestScaffoldPlatform_CRDHostRule(t *testing.T) {
	for _, profile := range []Profile{ProfileMinimal, ProfileExternal, ProfileManagedHA} {
		t.Run(string(profile), func(t *testing.T) {
			p, _, err := ScaffoldPlatform(PlatformOptions{
				Name: genILM, Namespace: genILM, Profile: profile, Set: map[string]bool{},
			})
			require.NoError(t, err)
			// The CRD requires: edge.enabled=true → edge.host != "" OR common.hostName != "".
			assert.NotEmpty(t, p.Spec.Common.HostName,
				"common.hostName must be non-empty so the edge-host CRD rule is satisfied")
		})
	}
}

// TestScaffoldPlatform_CRDProvisioningDeployRule verifies that when provisioning
// mode is deploy the scaffold satisfies the CRD rule
// "provisioning.deploy is required when mode=deploy" and that
// BootstrapSecretRef is non-empty (the field is marked +kubebuilder:validation:Required).
func TestScaffoldPlatform_CRDProvisioningDeployRule(t *testing.T) {
	p, _, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileManagedHA, Set: map[string]bool{},
	})
	require.NoError(t, err)
	require.NotNil(t, p.Spec.Provisioning)
	// Literal on purpose: the seed sets provisioningMode from modeDeploy, so asserting
	// against that same constant would pass whatever value it held.
	assert.Equal(t, "deploy", p.Spec.Provisioning.Mode)
	// The CRD requires provisioning.deploy to be present and bootstrapSecretRef non-empty.
	require.NotNil(t, p.Spec.Provisioning.Deploy,
		"provisioning.deploy must be set when mode=deploy")
	assert.NotEmpty(t, p.Spec.Provisioning.Deploy.BootstrapSecretRef,
		"provisioning.deploy.bootstrapSecretRef is required by the CRD when mode=deploy")
}

// TestScaffoldPlatform_TLSInternalNoCompanion verifies that when tls.source=internal
// (the default for all profiles) no companion sub-block is set, since the CRD only
// requires companions for non-internal sources.
func TestScaffoldPlatform_TLSInternalNoCompanion(t *testing.T) {
	for _, profile := range []Profile{ProfileMinimal, ProfileExternal, ProfileManagedHA} {
		t.Run(string(profile), func(t *testing.T) {
			p, _, err := ScaffoldPlatform(PlatformOptions{
				Name: genILM, Namespace: genILM, Profile: profile, Set: map[string]bool{},
			})
			require.NoError(t, err)
			require.NotNil(t, p.Spec.Edge)
			require.NotNil(t, p.Spec.Edge.TLS)
			assert.Equal(t, "internal", p.Spec.Edge.TLS.Source)
			assert.Nil(t, p.Spec.Edge.TLS.LetsEncrypt, "no letsEncrypt block for source=internal")
			assert.Nil(t, p.Spec.Edge.TLS.IssuerRef, "no issuerRef block for source=internal")
			assert.Nil(t, p.Spec.Edge.TLS.SecretRef, "no secretRef for source=internal")
		})
	}
}

// tlsCompanionCase is the test-row type for TestScaffoldPlatform_TLSCompanionPlaceholders.
type tlsCompanionCase struct {
	tlsSource        string
	checkLetsEncrypt bool
	checkIssuerRef   bool
	checkSecretRef   bool
}

// assertTLSCompanion checks that the scaffolded Platform has the correct TLS
// companion sub-block and at least one placeholder note for the given source.
func assertTLSCompanion(t *testing.T, tc tlsCompanionCase, p *otilmv1alpha1.Platform, notes []EffectiveNote) {
	t.Helper()
	require.NotNil(t, p.Spec.Edge.TLS)
	assert.Equal(t, tc.tlsSource, p.Spec.Edge.TLS.Source)
	if tc.checkLetsEncrypt {
		require.NotNil(t, p.Spec.Edge.TLS.LetsEncrypt,
			"letsEncrypt block required by CRD when source=letsEncrypt")
		assert.NotEmpty(t, p.Spec.Edge.TLS.LetsEncrypt.Email)
	}
	if tc.checkIssuerRef {
		require.NotNil(t, p.Spec.Edge.TLS.IssuerRef,
			"issuerRef block required by CRD when source=issuerRef")
		assert.NotEmpty(t, p.Spec.Edge.TLS.IssuerRef.Name)
		assert.Equal(t, "ClusterIssuer", p.Spec.Edge.TLS.IssuerRef.Kind)
	}
	if tc.checkSecretRef {
		require.NotNil(t, p.Spec.Edge.TLS.SecretRef,
			"secretRef required by CRD when source=secret")
		assert.NotEmpty(t, *p.Spec.Edge.TLS.SecretRef)
	}
	found := false
	for _, n := range notes {
		if n.Source == sourcePlaceholder {
			found = true
			break
		}
	}
	assert.True(t, found, "at least one placeholder note expected for tls-source=%s", tc.tlsSource)
}

// TestScaffoldPlatform_TLSCompanionPlaceholders verifies that each non-internal
// TLS source emits the required companion placeholder sub-block so the scaffold
// satisfies the CRD's XValidation rules for those sources.
func TestScaffoldPlatform_TLSCompanionPlaceholders(t *testing.T) {
	tests := []tlsCompanionCase{
		{genLetsEncrypt, true, false, false},
		{tlsIssuerRef, false, true, false},
		{tlsSecret, false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.tlsSource, func(t *testing.T) {
			p, notes, err := ScaffoldPlatform(PlatformOptions{
				Name: genILM, Namespace: genILM, Profile: ProfileMinimal,
				TLSSource: tc.tlsSource, Set: map[string]bool{genTLSSource: true},
			})
			require.NoError(t, err)
			assertTLSCompanion(t, tc, p, notes)
		})
	}
}

// TestScaffoldPlatform_GatewayAPIPlaceholder verifies that edge type=gatewayAPI
// emits a GatewayAPI sub-block with a placeholder gatewayClassName so the
// scaffold satisfies the CRD's XValidation rule requiring gatewayClassName or
// parentRef for gatewayAPI edges.
func TestScaffoldPlatform_GatewayAPIPlaceholder(t *testing.T) {
	p, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileMinimal,
		Edge: genGatewayAPI, Set: map[string]bool{flagEdge: true},
	})
	require.NoError(t, err)
	require.NotNil(t, p.Spec.Edge)
	assert.Equal(t, genGatewayAPI, p.Spec.Edge.Type)
	require.NotNil(t, p.Spec.Edge.GatewayAPI,
		"gatewayAPI block required by CRD when edge.type=gatewayAPI")
	require.NotNil(t, p.Spec.Edge.GatewayAPI.GatewayClassName)
	assert.NotEmpty(t, *p.Spec.Edge.GatewayAPI.GatewayClassName)

	found := false
	for _, n := range notes {
		if n.Field == "edge.gatewayAPI.gatewayClassName" && n.Source == sourcePlaceholder {
			found = true
			break
		}
	}
	assert.True(t, found, "placeholder note expected for edge.gatewayAPI.gatewayClassName")
}

// TestScaffoldPlatform_HostFlagOverridesPlaceholder verifies that when --host is
// provided it is used verbatim (source=flag) rather than synthesising a placeholder.
func TestScaffoldPlatform_HostFlagOverridesPlaceholder(t *testing.T) {
	p, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: genILM, Namespace: genILM, Profile: ProfileMinimal,
		HostName: "ilm.corp.example.com",
		Set:      map[string]bool{"host": true},
	})
	require.NoError(t, err)
	assert.Equal(t, "ilm.corp.example.com", p.Spec.Common.HostName)

	var hostNote *EffectiveNote
	for i := range notes {
		if notes[i].Field == fieldHostName {
			hostNote = &notes[i]
		}
	}
	require.NotNil(t, hostNote)
	assert.Equal(t, sourceFlag, hostNote.Source)
}

// TestScaffoldPlatform_PlaceholderHostNote verifies that when no --host flag is
// given a placeholder note is recorded for common.hostName.
func TestScaffoldPlatform_PlaceholderHostNote(t *testing.T) {
	_, notes, err := ScaffoldPlatform(PlatformOptions{
		Name: genMyApp, Namespace: genILM, Profile: ProfileMinimal, Set: map[string]bool{},
	})
	require.NoError(t, err)
	var hostNote *EffectiveNote
	for i := range notes {
		if notes[i].Field == fieldHostName {
			hostNote = &notes[i]
		}
	}
	require.NotNil(t, hostNote)
	assert.Equal(t, sourcePlaceholder, hostNote.Source)
	assert.Equal(t, "myapp.example.com", hostNote.Value)
}

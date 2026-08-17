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

// Package generate scaffolds otilm.com/v1alpha1 custom resources from option
// structs. A profile seeds defaults; explicitly-set flags always override;
// every effective value is captured as an EffectiveNote so callers can echo it
// as YAML comments.
package generate

import (
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// Profile is a starting template that seeds default field values. Explicit
// flags in PlatformOptions.Set always override the profile seed.
type Profile string

// Platform profile constants.
const (
	ProfileMinimal   Profile = "minimal"
	ProfileExternal  Profile = "external"
	ProfileManagedHA Profile = "managed-ha"
)

// PlatformOptions are the inputs to ScaffoldPlatform. Set tracks which flag
// names the caller explicitly provided so their values win over profile
// defaults and the effective-value source can be reported accurately.
type PlatformOptions struct {
	Name, Namespace, Version string
	HostName                 string // public FQDN; satisfies the CRD's edge-host requirement
	Profile                  Profile
	DBMode                   string // external|managed
	MessagingMode            string // external|managed
	BrokerType               string // rabbitmq|servicebus
	KeycloakMode             string // none|external|managed  (none => omit spec.keycloak)
	ProvisioningMode         string // external|deploy
	Edge                     string // ingress|gatewayAPI
	TLSSource                string // internal|letsEncrypt|issuerRef|secret
	HA                       bool
	NetworkPolicy            *bool
	DeletionPolicy           string // Retain|Delete
	Set                      map[string]bool
}

// EffectiveNote records one resolved field value and the source that
// determined it. Source is one of "flag", "profile", "default", or
// "placeholder".
type EffectiveNote struct {
	Field  string
	Value  string
	Source string
}

// profileSeed holds the defaults a profile seeds before flag overrides apply.
type profileSeed struct {
	dbMode, msgMode, brokerType, keycloakMode string
	provisioningMode, edge, tlsSource         string
	deletionPolicy                            string
	ha                                        bool
}

func seedFor(p Profile) (profileSeed, bool) {
	switch p {
	case ProfileMinimal:
		return profileSeed{
			dbMode: modeExternal, msgMode: modeExternal, brokerType: brokerRabbitMQ,
			keycloakMode: modeNone, provisioningMode: modeExternal,
			edge: edgeIngress, tlsSource: tlsInternal, deletionPolicy: policyRetain,
			ha: false,
		}, true
	case ProfileExternal:
		return profileSeed{
			dbMode: modeExternal, msgMode: modeExternal, brokerType: brokerRabbitMQ,
			keycloakMode: modeExternal, provisioningMode: modeExternal,
			edge: edgeIngress, tlsSource: tlsInternal, deletionPolicy: policyRetain,
			ha: false,
		}, true
	case ProfileManagedHA:
		return profileSeed{
			dbMode: modeManaged, msgMode: modeManaged, brokerType: brokerRabbitMQ,
			keycloakMode: modeManaged, provisioningMode: modeDeploy,
			edge: edgeIngress, tlsSource: tlsInternal, deletionPolicy: policyRetain,
			ha: true,
		}, true
	default:
		return profileSeed{}, false
	}
}

// Note source constants for EffectiveNote.Source.
const (
	sourceFlag        = "flag"
	sourceProfile     = "profile"
	sourceDefault     = "default"
	sourcePlaceholder = "placeholder"
)

// Spec vocabulary shared by the profile seeds, validators and note fields.
const (
	modeExternal = "external"
	modeManaged  = "managed"
	modeNone     = "none"
	modeDeploy   = "deploy"

	brokerRabbitMQ = "rabbitmq"
	edgeIngress    = "ingress"
	tlsInternal    = "internal"

	policyRetain = "Retain"
	tlsIssuerRef = "issuerRef"
	tlsSecret    = "secret"

	flagDBMode = "db-mode"
	flagEdge   = "edge"

	fieldHostName    = "common.hostName"
	fieldImageDigest = "image.digest"
)

var (
	validDBModes       = map[string]bool{modeExternal: true, modeManaged: true}
	validMsgModes      = map[string]bool{modeExternal: true, modeManaged: true}
	validBrokerTypes   = map[string]bool{brokerRabbitMQ: true, "servicebus": true}
	validKeycloakModes = map[string]bool{modeNone: true, modeExternal: true, modeManaged: true}
	validProvModes     = map[string]bool{modeExternal: true, modeDeploy: true}
	validEdges         = map[string]bool{edgeIngress: true, "gatewayAPI": true}
	validTLSSources    = map[string]bool{tlsInternal: true, "letsEncrypt": true, tlsIssuerRef: true, tlsSecret: true}
	validDeletionPols  = map[string]bool{policyRetain: true, "Delete": true}
)

// noteResolver folds a profile seed and an optional flag override into the
// final value, recording an EffectiveNote for each field.
type noteResolver struct {
	set   map[string]bool
	notes []EffectiveNote
}

// pick returns flagVal when flagName was explicitly set, else seedVal; it
// records the field/value/source as an EffectiveNote.
func (r *noteResolver) pick(field, flagName, flagVal, seedVal string) string {
	if r.set[flagName] {
		r.notes = append(r.notes, EffectiveNote{Field: field, Value: flagVal, Source: sourceFlag})
		return flagVal
	}
	r.notes = append(r.notes, EffectiveNote{Field: field, Value: seedVal, Source: sourceProfile})
	return seedVal
}

// placeholder records an EffectiveNote with source=placeholder and returns value.
func (r *noteResolver) placeholder(field, value string) string {
	r.notes = append(r.notes, EffectiveNote{Field: field, Value: value, Source: sourcePlaceholder})
	return value
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// resolvedFields holds the validated field values produced by resolveEnumFields.
type resolvedFields struct {
	dbMode, msgMode, brokerType string
	keycloakMode, provMode      string
	edge, tlsSource             string
	deletionPolicy              string
}

// resolveEnumFields picks and validates all enum-typed fields from o against seed.
func resolveEnumFields(r *noteResolver, o PlatformOptions, seed profileSeed) (resolvedFields, error) {
	f := resolvedFields{}

	f.dbMode = r.pick("database.mode", flagDBMode, o.DBMode, seed.dbMode)
	if !validDBModes[f.dbMode] {
		return f, fmt.Errorf("invalid db-mode %q (want external|managed)", f.dbMode)
	}

	f.msgMode = r.pick("messaging.mode", "messaging-mode", o.MessagingMode, seed.msgMode)
	if !validMsgModes[f.msgMode] {
		return f, fmt.Errorf("invalid messaging-mode %q (want external|managed)", f.msgMode)
	}

	f.brokerType = r.pick("messaging.brokerType", "broker-type", o.BrokerType, seed.brokerType)
	if !validBrokerTypes[f.brokerType] {
		return f, fmt.Errorf("invalid broker-type %q (want rabbitmq|servicebus)", f.brokerType)
	}

	f.keycloakMode = r.pick("keycloak.mode", "keycloak-mode", o.KeycloakMode, seed.keycloakMode)
	if !validKeycloakModes[f.keycloakMode] {
		return f, fmt.Errorf("invalid keycloak-mode %q (want none|external|managed)", f.keycloakMode)
	}

	f.provMode = r.pick("provisioning.mode", "provisioning-mode", o.ProvisioningMode, seed.provisioningMode)
	if !validProvModes[f.provMode] {
		return f, fmt.Errorf("invalid provisioning-mode %q (want external|deploy)", f.provMode)
	}
	if f.provMode == modeDeploy && f.brokerType != brokerRabbitMQ {
		return f, fmt.Errorf("provisioning mode=deploy requires broker-type=rabbitmq (got %q)", f.brokerType)
	}

	f.edge = r.pick("edge.type", flagEdge, o.Edge, seed.edge)
	if !validEdges[f.edge] {
		return f, fmt.Errorf("invalid edge %q (want ingress|gatewayAPI)", f.edge)
	}

	f.tlsSource = r.pick("edge.tls.source", "tls-source", o.TLSSource, seed.tlsSource)
	if !validTLSSources[f.tlsSource] {
		return f, fmt.Errorf("invalid tls-source %q (want internal|letsEncrypt|issuerRef|secret)", f.tlsSource)
	}

	f.deletionPolicy = r.pick("deletionPolicy", "deletion-policy", o.DeletionPolicy, seed.deletionPolicy)
	if !validDeletionPols[f.deletionPolicy] {
		return f, fmt.Errorf("invalid deletion-policy %q (want Retain|Delete)", f.deletionPolicy)
	}

	return f, nil
}

// wireProvisioningDeploy attaches the deploy block when provMode=deploy.
func wireProvisioningDeploy(r *noteResolver, p *otilmv1alpha1.Platform, provMode string) {
	if provMode != modeDeploy {
		return
	}
	bootstrapRef := r.placeholder("provisioning.deploy.bootstrapSecretRef", "<placeholder, e.g. ilm-provisioning-bootstrap>")
	p.Spec.Provisioning.Deploy = &otilmv1alpha1.ProvisioningDeploySpec{
		BootstrapSecretRef: bootstrapRef,
	}
}

// wireTLSCompanions attaches the TLS source-specific sub-block required by the CRD.
// source=internal needs no companion.
func wireTLSCompanions(r *noteResolver, p *otilmv1alpha1.Platform, tlsSource string) {
	switch tlsSource {
	case "letsEncrypt":
		email := r.placeholder("edge.tls.letsEncrypt.email", "<placeholder, e.g. admin@example.com>")
		p.Spec.Edge.TLS.LetsEncrypt = &otilmv1alpha1.LetsEncryptSpec{Email: email}
	case tlsIssuerRef:
		name := r.placeholder("edge.tls.issuerRef.name", "<placeholder>")
		p.Spec.Edge.TLS.IssuerRef = &otilmv1alpha1.CertManagerIssuerRef{
			Name: name,
			Kind: "ClusterIssuer",
		}
	case tlsSecret:
		secretRef := r.placeholder("edge.tls.secretRef", "<placeholder, e.g. ilm-ingress-tls>")
		p.Spec.Edge.TLS.SecretRef = strPtr(secretRef)
	}
}

// wireGatewayAPI attaches the gatewayAPI block when edge=gatewayAPI.
func wireGatewayAPI(r *noteResolver, p *otilmv1alpha1.Platform, edge string) {
	if edge != "gatewayAPI" {
		return
	}
	className := r.placeholder("edge.gatewayAPI.gatewayClassName", "<placeholder>")
	p.Spec.Edge.GatewayAPI = &otilmv1alpha1.GatewayAPISpec{
		GatewayClassName: strPtr(className),
	}
}

// resolveHostName resolves the hostName from the option set or generates a placeholder.
func resolveHostName(r *noteResolver, o PlatformOptions) string {
	if o.Set["host"] {
		r.notes = append(r.notes, EffectiveNote{Field: fieldHostName, Value: o.HostName, Source: sourceFlag})
		return o.HostName
	}
	hostName := fmt.Sprintf("%s.example.com", o.Name)
	r.placeholder(fieldHostName, hostName)
	return hostName
}

// resolveHAAndNetworkPolicy appends the HA and NetworkPolicy effective notes and
// returns the resolved values.
func resolveHAAndNetworkPolicy(r *noteResolver, o PlatformOptions, seed profileSeed) (ha bool, npEnabled bool) {
	ha = seed.ha
	haSource := sourceProfile
	if o.Set["ha"] {
		ha = o.HA
		haSource = sourceFlag
	}
	r.notes = append(r.notes, EffectiveNote{Field: "highAvailability.enabled", Value: fmt.Sprintf("%t", ha), Source: haSource})

	npEnabled = true
	npSource := sourceDefault
	if o.Set["network-policy"] && o.NetworkPolicy != nil {
		npEnabled = *o.NetworkPolicy
		npSource = sourceFlag
	}
	r.notes = append(r.notes, EffectiveNote{Field: "networkPolicy.enabled", Value: fmt.Sprintf("%t", npEnabled), Source: npSource})
	return ha, npEnabled
}

// ScaffoldPlatform builds a typed Platform from a profile + explicit flags.
// The profile seeds all defaults; any flag listed in o.Set overrides the seed.
// Every effective value is captured in the returned []EffectiveNote slice,
// sorted by field name for deterministic output.
//
// The resulting CR satisfies the CRD's CEL XValidation rules by emitting
// clearly-marked PLACEHOLDER values for user-specific data the scaffolder
// cannot know (hostname, TLS credentials, provisioning bootstrap secret).
// Notes with Source="placeholder" are rendered with a louder TODO comment by
// Render so the user cannot miss them.
func ScaffoldPlatform(o PlatformOptions) (*otilmv1alpha1.Platform, []EffectiveNote, error) {
	if o.Name == "" {
		return nil, nil, fmt.Errorf("platform name is required")
	}
	if o.Set == nil {
		o.Set = map[string]bool{}
	}

	seed, ok := seedFor(o.Profile)
	if !ok {
		return nil, nil, fmt.Errorf("unknown profile %q (want minimal|external|managed-ha)", o.Profile)
	}

	r := &noteResolver{set: o.Set}

	f, err := resolveEnumFields(r, o, seed)
	if err != nil {
		return nil, nil, err
	}

	ha, npEnabled := resolveHAAndNetworkPolicy(r, o, seed)

	// Version note (only when set).
	if o.Set["version"] || o.Version != "" {
		src := sourceDefault
		if o.Set["version"] {
			src = sourceFlag
		}
		r.notes = append(r.notes, EffectiveNote{Field: "version", Value: o.Version, Source: src})
	}

	hostName := resolveHostName(r, o)

	p := &otilmv1alpha1.Platform{
		TypeMeta: metav1.TypeMeta{
			APIVersion: otilmv1alpha1.GroupVersion.String(),
			Kind:       "Platform",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: o.Namespace,
		},
		Spec: otilmv1alpha1.PlatformSpec{
			Version:  o.Version,
			Common:   otilmv1alpha1.CommonSpec{HostName: hostName},
			Database: otilmv1alpha1.DatabaseSpec{Mode: f.dbMode},
			Messaging: otilmv1alpha1.MessagingSpec{
				Mode:       f.msgMode,
				BrokerType: f.brokerType,
			},
			Provisioning: &otilmv1alpha1.ProvisioningSpec{Mode: f.provMode},
			HighAvailability: &otilmv1alpha1.HighAvailabilitySpec{
				Enabled: ha,
			},
			NetworkPolicy: &otilmv1alpha1.NetworkPolicySpec{
				Enabled: &npEnabled,
			},
			Edge: &otilmv1alpha1.EdgeSpec{
				Enabled: true,
				Type:    f.edge,
				TLS:     &otilmv1alpha1.EdgeTLSSpec{Source: f.tlsSource},
			},
			DeletionPolicy: otilmv1alpha1.PlatformDeletionPolicy(f.deletionPolicy),
		},
	}

	// keycloak-mode "none" OMITS spec.keycloak entirely; the CRD enum only accepts
	// "external" and "managed" — "none" is a CLI-level sentinel that means omit.
	if f.keycloakMode != modeNone {
		p.Spec.Keycloak = &otilmv1alpha1.KeycloakSpec{Mode: f.keycloakMode}
	}

	wireProvisioningDeploy(r, p, f.provMode)
	wireTLSCompanions(r, p, f.tlsSource)
	wireGatewayAPI(r, p, f.edge)

	sort.SliceStable(r.notes, func(i, j int) bool {
		return r.notes[i].Field < r.notes[j].Field
	})
	return p, r.notes, nil
}

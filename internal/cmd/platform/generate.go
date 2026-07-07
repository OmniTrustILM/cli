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

package platform

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/cmd/shared"
	"github.com/OmniTrustILM/cli/internal/generate"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

// generateClientFor is the injectable seam that resolves a *k8s.Client.
// Tests override it to inject a fake without a live apiserver.
var generateClientFor = func(o *cli.Options) (*k8s.Client, error) { return o.Factory.Client() }

// generateFlags holds the bound flag values for `platform generate`.
type generateFlags struct {
	name, namespace, version string
	profile                  string
	dbMode, messagingMode    string
	brokerType, keycloakMode string
	provisioningMode         string
	edge, tlsSource          string
	host                     string
	ha                       bool
	networkPolicy            bool
	deletionPolicy           string
	interactive              bool
	apply                    bool
	dryRun                   string
	forceConflicts           bool
}

// NewGenerateCommand builds the `platform generate` subcommand.
func NewGenerateCommand(o *cli.Options) *cobra.Command {
	f := &generateFlags{}
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scaffold a Platform CR",
		Long: "Scaffold a Platform custom resource. A --profile seeds defaults " +
			"(minimal|external|managed-ha); any explicit flag always overrides the profile, " +
			"and the effective values are echoed as YAML comments. Use --apply to server-side " +
			"apply directly, or --dry-run=server to validate against the cluster's CEL rules.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGenerate(cmd, o, f)
		},
	}
	fs := cmd.Flags()
	fs.StringVar(&f.name, "name", "", "Platform resource name (required)")
	fs.StringVarP(&f.namespace, "namespace", "n", "", "Target namespace")
	fs.StringVar(&f.version, "version", "", "Platform version bundle (default: operator newest)")
	fs.StringVar(&f.profile, "profile", "external", "Profile: minimal|external|managed-ha")
	fs.StringVar(&f.dbMode, "db-mode", "", "Database mode: external|managed")
	fs.StringVar(&f.messagingMode, "messaging-mode", "", "Messaging mode: external|managed")
	fs.StringVar(&f.brokerType, "broker-type", "", "Broker type: rabbitmq|servicebus")
	fs.StringVar(&f.keycloakMode, "keycloak-mode", "", "Keycloak mode: none|external|managed (none omits spec.keycloak)")
	fs.StringVar(&f.provisioningMode, "provisioning-mode", "", "Provisioning mode: external|deploy")
	fs.StringVar(&f.edge, "edge", "", "Edge type: ingress|gatewayAPI")
	fs.StringVar(&f.tlsSource, "tls-source", "", "TLS source: internal|letsEncrypt|issuerRef|secret")
	fs.StringVar(&f.host, "host", "", "Public FQDN for the platform edge (sets common.hostName)")
	fs.BoolVar(&f.ha, "ha", false, "Enable the high-availability profile")
	fs.BoolVar(&f.networkPolicy, "network-policy", true, "Render default-deny NetworkPolicies")
	fs.StringVar(&f.deletionPolicy, "deletion-policy", "", "Deletion policy: Retain|Delete")
	fs.BoolVar(&f.interactive, "interactive", false, "Run an interactive TTY wizard")
	fs.BoolVar(&f.apply, "apply", false, "Server-side apply the generated CR (field manager ilmctl)")
	fs.StringVar(&f.dryRun, "dry-run", "none", "Dry-run mode for --apply: none|client|server (server runs CEL validation)")
	fs.BoolVar(&f.forceConflicts, "force-conflicts", false, "Force-apply on field-manager conflicts")
	return cmd
}

// runGenerate is the RunE handler for `platform generate`.
func runGenerate(cmd *cobra.Command, o *cli.Options, f *generateFlags) error {
	if f.interactive {
		if err := runInteractive(o, f); err != nil {
			return err
		}
	}

	set := shared.ChangedFlags(cmd)
	var npPtr *bool
	if set["network-policy"] {
		v := f.networkPolicy
		npPtr = &v
	}

	p, notes, err := generate.ScaffoldPlatform(generate.PlatformOptions{
		Name:             f.name,
		Namespace:        f.namespace,
		Version:          f.version,
		HostName:         f.host,
		Profile:          generate.Profile(f.profile),
		DBMode:           f.dbMode,
		MessagingMode:    f.messagingMode,
		BrokerType:       f.brokerType,
		KeycloakMode:     f.keycloakMode,
		ProvisioningMode: f.provisioningMode,
		Edge:             f.edge,
		TLSSource:        f.tlsSource,
		HA:               f.ha,
		NetworkPolicy:    npPtr,
		DeletionPolicy:   f.deletionPolicy,
		Set:              set,
	})
	if err != nil {
		return err
	}

	if f.apply {
		c, err := generateClientFor(o)
		if err != nil {
			return err
		}
		return shared.ApplyObject(cmd.Context(), o, c, p, f.dryRun, f.forceConflicts)
	}

	yaml, err := generate.Render(p, notes)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(o.Out, yaml)
	return nil
}

// runInteractive prompts for the core platform choices on a TTY, filling the flag
// struct in-place. It refuses to run when stdin is not a terminal.
func runInteractive(o *cli.Options, f *generateFlags) error {
	if !cli.IsTerminal(o.In) {
		return fmt.Errorf("--interactive requires a TTY")
	}
	prompts := []struct {
		label string
		dst   *string
	}{
		{"Platform name", &f.name},
		{"Namespace", &f.namespace},
		{"Database mode (external|managed)", &f.dbMode},
		{"Messaging mode (external|managed)", &f.messagingMode},
		{"Keycloak mode (none|external|managed)", &f.keycloakMode},
	}
	reader := bufio.NewReader(o.In)
	for _, p := range prompts {
		_, _ = fmt.Fprintf(o.Out, "%s [%s]: ", p.label, *p.dst)
		line, _ := reader.ReadString('\n')
		if v := strings.TrimSpace(line); v != "" {
			*p.dst = v
		}
	}
	return nil
}

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
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/version"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// NewUpgradeCommand builds `platform upgrade NAME --to vX`. The upgrade is
// forward-only: --to at or below the current observedVersion is rejected. The
// --ack-* flags set the per-module managed UpgradeAcknowledged flags the
// operator requires for a major version bump.
func NewUpgradeCommand(o *cli.Options) *cobra.Command {
	var (
		to           string
		ackDatabase  bool
		ackMessaging bool
		ackKeycloak  bool
		dryRun       bool
		force        bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade NAME",
		Short: "Upgrade a Platform to a newer version (forward-only)",
		Long: "Patch spec.version forward. Downgrades and same-version re-applies are " +
			"rejected. A major bump of managed database/messaging/Keycloak is blocked by the " +
			"operator until acknowledged: pass --ack-database/--ack-messaging/--ack-keycloak " +
			"to set the managed UpgradeAcknowledged flags.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to is required")
			}
			return runUpgrade(cmd, o, upgradeRequest{
				namespace: upgradeNamespace(cmd, o),
				name:      args[0],
				to:        to,
				ackDB:     ackDatabase,
				ackMsg:    ackMessaging,
				ackKC:     ackKeycloak,
				dryRun:    dryRun,
				force:     force,
			})
		},
	}
	cmd.Flags().StringVarP(new(string), "namespace", "n", "", "Namespace of the Platform (default from context)")
	cmd.Flags().StringVar(&to, "to", "", "Target version (required)")
	cmd.Flags().BoolVar(&ackDatabase, "ack-database", false, "Acknowledge a major managed-database upgrade")
	cmd.Flags().BoolVar(&ackMessaging, "ack-messaging", false, "Acknowledge a major managed-messaging upgrade")
	cmd.Flags().BoolVar(&ackKeycloak, "ack-keycloak", false, "Acknowledge a major managed-Keycloak upgrade")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report the planned change without patching")
	cmd.Flags().BoolVar(&force, "force", false, "Bypass the supported-version compatibility check (does not override the no-downgrade rule)")
	return cmd
}

// upgradeNamespace returns the namespace for the upgrade command, preferring an
// explicit -n/--namespace flag over the factory-resolved default.
func upgradeNamespace(cmd *cobra.Command, o *cli.Options) string {
	if nf := cmd.Flags().Lookup("namespace"); nf != nil && nf.Changed {
		return nf.Value.String()
	}
	return resolveNamespace("", o)
}

// upgradeRequest carries the resolved inputs for a single upgrade invocation.
type upgradeRequest struct {
	namespace, name, to  string
	ackDB, ackMsg, ackKC bool
	dryRun, force        bool
}

// runUpgrade performs the actual patch after all validations pass.
func runUpgrade(cmd *cobra.Command, o *cli.Options, req upgradeRequest) error {
	c, err := o.Factory.Client()
	if err != nil {
		return err
	}
	p, err := c.GetPlatform(cmd.Context(), req.namespace, req.name)
	if err != nil {
		return err
	}
	current := effectiveVersion(p)
	if err := guardForwardOnly(req.to, current); err != nil {
		return err
	}
	if err := guardCompatibility(req.to, req.force, o); err != nil {
		return err
	}
	patched := applyAckFlags(p, req.to, req.ackDB, req.ackMsg, req.ackKC)
	if req.dryRun {
		_, _ = fmt.Fprintf(o.Out, "(dry-run) would upgrade %s/%s from %s to %s\n", req.namespace, req.name, current, req.to)
		return nil
	}
	if err := c.Typed.Patch(cmd.Context(), patched, ctrlclient.MergeFrom(p)); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.Out, "Platform %s/%s upgraded to %s\n", req.namespace, req.name, req.to)
	return nil
}

// effectiveVersion returns the version to compare against for the forward-only
// guard: ObservedVersion when the operator has reconciled, Spec.Version otherwise.
func effectiveVersion(p *otilmv1alpha1.Platform) string {
	if p.Status.ObservedVersion != "" {
		return p.Status.ObservedVersion
	}
	return p.Spec.Version
}

// guardForwardOnly rejects --to values that are not strictly ahead of current.
// It is a no-op when current is empty (the Platform has never been reconciled).
func guardForwardOnly(to, current string) error {
	if current == "" {
		return nil
	}
	cmp, ok := compareVersions(to, current)
	if ok && cmp <= 0 {
		return fmt.Errorf("no downgrade: --to %s is not ahead of the current version %s", to, current)
	}
	return nil
}

// guardCompatibility rejects unsupported target versions unless --force is set,
// in which case it emits a warning and continues.
func guardCompatibility(to string, force bool, o *cli.Options) error {
	compat := version.CheckOperator(to)
	if compat.Supported {
		return nil
	}
	if !force {
		msg := fmt.Sprintf("version %s is not supported by this CLI build (supported: %s)",
			to, strings.Join(compat.SupportedVersions, ", "))
		if compat.Message != "" {
			msg += "; " + compat.Message
		}
		return fmt.Errorf("%s", msg)
	}
	_, _ = fmt.Fprintf(o.ErrOut,
		"warning: version %s is not in the supported set — proceeding because --force was set (supported: %s)\n",
		to, strings.Join(compat.SupportedVersions, ", "))
	return nil
}

// applyAckFlags deep-copies the Platform, sets spec.version, and conditionally
// sets the UpgradeAcknowledged flag on each managed subsystem.
func applyAckFlags(p *otilmv1alpha1.Platform, to string, ackDB, ackMsg, ackKC bool) *otilmv1alpha1.Platform {
	patched := p.DeepCopy()
	patched.Spec.Version = to
	if ackDB && patched.Spec.Database.Managed != nil {
		patched.Spec.Database.Managed.UpgradeAcknowledged = true
	}
	if ackMsg && patched.Spec.Messaging.Managed != nil {
		patched.Spec.Messaging.Managed.UpgradeAcknowledged = true
	}
	if ackKC && patched.Spec.Keycloak != nil && patched.Spec.Keycloak.Managed != nil {
		patched.Spec.Keycloak.Managed.UpgradeAcknowledged = true
	}
	return patched
}

// compareVersions compares two dotted-numeric versions (an optional leading "v" is
// tolerated). It returns (-1, 0, or 1, true) on success, or (0, false) when either
// side cannot be parsed as a dotted-numeric string.
func compareVersions(a, b string) (int, bool) {
	pa, oka := splitVersion(a)
	pb, okb := splitVersion(b)
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		switch {
		case x < y:
			return -1, true
		case x > y:
			return 1, true
		}
	}
	return 0, true
}

func splitVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, len(out) > 0
}

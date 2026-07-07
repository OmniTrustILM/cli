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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
)

// NewDeleteCommand builds `platform delete NAME`. It reports the effective
// deletionPolicy before asking for confirmation, since the Delete policy
// reclaims managed infrastructure and its data.
func NewDeleteCommand(o *cli.Options) *cobra.Command {
	var (
		namespace string
		yes       bool
		wait      bool
	)
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a Platform",
		Long: "Delete a Platform CR. The effective deletionPolicy is reported before " +
			"deletion. Use -y to skip the confirmation prompt; in non-interactive " +
			"streams -y is required.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ns := resolveNamespace(namespace, o)
			c, err := o.Factory.Client()
			if err != nil {
				return err
			}
			p, err := c.GetPlatform(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			if err := printDeletionPolicyWarning(o, ns, name, p); err != nil {
				return err
			}
			result, err := checkDeleteConfirmation(o, yes, name)
			if err != nil {
				return err
			}
			if result == confirmationDeclined {
				return nil
			}
			if err := c.Typed.Delete(cmd.Context(), p); err != nil {
				return err
			}
			return reportDeletion(cmd.Context(), o, c.Typed, ns, name, wait)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace of the Platform (default from context)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait until the Platform is fully removed")
	return cmd
}

// printDeletionPolicyWarning prints the effective deletion policy and a data-loss
// warning when the policy is Delete.
func printDeletionPolicyWarning(o *cli.Options, ns, name string, p *otilmv1alpha1.Platform) error {
	policy := p.Spec.DeletionPolicy
	if policy == "" {
		policy = otilmv1alpha1.PlatformDeletionPolicyRetain
	}
	_, _ = fmt.Fprintf(o.Out, "Platform %s/%s has deletionPolicy: %s\n", ns, name, policy)
	if policy == otilmv1alpha1.PlatformDeletionPolicyDelete {
		_, _ = fmt.Fprintln(o.Out, "WARNING: managed infrastructure (database, broker, Keycloak) and its data will be reclaimed.")
	} else {
		_, _ = fmt.Fprintln(o.Out, "Managed infrastructure and its data will be retained.")
	}
	return nil
}

// confirmationResult is returned by checkDeleteConfirmation to distinguish a
// hard error from the user declining the prompt.
type confirmationResult int

const (
	confirmationGranted confirmationResult = iota
	confirmationDeclined
)

// checkDeleteConfirmation enforces the interactive confirmation gate. It returns
// (confirmationGranted, nil) when the user confirms or -y was passed,
// (confirmationDeclined, nil) when the user declines, and (_, err) on a hard error
// such as a non-interactive stream without -y.
func checkDeleteConfirmation(o *cli.Options, yes bool, name string) (confirmationResult, error) {
	if yes {
		return confirmationGranted, nil
	}
	if !cli.IsTerminal(o.In) {
		return confirmationGranted, fmt.Errorf("refusing to delete without -y on a non-interactive stream")
	}
	if !confirmDelete(o, fmt.Sprintf("Delete Platform %q?", name)) {
		_, _ = fmt.Fprintln(o.Out, "Aborted.")
		return confirmationDeclined, nil
	}
	return confirmationGranted, nil
}

// reportDeletion prints the post-delete message. When wait is true it polls
// until the Platform is gone before printing the final confirmation.
func reportDeletion(ctx context.Context, o *cli.Options, c ctrlclient.Client, ns, name string, wait bool) error {
	if !wait {
		_, _ = fmt.Fprintf(o.Out, "Platform %s/%s deleted (removal may still be in progress)\n", ns, name)
		return nil
	}
	_, _ = fmt.Fprintf(o.Out, "Platform %s/%s deletion requested, waiting for removal…\n", ns, name)
	if err := waitPlatformGone(ctx, c, ns, name); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(o.Out, "Platform %s/%s deleted\n", ns, name)
	return nil
}

// confirmDelete reads a yes/no answer from the input stream and returns true for "y"/"yes".
func confirmDelete(o *cli.Options, prompt string) bool {
	_, _ = fmt.Fprintf(o.Out, "%s [y/N]: ", prompt)
	reader := bufio.NewReader(o.In)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

// waitPlatformGone polls until the Platform is no longer found in the cluster,
// timing out after five minutes.
func waitPlatformGone(ctx context.Context, c ctrlclient.Client, ns, name string) error {
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		var p otilmv1alpha1.Platform
		err := c.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, &p)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for Platform %q to be removed", name)
}

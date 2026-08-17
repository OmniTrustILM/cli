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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

// NewCredentialsCommand builds `platform credentials NAME`. It resolves the
// registerAdmin certificate/password Secret references and reports the active
// method(s) with the Secret name, keys, and namespace. Values are redacted
// unless -y is passed. This command never generates, mints, or patches any
// credential.
func NewCredentialsCommand(o *cli.Options) *cobra.Command {
	var reveal bool
	cmd := &cobra.Command{
		Use:   "credentials NAME",
		Short: "Show first-admin credential references (never generates)",
		Long: "Resolve the registerAdmin certificate/password Secret references and " +
			"report the active method(s), Secret name, keys, and namespace. Values " +
			"are redacted unless -y. No credentials are ever generated or modified.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ns := resolveNamespace("", o)
			if nf := cmd.Flags().Lookup("namespace"); nf != nil && nf.Changed {
				ns = nf.Value.String()
			}

			c, err := o.Factory.Client()
			if err != nil {
				return err
			}
			p, err := c.GetPlatform(cmd.Context(), ns, name)
			if err != nil {
				return err
			}
			ra := p.Spec.RegisterAdmin
			if ra == nil || !ra.Enabled {
				_, _ = fmt.Fprintf(o.Out, "Platform %s/%s has no registerAdmin bootstrap configured.\n", ns, name)
				return nil
			}
			reportCertificate(cmd.Context(), o, c, ns, ra, reveal)
			reportPassword(cmd.Context(), o, c, ns, ra, reveal)
			return nil
		},
	}
	cmd.Flags().StringVarP(new(string), "namespace", "n", "", "Namespace of the Platform (default from context)")
	cmd.Flags().BoolVarP(&reveal, "yes", "y", false, "Reveal secret values instead of redacting")
	return cmd
}

// certActive reports whether the certificate method is active (it defaults to true).
func certActive(c *otilmv1alpha1.AdminCertificateSpec) bool {
	if c == nil {
		return true // certificate method defaults to ON
	}
	return c.Enabled == nil || *c.Enabled
}

// certSourceProvided marks a registerAdmin certificate backed by a user-supplied Secret.
const certSourceProvided = "provided"

func reportCertificate(ctx context.Context, o *cli.Options, c *k8s.Client, ns string, ra *otilmv1alpha1.RegisterAdminSpec, reveal bool) {
	cert := ra.Certificate
	if !certActive(cert) {
		return
	}
	if cert == nil {
		cert = &otilmv1alpha1.AdminCertificateSpec{Source: certSourceProvided}
	}
	src := defaultStr(cert.Source, certSourceProvided)
	_, _ = fmt.Fprintf(o.Out, "Method: certificate (source=%s)\n", src)
	if src != certSourceProvided {
		_, _ = fmt.Fprintln(o.Out, "  cert-manager-issued (no user-supplied Secret to resolve)")
		return
	}
	if cert.SecretRef == nil || *cert.SecretRef == "" {
		_, _ = fmt.Fprintln(o.Out, "  (no secretRef set)")
		return
	}
	certKey := defaultStr(cert.CertKey, "tls.crt")
	keyKey := defaultStr(cert.PrivateKeyKey, "tls.key")
	printSecret(ctx, o, c, ns, *cert.SecretRef, []string{certKey, keyKey}, reveal)
}

func reportPassword(ctx context.Context, o *cli.Options, c *k8s.Client, ns string, ra *otilmv1alpha1.RegisterAdminSpec, reveal bool) {
	pw := ra.Password
	if pw == nil || !pw.Enabled {
		return
	}
	_, _ = fmt.Fprintln(o.Out, "Method: password (Keycloak realm user)")
	if pw.SecretRef == "" {
		_, _ = fmt.Fprintln(o.Out, "  (no secretRef set)")
		return
	}
	printSecret(ctx, o, c, ns, pw.SecretRef, []string{defaultStr(pw.PasswordKey, "password")}, reveal)
}

// printSecret resolves a Secret by name and prints the requested keys,
// redacting values by default. A missing Secret is reported inline and is
// never fatal. No Secret is ever created or modified.
func printSecret(ctx context.Context, o *cli.Options, c *k8s.Client, ns, name string, keys []string, reveal bool) {
	var sec corev1.Secret
	err := c.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, &sec)
	if apierrors.IsNotFound(err) {
		_, _ = fmt.Fprintf(o.Out, "  Secret %s/%s: not found\n", ns, name)
		return
	}
	if err != nil {
		_, _ = fmt.Fprintf(o.Out, "  Secret %s/%s: error: %v\n", ns, name, err)
		return
	}
	_, _ = fmt.Fprintf(o.Out, "  Secret: %s (namespace %s)\n", name, ns)
	present := make([]string, 0, len(sec.Data))
	for k := range sec.Data {
		present = append(present, k)
	}
	sort.Strings(present)
	for _, k := range keys {
		val := "***REDACTED***"
		if reveal {
			if b, ok := sec.Data[k]; ok {
				val = string(b)
			} else {
				val = "(key absent)"
			}
		} else if _, ok := sec.Data[k]; !ok {
			val = "(key absent)"
		}
		_, _ = fmt.Fprintf(o.Out, "    %s = %s\n", k, val)
	}
	_, _ = fmt.Fprintf(o.Out, "    keys present: %s\n", strings.Join(present, ", "))
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

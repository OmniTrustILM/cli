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

package infra

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spf13/cobra"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
)

type uninstallFlags struct {
	namespace string
	keepCRDs  bool
	yes       bool
}

// NewUninstallCommand builds the `uninstall` command.
func NewUninstallCommand(o *cli.Options) *cobra.Command {
	f := &uninstallFlags{}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Remove the ILM operator",
		GroupID: string(cli.GroupInfrastructure),
		Long: "Remove the ILM operator. CRDs are KEPT by default because deleting them cascades and " +
			"deletes every Platform/Connector/Proxy custom resource in the cluster.",
		RunE: func(cmd *cobra.Command, _ []string) error { return runUninstall(cmd, o, f) },
	}
	fs := cmd.Flags()
	fs.StringVarP(&f.namespace, "namespace", "n", "ilm-operator-system", "operator namespace")
	fs.BoolVar(&f.keepCRDs, "keep-crds", true, "keep CRDs (deleting them cascades all custom resources)")
	fs.BoolVarP(&f.yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func runUninstall(cmd *cobra.Command, o *cli.Options, f *uninstallFlags) error {
	if !f.yes {
		return fmt.Errorf("uninstall is destructive; re-run with -y/--yes to confirm")
	}
	c, err := clientFor(o)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	deleted, err := deleteWorkloads(ctx, c, f.namespace)
	if err != nil {
		return err
	}

	crdDeleted, err := deleteCRDs(ctx, c, o, f.keepCRDs)
	if err != nil {
		return err
	}
	deleted += crdDeleted

	_, _ = fmt.Fprintf(o.Out, "Removed %d operator object(s).\n", deleted)
	return nil
}

// deleteWorkloads deletes the operator Deployment and Service, returning the count deleted.
func deleteWorkloads(ctx context.Context, c *k8s.Client, ns string) (int, error) {
	deleted := 0
	for _, obj := range operatorWorkloads(ns) {
		ok, err := deleteIgnoreNotFound(ctx, c, obj)
		if err != nil {
			return deleted, err
		}
		if ok {
			deleted++
		}
	}
	return deleted, nil
}

// deleteCRDs deletes the ILM CRDs when keepCRDs is false, returning the count deleted.
func deleteCRDs(ctx context.Context, c *k8s.Client, o *cli.Options, keepCRDs bool) (int, error) {
	if keepCRDs {
		_, _ = fmt.Fprintln(o.Out, "Kept CRDs (custom resources retained). Re-run with --keep-crds=false to remove them.")
		return 0, nil
	}
	deleted := 0
	for _, name := range ilmCRDNames() {
		crd := &apiextv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: name}}
		ok, err := deleteIgnoreNotFound(ctx, c, crd)
		if err != nil {
			return deleted, err
		}
		if ok {
			deleted++
		}
	}
	_, _ = fmt.Fprintln(o.Out, "Deleted CRDs (all Platform/Connector/Proxy resources were cascaded).")
	return deleted, nil
}

func operatorWorkloads(ns string) []ctrlclient.Object {
	return []ctrlclient.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "ilm-operator-controller-manager", Namespace: ns}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "ilm-operator-controller-manager-metrics-service", Namespace: ns}},
	}
}

func ilmCRDNames() []string {
	return []string{"platforms.otilm.com", "connectors.otilm.com", "proxies.otilm.com"}
}

// deleteIgnoreNotFound deletes obj and returns (true, nil) when the object
// existed and was removed, (false, nil) when it was already absent, or
// (false, err) for any other failure. The error message includes the namespace
// when the object is namespaced.
func deleteIgnoreNotFound(ctx context.Context, c *k8s.Client, obj ctrlclient.Object) (bool, error) {
	if err := c.Typed.Delete(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		name := obj.GetName()
		if ns := obj.GetNamespace(); ns != "" {
			name = ns + "/" + name
		}
		return false, fmt.Errorf("delete %s: %w", name, err)
	}
	return true, nil
}

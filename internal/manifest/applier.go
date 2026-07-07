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

package manifest

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

// DryRunMode mirrors kubectl --dry-run=none|client|server.
type DryRunMode int

const (
	// DryRunNone performs the apply against the cluster.
	DryRunNone DryRunMode = iota
	// DryRunClient never contacts the server; objects are recorded as applied with a (dry-run) suffix.
	DryRunClient
	// DryRunServer validates server-side without persisting.
	DryRunServer
)

// ApplyResult records the outcome of an apply pass, keyed by object id (Kind/namespace/name).
type ApplyResult struct {
	Applied   []string
	Unchanged []string
	Conflicts []string
}

func (r *ApplyResult) merge(o ApplyResult) {
	r.Applied = append(r.Applied, o.Applied...)
	r.Unchanged = append(r.Unchanged, o.Unchanged...)
	r.Conflicts = append(r.Conflicts, o.Conflicts...)
}

// Applier performs ordered server-side apply with FieldManager as the field owner.
type Applier struct {
	Client         *k8s.Client
	FieldManager   string
	ForceConflicts bool
	DryRun         DryRunMode
}

// crdGVR is the GVR the dynamic client uses to poll CRD status.
var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

// objID returns the canonical object id string Kind/namespace/name.
func objID(o *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", o.GetKind(), o.GetNamespace(), o.GetName())
}

// Apply server-side applies each object using the configured field manager.
// On a conflict without ForceConflicts, the object id is appended to
// ApplyResult.Conflicts and a non-nil error is returned.
func (a *Applier) Apply(ctx context.Context, objs []*unstructured.Unstructured) (ApplyResult, error) {
	var res ApplyResult
	mgr := a.FieldManager
	if mgr == "" {
		mgr = "ilmctl"
	}
	for _, o := range objs {
		if a.DryRun == DryRunClient {
			res.Applied = append(res.Applied, objID(o)+" (dry-run)")
			continue
		}
		opts := []ctrlclient.ApplyOption{ctrlclient.FieldOwner(mgr)}
		if a.ForceConflicts {
			opts = append(opts, ctrlclient.ForceOwnership)
		}
		if a.DryRun == DryRunServer {
			opts = append(opts, ctrlclient.DryRunAll)
		}
		ac := ctrlclient.ApplyConfigurationFromUnstructured(o.DeepCopy())
		err := a.Client.Typed.Apply(ctx, ac, opts...)
		switch {
		case err == nil:
			id := objID(o)
			if a.DryRun == DryRunServer {
				id += " (dry-run)"
			}
			res.Applied = append(res.Applied, id)
		case apierrors.IsConflict(err):
			res.Conflicts = append(res.Conflicts, objID(o))
			return res, fmt.Errorf(
				"server-side apply conflict on %s: another field manager owns fields; re-run with --force-conflicts or use the GitOps path: %w",
				objID(o), err)
		default:
			return res, fmt.Errorf("apply %s: %w", objID(o), err)
		}
	}
	return res, nil
}

// ApplyOrdered applies CRDs first, waits for each to become Established (skipped
// in dry-run modes), then applies the controller/RBAC/namespace objects.
// Results from both passes are merged into a single ApplyResult.
func (a *Applier) ApplyOrdered(ctx context.Context, crds, controller []*unstructured.Unstructured) (ApplyResult, error) {
	var res ApplyResult

	crdRes, err := a.Apply(ctx, crds)
	res.merge(crdRes)
	if err != nil {
		return res, err
	}

	if a.DryRun == DryRunNone {
		if werr := a.WaitCRDsEstablished(ctx, crds, 5*time.Minute); werr != nil {
			return res, werr
		}
	}

	ctrlRes, err := a.Apply(ctx, controller)
	res.merge(ctrlRes)
	return res, err
}

// WaitCRDsEstablished blocks until every CustomResourceDefinition in crds
// reports status.conditions[type=Established]==True, or the timeout elapses.
// Objects that are not CRDs are silently skipped.
func (a *Applier) WaitCRDsEstablished(ctx context.Context, crds []*unstructured.Unstructured, timeout time.Duration) error {
	for _, c := range crds {
		if c.GetKind() != "CustomResourceDefinition" {
			continue
		}
		name := c.GetName()
		err := wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, timeout, true,
			func(ctx context.Context) (bool, error) {
				got, gerr := a.Client.Dynamic.Resource(crdGVR).Get(ctx, name, metav1.GetOptions{})
				if gerr != nil {
					if apierrors.IsNotFound(gerr) {
						return false, nil
					}
					return false, gerr
				}
				return crdEstablished(got), nil
			})
		if err != nil {
			return fmt.Errorf("CRD %s did not become Established: %w", name, err)
		}
	}
	return nil
}

// crdEstablished reports whether the CRD's status.conditions contains
// type=Established with status=True.
func crdEstablished(u *unstructured.Unstructured) bool {
	conds, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == "Established" && strings.EqualFold(fmt.Sprint(m["status"]), "true") {
			return true
		}
	}
	return false
}

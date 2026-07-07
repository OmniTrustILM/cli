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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

const (
	olmGroup            = "operators.coreos.com"
	olmCatalogName      = "ilm-operator-catalog"
	olmGroupName        = "ilm-operator-group"
	olmSubscriptionName = "ilm-operator"
	olmPackageName      = "ilm-operator"
)

// OLMOptions configures the OLM install method.
type OLMOptions struct {
	Namespace    string
	Channel      string
	CatalogImage string
	DryRun       DryRunMode
}

// DetectOLM reports whether Operator Lifecycle Manager is installed on the
// cluster by probing for the operators.coreos.com API group. It never installs
// OLM; callers must fail fast when this returns false.
func DetectOLM(_ context.Context, c *k8s.Client) (bool, error) {
	groups, err := c.Discovery.ServerGroups()
	if err != nil {
		return false, fmt.Errorf("discover server groups: %w", err)
	}
	for _, g := range groups.Groups {
		if g.Name == olmGroup {
			return true, nil
		}
	}
	return false, nil
}

// ApplyOLM creates the three OLM objects that instruct OLM to install the
// operator: a CatalogSource, an OperatorGroup, and a Subscription. All three
// are built as unstructured objects to avoid a compile dependency on the OLM
// Go module.
func ApplyOLM(ctx context.Context, c *k8s.Client, o OLMOptions) (ApplyResult, error) {
	if o.CatalogImage == "" {
		return ApplyResult{}, fmt.Errorf("OLM install requires a catalog image; none is published yet, so pass --catalog-image")
	}
	objs := []*unstructured.Unstructured{
		buildCatalogSource(o.Namespace, o.CatalogImage),
		buildOperatorGroup(o.Namespace),
		buildSubscription(o.Namespace, o.Channel),
	}
	a := &Applier{Client: c, FieldManager: "ilmctl", DryRun: o.DryRun}
	return a.Apply(ctx, objs)
}

func buildCatalogSource(ns, image string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": olmGroup + "/v1alpha1",
		"kind":       "CatalogSource",
		"metadata":   map[string]any{"name": olmCatalogName, "namespace": ns},
		"spec": map[string]any{
			"sourceType":  "grpc",
			"image":       image,
			"displayName": "OmniTrust ILM Operators",
			"publisher":   "OmniTrustILM",
		},
	}}
}

func buildOperatorGroup(ns string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": olmGroup + "/v1",
		"kind":       "OperatorGroup",
		"metadata":   map[string]any{"name": olmGroupName, "namespace": ns},
		"spec":       map[string]any{"targetNamespaces": []any{ns}},
	}}
}

func buildSubscription(ns, channel string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": olmGroup + "/v1alpha1",
		"kind":       "Subscription",
		"metadata":   map[string]any{"name": olmSubscriptionName, "namespace": ns},
		"spec": map[string]any{
			"channel":             channel,
			"name":                olmPackageName,
			"source":              olmCatalogName,
			"sourceNamespace":     ns,
			"installPlanApproval": "Automatic",
		},
	}}
}

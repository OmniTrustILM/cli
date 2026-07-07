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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

// namespaceObj builds an unstructured Namespace object for the given name.
func namespaceObj(ns string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": ns},
	}}
}

// splitByCRD partitions a flat object list into CRDs and everything else.
func splitByCRD(objs []*unstructured.Unstructured) (crds, rest []*unstructured.Unstructured) {
	for _, o := range objs {
		if o.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, o)
		} else {
			rest = append(rest, o)
		}
	}
	return crds, rest
}

// deploymentsOf returns the Deployment objects among objs. It is used to feed
// the readiness waiter the Deployments that were actually applied, without
// hardcoding a controller name.
func deploymentsOf(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	var deps []*unstructured.Unstructured
	for _, o := range objs {
		if o.GetKind() == "Deployment" {
			deps = append(deps, o)
		}
	}
	return deps
}

// printApplyResult renders an ApplyResult summary table via the Printer.
func printApplyResult(o *cli.Options, res manifest.ApplyResult) {
	t := render.Table{Columns: []string{"ACTION", "OBJECT"}}
	for _, id := range res.Applied {
		t.Rows = append(t.Rows, []string{"applied", id})
	}
	for _, id := range res.Unchanged {
		t.Rows = append(t.Rows, []string{"unchanged", id})
	}
	for _, id := range res.Conflicts {
		t.Rows = append(t.Rows, []string{"conflict", id})
	}
	if err := o.Printer.PrintTable(t); err != nil {
		_, _ = fmt.Fprintln(o.ErrOut, "render:", err)
	}
}

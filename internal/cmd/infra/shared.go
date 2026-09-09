/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

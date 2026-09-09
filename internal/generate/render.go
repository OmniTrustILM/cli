/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package generate

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// Render emits a CR as YAML, prefixed with an effective-value comment block
// when notes are supplied. The comment block always precedes the document body
// so the rendered output is valid YAML that can be piped directly to kubectl.
func Render(obj runtime.Object, notes []EffectiveNote) (string, error) {
	body, err := marshalObject(obj)
	if err != nil {
		return "", err
	}
	if len(notes) == 0 {
		return body, nil
	}
	var b strings.Builder
	b.WriteString("# Effective values (explicit flags override profile defaults):\n")
	for _, n := range notes {
		if n.Source == sourcePlaceholder {
			fmt.Fprintf(&b, "# %s = %s (%s) # TODO: replace this placeholder\n", n.Field, n.Value, n.Source)
		} else {
			fmt.Fprintf(&b, "# %s = %s (%s)\n", n.Field, n.Value, n.Source)
		}
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String(), nil
}

// marshalObject serialises a runtime.Object to YAML, stripping the
// creationTimestamp: null noise sigs.k8s.io/yaml emits for empty ObjectMeta
// and the empty status block the marshaller appends for typed CRs.
func marshalObject(obj runtime.Object) (string, error) {
	raw, err := yaml.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("marshal object: %w", err)
	}
	out := strings.ReplaceAll(string(raw), "  creationTimestamp: null\n", "")
	out = strings.ReplaceAll(out, "status: {}\n", "")
	return out, nil
}

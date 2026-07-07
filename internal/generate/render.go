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

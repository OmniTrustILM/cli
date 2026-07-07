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

	opconvert "github.com/OmniTrustILM/operator/pkg/convert"
	"sigs.k8s.io/yaml"
)

// Migrate converts a Helm values.yaml document into a Platform scaffold YAML by
// delegating to the operator's pkg/convert. The returned string is the full
// scaffold (header TODOs, the CR body, and footer notes) the operator emits.
func Migrate(valuesYAML []byte, name, namespace string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("platform name is required")
	}
	var values map[string]any
	if err := yaml.Unmarshal(valuesYAML, &values); err != nil {
		return "", fmt.Errorf("parse values.yaml: %w", err)
	}
	if values == nil {
		values = map[string]any{}
	}
	res := opconvert.Convert(values, name, namespace)
	out, err := res.Render()
	if err != nil {
		return "", fmt.Errorf("render platform scaffold: %w", err)
	}
	return out, nil
}

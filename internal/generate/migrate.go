/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

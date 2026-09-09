/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package bundle collects, redacts, signs and reads ILM support bundles.
package bundle

import (
	"sigs.k8s.io/yaml"
)

// Placeholder is substituted for every secret value when redaction is on.
const Placeholder = "***REDACTED***"

// Redactor masks sensitive material in collected artifacts. Redaction is
// intentionally conservative: it never alters resource identity, only the
// value payloads of Kubernetes Secrets.
type Redactor struct{}

// NewRedactor returns a ready-to-use Redactor.
func NewRedactor() *Redactor { return &Redactor{} }

// RedactSecret replaces every value in a Secret's data map with Placeholder,
// preserving the key set so an analyst can still see which keys existed.
// A nil input returns a non-nil empty map.
func (r *Redactor) RedactSecret(data map[string][]byte) map[string]string {
	out := make(map[string]string, len(data))
	for k := range data {
		out[k] = Placeholder
	}
	return out
}

// RedactYAML parses a single Kubernetes manifest and, if it is a core/v1
// Secret, replaces its data and stringData payloads with Placeholder. Any
// other document is returned unchanged so the bundle never loses non-secret
// fidelity. On unparseable input the original bytes are returned unchanged.
func (r *Redactor) RedactYAML(in []byte) []byte {
	var obj map[string]any
	if err := yaml.Unmarshal(in, &obj); err != nil || obj == nil {
		return in
	}
	if kind, _ := obj["kind"].(string); kind != "Secret" {
		return in
	}
	redactStringMap(obj, "data")
	redactStringMap(obj, "stringData")
	out, err := yaml.Marshal(obj)
	if err != nil {
		return in
	}
	return out
}

// redactStringMap replaces every value under the named top-level mapping key
// with Placeholder, leaving the key set intact.
func redactStringMap(obj map[string]any, field string) {
	m, ok := obj[field].(map[string]any)
	if !ok {
		return
	}
	for k := range m {
		m[k] = Placeholder
	}
}

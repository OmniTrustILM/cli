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

package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   map[string][]byte
		want map[string]string
	}{
		{
			name: "all values replaced, keys preserved",
			in: map[string][]byte{
				"password":          []byte("hunter2"),
				bundleRedactTLSCert: []byte("-----BEGIN CERTIFICATE-----"),
				"empty":             []byte(""),
			},
			want: map[string]string{
				"password":          Placeholder,
				bundleRedactTLSCert: Placeholder,
				"empty":             Placeholder,
			},
		},
		{
			name: "nil input yields empty map",
			in:   nil,
			want: map[string]string{},
		},
	}
	r := NewRedactor()
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.RedactSecret(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRedactSecret_NoLeakOfOriginalBytes(t *testing.T) {
	t.Parallel()
	r := NewRedactor()
	sensitive := []byte("super-secret-value-that-must-not-appear")
	out := r.RedactSecret(map[string][]byte{"key": sensitive})
	require.Equal(t, 1, len(out))
	assert.Equal(t, Placeholder, out["key"], "value must be placeholder")
	assert.NotContains(t, out["key"], string(sensitive), "original bytes must not appear in output")
}

func TestRedactYAML(t *testing.T) {
	t.Parallel()
	r := NewRedactor()

	t.Run("redacts a v1 Secret stringData and data blocks", func(t *testing.T) {
		t.Parallel()
		in := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: ilm-admin
  namespace: ilm
type: Opaque
data:
  password: aHVudGVyMg==
stringData:
  token: plaintext-token
`)
		out := r.RedactYAML(in)
		s := string(out)
		assert.NotContains(t, s, "aHVudGVyMg==")
		assert.NotContains(t, s, "plaintext-token")
		assert.Contains(t, s, Placeholder)
		// Non-secret identity is preserved.
		assert.Contains(t, s, "name: ilm-admin")
		assert.Contains(t, s, "kind: Secret")
	})

	t.Run("leaves a non-Secret document untouched", func(t *testing.T) {
		t.Parallel()
		in := []byte(`apiVersion: otilm.com/v1alpha1
kind: Platform
metadata:
  name: ilm
spec:
  version: 2.18.0
`)
		out := r.RedactYAML(in)
		require.YAMLEq(t, string(in), string(out))
	})

	t.Run("passes through invalid yaml unchanged", func(t *testing.T) {
		t.Parallel()
		in := []byte("::: not yaml :::")
		assert.Equal(t, in, r.RedactYAML(in))
	})

	t.Run("secret with only data block is fully redacted", func(t *testing.T) {
		t.Parallel()
		in := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: tls-secret
data:
  tls.crt: LS0tLS1CRUdJTg==
  tls.key: c2VjcmV0a2V5
`)
		out := r.RedactYAML(in)
		s := string(out)
		assert.NotContains(t, s, "LS0tLS1CRUdJTg==", "data value must not appear")
		assert.NotContains(t, s, "c2VjcmV0a2V5", "data value must not appear")
		assert.Contains(t, s, Placeholder)
		assert.Contains(t, s, bundleRedactTLSCert)
		assert.Contains(t, s, "tls.key")
	})

	t.Run("secret with only stringData block is fully redacted", func(t *testing.T) {
		t.Parallel()
		in := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: app-secret
stringData:
  api-key: very-secret-api-key-value
`)
		out := r.RedactYAML(in)
		s := string(out)
		assert.NotContains(t, s, "very-secret-api-key-value", "stringData value must not appear")
		assert.Contains(t, s, Placeholder)
		assert.Contains(t, s, "api-key")
	})

	t.Run("empty data/stringData maps in a Secret produce no original values", func(t *testing.T) {
		t.Parallel()
		in := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: empty-secret
data: {}
`)
		out := r.RedactYAML(in)
		// Should not error; output is valid YAML with kind preserved.
		s := string(out)
		assert.Contains(t, s, "kind: Secret")
	})
}

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

package render

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newPrinterWithFormat(t *testing.T, format string) (*Printer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)
	if format != "" {
		require.NoError(t, fs.Parse([]string{"-o", format}))
	}
	return p, out
}

func sampleConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "ilm", Name: "demo"},
		Data:       map[string]string{"k": "v"},
	}
}

func TestNewPrinter(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	p := NewPrinter(out, errOut)
	assert.NotNil(t, p)
	assert.Equal(t, out, p.Out)
	assert.Equal(t, errOut, p.ErrOut)
	assert.Equal(t, ColorAuto, p.Color)
}

func TestAddFlags_RegistersFlags(t *testing.T) {
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	p.AddFlags(fs)

	require.NoError(t, fs.Parse([]string{"--color"}))
	assert.Equal(t, ColorAlways, p.resolveColor())

	p2 := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	fs2 := pflag.NewFlagSet("test2", pflag.ContinueOnError)
	p2.AddFlags(fs2)
	require.NoError(t, fs2.Parse([]string{"--no-color"}))
	assert.Equal(t, ColorNever, p2.resolveColor())
}

func TestResolveColor_Auto(t *testing.T) {
	p := NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	assert.Equal(t, ColorAuto, p.resolveColor())
}

func TestColorEnabled_Never(t *testing.T) {
	assert.False(t, ColorEnabled(ColorNever, &bytes.Buffer{}))
}

func TestColorEnabled_Always(t *testing.T) {
	assert.True(t, ColorEnabled(ColorAlways, &bytes.Buffer{}))
}

func TestColorEnabled_Auto_NonTTY(t *testing.T) {
	// bytes.Buffer is not a *os.File — auto mode must return false for non-TTY writers.
	assert.False(t, ColorEnabled(ColorAuto, &bytes.Buffer{}))
}

func TestColorEnabled_Auto_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// Even with a real os.File, NO_COLOR env must take precedence.
	assert.False(t, ColorEnabled(ColorAuto, os.Stdout))
}

func TestPrinter_Format(t *testing.T) {
	p, _ := newPrinterWithFormat(t, "json")
	assert.Equal(t, "json", p.Format())

	pTable, _ := newPrinterWithFormat(t, "")
	assert.Equal(t, "", pTable.Format())
}

func TestPrinter_Structured(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"", false},
		{"json", true},
		{"yaml", true},
		{"name", true},
		{"wide", false},
	}
	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			p, _ := newPrinterWithFormat(t, tt.format)
			assert.Equal(t, tt.want, p.Structured())
		})
	}
}

func TestPrinter_PrintObject_JSON(t *testing.T) {
	p, out := newPrinterWithFormat(t, "json")
	require.NoError(t, p.PrintObject(sampleConfigMap()))
	var got map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	assert.Equal(t, "ConfigMap", got["kind"])
	meta := got["metadata"].(map[string]any)
	assert.Equal(t, "demo", meta["name"])
}

func TestPrinter_PrintObject_YAML(t *testing.T) {
	p, out := newPrinterWithFormat(t, "yaml")
	require.NoError(t, p.PrintObject(sampleConfigMap()))
	assert.Contains(t, out.String(), "kind: ConfigMap")
	assert.Contains(t, out.String(), "name: demo")
}

func TestPrinter_PrintObject_Name(t *testing.T) {
	p, out := newPrinterWithFormat(t, "name")
	require.NoError(t, p.PrintObject(sampleConfigMap()))
	assert.Contains(t, out.String(), "configmap/demo")
}

func TestPrinter_PrintObject_GoTemplate(t *testing.T) {
	out := &bytes.Buffer{}
	p := NewPrinter(out, &bytes.Buffer{})
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	p.AddFlags(fs)
	require.NoError(t, fs.Parse([]string{"-o", "go-template={{.metadata.name}}"}))
	require.NoError(t, p.PrintObject(sampleConfigMap()))
	assert.Equal(t, "demo", out.String())
}

func TestPrinter_PrintObject_TableFormatIsError(t *testing.T) {
	// With no -o (table mode), PrintObject has no structured printer to delegate to.
	p, _ := newPrinterWithFormat(t, "")
	require.Error(t, p.PrintObject(sampleConfigMap()))
}

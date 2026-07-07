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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_RendersPlatformFromValues(t *testing.T) {
	values := []byte(`
database:
  host: db.example.com
  name: ilm
messaging:
  host: mq.example.com
`)
	out, err := Migrate(values, "ilm", "ilm")
	require.NoError(t, err)
	assert.Contains(t, out, "kind: Platform")
	assert.Contains(t, out, "name: ilm")
}

func TestMigrate_EmptyValuesStillRenders(t *testing.T) {
	out, err := Migrate([]byte("{}"), "ilm", "ilm")
	require.NoError(t, err)
	assert.Contains(t, out, "kind: Platform")
}

func TestMigrate_InvalidYAML(t *testing.T) {
	_, err := Migrate([]byte("::: not yaml :::"), "ilm", "ilm")
	assert.Error(t, err)
}

func TestMigrate_EmptyName(t *testing.T) {
	_, err := Migrate([]byte("{}"), "", "ilm")
	assert.Error(t, err)
}

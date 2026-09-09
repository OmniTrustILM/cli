/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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
	out, err := Migrate(values, genILM, genILM)
	require.NoError(t, err)
	assert.Contains(t, out, "kind: Platform")
	assert.Contains(t, out, "name: ilm")
}

func TestMigrate_EmptyValuesStillRenders(t *testing.T) {
	out, err := Migrate([]byte("{}"), genILM, genILM)
	require.NoError(t, err)
	assert.Contains(t, out, "kind: Platform")
}

func TestMigrate_InvalidYAML(t *testing.T) {
	_, err := Migrate([]byte("::: not yaml :::"), genILM, genILM)
	assert.Error(t, err)
}

func TestMigrate_EmptyName(t *testing.T) {
	_, err := Migrate([]byte("{}"), "", genILM)
	assert.Error(t, err)
}

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package platform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrate_FromValuesFile(t *testing.T) {
	dir := t.TempDir()
	vf := filepath.Join(dir, "values.yaml")
	require.NoError(t, os.WriteFile(vf, []byte("database:\n  host: db\n  name: ilm\n"), 0o600))

	o, out, _ := newTestOptions(nil)
	cmd := NewMigrateCommand(o)
	cmd.SetArgs([]string{"--values", vf, platformFlagName, platformName, platformFlagNS, platformName})
	cmd.SetOut(out)
	cmd.SetErr(out)
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "kind: Platform")
}

func TestMigrate_MissingValuesFile(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewMigrateCommand(o)
	cmd.SetArgs([]string{"--values", "/nonexistent/values.yaml", platformFlagName, platformName})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

func TestMigrate_RequiresValues(t *testing.T) {
	o, out, _ := newTestOptions(nil)
	cmd := NewMigrateCommand(o)
	cmd.SetArgs([]string{platformFlagName, platformName})
	cmd.SetOut(out)
	cmd.SetErr(out)
	assert.Error(t, cmd.Execute())
}

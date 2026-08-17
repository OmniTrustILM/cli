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

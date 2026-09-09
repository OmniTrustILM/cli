/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package infra

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/render"
)

func TestInfraCommandsCarryGroup(t *testing.T) {
	var out, errOut bytes.Buffer
	o := &cli.Options{Out: &out, ErrOut: &errOut, Printer: render.NewPrinter(&out, &errOut)}
	for _, c := range []struct {
		name string
		gid  string
	}{
		{"init", NewInitCommand(o).GroupID},
		{"upgrade", NewUpgradeCommand(o).GroupID},
		{"uninstall", NewUninstallCommand(o).GroupID},
	} {
		assert.Equal(t, string(cli.GroupInfrastructure), c.gid, c.name)
	}
}

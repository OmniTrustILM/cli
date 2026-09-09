/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package shared

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OmniTrustILM/cli/internal/render"
)

func TestOResolved(t *testing.T) {
	p := render.NewPrinter(&bytes.Buffer{}, &bytes.Buffer{})
	assert.False(t, OResolved(p), "no -o means table output")
	*p.FormatPtrForTest() = "json"
	assert.True(t, OResolved(p))
}

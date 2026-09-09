/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package buildinfo

import (
	"testing"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestModuleName(t *testing.T) {
	assert.Equal(t, "ilmctl", BinaryName)
}

func TestOperatorTypesImportable(t *testing.T) {
	// Proves the replace directive resolves and the operator API is import-light.
	assert.Equal(t, "otilm.com", otilmv1alpha1.GroupVersion.Group)
	assert.Equal(t, "v1alpha1", otilmv1alpha1.GroupVersion.Version)
}

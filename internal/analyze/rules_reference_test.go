/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReferenceAnalyzer(t *testing.T) {
	t.Parallel()
	a := referenceAnalyzer{}

	t.Run("missing refs become fail findings", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{MissingRefs: []string{"Secret/ns/ilm-db", "Issuer/ns/edge"}}
		got := a.Analyze(s)
		require.Len(t, got, 2)
		for _, f := range got {
			assert.Equal(t, SeverityFail, f.Severity)
			assert.Equal(t, "reference", f.Rule)
			assert.NotEmpty(t, f.Remediation)
		}
		assert.Equal(t, "Secret/ns/ilm-db", got[0].Resource)
	})

	t.Run("no missing refs (bundle) emits nothing", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, a.Analyze(&Snapshot{}))
	})
}

func TestReferenceAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "reference", referenceAnalyzer{}.Name())
}

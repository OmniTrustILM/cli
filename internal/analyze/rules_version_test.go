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

package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionAnalyzer(t *testing.T) {
	t.Parallel()
	a := versionAnalyzer{}

	t.Run("empty operator version emits nothing", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, a.Analyze(&Snapshot{}))
	})

	t.Run("operator version not in SupportedVersions is a warn (not fail)", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{OperatorVersion: analyzeVer9900, SupportedVersions: []string{analyzeVer2180}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityWarn, got[0].Severity)
		assert.Equal(t, "version", got[0].Rule)
		assert.Contains(t, got[0].Evidence, analyzeVer9900)
		assert.Contains(t, got[0].Evidence, analyzeVer2180)
	})

	t.Run("operator version in SupportedVersions emits nothing", func(t *testing.T) {
		t.Parallel()
		// The decision is driven entirely by the snapshot's SupportedVersions field.
		s := &Snapshot{OperatorVersion: analyzeVer2180, SupportedVersions: []string{analyzeVer2180, "2.17.0"}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("empty SupportedVersions emits nothing (offline or unknown)", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{OperatorVersion: analyzeVer9900, SupportedVersions: nil}
		assert.Empty(t, a.Analyze(s))
	})
}

func TestVersionAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "version", versionAnalyzer{}.Name())
}

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

package bundle

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaVersionStable(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1", SchemaVersion)
}

func TestNewManifest(t *testing.T) {
	t.Parallel()
	opts := CollectOptions{
		Namespaces:    []string{bundleNamespace},
		AllNamespaces: false,
		IncludeLogs:   true,
		Since:         time.Hour,
		Redact:        true,
		Format:        FormatZip,
	}
	m := NewManifest(bundleV010, opts)

	assert.Equal(t, SchemaVersion, m.SchemaVersion)
	assert.Equal(t, bundleV010, m.ClientVersion)
	assert.True(t, m.Redacted, "Redacted mirrors Options.Redact")
	assert.Equal(t, opts, m.Options)
	assert.False(t, m.CreatedAt.IsZero(), "CreatedAt is stamped")
	assert.NotNil(t, m.Skipped)
	assert.NotNil(t, m.Files)
}

func TestManifestRecordHelpers(t *testing.T) {
	t.Parallel()
	m := NewManifest(bundleV010, CollectOptions{Redact: false, Format: FormatTGZ})
	assert.False(t, m.Redacted)

	m.AddFile("versions.json")
	m.AddFile("config/platforms/ilm.yaml")
	m.Skip("secrets/ilm-admin.yaml", "forbidden: cannot list secrets")

	require.Len(t, m.Files, 2)
	require.Len(t, m.Skipped, 1)
	assert.Equal(t, "secrets/ilm-admin.yaml", m.Skipped[0].Path)
	assert.Contains(t, m.Skipped[0].Reason, "forbidden")
}

func TestManifestJSONRoundTrip(t *testing.T) {
	t.Parallel()
	m := NewManifest(bundleV010, CollectOptions{
		Namespaces:  []string{bundleNamespace},
		IncludeLogs: true,
		Redact:      true,
		Format:      FormatZip,
		Since:       2 * time.Hour,
	})
	m.AddFile("manifest.json")
	m.Skip("nodes.json", "forbidden")

	raw, err := json.Marshal(m)
	require.NoError(t, err)

	var got Manifest
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, m.SchemaVersion, got.SchemaVersion)
	assert.Equal(t, m.ClientVersion, got.ClientVersion)
	assert.Equal(t, m.Options.Format, got.Options.Format)
	assert.Equal(t, m.Options.Since, got.Options.Since)
	assert.Equal(t, m.Files, got.Files)
	assert.Equal(t, m.Skipped, got.Skipped)
	assert.Equal(t, m.Redacted, got.Redacted)
}

func TestManifestJSONFieldNames(t *testing.T) {
	t.Parallel()
	m := NewManifest(bundleV010, CollectOptions{Redact: true, Format: FormatZip})
	raw, err := json.Marshal(m)
	require.NoError(t, err)

	var obj map[string]any
	require.NoError(t, json.Unmarshal(raw, &obj))

	// Verify exact JSON field names per contract.
	assert.Contains(t, obj, "schemaVersion")
	assert.Contains(t, obj, "createdAt")
	assert.Contains(t, obj, "clientVersion")
	assert.Contains(t, obj, "options")
	assert.Contains(t, obj, "skipped")
	assert.Contains(t, obj, "files")
	assert.Contains(t, obj, "redacted")
	assert.Equal(t, "1", obj["schemaVersion"])
}

func TestManifestWithSkippedItems(t *testing.T) {
	t.Parallel()
	m := NewManifest(bundleV010, CollectOptions{Format: FormatZip, Redact: true})
	m.Skip("secrets/a.yaml", "forbidden: 403")
	m.Skip("configmaps/b.yaml", "timeout")

	require.Len(t, m.Skipped, 2)
	assert.Equal(t, "secrets/a.yaml", m.Skipped[0].Path)
	assert.Equal(t, "forbidden: 403", m.Skipped[0].Reason)
	assert.Equal(t, "configmaps/b.yaml", m.Skipped[1].Path)
	assert.Equal(t, "timeout", m.Skipped[1].Reason)
}

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package bundle

import "time"

// SchemaVersion is the on-disk schema version of manifest.json. Bump only on
// breaking changes; the reader gates on it.
const SchemaVersion = "1"

// Format is the bundle archive format.
type Format string

const (
	// FormatZip is the default archive format.
	FormatZip Format = "zip"
	// FormatTGZ is the gzip-compressed tar alternative.
	FormatTGZ Format = "tgz"
)

// ManifestName is the well-known entry that indexes a bundle.
const ManifestName = "manifest.json"

// CollectOptions configures a single Collect run.
type CollectOptions struct {
	// Namespaces scopes collection; empty together with AllNamespaces means
	// cluster scope.
	Namespaces    []string      `json:"namespaces,omitempty"`
	AllNamespaces bool          `json:"allNamespaces"`
	IncludeLogs   bool          `json:"includeLogs"`
	Since         time.Duration `json:"since,omitempty"`
	// Redact masks Secret payloads; defaults to true at the command layer.
	Redact bool   `json:"redact"`
	Format Format `json:"format"`
	Sign   bool   `json:"sign"`
}

// SkippedItem records an artifact that could not be collected (typically an
// RBAC denial), so a bundle is never silently partial.
type SkippedItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Manifest is the versioned index written as manifest.json at the bundle root.
type Manifest struct {
	SchemaVersion string         `json:"schemaVersion"`
	CreatedAt     time.Time      `json:"createdAt"`
	ClientVersion string         `json:"clientVersion"`
	Options       CollectOptions `json:"options"`
	Skipped       []SkippedItem  `json:"skipped"`
	Files         []string       `json:"files"`
	Redacted      bool           `json:"redacted"`
}

// NewManifest stamps a fresh manifest for a collection run.
func NewManifest(clientVersion string, opts CollectOptions) *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		CreatedAt:     time.Now().UTC(),
		ClientVersion: clientVersion,
		Options:       opts,
		Skipped:       []SkippedItem{},
		Files:         []string{},
		Redacted:      opts.Redact,
	}
}

// AddFile records a successfully written bundle entry (relative path).
func (m *Manifest) AddFile(path string) { m.Files = append(m.Files, path) }

// Skip records an artifact that was not collected and why.
func (m *Manifest) Skip(path, reason string) {
	m.Skipped = append(m.Skipped, SkippedItem{Path: path, Reason: reason})
}

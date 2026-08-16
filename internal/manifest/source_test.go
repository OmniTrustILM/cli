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

package manifest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolve asserts the resolved refs, including the public release URLs. It
// is deliberately not parallel: it reads the package-level release base that
// the fetch tests below repoint at an httptest server.
func TestResolve(t *testing.T) {
	tests := []struct {
		name           string
		src            Source
		wantErrMsg     string
		wantKind       SourceKind
		wantCRDs       string
		wantController string
		wantChecksums  string
		wantVersion    string
	}{
		{
			name:           "manifest highest precedence",
			src:            Source{Manifest: manifestTmpYAML, FromSource: "/op", Ref: "abc", Version: "v1"},
			wantKind:       SourceManifest,
			wantCRDs:       manifestTmpYAML,
			wantController: manifestTmpYAML,
		},
		{
			name:           "from-source beats ref and version",
			src:            Source{FromSource: "/op", Ref: "abc", Version: "v1"},
			wantKind:       SourceFromSource,
			wantCRDs:       "/op/deploy/manifests/ilm-operator.crds.yaml",
			wantController: "/op/deploy/manifests/ilm-operator.yaml",
		},
		{
			name:           "ref beats version",
			src:            Source{Ref: "1a2b3c", Version: "v1"},
			wantKind:       SourceRef,
			wantCRDs:       "https://raw.githubusercontent.com/OmniTrustILM/operator/1a2b3c/deploy/manifests/ilm-operator.crds.yaml",
			wantController: "https://raw.githubusercontent.com/OmniTrustILM/operator/1a2b3c/deploy/manifests/ilm-operator.yaml",
		},
		{
			name:           "version resolves the release assets",
			src:            Source{Version: manifestReleaseTag},
			wantKind:       SourceVersion,
			wantCRDs:       manifestReleasesHost + "/download/v1.0.0/ilm-operator.crds.yaml",
			wantController: manifestReleasesHost + "/download/v1.0.0/ilm-operator.yaml",
			wantChecksums:  manifestReleasesHost + "/download/v1.0.0/checksums.txt",
			wantVersion:    manifestReleaseTag,
		},
		{
			name:           "empty source resolves the latest release",
			src:            Source{},
			wantKind:       SourceVersion,
			wantCRDs:       manifestReleasesHost + "/latest/download/ilm-operator.crds.yaml",
			wantController: manifestReleasesHost + "/latest/download/ilm-operator.yaml",
			wantChecksums:  manifestReleasesHost + "/latest/download/checksums.txt",
		},
		{
			name:       "a version that is not a tag is rejected",
			src:        Source{Version: "../../etc/passwd"},
			wantErrMsg: "invalid --version",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.src)
			if tc.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantKind, got.Kind)
			assert.Equal(t, tc.wantCRDs, got.CRDsRef)
			assert.Equal(t, tc.wantController, got.ControllerRef)
			assert.Equal(t, tc.wantChecksums, got.ChecksumsRef)
			assert.Equal(t, tc.wantVersion, got.Version)
		})
	}
}

// fakeRelease serves a GitHub-shaped release host: assets live under
// /download/<tag>/<asset>, and /latest/download/<asset> redirects there — the
// same contract the real releases host offers. It records every served path so
// tests can prove which release the assets were read from.
type fakeRelease struct {
	tag    string
	assets map[string][]byte

	mu    sync.Mutex
	paths []string
}

// newFakeRelease builds a release serving the CRDs and controller documents
// plus a matching sha256sum-format checksums.txt.
func newFakeRelease(tag string) *fakeRelease {
	assets := map[string][]byte{
		manifestCRDsAsset:       []byte(manifestCRDsBody),
		manifestControllerAsset: []byte(manifestControllerBody),
	}
	assets[manifestChecksumsAsset] = checksumsDoc(assets, manifestCRDsAsset, manifestControllerAsset)
	return &fakeRelease{tag: tag, assets: assets}
}

// checksumsDoc renders `sha256sum` output for the named assets.
func checksumsDoc(assets map[string][]byte, names ...string) []byte {
	var b strings.Builder
	for _, n := range names {
		sum := sha256.Sum256(assets[n])
		_, _ = fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(sum[:]), n)
	}
	return []byte(b.String())
}

// start serves the release from an httptest server, points the release paths at
// it for the duration of the test, and returns the server.
func (f *fakeRelease) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	restore := SetReleaseBaseForTest(srv.URL)
	t.Cleanup(restore)
	return srv
}

func (f *fakeRelease) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()

	if name, ok := strings.CutPrefix(r.URL.Path, "/latest/download/"); ok {
		http.Redirect(w, r, "/download/"+f.tag+"/"+name, http.StatusFound)
		return
	}
	name, ok := strings.CutPrefix(r.URL.Path, "/download/"+f.tag+"/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, ok := f.assets[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write(body)
}

// served returns the paths the release host answered, in order.
func (f *fakeRelease) served() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

// resolveFetch resolves src and fetches it in one step, as the callers do.
func resolveFetch(t *testing.T, src Source) (Fetched, error) {
	t.Helper()
	res, err := Resolve(src)
	require.NoError(t, err)
	return FetchAll(context.Background(), res)
}

// TestFetchAll_Version verifies that --version reads both documents from the
// pinned release and returns the tag it applied.
func TestFetchAll_Version(t *testing.T) {
	rel := newFakeRelease(manifestReleaseTag)
	rel.start(t)

	got, err := resolveFetch(t, Source{Version: manifestReleaseTag})
	require.NoError(t, err)
	assert.Equal(t, manifestCRDsBody, string(got.CRDs))
	assert.Equal(t, manifestControllerBody, string(got.Controller))
	assert.Equal(t, manifestReleaseTag, got.Version)
	// checksums.txt must have been read from the same release.
	assert.Contains(t, rel.served(), "/download/"+manifestReleaseTag+"/"+manifestChecksumsAsset)
}

// TestFetchAll_LatestPinsToResolvedTag verifies the default path: the latest
// release is resolved through the redirect (no API call), and the remaining
// assets are read from the tag the redirect landed on, so one run never mixes
// documents from two releases.
func TestFetchAll_LatestPinsToResolvedTag(t *testing.T) {
	rel := newFakeRelease(manifestLatestTag)
	rel.start(t)

	got, err := resolveFetch(t, Source{})
	require.NoError(t, err)
	assert.Equal(t, manifestLatestTag, got.Version)
	assert.Equal(t, manifestCRDsBody, string(got.CRDs))

	served := rel.served()
	assert.Contains(t, served, "/latest/download/"+manifestCRDsAsset, "the latest redirect is the entry point")
	for _, want := range []string{manifestControllerAsset, manifestChecksumsAsset} {
		assert.Contains(t, served, "/download/"+manifestLatestTag+"/"+want,
			"%s must be read from the tag the redirect resolved to", want)
		assert.NotContains(t, served, "/latest/download/"+want,
			"%s must not be re-resolved through the latest redirect", want)
	}
}

// TestFetchAll_ChecksumMismatch verifies that a manifest whose digest does not
// match checksums.txt is refused, with an error naming the asset.
func TestFetchAll_ChecksumMismatch(t *testing.T) {
	rel := newFakeRelease(manifestReleaseTag)
	rel.assets[manifestControllerAsset] = []byte("kind: Deployment # tampered\n")
	rel.start(t)

	_, err := resolveFetch(t, Source{Version: manifestReleaseTag})
	require.Error(t, err)
	assert.Contains(t, err.Error(), manifestControllerAsset)
	assert.Contains(t, err.Error(), "checksum")
}

// TestFetchAll_MissingChecksumEntry verifies that an asset with no entry in
// checksums.txt is refused rather than applied unverified.
func TestFetchAll_MissingChecksumEntry(t *testing.T) {
	rel := newFakeRelease(manifestReleaseTag)
	rel.assets[manifestChecksumsAsset] = checksumsDoc(rel.assets, manifestCRDsAsset)
	rel.start(t)

	_, err := resolveFetch(t, Source{Version: manifestReleaseTag})
	require.Error(t, err)
	assert.Contains(t, err.Error(), manifestControllerAsset)
	assert.Contains(t, err.Error(), "no entry")
}

// TestFetchAll_DuplicateChecksumEntry verifies that a checksums.txt listing an
// asset more than once is refused as malformed — whether the two digests agree
// or contradict each other. An ambiguous document must never get to decide
// which bytes are the published ones.
func TestFetchAll_DuplicateChecksumEntry(t *testing.T) {
	tests := []struct {
		name string
		// duplicate renders the second entry for the controller asset.
		duplicate func(assets map[string][]byte) []byte
	}{
		{
			name: "agreeing digests",
			// The honest entry, repeated verbatim: both digests are correct.
			duplicate: func(a map[string][]byte) []byte { return checksumsDoc(a, manifestControllerAsset) },
		},
		{
			name: "contradicting digests",
			duplicate: func(map[string][]byte) []byte {
				return []byte(strings.Repeat("f", 64) + "  " + manifestControllerAsset + "\n")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rel := newFakeRelease(manifestReleaseTag)
			honest := checksumsDoc(rel.assets, manifestCRDsAsset, manifestControllerAsset)
			rel.assets[manifestChecksumsAsset] = append(honest, tc.duplicate(rel.assets)...)
			rel.start(t)

			_, err := resolveFetch(t, Source{Version: manifestReleaseTag})
			require.Error(t, err)
			assert.Contains(t, err.Error(), manifestControllerAsset)
			assert.Contains(t, err.Error(), "more than once")
		})
	}
}

// TestFetchAll_MissingChecksumsAsset verifies that a release without a
// checksums.txt is refused: nothing is applied unverified.
func TestFetchAll_MissingChecksumsAsset(t *testing.T) {
	rel := newFakeRelease(manifestReleaseTag)
	delete(rel.assets, manifestChecksumsAsset)
	rel.start(t)

	_, err := resolveFetch(t, Source{Version: manifestReleaseTag})
	require.Error(t, err)
	assert.Contains(t, err.Error(), manifestChecksumsAsset)
	assert.Contains(t, err.Error(), manifestReleaseTag)
}

// TestFetchAll_UnknownVersion verifies that a 404 on a pinned release names the
// version, the asset URL, and where to look for the published releases.
func TestFetchAll_UnknownVersion(t *testing.T) {
	rel := newFakeRelease(manifestReleaseTag)
	srv := rel.start(t)

	_, err := resolveFetch(t, Source{Version: "v9.9.9"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v9.9.9")
	assert.Contains(t, err.Error(), srv.URL+"/download/v9.9.9/"+manifestCRDsAsset)
	assert.NotContains(t, err.Error(), "checksum", "the release itself is missing, not its checksums")
}

// TestFetchAll_NoReleasePublished verifies that a 404 on the latest redirect
// still steers the user at the developer sources.
func TestFetchAll_NoReleasePublished(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	defer SetReleaseBaseForTest(srv.URL)()

	_, err := resolveFetch(t, Source{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no published operator release")
	assert.Contains(t, err.Error(), "--version")
	assert.Contains(t, err.Error(), "--ref")
	assert.Contains(t, err.Error(), "--from-source")
}

// TestFetchAll_LatestWithoutRedirect verifies that a releases host that answers
// the latest path without redirecting to a tag is an actionable error rather
// than an unpinned install.
func TestFetchAll_LatestWithoutRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(manifestCRDsBody))
	}))
	defer srv.Close()
	defer SetReleaseBaseForTest(srv.URL)()

	_, err := resolveFetch(t, Source{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--version")
}

// TestFetchAll_DeveloperSourcesSkipChecksums verifies that --from-source (like
// --manifest and --ref) reads the working tree as-is: no checksums document
// exists for a checkout, and none is demanded.
func TestFetchAll_DeveloperSourcesSkipChecksums(t *testing.T) {
	dir := t.TempDir()
	manifests := filepath.Join(dir, "deploy", "manifests")
	require.NoError(t, os.MkdirAll(manifests, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(manifests, manifestCRDsAsset), []byte(manifestCRDsBody), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(manifests, manifestControllerAsset), []byte(manifestControllerBody), 0o600))

	got, err := resolveFetch(t, Source{FromSource: dir})
	require.NoError(t, err)
	assert.Equal(t, manifestCRDsBody, string(got.CRDs))
	assert.Equal(t, manifestControllerBody, string(got.Controller))
	assert.Empty(t, got.Version, "a checkout has no release tag")
}

// TestFetchAll_DeveloperSourceMissingFile verifies the developer paths still
// surface a plain read error.
func TestFetchAll_DeveloperSourceMissingFile(t *testing.T) {
	_, err := resolveFetch(t, Source{FromSource: t.TempDir()})
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestFetch_File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "m.yaml")
	require.NoError(t, os.WriteFile(p, []byte(manifestKindFoo), 0o600))

	got, err := Fetch(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, manifestKindFoo, string(got))

	gotURL, err := Fetch(context.Background(), "file://"+p)
	require.NoError(t, err)
	assert.Equal(t, manifestKindFoo, string(gotURL))
}

func TestFetch_HTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("kind: Bar\n"))
	}))
	defer srv.Close()

	got, err := Fetch(context.Background(), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "kind: Bar\n", string(got))
}

func TestFetch_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetch_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := Fetch(context.Background(), filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, os.ErrNotExist))
}

func TestSplit(t *testing.T) {
	t.Parallel()
	raw := []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: platforms.otilm.com
---
# a comment-only doc is skipped
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ilm-operator-controller-manager
  namespace: ilm-operator-system
`)
	objs, err := Split(raw)
	require.NoError(t, err)
	require.Len(t, objs, 2)
	assert.Equal(t, "CustomResourceDefinition", objs[0].GetKind())
	assert.Equal(t, "platforms.otilm.com", objs[0].GetName())
	assert.Equal(t, "Deployment", objs[1].GetKind())
	assert.Equal(t, "ilm-operator-system", objs[1].GetNamespace())
}

func TestSplit_Invalid(t *testing.T) {
	t.Parallel()
	_, err := Split([]byte("\t\tnot: [valid"))
	require.Error(t, err)
}

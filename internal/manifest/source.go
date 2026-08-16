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

// Package manifest resolves operator install sources and applies them to a
// cluster via ordered server-side apply.
package manifest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// SourceKind expresses the highest-first resolution order.
type SourceKind int

const (
	// SourceManifest is an explicit --manifest file or URL.
	SourceManifest SourceKind = iota
	// SourceFromSource is a local operator checkout (--from-source).
	SourceFromSource
	// SourceRef is a committed manifest at a git ref (--ref).
	SourceRef
	// SourceVersion is a published release: an explicit --version tag, or the
	// latest release when no source flag is given.
	SourceVersion
)

// Source captures the raw install-source flags.
type Source struct {
	Kind       SourceKind
	Manifest   string // --manifest file|url
	FromSource string // --from-source operator path
	Ref        string // --ref commit|tag|branch
	Version    string // --version release tag
}

// Resolved is the pair of refs (file path or URL) the applier consumes.
type Resolved struct {
	CRDsRef       string
	ControllerRef string
	// ChecksumsRef points at the release's checksums.txt. It is set for the
	// release sources only; the developer sources leave it empty.
	ChecksumsRef string
	Kind         SourceKind
	// Version is the release tag the refs are pinned to. It is empty for the
	// developer sources and for the default path, which learns the tag from the
	// latest-release redirect at fetch time.
	Version string
}

const (
	crdsFile       = "deploy/manifests/ilm-operator.crds.yaml"
	controllerFile = "deploy/manifests/ilm-operator.yaml"
	rawBase        = "https://raw.githubusercontent.com/OmniTrustILM/operator"

	// Release asset names, as published by the operator's release workflow.
	crdsAsset       = "ilm-operator.crds.yaml"
	controllerAsset = "ilm-operator.yaml"
	checksumsAsset  = "checksums.txt"
)

// releaseBase is the operator's GitHub releases root. It is a var so tests can
// serve a release from an httptest server; production code never reassigns it.
var releaseBase = "https://github.com/OmniTrustILM/operator/releases"

// SetReleaseBaseForTest points the release refs at base and returns a restore
// function. It exists so tests here and in dependent packages can exercise the
// release paths hermetically, without network access; production code must not
// call it.
func SetReleaseBaseForTest(base string) (restore func()) {
	prev := releaseBase
	releaseBase = base
	return func() { releaseBase = prev }
}

// tagPattern matches what a release tag may contribute to an asset URL. A value
// carrying a slash, a query or whitespace would silently rewrite the URL, so it
// is rejected before it is ever interpolated.
var tagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]*$`)

// Resolve picks the source per precedence (--manifest > --from-source > --ref >
// --version) and returns the CRDs-doc and controller-doc refs. With no source
// flag at all it resolves the operator's latest release.
func Resolve(s Source) (Resolved, error) {
	switch {
	case s.Manifest != "":
		// A single --manifest file already bundles CRDs + controller.
		return Resolved{CRDsRef: s.Manifest, ControllerRef: s.Manifest, Kind: SourceManifest}, nil
	case s.FromSource != "":
		base := strings.TrimRight(s.FromSource, "/")
		return Resolved{
			CRDsRef:       base + "/" + crdsFile,
			ControllerRef: base + "/" + controllerFile,
			Kind:          SourceFromSource,
		}, nil
	case s.Ref != "":
		return Resolved{
			CRDsRef:       fmt.Sprintf("%s/%s/%s", rawBase, s.Ref, crdsFile),
			ControllerRef: fmt.Sprintf("%s/%s/%s", rawBase, s.Ref, controllerFile),
			Kind:          SourceRef,
		}, nil
	case s.Version != "":
		tag := strings.TrimSpace(s.Version)
		if !tagPattern.MatchString(tag) {
			return Resolved{}, fmt.Errorf("invalid --version %q: expected a release tag such as v1.0.0", s.Version)
		}
		return releaseResolved(tag), nil
	default:
		// No source flag: the latest published release.
		return releaseResolved(""), nil
	}
}

// releaseResolved builds the refs for a published release. An empty tag targets
// the latest release through the stable /releases/latest/download/<asset>
// redirect: no API call, so no token, no rate limit and no JSON to parse.
func releaseResolved(tag string) Resolved {
	dir := releaseBase + "/latest/download"
	if tag != "" {
		dir = releaseBase + "/download/" + tag
	}
	return Resolved{
		CRDsRef:       dir + "/" + crdsAsset,
		ControllerRef: dir + "/" + controllerAsset,
		ChecksumsRef:  dir + "/" + checksumsAsset,
		Kind:          SourceVersion,
		Version:       tag,
	}
}

// Fetched carries the manifest documents of a resolved source, plus the release
// tag they were read from (empty for the developer sources).
type Fetched struct {
	CRDs       []byte
	Controller []byte
	Version    string
}

// FetchAll reads the CRDs and controller documents of a resolved source.
//
// For a release source — an explicit --version, or the latest release the
// default path resolves — both documents are verified against the release's
// checksums.txt before they are returned, so a manifest that does not match the
// published release never reaches the cluster. The default path additionally
// pins to the tag the latest-release redirect landed on and reads the remaining
// assets from that tag, so one run can never mix documents from two releases.
//
// The --manifest, --from-source and --ref sources are deliberately
// checksum-free: they are developer paths pointing at working trees and
// arbitrary git refs, for which no published checksums document exists.
func FetchAll(ctx context.Context, r Resolved) (Fetched, error) {
	if r.ChecksumsRef == "" {
		crds, err := Fetch(ctx, r.CRDsRef)
		if err != nil {
			return Fetched{}, err
		}
		controller, err := Fetch(ctx, r.ControllerRef)
		if err != nil {
			return Fetched{}, err
		}
		return Fetched{CRDs: crds, Controller: controller}, nil
	}
	return fetchRelease(ctx, r)
}

// fetchRelease downloads and verifies one release's manifests.
func fetchRelease(ctx context.Context, r Resolved) (Fetched, error) {
	// The CRDs document goes first: on the default path its redirect reveals the
	// tag `latest` resolves to, and the rest of the release is then read from
	// exactly that tag.
	crds, hops, err := fetchHTTPHops(ctx, r.CRDsRef)
	if err != nil {
		return Fetched{}, releaseFetchError(err, r, crdsAsset)
	}
	if r.Version == "" {
		tag := releaseTagFromHops(hops)
		if tag == "" {
			return Fetched{}, fmt.Errorf(
				"cannot tell which release %s resolved to; pin it with --version vX.Y.Z", r.CRDsRef)
		}
		r = releaseResolved(tag)
	}

	controller, err := Fetch(ctx, r.ControllerRef)
	if err != nil {
		return Fetched{}, releaseFetchError(err, r, controllerAsset)
	}
	sums, err := Fetch(ctx, r.ChecksumsRef)
	if err != nil {
		return Fetched{}, releaseFetchError(err, r, checksumsAsset)
	}

	for _, doc := range []struct {
		asset string
		data  []byte
	}{{crdsAsset, crds}, {controllerAsset, controller}} {
		if err := verifyChecksum(sums, doc.asset, doc.data); err != nil {
			return Fetched{}, fmt.Errorf("operator release %s: %w", r.Version, err)
		}
	}
	return Fetched{CRDs: crds, Controller: controller, Version: r.Version}, nil
}

// releaseFetchError turns a failed asset download into guidance. A 404 is the
// interesting case: either the pinned release does not publish that asset, or —
// on the default path — the operator has no published release at all.
func releaseFetchError(err error, r Resolved, asset string) error {
	if !errors.Is(err, errNotFound) {
		return err
	}
	if r.Version == "" {
		return fmt.Errorf(
			"no published operator release at %s: pick a release with --version vX.Y.Z, "+
				"or install from a commit with --ref <sha> or a local checkout with --from-source <path>",
			r.CRDsRef)
	}
	return fmt.Errorf(
		"operator release %s does not publish %s (looked at %s/download/%s/%s): "+
			"check the published releases at %s",
		r.Version, asset, releaseBase, r.Version, asset, releaseBase)
}

// releaseTagFromHops returns the release tag from the first hop shaped like
// <base>/download/<tag>/<asset>. GitHub answers /releases/latest/download/<a>
// with a redirect to /releases/download/<tag>/<a> before handing off to its
// asset CDN, so the tag is read from the hop, not from the final URL.
func releaseTagFromHops(hops []string) string {
	for _, h := range hops {
		const marker = "/download/"
		i := strings.LastIndex(h, marker)
		if i < 0 {
			continue
		}
		tag, _, ok := strings.Cut(h[i+len(marker):], "/")
		if ok && tag != "" {
			return tag
		}
	}
	return ""
}

// verifyChecksum checks data against asset's entry in a sha256sum-format
// checksums document. A missing or ambiguous entry is as fatal as a mismatch:
// ilmctl never applies a manifest it could not verify.
func verifyChecksum(checksums []byte, asset string, data []byte) error {
	want, err := checksumFor(checksums, asset)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, want) {
		return fmt.Errorf(
			"checksum mismatch for %s (downloaded %s, %s says %s); "+
				"refusing to apply a manifest that does not match the published release",
			asset, got, checksumsAsset, want)
	}
	return nil
}

// checksumFor finds asset's digest in a `sha256sum` document, tolerating both
// the text (`digest  name`) and binary (`digest *name`) forms.
//
// A file listed more than once is rejected even when the entries agree: a
// checksums document that names the same artifact twice is malformed, and
// picking one of its entries would let the choice — rather than the release —
// decide what counts as verified.
func checksumFor(checksums []byte, asset string) (string, error) {
	var digests []string
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
		if name == asset {
			digests = append(digests, fields[0])
		}
	}
	switch len(digests) {
	case 0:
		return "", fmt.Errorf(
			"%s has no entry for %s; refusing to apply an unverified manifest", checksumsAsset, asset)
	case 1:
		return digests[0], nil
	default:
		return "", fmt.Errorf(
			"%s lists %s more than once (%d entries); "+
				"refusing to apply a manifest whose published checksum is ambiguous",
			checksumsAsset, asset, len(digests))
	}
}

// Fetch reads a manifest ref. Supported schemes: bare file path, file://, http(s)://.
func Fetch(ctx context.Context, ref string) ([]byte, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return fetchHTTP(ctx, ref)
	}
	path := strings.TrimPrefix(ref, "file://")
	data, err := os.ReadFile(path) //nolint:gosec // user-supplied install manifest path is intentional
	if err != nil {
		return nil, fmt.Errorf("read manifest %q: %w", path, err)
	}
	return data, nil
}

// errNotFound marks a 404 so the release paths can turn it into guidance
// instead of a bare status line.
var errNotFound = errors.New("not found")

func fetchHTTP(ctx context.Context, url string) ([]byte, error) {
	body, _, err := fetchHTTPHops(ctx, url)
	return body, err
}

// fetchHTTPHops is fetchHTTP plus the URLs the request was redirected through
// (the redirect targets in order, then the URL that finally answered).
func fetchHTTPHops(ctx context.Context, url string) ([]byte, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("build request for %q: %w", url, err)
	}
	var hops []string
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			hops = append(hops, r.URL.String())
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil, fmt.Errorf("fetch %q: %w (HTTP 404)", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch %q: unexpected status %d %s", url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body of %q: %w", url, err)
	}
	return body, append(hops, resp.Request.URL.String()), nil
}

// Split decodes a multi-document YAML stream into unstructured objects,
// skipping empty/comment-only documents.
func Split(raw []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	dec := apiyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(raw)))
	for {
		doc, err := dec.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read yaml document: %w", err)
		}
		if len(bytes.TrimSpace(stripComments(doc))) == 0 {
			continue
		}
		m := map[string]any{}
		if err := yaml.Unmarshal(doc, &m); err != nil {
			return nil, fmt.Errorf("unmarshal yaml document: %w", err)
		}
		if len(m) == 0 {
			continue
		}
		objs = append(objs, &unstructured.Unstructured{Object: m})
	}
	return objs, nil
}

// stripComments removes whole-line comments so comment-only documents are
// treated as empty.
func stripComments(doc []byte) []byte {
	var b bytes.Buffer
	for _, line := range bytes.Split(doc, []byte("\n")) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("#")) {
			continue
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

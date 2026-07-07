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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	// SourceVersion is a published release tag (--version, the default).
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
	Kind          SourceKind
}

// ErrUnreleased is returned for the default --version path while the operator
// has no published release; callers steer the user at --ref / --from-source.
var ErrUnreleased = errors.New("operator has no published release yet; use --ref <commit> or --from-source")

const (
	crdsFile       = "deploy/manifests/ilm-operator.crds.yaml"
	controllerFile = "deploy/manifests/ilm-operator.yaml"
	rawBase        = "https://raw.githubusercontent.com/OmniTrustILM/operator"
)

// Resolve picks the source per precedence (--manifest > --from-source > --ref >
// --version) and returns the CRDs-doc and controller-doc refs. The default
// --version path returns ErrUnreleased.
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
	default:
		return Resolved{}, ErrUnreleased
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

func fetchHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %q: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %q: unexpected status %d %s", url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body of %q: %w", url, err)
	}
	return body, nil
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

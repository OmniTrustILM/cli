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

package infra

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/manifest"
	"github.com/OmniTrustILM/cli/internal/render"
)

// newTestOptions builds a minimal cli.Options for infra tests with a Printer
// wired to the supplied buffers. Factory is intentionally nil; tests inject via clientFor.
func newTestOptions(out, errOut *bytes.Buffer) *cli.Options {
	return &cli.Options{
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
	}
}

const (
	infraAllNamespaces = "all-namespaces"
	infraDryRunClient  = "--dry-run=client"
	infraFromSource    = "--from-source"
	infraDryRun        = "dry-run"
	infraOperatorSys   = "ilm-operator-system"
	infraVer2180       = "2.18.0"
	infraHealthy       = "Healthy"
	infraReady         = "Ready"
	infraRunning       = "Running"
	infraCNPG          = "infra:cnpg"
	infraPlatformsCRD  = "platforms.otilm.com"
	infraPhaseKey      = "phase"
	infraManifestFlag  = "--manifest"
	infraWaitFlag      = "--wait"
	infraTimeoutFlag   = "--timeout"
	infraController    = "my-controller"
	infraCatalogFlag   = "--catalog-image"
	infraCatalogImage  = "example.com/ilm-operator-catalog:latest"
	infraMethodOLM     = "--method=olm"
	infraChannelFlag   = "--channel"
	infraChannelStable = "stable"
	infraVersionFlag   = "--version"
	infraCRDsAsset     = "ilm-operator.crds.yaml"
	infraCtrlAsset     = "ilm-operator.yaml"
	infraReleaseTag    = "v1.0.0"
)

// releaseAssets renders the assets of a published operator release: the two
// install manifests plus the sha256sum-format checksums.txt over them.
func releaseAssets() map[string][]byte {
	assets := map[string][]byte{
		infraCRDsAsset: []byte(fakeOperatorCRDs),
		infraCtrlAsset: []byte(fakeOperatorController),
	}
	var sums strings.Builder
	for _, n := range []string{infraCRDsAsset, infraCtrlAsset} {
		sum := sha256.Sum256(assets[n])
		_, _ = fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), n)
	}
	assets["checksums.txt"] = []byte(sums.String())
	return assets
}

// serveRelease publishes assets as release tag from an httptest server shaped
// like the GitHub releases host (/download/<tag>/<asset>, plus the
// /latest/download/<asset> redirect) and points the manifest package's release
// base at it for the duration of the test, so the release paths never touch the
// network. Assets mutated after releaseAssets() but before this call model a
// download that no longer matches the published checksums.
func serveRelease(t *testing.T, tag string, assets map[string][]byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutPrefix(r.URL.Path, "/latest/download/"); ok {
			http.Redirect(w, r, "/download/"+tag+"/"+name, http.StatusFound)
			return
		}
		name, ok := strings.CutPrefix(r.URL.Path, "/download/"+tag+"/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		body, ok := assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(manifest.SetReleaseBaseForTest(srv.URL))
}

// serveNoRelease points the release base at a host that publishes nothing, the
// shape GitHub answers with when a repository has no releases at all.
func serveNoRelease(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	t.Cleanup(manifest.SetReleaseBaseForTest(srv.URL))
}

// establishedCRDClient builds a fake Client whose scheme includes
// apiextensions CRDs. When olmPresent is true the discovery fake advertises
// the operators.coreos.com group (reuses buildInitClient logic).
func establishedCRDClient(t *testing.T, olmPresent bool) *k8s.Client {
	t.Helper()
	c := buildInitClient(t, olmPresent)

	// Extend the scheme to handle apiextv1 CRD objects for tests that
	// create or assert on CRD resources.
	require.NoError(t, apiextv1.AddToScheme(c.Scheme))

	// Rebuild the typed client with the extended scheme.
	c.Typed = ctrlfake.NewClientBuilder().WithScheme(c.Scheme).Build()
	return c
}

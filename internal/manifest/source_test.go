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
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		src            Source
		wantErr        error
		wantKind       SourceKind
		wantCRDs       string
		wantController string
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
			name:    "version default is unreleased",
			src:     Source{Version: "v2.18.0"},
			wantErr: ErrUnreleased,
		},
		{
			name:    "empty source is unreleased",
			src:     Source{},
			wantErr: ErrUnreleased,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Resolve(tc.src)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantKind, got.Kind)
			assert.Equal(t, tc.wantCRDs, got.CRDsRef)
			assert.Equal(t, tc.wantController, got.ControllerRef)
		})
	}
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

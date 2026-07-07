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

package diag

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/OmniTrustILM/cli/internal/cli"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/render"
	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// fakeOptions returns a cli.Options backed by a fake k8s client seeded with plat.
func fakeOptions(t *testing.T, plat *otilmv1alpha1.Platform) *cli.Options {
	t.Helper()
	fakeClient := k8s.NewFakeClient(t, k8s.FakeClientOptions{Objects: []ctrlclient.Object{plat}})
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &cli.Options{
		Out:     out,
		ErrOut:  errOut,
		Printer: render.NewPrinter(out, errOut),
		Factory: k8s.NewFactoryWithClient(fakeClient),
	}
}

// runningPlatform returns a Platform in Running phase with Available=True,
// suitable for seeding a fake cluster in collect/analyze tests.
func runningPlatform() *otilmv1alpha1.Platform {
	return &otilmv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ilm", Namespace: "default", Generation: 1,
		},
		Spec: otilmv1alpha1.PlatformSpec{Version: "2.18.0"},
		Status: otilmv1alpha1.PlatformStatus{
			Phase:              otilmv1alpha1.PlatformPhaseRunning,
			ObservedGeneration: 1,
			ObservedVersion:    "2.18.0",
			Conditions: []metav1.Condition{
				{Type: "Available", Status: metav1.ConditionTrue, Reason: "AllComponentsReady"},
			},
		},
	}
}

// TestCollectAndAnalyze_E2E exercises the full collect→analyze pipeline using a
// fake client (no real cluster, no cosign). It verifies that a bundle collected
// from a fake Platform can be loaded and analyzed without error.
func TestCollectAndAnalyze_E2E(t *testing.T) {
	t.Parallel()

	o := fakeOptions(t, runningPlatform())
	errOut := o.ErrOut.(*bytes.Buffer)

	// --- Collect into a temp file ---
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "test.zip")

	fo := newCollectOptions()
	fo.Output = bundlePath
	fo.IncludeLogs = false // no pods in fake cluster

	err := runCollect(context.Background(), o, fo)
	require.NoError(t, err)

	// Verify the bundle file was written.
	info, err := os.Stat(bundlePath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))

	// --- Analyze the bundle ---
	var analyzeOut bytes.Buffer
	err = runAnalyze(&cli.Options{Out: &analyzeOut, ErrOut: errOut, Printer: render.NewPrinter(&analyzeOut, errOut)}, bundlePath, "md")
	// A Platform in Running phase with Available=True should produce no fail findings.
	assert.NoError(t, err, "clean bundle should produce no fail-severity findings")
	assert.Contains(t, analyzeOut.String(), "# ILM Diagnostics")
}

// TestCollect_OutputDir exercises the --output-dir branch of runCollect, which
// calls unpackZip and writes individual files into the target directory instead
// of writing a single archive file.
func TestCollect_OutputDir(t *testing.T) {
	t.Parallel()

	o := fakeOptions(t, runningPlatform())

	outDir := t.TempDir()
	fo := newCollectOptions()
	fo.OutputDir = outDir
	fo.IncludeLogs = false

	err := runCollect(context.Background(), o, fo)
	require.NoError(t, err)

	// The bundle must have been unpacked: at minimum manifest.json is present.
	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "--output-dir: expected at least one file unpacked")

	var hasManifest bool
	for _, e := range entries {
		if e.Name() == "manifest.json" {
			hasManifest = true
			break
		}
	}
	assert.True(t, hasManifest, "--output-dir: manifest.json must be present in unpacked bundle")

	// stdout must contain the "unpacked to" message.
	outStr := o.Out.(*bytes.Buffer).String()
	assert.Contains(t, outStr, "Bundle unpacked to")
	assert.Contains(t, outStr, outDir)
}

// TestCollect_AllNamespaces exercises the --all-namespaces branch of runCollect,
// confirming that the collector runs without falling through to namespace
// resolution from the Factory (which would use ConfigFlags).
func TestCollect_AllNamespaces(t *testing.T) {
	t.Parallel()

	o := fakeOptions(t, runningPlatform())

	tmpDir := t.TempDir()
	fo := newCollectOptions()
	fo.Output = filepath.Join(tmpDir, "bundle.zip")
	fo.AllNamespaces = true
	fo.IncludeLogs = false

	err := runCollect(context.Background(), o, fo)
	require.NoError(t, err)

	info, err := os.Stat(fo.Output)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

// TestCollect_TGZFormat exercises the tgz format path of runCollect (without
// --output-dir, so the archive is written to a named file).
func TestCollect_TGZFormat(t *testing.T) {
	t.Parallel()

	o := fakeOptions(t, runningPlatform())

	tmpDir := t.TempDir()
	fo := newCollectOptions()
	fo.Output = filepath.Join(tmpDir, "bundle.tgz")
	fo.Format = "tgz"
	fo.IncludeLogs = false

	err := runCollect(context.Background(), o, fo)
	require.NoError(t, err)

	info, err := os.Stat(fo.Output)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))

	outStr := o.Out.(*bytes.Buffer).String()
	assert.Contains(t, outStr, "Bundle written to")
}

// TestCollect_DefaultOutputName verifies the path in runCollect where --output
// is not specified, so the bundle is written to a timestamped name returned by
// defaultBundleName. The test changes CWD to a temp dir so the generated file
// lands there and is cleaned up automatically.
//
// Note: t.Chdir cannot be combined with t.Parallel (Go 1.21+), so this test
// runs sequentially.
func TestCollect_DefaultOutputName(t *testing.T) {
	o := fakeOptions(t, runningPlatform())

	// Change CWD so the timestamped bundle is created inside t.TempDir().
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	fo := newCollectOptions()
	// fo.Output is intentionally left empty — runCollect will use defaultBundleName.
	fo.IncludeLogs = false

	err := runCollect(context.Background(), o, fo)
	require.NoError(t, err)

	// At least one file matching the expected pattern must exist in tmpDir.
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "runCollect must create a bundle file in the working directory")

	outStr := o.Out.(*bytes.Buffer).String()
	assert.Contains(t, outStr, "Bundle written to")
	assert.Contains(t, outStr, "ilm-diagnostics-")
}

// TestCollect_Sign_CosignAbsent verifies that when --sign is requested but
// cosign is not available on PATH, runCollect returns a descriptive error that
// mentions cosign. This confirms that bundle.Sign is invoked and that the
// --sign flag is wired through the collect pipeline.
//
// A hermetic happy-path test for --sign would require shimming the cosign
// binary; the sign seam (bundle.signerFinder / bundle.signRunner) is
// internal to the bundle package and not exposed for cross-package injection,
// so we instead assert on the failure mode.
//
// Note: t.Setenv cannot be combined with t.Parallel (Go 1.17+), so this test
// runs sequentially.
func TestCollect_Sign_CosignAbsent(t *testing.T) {
	o := fakeOptions(t, runningPlatform())

	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "signed.zip")

	fo := newCollectOptions()
	fo.Output = bundlePath
	fo.Sign = true
	fo.IncludeLogs = false

	// Point PATH at an empty directory so cosign cannot be found.
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)

	err := runCollect(context.Background(), o, fo)
	require.Error(t, err, "--sign with no cosign on PATH must fail")
	assert.Contains(t, err.Error(), "cosign", "error must mention cosign")
}

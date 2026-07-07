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
	"context"
	"fmt"
	"os/exec"
)

// signerFinder locates the cosign binary; overridable in tests.
var signerFinder = lookCosign

// signRunner executes the signing command; overridable in tests.
var signRunner = runCosign

func lookCosign() (string, error) {
	return exec.LookPath("cosign")
}

func runCosign(ctx context.Context, bin, blobPath, sigPath string) error {
	cmd := exec.CommandContext(ctx, bin, "sign-blob", "--yes", //nolint:gosec // bin is resolved via exec.LookPath
		"--output-signature", sigPath, blobPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cosign sign-blob: %w: %s", err, string(out))
	}
	return nil
}

// Sign signs the bundle at path with cosign, writing "<path>.sig", and returns
// the signature path. It fails clearly when cosign is not on PATH.
func Sign(ctx context.Context, path string) (string, error) {
	bin, err := signerFinder()
	if err != nil {
		return "", fmt.Errorf("cosign not found on PATH (required for --sign): %w", err)
	}
	sigPath := path + ".sig"
	if err := signRunner(ctx, bin, path, sigPath); err != nil {
		return "", err
	}
	return sigPath, nil
}

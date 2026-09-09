/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
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

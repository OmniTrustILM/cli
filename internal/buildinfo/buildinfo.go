/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

// Package buildinfo holds module-wide constants and ldflags-injected build
// metadata shared across the CLI.
package buildinfo

// BinaryName is the canonical standalone binary name.
const BinaryName = "ilmctl"

// PluginBinaryName is the kubectl plugin executable name.
const PluginBinaryName = "kubectl-ilm"

// PluginWord is the kubectl subcommand word (`kubectl ilm`).
const PluginWord = "ilm"

// Build metadata injected at link time via -ldflags (see Makefile / Dockerfile):
//
//	-X github.com/OmniTrustILM/cli/internal/buildinfo.GitVersion=<version>
//	-X github.com/OmniTrustILM/cli/internal/buildinfo.GitCommit=<commit>
//	-X github.com/OmniTrustILM/cli/internal/buildinfo.BuildDate=<date>
//
// When the linker does not inject a value the variable retains its zero value
// (""); callers should treat "" as the sentinel for an unset field and substitute
// a sensible default (e.g. "dev", "none", "unknown").
var (
	GitVersion string
	GitCommit  string
	BuildDate  string
)

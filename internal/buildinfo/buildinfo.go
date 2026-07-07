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

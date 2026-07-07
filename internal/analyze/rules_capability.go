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

package analyze

import (
	"fmt"

	"github.com/OmniTrustILM/cli/internal/capabilities"
)

// capabilityAnalyzer flags a Platform that selects a managed mode whose upstream
// operator is absent, steering the user at the matching `ilmctl deps install`.
type capabilityAnalyzer struct{}

func (capabilityAnalyzer) Name() string { return "capability" }

func (a capabilityAnalyzer) Analyze(s *Snapshot) []Finding {
	// Capability presence is only knowable live; a bundle reports none, so the
	// rule is silent offline.
	if len(s.Capabilities) == 0 {
		return nil
	}
	present := make(map[capabilities.Dep]bool, len(s.Capabilities))
	for _, r := range s.Capabilities {
		present[r.Dep] = r.Present
	}

	// Union the modes across every Platform, then ask which deps they require.
	var modes capabilities.Modes
	for _, p := range s.Platforms {
		modes.DBManaged = modes.DBManaged || p.SpecModes.DBManaged
		modes.MessagingManaged = modes.MessagingManaged || p.SpecModes.MessagingManaged
		modes.KeycloakManaged = modes.KeycloakManaged || p.SpecModes.KeycloakManaged
		if p.SpecModes.Edge != "" {
			modes.Edge = p.SpecModes.Edge
		}
		if p.SpecModes.TLSSource != "" {
			modes.TLSSource = p.SpecModes.TLSSource
		}
	}

	var out []Finding
	for _, dep := range capabilities.RequiredFor(modes) {
		if present[dep] {
			continue
		}
		out = append(out, Finding{
			// Resource is intentionally empty: a missing upstream operator is a
			// cluster-scoped condition, not tied to any single Platform CR.
			Severity:    SeverityFail,
			Rule:        a.Name(),
			Title:       fmt.Sprintf("required upstream operator %q is missing", dep),
			Evidence:    fmt.Sprintf("a managed Platform mode requires %s but it is not installed", dep),
			Remediation: fmt.Sprintf("ilmctl deps install --only %s", dep),
		})
	}
	return out
}

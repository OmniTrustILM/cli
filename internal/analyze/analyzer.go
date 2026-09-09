/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package analyze

import "sort"

// Analyzer is one deterministic rule over a Snapshot. Name identifies the rule
// (stamped into Finding.Rule); Analyze returns zero or more findings.
type Analyzer interface {
	Name() string
	Analyze(s *Snapshot) []Finding
}

// Registry runs an ordered set of analyzers and merges their findings.
type Registry struct {
	analyzers []Analyzer
}

// NewRegistry builds a registry from an explicit analyzer list.
func NewRegistry(a ...Analyzer) *Registry {
	return &Registry{analyzers: a}
}

// DefaultRegistry returns a registry pre-loaded with the standard rule set.
func DefaultRegistry() *Registry {
	return NewRegistry(
		newConditionAnalyzer(),
		newPhaseAnalyzer(),
		reconcileAnalyzer{},
		newWorkloadAnalyzer(),
		capabilityAnalyzer{},
		referenceAnalyzer{},
		versionAnalyzer{},
		newEventAnalyzer(),
		newLogsigAnalyzer(),
	)
}

// Analyzers exposes the registered analyzers (read-only inspection).
func (r *Registry) Analyzers() []Analyzer { return r.analyzers }

// Run executes every analyzer over s and returns all findings ordered
// fail > warn > info > ok, then by resource, then by title — a deterministic
// order for both human tables and -o json output.
func (r *Registry) Run(s *Snapshot) []Finding {
	var all []Finding
	for _, a := range r.analyzers {
		all = append(all, a.Analyze(s)...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		ri, rj := severityRank(all[i].Severity), severityRank(all[j].Severity)
		if ri != rj {
			return ri > rj // higher severity first
		}
		if all[i].Resource != all[j].Resource {
			return all[i].Resource < all[j].Resource
		}
		return all[i].Title < all[j].Title
	})
	return all
}

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

import "fmt"

// referenceAnalyzer turns pre-resolved missing references (secret/issuer/
// configmap names the live builder could not find) into findings. A bundle
// leaves Snapshot.MissingRefs nil, so this rule is correctly silent offline.
type referenceAnalyzer struct{}

func (referenceAnalyzer) Name() string { return "reference" }

func (a referenceAnalyzer) Analyze(s *Snapshot) []Finding {
	out := make([]Finding, 0, len(s.MissingRefs))
	for _, ref := range s.MissingRefs {
		out = append(out, Finding{
			Severity:    SeverityFail,
			Rule:        a.Name(),
			Resource:    ref,
			Title:       "referenced object not found",
			Evidence:    fmt.Sprintf("a Platform/Connector/Proxy spec references %s, which does not exist", ref),
			Remediation: "create the missing Secret/Issuer/ConfigMap or correct the reference in the CR spec",
		})
	}
	return out
}

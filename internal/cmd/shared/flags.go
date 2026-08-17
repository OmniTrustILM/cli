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

package shared

import (
	"time"

	"github.com/OmniTrustILM/cli/internal/k8s"
)

// Operator label keys (mirrors operator internal/builder/common/labels.go).
// Components are labelled app.kubernetes.io/name=<component>,
// app.kubernetes.io/instance=<platform>.
const (
	nameLabel     = "app.kubernetes.io/name"
	instanceLabel = "app.kubernetes.io/instance"
)

// componentCore is the platform's core component name — the default logs target.
const componentCore = "core"

// PlatformLogComponents lists the platform components the operator renders as
// Deployments, using the operator's real component names.
// provisioning-rabbitmq only exists when provisioning.mode=deploy; the logs
// command surfaces a clear "no pods" error otherwise.
var PlatformLogComponents = []string{
	componentCore, "auth", "auth-opa-policies", "scheduler",
	"fe-administrator", "utils", "api-gateway", "provisioning-rabbitmq",
}

// ComponentSelector returns the Deployment label selector for one platform
// component.
func ComponentSelector(platform, component string) map[string]string {
	return map[string]string{
		nameLabel:     component,
		instanceLabel: platform,
	}
}

// LogsRequest carries the parameters for a pod-log tail operation. Using a
// struct instead of individual positional arguments keeps execLogs signatures
// within the parameter-count threshold enforced by static analysis.
type LogsRequest struct {
	Namespace string
	Name      string
	Component string // empty for resource types that have a single implicit container
	Follow    bool
	Since     time.Duration
	Tail      int64
}

// ResolveNamespace folds -n / -A into the namespace list a read command
// iterates. allNamespaces wins (returns a single empty entry meaning
// cluster-wide); an explicit list is honoured next; otherwise the factory's
// resolved namespace is used.
func ResolveNamespace(f *k8s.Factory, allNamespaces bool, explicit []string) ([]string, error) {
	if allNamespaces {
		return []string{""}, nil
	}
	if len(explicit) > 0 {
		return explicit, nil
	}
	ns, _, err := f.Namespace()
	if err != nil {
		return nil, err
	}
	return []string{ns}, nil
}

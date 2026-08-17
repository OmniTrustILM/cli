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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// defaultRestartThreshold flags a container restarting more often than this.
const defaultRestartThreshold int32 = 5

// fatalWaitReasons are pod waiting reasons that always mean a broken workload.
var fatalWaitReasons = map[string]bool{
	"CrashLoopBackOff": true,
	"ImagePullBackOff": true,
	"ErrImagePull":     true,
}

// workloadAnalyzer inspects operator-owned Deployments and Pods for standard
// Kubernetes ill-health (under-replication, crash loops, image pull failures,
// OOM kills, restart storms).
type workloadAnalyzer struct {
	restartThreshold int32
}

func newWorkloadAnalyzer() workloadAnalyzer {
	return workloadAnalyzer{restartThreshold: defaultRestartThreshold}
}

func (workloadAnalyzer) Name() string { return "workload" }

func (a workloadAnalyzer) Analyze(s *Snapshot) []Finding {
	thresh := a.restartThreshold
	if thresh <= 0 {
		thresh = defaultRestartThreshold
	}
	var out []Finding
	for _, r := range s.Resources() {
		ref := r.ResourceRef()
		for i := range r.Deployments {
			if f, ok := evalDeployment(ref, a.Name(), &r.Deployments[i]); ok {
				out = append(out, f)
			}
		}
		for i := range r.Pods {
			out = append(out, evalPod(ref, a.Name(), thresh, &r.Pods[i])...)
		}
	}
	return out
}

func evalDeployment(ref, rule string, d *appsv1.Deployment) (Finding, bool) {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	if d.Status.ReadyReplicas >= desired {
		return Finding{}, false
	}
	return Finding{
		Severity:    SeverityFail,
		Rule:        rule,
		Resource:    ref,
		Title:       fmt.Sprintf("deployment %s under-replicated", d.Name),
		Evidence:    fmt.Sprintf("%s ready %d/%d", d.Name, d.Status.ReadyReplicas, desired),
		Remediation: "inspect pod status and events for the failing deployment",
	}, true
}

func evalPod(ref, rule string, thresh int32, p *corev1.Pod) []Finding {
	var out []Finding
	for i := range p.Status.ContainerStatuses {
		cs := p.Status.ContainerStatuses[i]
		switch {
		case cs.State.Waiting != nil && fatalWaitReasons[cs.State.Waiting.Reason]:
			out = append(out, Finding{
				Severity:    SeverityFail,
				Rule:        rule,
				Resource:    ref,
				Title:       fmt.Sprintf("pod %s not ready", p.Name),
				Evidence:    fmt.Sprintf("pod %s container %s: %s", p.Name, cs.Name, cs.State.Waiting.Reason),
				Remediation: "check the container image and logs",
			})
		case oomKilled(cs):
			out = append(out, Finding{
				Severity:    SeverityFail,
				Rule:        rule,
				Resource:    ref,
				Title:       fmt.Sprintf("pod %s OOMKilled", p.Name),
				Evidence:    fmt.Sprintf("pod %s container %s: OOMKilled", p.Name, cs.Name),
				Remediation: "raise the container memory limit or reduce its footprint",
			})
		case cs.RestartCount > thresh:
			out = append(out, Finding{
				Severity: SeverityWarn,
				Rule:     rule,
				Resource: ref,
				Title:    fmt.Sprintf("pod %s restarting", p.Name),
				Evidence: fmt.Sprintf("pod %s container %s restarted %d times", p.Name, cs.Name, cs.RestartCount),
			})
		}
	}
	if p.Status.Phase == corev1.PodPending && len(p.Status.ContainerStatuses) == 0 {
		out = append(out, Finding{
			Severity:    SeverityWarn,
			Rule:        rule,
			Resource:    ref,
			Title:       fmt.Sprintf("pod %s pending", p.Name),
			Evidence:    fmt.Sprintf("pod %s is Pending (unscheduled)", p.Name),
			Remediation: "check node capacity, taints and scheduling events",
		})
	}
	return out
}

// reasonOOMKilled is the kubelet's termination reason for an OOM-killed container.
const reasonOOMKilled = "OOMKilled"

func oomKilled(cs corev1.ContainerStatus) bool {
	if cs.State.Terminated != nil && cs.State.Terminated.Reason == reasonOOMKilled {
		return true
	}
	return cs.LastTerminationState.Terminated != nil &&
		cs.LastTerminationState.Terminated.Reason == reasonOOMKilled
}

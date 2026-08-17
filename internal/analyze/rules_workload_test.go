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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ptrInt32(i int32) *int32 { return &i }

func deploy(name string, desired *int32, ready int32) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: desired},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: ready},
	}
}

func waitingPod(name, container, reason string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  container,
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}},
			}},
		},
	}
}

func TestWorkloadAnalyzer(t *testing.T) {
	t.Parallel()
	a := newWorkloadAnalyzer()

	t.Run("deployment below desired is fail", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Deployments: []appsv1.Deployment{deploy(analyzeCoreComponent, ptrInt32(3), 1)},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityFail, got[0].Severity)
		assert.Equal(t, "workload", got[0].Rule)
		assert.Contains(t, got[0].Evidence, analyzeCoreComponent)
		assert.Contains(t, got[0].Evidence, "1/3")
	})

	t.Run("healthy deployment emits nothing", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Deployments: []appsv1.Deployment{deploy(analyzeCoreComponent, ptrInt32(2), 2)},
		}}}
		assert.Empty(t, a.Analyze(s))
	})

	t.Run("crashloop pod is fail", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Pods: []corev1.Pod{waitingPod("core-abc", analyzeCoreComponent, "CrashLoopBackOff")},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityFail, got[0].Severity)
		assert.Contains(t, got[0].Evidence, "CrashLoopBackOff")
		assert.Contains(t, got[0].Evidence, "core-abc")
	})

	t.Run("imagepull pod is fail", func(t *testing.T) {
		t.Parallel()
		s := &Snapshot{Connectors: []ResourceSnapshot{{
			GVK: GVKConnector, Namespace: "ns", Name: "c",
			Pods: []corev1.Pod{waitingPod("c-xyz", "connector", "ImagePullBackOff")},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityFail, got[0].Severity)
	})

	t.Run("high restart count is warn", func(t *testing.T) {
		t.Parallel()
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "core-1", Namespace: "ns"},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				ContainerStatuses: []corev1.ContainerStatus{{
					Name:         analyzeCoreComponent,
					RestartCount: 12,
					State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				}},
			},
		}
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Pods: []corev1.Pod{pod},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityWarn, got[0].Severity)
		assert.Contains(t, got[0].Evidence, "12")
	})

	t.Run("OOMKilled pod is fail", func(t *testing.T) {
		t.Parallel()
		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "core-2", Namespace: "ns"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: analyzeCoreComponent,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{Reason: reasonOOMKilled},
					},
				}},
			},
		}
		s := &Snapshot{Platforms: []ResourceSnapshot{{
			GVK: analyzeAPIGroup, Namespace: "ns", Name: analyzePlatformName,
			Pods: []corev1.Pod{pod},
		}}}
		got := a.Analyze(s)
		require.Len(t, got, 1)
		assert.Equal(t, SeverityFail, got[0].Severity)
		assert.Contains(t, got[0].Evidence, reasonOOMKilled)
	})
}

func TestWorkloadAnalyzerName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "workload", newWorkloadAnalyzer().Name())
}

/*
Copyright The ILM Authors.

SPDX-License-Identifier: Apache-2.0
*/

package manifest

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// deployPollInterval is how often WaitDeploymentsAvailable re-reads Deployment
// status while polling.
const deployPollInterval = 200 * time.Millisecond

// conditionTrue is the status a passing condition reports ("True").
const conditionTrue = "True"

// WaitDeploymentsAvailable blocks until every Deployment among applied reports
// Available, or the timeout elapses. Objects that are not Deployments are
// silently skipped; when applied contains no Deployments the call is a no-op
// success. A Deployment is considered Available when its rollout has completed:
// status.observedGeneration >= metadata.generation, and both updatedReplicas
// and availableReplicas equal the desired replica count (defaulting to 1 when
// spec.replicas is unset). The Available status condition being True is also
// accepted. On timeout the returned error names the Deployment(s) that never
// became Available.
func (a *Applier) WaitDeploymentsAvailable(ctx context.Context, applied []*unstructured.Unstructured, timeout time.Duration) error {
	for _, obj := range applied {
		if obj.GetKind() != kindDeployment {
			continue
		}
		ns, name := obj.GetNamespace(), obj.GetName()
		err := wait.PollUntilContextTimeout(ctx, deployPollInterval, timeout, true,
			func(ctx context.Context) (bool, error) {
				dep := &appsv1.Deployment{}
				gerr := a.Client.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, dep)
				if gerr != nil {
					if apierrors.IsNotFound(gerr) {
						return false, nil
					}
					return false, gerr
				}
				return deploymentAvailable(dep), nil
			})
		if err != nil {
			return fmt.Errorf("deployment %s/%s did not become Available: %w", ns, name, err)
		}
	}
	return nil
}

// deploymentAvailable reports whether a Deployment's rollout has completed. It
// accepts either the replica-count criterion (observed generation is current
// and updated/available replicas match the desired count) or an Available
// status condition of True.
func deploymentAvailable(dep *appsv1.Deployment) bool {
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	if dep.Status.ObservedGeneration >= dep.Generation &&
		dep.Status.UpdatedReplicas == desired &&
		dep.Status.AvailableReplicas == desired {
		return true
	}
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == conditionTrue {
			return true
		}
	}
	return false
}

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

package k8s

import (
	"context"
	"errors"
	"io"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// OperatorManagedByLabel and OperatorPlatformLabel select operator-owned
// workloads for a Platform; they mirror the labels the operator stamps on its
// children.
const (
	OperatorManagedByLabel = "app.kubernetes.io/managed-by"
	OperatorManagedByValue = "ilm-operator"
	OperatorPlatformLabel  = "otilm.com/platform"
	// OperatorDeploymentName is the controller-manager Deployment created by the
	// flat manifest in deploy/manifests/ilm-operator.yaml.
	OperatorDeploymentName = "ilm-operator-controller-manager"
)

// NewFactory builds a Factory with the shared scheme. ConfigFlags must be non-nil.
func NewFactory(cf *genericclioptions.ConfigFlags) (*Factory, error) {
	if cf == nil {
		return nil, errors.New("k8s: ConfigFlags must not be nil")
	}
	s, err := NewScheme()
	if err != nil {
		return nil, err
	}
	return &Factory{ConfigFlags: cf, Scheme: s}, nil
}

// restConfigForTest, when non-nil, overrides RESTConfig() so hermetic tests can
// inject a synthetic *rest.Config without touching the ambient kubeconfig.
// This variable is package-private and must only be set inside _test.go files.
var restConfigForTest func(*Factory) (*rest.Config, error)

// restMapperForTest, when non-nil, overrides the mapper construction inside
// Client() so hermetic tests avoid the live discovery call that ToRESTMapper()
// makes.  This variable is package-private and must only be set inside _test.go
// files.
var restMapperForTest func(*Factory) (meta.RESTMapper, error)

// RESTConfig resolves the *rest.Config from the kubectl-style flags.
func (f *Factory) RESTConfig() (*rest.Config, error) {
	if restConfigForTest != nil {
		return restConfigForTest(f)
	}
	return f.ConfigFlags.ToRESTConfig()
}

// Namespace returns the selected namespace and whether it was explicitly set
// (-n / context). The caller handles -A (all namespaces).
func (f *Factory) Namespace() (string, bool, error) {
	if f.ConfigFlags == nil {
		return "default", false, nil
	}
	ns, explicit, err := f.ConfigFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return "", false, err
	}
	return ns, explicit, nil
}

// Client builds the typed + dynamic + discovery client bundle.
func (f *Factory) Client() (*Client, error) {
	if f.fixedClient != nil {
		return f.fixedClient, nil
	}
	cfg, err := f.RESTConfig()
	if err != nil {
		return nil, err
	}
	var mapper meta.RESTMapper
	if restMapperForTest != nil {
		mapper, err = restMapperForTest(f)
	} else {
		mapper, err = f.ConfigFlags.ToRESTMapper()
	}
	if err != nil {
		return nil, err
	}
	typed, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: f.Scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{
		Typed:     typed,
		Dynamic:   dyn,
		Discovery: cs.Discovery(),
		Clientset: cs,
		Mapper:    mapper,
		Scheme:    f.Scheme,
	}, nil
}

// Client is the typed + dynamic + discovery surface the read/inspect/install code
// consumes. It is constructed by Factory.Client (live) or NewFakeClient (tests).
type Client struct {
	Typed     ctrlclient.Client
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Clientset kubernetes.Interface
	Mapper    meta.RESTMapper
	Scheme    *runtime.Scheme
}

// GetPlatform fetches one Platform.
func (c *Client) GetPlatform(ctx context.Context, ns, name string) (*otilmv1alpha1.Platform, error) {
	out := &otilmv1alpha1.Platform{}
	if err := c.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPlatforms lists Platforms in a namespace (empty ns => all namespaces).
func (c *Client) ListPlatforms(ctx context.Context, ns string) (*otilmv1alpha1.PlatformList, error) {
	out := &otilmv1alpha1.PlatformList{}
	if err := c.Typed.List(ctx, out, ctrlclient.InNamespace(ns)); err != nil {
		return nil, err
	}
	return out, nil
}

// GetConnector fetches one Connector.
func (c *Client) GetConnector(ctx context.Context, ns, name string) (*otilmv1alpha1.Connector, error) {
	out := &otilmv1alpha1.Connector{}
	if err := c.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListConnectors lists Connectors in a namespace.
func (c *Client) ListConnectors(ctx context.Context, ns string) (*otilmv1alpha1.ConnectorList, error) {
	out := &otilmv1alpha1.ConnectorList{}
	if err := c.Typed.List(ctx, out, ctrlclient.InNamespace(ns)); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProxy fetches one Proxy.
func (c *Client) GetProxy(ctx context.Context, ns, name string) (*otilmv1alpha1.Proxy, error) {
	out := &otilmv1alpha1.Proxy{}
	if err := c.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListProxies lists Proxies in a namespace.
func (c *Client) ListProxies(ctx context.Context, ns string) (*otilmv1alpha1.ProxyList, error) {
	out := &otilmv1alpha1.ProxyList{}
	if err := c.Typed.List(ctx, out, ctrlclient.InNamespace(ns)); err != nil {
		return nil, err
	}
	return out, nil
}

// OperatorDeployment fetches the operator controller-manager Deployment.
func (c *Client) OperatorDeployment(ctx context.Context, ns string) (*appsv1.Deployment, error) {
	out := &appsv1.Deployment{}
	if err := c.Typed.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: OperatorDeploymentName}, out); err != nil {
		return nil, err
	}
	return out, nil
}

// Events returns the events whose involvedObject is the given object (matched by
// namespace + name + UID).
func (c *Client) Events(ctx context.Context, ns string, involved ctrlclient.Object) ([]corev1.Event, error) {
	all := &corev1.EventList{}
	if err := c.Typed.List(ctx, all, ctrlclient.InNamespace(ns)); err != nil {
		return nil, err
	}
	out := make([]corev1.Event, 0, len(all.Items))
	for i := range all.Items {
		ref := all.Items[i].InvolvedObject
		if ref.Name == involved.GetName() && (ref.UID == "" || ref.UID == involved.GetUID()) {
			out = append(out, all.Items[i])
		}
	}
	return out, nil
}

// DeploymentsForPlatform returns the Deployments the operator created for a
// Platform (matched by managed-by + platform labels).
func (c *Client) DeploymentsForPlatform(ctx context.Context, ns, name string) ([]appsv1.Deployment, error) {
	list := &appsv1.DeploymentList{}
	if err := c.Typed.List(ctx, list,
		ctrlclient.InNamespace(ns),
		ctrlclient.MatchingLabels{
			OperatorManagedByLabel: OperatorManagedByValue,
			OperatorPlatformLabel:  name,
		},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// PodsFor lists pods matching the given label selector.
func (c *Client) PodsFor(ctx context.Context, ns string, sel map[string]string) ([]corev1.Pod, error) {
	list := &corev1.PodList{}
	if err := c.Typed.List(ctx, list, ctrlclient.InNamespace(ns), ctrlclient.MatchingLabels(sel)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// PodLogs streams a pod container's logs via the typed clientset. Logs are a
// subresource: they must be requested through the core/v1 REST client (which the
// clientset's CoreV1().GetLogs uses) — NOT the discovery client, whose REST
// client is scoped to the discovery group and cannot encode PodLogOptions.
func (c *Client) PodLogs(ctx context.Context, ns, pod, container string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	if c.Clientset == nil {
		return nil, errors.New("k8s: no clientset available to stream pod logs")
	}
	o := opts
	if o == nil {
		o = &corev1.PodLogOptions{}
	}
	o.Container = container
	return c.Clientset.CoreV1().Pods(ns).GetLogs(pod, o).Stream(ctx)
}

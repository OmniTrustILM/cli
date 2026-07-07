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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/version"
)

// defaultOperatorNamespace matches the operator's published manifest.
const defaultOperatorNamespace = "ilm-operator-system"

// GVKPlatform, GVKConnector, GVKProxy are the canonical "Kind.group/version"
// identifiers shared by the live builder and the bundle reader. Exporting them
// ensures a single definition — any API-group rename is caught at compile time.
const (
	GVKPlatform  = "Platform.otilm.com/v1alpha1"
	GVKConnector = "Connector.otilm.com/v1alpha1"
	GVKProxy     = "Proxy.otilm.com/v1alpha1"
)

// BuildOptions scopes a live snapshot build.
type BuildOptions struct {
	Namespaces        []string
	AllNamespaces     bool
	IncludeLogs       bool   // reserved; live logs attached by caller via Logs map
	OperatorNamespace string // default "ilm-operator-system"
}

// Builder assembles a Snapshot from a live cluster.
type Builder struct {
	Client *k8s.Client
	Caps   *capabilities.Reporter
}

// NewBuilder wires a Builder. caps may be nil (Capabilities is left empty).
func NewBuilder(c *k8s.Client, caps *capabilities.Reporter) *Builder {
	return &Builder{Client: c, Caps: caps}
}

// Build reads operator state, every CR with its workload/event context, and
// pre-resolves missing Secret references into the Snapshot the analyzer consumes.
func (b *Builder) Build(ctx context.Context, o BuildOptions) (*Snapshot, error) {
	snap := &Snapshot{
		ClientVersion:     version.Client().ClientVersion,
		SupportedVersions: version.SupportedVersions(),
	}
	b.fillOperator(ctx, o, snap)
	if b.Caps != nil {
		snap.Capabilities = b.Caps.Detect()
	}

	missing := map[string]bool{}
	for _, ns := range b.scopes(o) {
		if err := b.collectPlatforms(ctx, ns, snap, missing); err != nil {
			return nil, err
		}
		if err := b.collectConnectors(ctx, ns, snap); err != nil {
			return nil, err
		}
		if err := b.collectProxies(ctx, ns, snap); err != nil {
			return nil, err
		}
	}
	for ref := range missing {
		snap.MissingRefs = append(snap.MissingRefs, ref)
	}
	return snap, nil
}

func (b *Builder) fillOperator(ctx context.Context, o BuildOptions, snap *Snapshot) {
	ns := o.OperatorNamespace
	if ns == "" {
		ns = defaultOperatorNamespace
	}
	dep, err := b.Client.OperatorDeployment(ctx, ns)
	if err != nil {
		return // absent/forbidden operator is not fatal
	}
	snap.OperatorReady = dep.Status.ReadyReplicas >= 1
	if cs := dep.Spec.Template.Spec.Containers; len(cs) > 0 {
		snap.OperatorVersion = imageTag(cs[0].Image)
	}
}

func (b *Builder) collectPlatforms(ctx context.Context, ns string, snap *Snapshot, missing map[string]bool) error {
	list, err := b.Client.ListPlatforms(ctx, ns)
	if err != nil {
		if tolerable(err) {
			return nil
		}
		return err
	}
	for i := range list.Items {
		p := &list.Items[i]
		secrets, issuers := PlatformRefs(p)
		rs := ResourceSnapshot{
			GVK:             GVKPlatform,
			Namespace:       p.Namespace,
			Name:            p.Name,
			Phase:           string(p.Status.Phase),
			ObservedVersion: p.Status.ObservedVersion,
			ObservedGen:     p.Status.ObservedGeneration,
			Generation:      p.Generation,
			Conditions:      p.Status.Conditions,
			SpecModes:       PlatformModes(p),
			SecretRefs:      secrets,
			IssuerRefs:      issuers,
		}
		if deps, derr := b.Client.DeploymentsForPlatform(ctx, p.Namespace, p.Name); derr == nil {
			rs.Deployments = deps
		}
		if ev, eerr := b.Client.Events(ctx, p.Namespace, p); eerr == nil {
			rs.Events = ev
		}
		b.resolveMissing(ctx, p.Namespace, secrets, missing)
		snap.Platforms = append(snap.Platforms, rs)
	}
	return nil
}

func (b *Builder) collectConnectors(ctx context.Context, ns string, snap *Snapshot) error {
	list, err := b.Client.ListConnectors(ctx, ns)
	if err != nil {
		if tolerable(err) {
			return nil
		}
		return err
	}
	for i := range list.Items {
		c := &list.Items[i]
		rs := ResourceSnapshot{
			GVK:         GVKConnector,
			Namespace:   c.Namespace,
			Name:        c.Name,
			Phase:       string(c.Status.Phase),
			ObservedGen: c.Status.ObservedGeneration,
			Generation:  c.Generation,
			Conditions:  c.Status.Conditions,
		}
		if ev, eerr := b.Client.Events(ctx, c.Namespace, c); eerr == nil {
			rs.Events = ev
		}
		snap.Connectors = append(snap.Connectors, rs)
	}
	return nil
}

func (b *Builder) collectProxies(ctx context.Context, ns string, snap *Snapshot) error {
	list, err := b.Client.ListProxies(ctx, ns)
	if err != nil {
		if tolerable(err) {
			return nil
		}
		return err
	}
	for i := range list.Items {
		p := &list.Items[i]
		rs := ResourceSnapshot{
			GVK:             GVKProxy,
			Namespace:       p.Namespace,
			Name:            p.Name,
			Phase:           string(p.Status.Phase),
			ObservedVersion: p.Status.ObservedVersion,
			ObservedGen:     p.Status.ObservedGeneration,
			Generation:      p.Generation,
			Conditions:      p.Status.Conditions,
			SecretRefs:      ProxyRefs(p),
		}
		if ev, eerr := b.Client.Events(ctx, p.Namespace, p); eerr == nil {
			rs.Events = ev
		}
		snap.Proxies = append(snap.Proxies, rs)
	}
	return nil
}

// resolveMissing flags any referenced Secret that does not exist. A forbidden
// read is treated as "exists" so an RBAC gap never produces a false positive.
func (b *Builder) resolveMissing(ctx context.Context, ns string, secretRefs []string, missing map[string]bool) {
	for _, name := range secretRefs {
		var sec corev1.Secret
		err := b.Client.Typed.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &sec)
		if apierrors.IsNotFound(err) {
			missing[fmt.Sprintf("Secret/%s/%s", ns, name)] = true
		}
	}
}

func (b *Builder) scopes(o BuildOptions) []string {
	if o.AllNamespaces || len(o.Namespaces) == 0 {
		return []string{""}
	}
	return o.Namespaces
}

// imageTag returns the tag of an image reference, or "" when none is present.
func imageTag(image string) string {
	at := strings.LastIndexByte(image, '@')
	if at >= 0 {
		image = image[:at]
	}
	slash := strings.LastIndexByte(image, '/')
	colon := strings.LastIndexByte(image, ':')
	if colon > slash {
		return image[colon+1:]
	}
	return ""
}

// BuildLive is a convenience wrapper that constructs a Builder and calls Build.
// It is the preferred call-site for commands that already hold a *k8s.Client and
// a *capabilities.Reporter.
func BuildLive(ctx context.Context, c *k8s.Client, rep *capabilities.Reporter, o BuildOptions) (*Snapshot, error) {
	return NewBuilder(c, rep).Build(ctx, o)
}

// tolerable reports whether a list error should degrade to an empty list rather
// than abort the whole snapshot.
func tolerable(err error) bool {
	return apierrors.IsForbidden(err) || apierrors.IsNotFound(err) || apierrors.IsUnauthorized(err)
}

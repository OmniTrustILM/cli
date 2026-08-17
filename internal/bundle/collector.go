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

package bundle

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/k8s"
	"github.com/OmniTrustILM/cli/internal/version"
	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"
)

// operatorNamespace is where the operator Deployment lives by default.
const operatorNamespace = "ilm-operator-system"

// managedInfra pairs a foreign GVR with its bundle filename suffix.
type managedInfra struct {
	gvr  schema.GroupVersionResource
	kind string
}

var managedInfraKinds = []managedInfra{
	{k8s.GVRCNPGCluster, "cnpgcluster"},
	{k8s.GVRRabbitmqCluster, "rabbitmqcluster"},
	{k8s.GVRKeycloak, "keycloak"},
}

// Collector produces a support bundle from a live cluster.
type Collector struct {
	Client *k8s.Client
	Caps   *capabilities.Reporter
}

// NewCollector wires a Collector. Caps may be nil (capabilities.json is skipped).
func NewCollector(c *k8s.Client, caps *capabilities.Reporter) *Collector {
	return &Collector{Client: c, Caps: caps}
}

// Collect walks versions, configuration, state and logs into w, redacting
// Secret material by default and recording every RBAC-skipped item in the
// returned (and embedded) manifest. Forbidden/Unauthorized reads degrade
// gracefully; any other error is recorded as skipped and collection continues.
// The terminal manifest write and archive close are the only operations that
// can abort the entire run.
func (c *Collector) Collect(ctx context.Context, o CollectOptions, w io.Writer) (Manifest, error) {
	aw, err := newArchiveWriter(o.Format, w)
	if err != nil {
		return Manifest{}, err
	}
	m := NewManifest(version.Client().ClientVersion, o)
	red := NewRedactor()

	if err := c.collectVersions(ctx, aw, m); err != nil {
		return Manifest{}, err
	}
	if err := c.collectCapabilities(aw, m); err != nil {
		return Manifest{}, err
	}
	if err := c.collectClusterInfo(ctx, aw, m); err != nil {
		return Manifest{}, err
	}
	if err := c.collectConfigAndState(ctx, o, red, aw, m); err != nil {
		return Manifest{}, err
	}

	// manifest.json is written last so it reflects everything above.
	if err := writeJSON(aw, ManifestName, m); err != nil {
		return *m, err
	}
	if err := aw.Close(); err != nil {
		return *m, err
	}
	return *m, nil
}

func (c *Collector) collectVersions(_ context.Context, aw archiveWriter, m *Manifest) error {
	info := version.Client()
	_, err := c.record(m, "versions.json", func() error { return writeJSON(aw, "versions.json", info) })
	return err
}

func (c *Collector) collectCapabilities(aw archiveWriter, m *Manifest) error {
	if c.Caps == nil {
		return nil
	}
	_, err := c.record(m, "capabilities.json", func() error {
		return writeJSON(aw, "capabilities.json", c.Caps.Detect())
	})
	return err
}

func (c *Collector) collectClusterInfo(ctx context.Context, aw archiveWriter, m *Manifest) error {
	if err := c.collectServerVersion(ctx, aw, m); err != nil {
		return err
	}
	if err := c.collectNodes(ctx, aw, m); err != nil {
		return err
	}
	if err := c.collectCRDs(ctx, aw, m); err != nil {
		return err
	}
	return c.collectOperatorDeployment(ctx, aw, m)
}

func (c *Collector) collectServerVersion(_ context.Context, aw archiveWriter, m *Manifest) error {
	if c.Client.Discovery == nil {
		return nil
	}
	_, err := c.record(m, "cluster/info.json", func() error {
		v, err := c.Client.Discovery.ServerVersion()
		if err != nil {
			return err
		}
		return writeJSON(aw, "cluster/info.json", v)
	})
	return err
}

func (c *Collector) collectNodes(ctx context.Context, aw archiveWriter, m *Manifest) error {
	_, err := c.record(m, "cluster/nodes.json", func() error {
		var nodes corev1.NodeList
		if err := c.Client.Typed.List(ctx, &nodes); err != nil {
			return err
		}
		return writeJSON(aw, "cluster/nodes.json", nodes.Items)
	})
	return err
}

func (c *Collector) collectCRDs(ctx context.Context, aw archiveWriter, m *Manifest) error {
	_, err := c.record(m, "cluster/crds.json", func() error {
		var crds apiextv1.CustomResourceDefinitionList
		if err := c.Client.Typed.List(ctx, &crds); err != nil {
			return err
		}
		names := make([]string, 0, len(crds.Items))
		for i := range crds.Items {
			names = append(names, crds.Items[i].Name)
		}
		return writeJSON(aw, "cluster/crds.json", names)
	})
	return err
}

func (c *Collector) collectOperatorDeployment(ctx context.Context, aw archiveWriter, m *Manifest) error {
	_, err := c.record(m, "cluster/operator.yaml", func() error {
		dep, err := c.Client.OperatorDeployment(ctx, operatorNamespace)
		if apierrors.IsNotFound(err) {
			return nil // operator not in default namespace; nothing to record
		}
		if err != nil {
			return err
		}
		return writeCR(aw, "cluster/operator.yaml", dep, false, nil, c.Client.Scheme)
	})
	return err
}

func (c *Collector) collectConfigAndState(ctx context.Context, o CollectOptions, red *Redactor, aw archiveWriter, m *Manifest) error {
	for _, ns := range c.scopes(o) {
		if err := c.collectPlatforms(ctx, ns, o, red, aw, m); err != nil {
			return err
		}
		if err := c.collectConnectors(ctx, ns, o, aw, m); err != nil {
			return err
		}
		if err := c.collectProxies(ctx, ns, o, aw, m); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) collectPlatforms(ctx context.Context, ns string, o CollectOptions, red *Redactor, aw archiveWriter, m *Manifest) error {
	var list *otilmv1alpha1.PlatformList
	ok, err := c.record(m, fmt.Sprintf("config/platforms[%s]", ns), func() error {
		var e error
		list, e = c.Client.ListPlatforms(ctx, ns)
		return e
	})
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for i := range list.Items {
		if err := c.collectOnePlatform(ctx, &list.Items[i], o, red, aw, m); err != nil {
			return err
		}
	}
	return nil
}

// collectOnePlatform writes a single Platform's CR, events, managed infra,
// child Deployments, and (optionally) component logs into the archive.
func (c *Collector) collectOnePlatform(ctx context.Context, p *otilmv1alpha1.Platform, o CollectOptions, red *Redactor, aw archiveWriter, m *Manifest) error {
	base := fmt.Sprintf("%s_%s", p.Namespace, p.Name)
	if _, err := c.record(m, "config/platforms/"+base+".yaml", func() error {
		return writeCR(aw, "config/platforms/"+base+".yaml", p, o.Redact, red, c.Client.Scheme)
	}); err != nil {
		return err
	}
	if err := c.collectEvents(ctx, p, "state/events/platform_"+base+".json", aw, m); err != nil {
		return err
	}
	if err := c.collectManagedInfra(ctx, p.Namespace, base, aw, m); err != nil {
		return err
	}
	if err := c.collectPlatformDeployments(ctx, p.Namespace, p.Name, base, aw, m); err != nil {
		return err
	}
	if o.IncludeLogs {
		return c.collectPlatformLogs(ctx, p, o, aw, m)
	}
	return nil
}

func (c *Collector) collectConnectors(ctx context.Context, ns string, o CollectOptions, aw archiveWriter, m *Manifest) error { //nolint:dupl // mirrors collectProxies; differ only in type and paths
	var list *otilmv1alpha1.ConnectorList
	objs, err := c.listSecretlessResources(ctx, m, fmt.Sprintf("config/connectors[%s]", ns), func() error {
		var e error
		list, e = c.Client.ListConnectors(ctx, ns)
		return e
	})
	if err != nil || !objs {
		return err
	}
	for i := range list.Items {
		conn := &list.Items[i]
		if err := c.collectSecretlessResource(ctx, conn, "config/connectors", "state/events/connector", aw, m); err != nil {
			return err
		}
		if err := c.collectWorkloadLogs(ctx, o, conn.Namespace, conn.Name, workloadKindConnector, aw, m); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) collectProxies(ctx context.Context, ns string, o CollectOptions, aw archiveWriter, m *Manifest) error { //nolint:dupl // mirrors collectConnectors; differ only in type and paths
	var list *otilmv1alpha1.ProxyList
	objs, err := c.listSecretlessResources(ctx, m, fmt.Sprintf("config/proxies[%s]", ns), func() error {
		var e error
		list, e = c.Client.ListProxies(ctx, ns)
		return e
	})
	if err != nil || !objs {
		return err
	}
	for i := range list.Items {
		px := &list.Items[i]
		if err := c.collectSecretlessResource(ctx, px, "config/proxies", "state/events/proxy", aw, m); err != nil {
			return err
		}
		if err := c.collectWorkloadLogs(ctx, o, px.Namespace, px.Name, workloadKindProxy, aw, m); err != nil {
			return err
		}
	}
	return nil
}

// Workload kinds whose pods the collector tails; the kind also serves as the
// container-name hint for the workload's primary container.
const (
	workloadKindConnector = "connector"
	workloadKindProxy     = "proxy"
)

// Operator-applied pod label keys that select a single workload's pods.
const (
	connectorPodLabel = "otilm.com/connector"
	proxyPodLabel     = "otilm.com/proxy"
)

// workloadSelectorLabel maps a workload kind to the operator's unique pod label
// key for that kind.
var workloadSelectorLabel = map[string]string{
	workloadKindConnector: connectorPodLabel,
	workloadKindProxy:     proxyPodLabel,
}

// collectWorkloadLogs streams the pod logs of a connector or proxy into
// logs/<ns>_<kind>_<name>.log when IncludeLogs is set. The kind is "connector"
// or "proxy"; it selects the pod via the operator's unique otilm.com/<kind>
// label and is also the container-name hint's fallback (the resource name).
func (c *Collector) collectWorkloadLogs(ctx context.Context, o CollectOptions, ns, name, kind string, aw archiveWriter, m *Manifest) error {
	if !o.IncludeLogs {
		return nil
	}
	spec := podLogSpec{
		ns:            ns,
		sel:           map[string]string{workloadSelectorLabel[kind]: name},
		path:          fmt.Sprintf("logs/%s_%s_%s.log", ns, kind, name),
		containerHint: name,
		since:         logSinceSeconds(o.Since),
	}
	return c.collectPodLogs(ctx, spec, aw, m)
}

// listSecretlessResources is the common list-record preamble for secretless
// resource types (Connectors, Proxies). It returns (true, nil) when the list
// call succeeded, (false, nil) on an RBAC skip, and (false, err) on a fatal error.
func (c *Collector) listSecretlessResources(_ context.Context, m *Manifest, listPath string, listFn func() error) (bool, error) {
	return c.record(m, listPath, listFn)
}

// collectSecretlessResource serialises a single CR that carries no embedded
// secret material (Connectors and Proxies) and records its events.
func (c *Collector) collectSecretlessResource(ctx context.Context, obj ctrlclient.Object, configPrefix, eventPrefix string, aw archiveWriter, m *Manifest) error {
	base := fmt.Sprintf("%s_%s", obj.GetNamespace(), obj.GetName())
	cfgPath := configPrefix + "/" + base + ".yaml"
	if _, err := c.record(m, cfgPath, func() error {
		return writeCR(aw, cfgPath, obj, false, nil, c.Client.Scheme)
	}); err != nil {
		return err
	}
	return c.collectEvents(ctx, obj, eventPrefix+"_"+base+".json", aw, m)
}

func (c *Collector) collectEvents(ctx context.Context, obj ctrlclient.Object, path string, aw archiveWriter, m *Manifest) error {
	_, err := c.record(m, path, func() error {
		ev, err := c.Client.Events(ctx, obj.GetNamespace(), obj)
		if err != nil {
			return err
		}
		return writeJSON(aw, path, ev)
	})
	return err
}

func (c *Collector) collectManagedInfra(ctx context.Context, ns, base string, aw archiveWriter, m *Manifest) error {
	if c.Client.Dynamic == nil {
		return nil // dynamic client unavailable in test or minimal client; skip gracefully
	}
	for _, mi := range managedInfraKinds {
		path := fmt.Sprintf("state/managed-infra/%s_%s.json", base, mi.kind)
		gvr := mi.gvr
		if _, err := c.record(m, path, func() error {
			st, err := c.Client.ForeignStatus(ctx, gvr, ns, nameFromBase(base))
			if apierrors.IsNotFound(err) {
				return nil
			}
			if err != nil {
				return err
			}
			return writeJSON(aw, path, st)
		}); err != nil {
			return err
		}
	}
	return nil
}

// collectPlatformDeployments writes the Deployments owned by a Platform to
// state/workloads/platform_<ns>_<name>.json so the offline reader can populate
// rs.Deployments exactly as the live builder does via DeploymentsForPlatform.
func (c *Collector) collectPlatformDeployments(ctx context.Context, ns, name, base string, aw archiveWriter, m *Manifest) error {
	wPath := fmt.Sprintf("state/workloads/platform_%s.json", base)
	_, err := c.record(m, wPath, func() error {
		deps, err := c.Client.DeploymentsForPlatform(ctx, ns, name)
		if err != nil {
			return err
		}
		return writeJSON(aw, wPath, deps)
	})
	return err
}

func (c *Collector) collectPlatformLogs(ctx context.Context, p *otilmv1alpha1.Platform, o CollectOptions, aw archiveWriter, m *Manifest) error {
	since := logSinceSeconds(o.Since)
	for _, comp := range platformLogComponents {
		if err := c.collectComponentLogs(ctx, p, comp, since, aw, m); err != nil {
			return err
		}
	}
	return nil
}

// logSinceSeconds converts a duration to the *int64 sinceSeconds field expected
// by the Kubernetes pod logs API. Returns nil when d is zero (fetch all logs).
func logSinceSeconds(d time.Duration) *int64 {
	if d <= 0 {
		return nil
	}
	s := int64(d / time.Second)
	return &s
}

// podLogSpec describes one pod-log stream: which pod to select, where to write
// it, and which container to read from.
type podLogSpec struct {
	ns            string
	sel           map[string]string
	path          string
	containerHint string
	since         *int64
}

// collectComponentLogs fetches and writes logs for a single platform component
// pod, keeping the logs/<ns>_<platform>_<comp>.log path scheme.
func (c *Collector) collectComponentLogs(ctx context.Context, p *otilmv1alpha1.Platform, comp string, since *int64, aw archiveWriter, m *Manifest) error {
	return c.collectPodLogs(ctx, podLogSpec{
		ns: p.Namespace,
		sel: map[string]string{
			"app.kubernetes.io/instance":  p.Name,
			"app.kubernetes.io/component": comp,
		},
		path:          fmt.Sprintf("logs/%s_%s_%s.log", p.Namespace, p.Name, comp),
		containerHint: comp,
		since:         since,
	}, aw, m)
}

// collectPodLogs is the reusable log-fetch core: it locates the first pod
// matching spec.sel, picks a container (the one named spec.containerHint, else
// the pod's first), streams its logs and writes them to spec.path. When no pod
// matches it records nothing and returns nil. All reads route through record so
// RBAC denials degrade gracefully while other errors abort the run.
func (c *Collector) collectPodLogs(ctx context.Context, spec podLogSpec, aw archiveWriter, m *Manifest) error {
	_, err := c.record(m, spec.path, func() error {
		pods, err := c.Client.PodsFor(ctx, spec.ns, spec.sel)
		if err != nil {
			return err
		}
		if len(pods) == 0 {
			return nil // workload not present; nothing to record
		}
		// A multi-container pod (init/sidecar + app) requires an explicit
		// container: pick the one named after the hint, else the first.
		container := primaryContainer(&pods[0], spec.containerHint)
		rc, err := c.Client.PodLogs(ctx, spec.ns, pods[0].Name, container, &corev1.PodLogOptions{SinceSeconds: spec.since})
		if err != nil {
			return err
		}
		defer func() { _ = rc.Close() }()
		body, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		return aw.WriteFile(spec.path, body)
	})
	return err
}

// primaryContainer returns the container to read logs from: the one whose name
// matches the component, else the pod's first container. It returns "" only when
// the pod declares no containers.
func primaryContainer(pod *corev1.Pod, comp string) string {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == comp {
			return pod.Spec.Containers[i].Name
		}
	}
	if len(pod.Spec.Containers) > 0 {
		return pod.Spec.Containers[0].Name
	}
	return ""
}

// componentCore is the platform's core component (and container) name.
const componentCore = "core"

// platformLogComponents is the ordered set of ILM platform component names
// whose pods' logs the collector streams when IncludeLogs is set.
var platformLogComponents = []string{
	componentCore, "auth", "auth-opa-policies", "scheduler",
	"fe-administrator", "utils", "api-gateway", "provisioning-rabbitmq",
}

// scopes resolves the namespace list to iterate.
func (c *Collector) scopes(o CollectOptions) []string {
	if o.AllNamespaces || len(o.Namespaces) == 0 {
		return []string{""} // cluster scope: list with empty namespace
	}
	return o.Namespaces
}

// record runs fn and classifies the result. On a Forbidden or Unauthorized
// error it records the path as skipped and returns (false, nil) so the caller
// continues. On any other error it returns (false, err) — a fatal signal that
// the caller must propagate to abort Collect. On success it records the path
// in the manifest's Files list and returns (true, nil).
func (c *Collector) record(m *Manifest, path string, fn func() error) (ok bool, fatal error) {
	err := fn()
	if err == nil {
		m.AddFile(path)
		return true, nil
	}
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		m.Skip(path, err.Error())
		return false, nil
	}
	return false, err
}

func nameFromBase(base string) string {
	for i := len(base) - 1; i >= 0; i-- {
		if base[i] == '_' {
			return base[i+1:]
		}
	}
	return base
}

// --- archive abstraction -------------------------------------------------

type archiveWriter interface {
	WriteFile(path string, body []byte) error
	Close() error
}

func newArchiveWriter(f Format, w io.Writer) (archiveWriter, error) {
	switch f {
	case FormatTGZ:
		gz := gzip.NewWriter(w)
		return &tgzWriter{gz: gz, tw: tar.NewWriter(gz)}, nil
	case FormatZip, "":
		return &zipWriter{zw: zip.NewWriter(w)}, nil
	default:
		return nil, fmt.Errorf("unsupported bundle format %q", f)
	}
}

type zipWriter struct{ zw *zip.Writer }

func (z *zipWriter) WriteFile(path string, body []byte) error {
	f, err := z.zw.Create(path)
	if err != nil {
		return err
	}
	_, err = f.Write(body)
	return err
}
func (z *zipWriter) Close() error { return z.zw.Close() }

type tgzWriter struct {
	gz *gzip.Writer
	tw *tar.Writer
}

func (t *tgzWriter) WriteFile(path string, body []byte) error {
	hdr := &tar.Header{Name: path, Mode: 0o600, Size: int64(len(body)), ModTime: time.Now()}
	if err := t.tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := t.tw.Write(body)
	return err
}
func (t *tgzWriter) Close() error {
	if err := t.tw.Close(); err != nil {
		return err
	}
	return t.gz.Close()
}

func writeJSON(aw archiveWriter, path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return aw.WriteFile(path, b)
}

// writeCR marshals a CR to YAML, optionally redacting embedded secret material.
// It populates TypeMeta (kind/apiVersion) from the scheme so that the YAML
// output is complete even when the object was returned by a fake client that
// does not set TypeMeta.
func writeCR(aw archiveWriter, path string, obj ctrlclient.Object, redact bool, red *Redactor, scheme *runtime.Scheme) error {
	if scheme != nil {
		if gvk, err := apiutil.GVKForObject(obj, scheme); err == nil {
			obj.GetObjectKind().SetGroupVersionKind(gvk)
		}
	}
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	if redact {
		b = red.RedactYAML(b)
	}
	return aw.WriteFile(path, b)
}

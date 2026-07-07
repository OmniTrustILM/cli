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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	otilmv1alpha1 "github.com/OmniTrustILM/operator/api/v1alpha1"

	"github.com/OmniTrustILM/cli/internal/analyze"
	"github.com/OmniTrustILM/cli/internal/capabilities"
	"github.com/OmniTrustILM/cli/internal/version"
)

// errWrapFmt is the canonical error-wrapping format used throughout this file.
const errWrapFmt = "%s: %w"

// Read opens a collected bundle and reconstructs the analyze.Snapshot the live
// check produces, so DefaultRegistry yields identical findings offline.
//
// The archive format is inferred from the file extension (.zip or .tgz/.tar.gz).
// manifest.json is read first; if its schemaVersion does not match SchemaVersion
// an error is returned so future/past bundles are never silently mis-parsed.
func Read(p string) (*analyze.Snapshot, Manifest, error) {
	entries, err := openArchive(p)
	if err != nil {
		return nil, Manifest{}, err
	}

	m, err := parseManifest(entries)
	if err != nil {
		return nil, Manifest{}, err
	}

	snap := &analyze.Snapshot{
		ClientVersion:     m.ClientVersion,
		SupportedVersions: version.SupportedVersions(),
	}

	if err := populateSnapshotFromEntries(snap, entries, m); err != nil {
		return nil, m, err
	}
	applyOperatorFromDeployment(snap, entries)
	applyCapabilities(snap, entries)

	return snap, m, nil
}

// parseManifest reads and validates the manifest entry from the archive map.
func parseManifest(entries map[string][]byte) (Manifest, error) {
	raw, ok := entries[ManifestName]
	if !ok {
		return Manifest{}, fmt.Errorf("bundle missing %s", ManifestName)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf(errWrapFmt, "parse "+ManifestName, err)
	}
	if m.SchemaVersion != SchemaVersion {
		return m, fmt.Errorf("unsupported bundle schema %q (expected %q)", m.SchemaVersion, SchemaVersion)
	}
	return m, nil
}

// populateSnapshotFromEntries iterates archive entries and appends parsed
// resource snapshots to snap. It returns the first parse error encountered.
func populateSnapshotFromEntries(snap *analyze.Snapshot, entries map[string][]byte, m Manifest) error {
	for name, body := range entries {
		switch {
		case strings.HasPrefix(name, "config/platforms/") && strings.HasSuffix(name, ".yaml"):
			rs, err := platformSnapshot(body, entries)
			if err != nil {
				return fmt.Errorf(errWrapFmt, name, err)
			}
			snap.Platforms = append(snap.Platforms, rs)
		case strings.HasPrefix(name, "config/connectors/") && strings.HasSuffix(name, ".yaml"):
			rs, err := connectorSnapshot(body, entries)
			if err != nil {
				return fmt.Errorf(errWrapFmt, name, err)
			}
			snap.Connectors = append(snap.Connectors, rs)
		case strings.HasPrefix(name, "config/proxies/") && strings.HasSuffix(name, ".yaml"):
			rs, err := proxySnapshot(body, entries)
			if err != nil {
				return fmt.Errorf(errWrapFmt, name, err)
			}
			snap.Proxies = append(snap.Proxies, rs)
		}
	}
	_ = m // m is validated before this call; kept for potential future use
	return nil
}

// applyOperatorFromDeployment derives snap.OperatorReady and
// snap.OperatorVersion from cluster/operator.yaml exactly as the live builder
// does in fillOperator: ReadyReplicas >= 1 and the image tag of the first
// container. If the file is absent or unparseable both fields are left at
// their zero values (false / ""), matching the live builder's behaviour when
// the operator Deployment cannot be fetched.
func applyOperatorFromDeployment(snap *analyze.Snapshot, entries map[string][]byte) {
	raw, ok := entries["cluster/operator.yaml"]
	if !ok {
		return
	}
	var dep appsv1.Deployment
	if err := yaml.Unmarshal(raw, &dep); err != nil {
		return
	}
	snap.OperatorReady = dep.Status.ReadyReplicas >= 1
	if cs := dep.Spec.Template.Spec.Containers; len(cs) > 0 {
		snap.OperatorVersion = imageTag(cs[0].Image)
	}
}

// applyCapabilities reads the capabilities.json entry written by the collector
// and populates snap.Capabilities so the capabilityAnalyzer fires identically
// offline and live. When the entry is absent (Caps was nil at collect time) the
// field is left nil, matching the live builder's behaviour when Caps is nil.
func applyCapabilities(snap *analyze.Snapshot, entries map[string][]byte) {
	raw, ok := entries["capabilities.json"]
	if !ok {
		return
	}
	var results []capabilities.Result
	if err := json.Unmarshal(raw, &results); err != nil {
		return
	}
	snap.Capabilities = results
}

// imageTag returns the tag of an image reference, or "" when none is present.
// It replicates analyze.imageTag exactly so offline and live derivations are
// identical without coupling the two packages.
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

// platformSnapshot reconstructs a ResourceSnapshot from a serialised Platform
// CR, reusing the exported spec-extraction helpers from internal/analyze so the
// offline reader produces the same SpecModes/SecretRefs/IssuerRefs as the live
// builder.
func platformSnapshot(body []byte, entries map[string][]byte) (analyze.ResourceSnapshot, error) {
	var p otilmv1alpha1.Platform
	if err := yaml.Unmarshal(body, &p); err != nil {
		return analyze.ResourceSnapshot{}, err
	}
	secrets, issuers := analyze.PlatformRefs(&p)
	rs := baseSnapshot(analyze.GVKPlatform, p.Namespace, p.Name,
		string(p.Status.Phase), p.Generation, p.Status.ObservedGeneration,
		p.Status.Conditions)
	rs.ObservedVersion = p.Status.ObservedVersion
	rs.SpecModes = analyze.PlatformModes(&p)
	rs.SecretRefs = secrets
	rs.IssuerRefs = issuers
	rs.Events = eventsFor(entries, "platform_", p.Namespace, p.Name)
	rs.Logs = logsFor(entries, p.Namespace, p.Name)
	rs.Deployments = deploymentsFor(entries, "platform_", p.Namespace, p.Name)
	return rs, nil
}

// connectorSnapshot reconstructs a ResourceSnapshot from a serialised Connector CR.
func connectorSnapshot(body []byte, entries map[string][]byte) (analyze.ResourceSnapshot, error) {
	var c otilmv1alpha1.Connector
	if err := yaml.Unmarshal(body, &c); err != nil {
		return analyze.ResourceSnapshot{}, err
	}
	rs := baseSnapshot(analyze.GVKConnector, c.Namespace, c.Name,
		string(c.Status.Phase), c.Generation, c.Status.ObservedGeneration,
		c.Status.Conditions)
	rs.Events = eventsFor(entries, "connector_", c.Namespace, c.Name)
	rs.Logs = workloadLogsFor(entries, "connector", c.Namespace, c.Name)
	return rs, nil
}

// proxySnapshot reconstructs a ResourceSnapshot from a serialised Proxy CR.
func proxySnapshot(body []byte, entries map[string][]byte) (analyze.ResourceSnapshot, error) {
	var x otilmv1alpha1.Proxy
	if err := yaml.Unmarshal(body, &x); err != nil {
		return analyze.ResourceSnapshot{}, err
	}
	rs := baseSnapshot(analyze.GVKProxy, x.Namespace, x.Name,
		string(x.Status.Phase), x.Generation, x.Status.ObservedGeneration,
		x.Status.Conditions)
	rs.ObservedVersion = x.Status.ObservedVersion
	rs.SecretRefs = analyze.ProxyRefs(&x)
	rs.Events = eventsFor(entries, "proxy_", x.Namespace, x.Name)
	rs.Logs = workloadLogsFor(entries, "proxy", x.Namespace, x.Name)
	return rs, nil
}

// baseSnapshot creates a ResourceSnapshot with the fields shared by all three CR
// kinds, eliminating the duplication that would otherwise trip the dupl linter.
func baseSnapshot(gvk, ns, name, phase string, gen, observedGen int64, conds []metav1.Condition) analyze.ResourceSnapshot {
	return analyze.ResourceSnapshot{
		GVK:         gvk,
		Namespace:   ns,
		Name:        name,
		Phase:       phase,
		Generation:  gen,
		ObservedGen: observedGen,
		Conditions:  conds,
	}
}

// eventsFor looks up state/events/<prefix><ns>_<name>.json in the archive and
// deserialises the event list.  The kind prefix ("platform_", "connector_",
// "proxy_") matches the path the collector writes.
func eventsFor(entries map[string][]byte, prefix, ns, name string) []corev1.Event {
	key := fmt.Sprintf("state/events/%s%s_%s.json", prefix, ns, name)
	b, ok := entries[key]
	if !ok {
		return nil
	}
	var ev []corev1.Event
	if err := json.Unmarshal(b, &ev); err != nil {
		return nil
	}
	return ev
}

// deploymentsFor reads the workload Deployments written by the collector for a
// resource. The kind prefix ("platform_") matches the path written by
// collectPlatformDeployments: state/workloads/<prefix><ns>_<name>.json.
func deploymentsFor(entries map[string][]byte, prefix, ns, name string) []appsv1.Deployment {
	key := fmt.Sprintf("state/workloads/%s%s_%s.json", prefix, ns, name)
	b, ok := entries[key]
	if !ok {
		return nil
	}
	var deps []appsv1.Deployment
	if err := json.Unmarshal(b, &deps); err != nil {
		return nil
	}
	return deps
}

// workloadLogsFor looks up the single logs/<ns>_<kind>_<name>.log entry the
// collector writes for a Connector or Proxy and returns it keyed by the
// resource name, so the offline logsig analyzer scans it exactly as it does
// Platform component logs. Returns nil when no log entry was collected.
func workloadLogsFor(entries map[string][]byte, kind, ns, name string) map[string]string {
	key := fmt.Sprintf("logs/%s_%s_%s.log", ns, kind, name)
	b, ok := entries[key]
	if !ok {
		return nil
	}
	return map[string]string{name: string(b)}
}

// logsFor scans the archive for logs/<ns>_<name>_<component>.log entries that
// belong to the named Platform and returns them keyed by component.  Only
// Platform resources have logs; Connector and Proxy skip this call.
func logsFor(entries map[string][]byte, ns, name string) map[string]string {
	prefix := fmt.Sprintf("logs/%s_%s_", ns, name)
	out := map[string]string{}
	for k, v := range entries {
		if strings.HasPrefix(k, prefix) && strings.HasSuffix(k, ".log") {
			comp := strings.TrimSuffix(strings.TrimPrefix(k, prefix), ".log")
			out[comp] = string(v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// openArchive reads every file entry from a zip or tgz bundle into a path-keyed
// map.  The format is inferred from the extension; a gzip magic-byte sniff is
// used as a fallback for extensionless files.
func openArchive(p string) (map[string][]byte, error) {
	raw, err := os.ReadFile(p) //nolint:gosec // p is a user-supplied bundle path; intentional.
	if err != nil {
		return nil, err
	}
	switch {
	case strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".tar.gz"):
		return readTGZ(raw)
	case strings.HasSuffix(p, ".zip"):
		return readZipBytes(raw)
	default:
		// Sniff: gzip magic bytes 0x1f 0x8b => tgz, else attempt zip.
		if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
			return readTGZ(raw)
		}
		return readZipBytes(raw)
	}
}

func readZipBytes(raw []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		out[path.Clean(f.Name)] = b
	}
	return out, nil
}

func readTGZ(raw []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		out[path.Clean(hdr.Name)] = b
	}
	return out, nil
}

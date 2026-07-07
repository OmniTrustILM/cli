# Architecture

## Two-layer client model

The CLI is designed around two distinct layers. Only the first is implemented in
Phase 1.

**Kubernetes layer (Phase 1):** talks to the Kubernetes API server. Reads and writes
`otilm.com/v1alpha1` custom resources (`Platform`, `Connector`, `Proxy`) and reads
the workloads and managed infrastructure the operator created. Authentication is the
kubeconfig; authorization is cluster RBAC.

**ILM Core layer (Phase 2/3, designed only):** will talk to the Core REST API via a
generated Go SDK client with a pluggable transport (OAuth2 bearer JWT or edge mTLS).
Out of scope for Phase 1; no ILM Core endpoints are contacted.

## Dual-invocation

One binary is published under two names: `ilmctl` (standalone) and `kubectl-ilm`
(kubectl plugin). At startup, the binary checks `os.Args[0]` and adjusts the root
command `Use` and help text accordingly. Behaviour is otherwise identical. The
kubectl plugin does not inherit a client from the `kubectl` process — it resolves the
kubeconfig itself using the same logic as `ilmctl`.

## Package map

```
cmd/ilmctl/main.go          Dual-invocation entrypoint (os.Args[0] dispatch)
internal/
  cli/                      Root command, help-groups, global flags, exit codes
  cmd/
    infra/                  init, upgrade, uninstall, status, check
    deps/                   deps check, deps install
    platform/               platform get/describe/status/events/wait/logs/
                              generate/migrate/apply/edit/upgrade/delete/credentials
    connector/              connector get/describe/status/events/wait/logs/generate
    proxy/                  proxy get/describe/status/events/wait/logs/generate
    diag/                   diagnostics (collect), diagnostics analyze
    shared/                 cross-command flag/wait helpers
  k8s/                      rest.Config, scheme, typed + dynamic + discovery client
  capabilities/             Upstream-operator detection (cert-manager/CNPG/RabbitMQ/
                              Keycloak/Gateway API/ServiceMonitor)
  health/                   Phase and condition interpretation; KnownConditions catalog
  analyze/                  Analyzer engine: snapshot → findings
                              (shared by `check` + `diagnostics analyze`)
  render/                   kubectl-identical printers, human tables, colour/TTY
  manifest/                 Operator-manifest source resolution, ordered SSA applier,
                              OLM method, upstream-dep catalog
  generate/                 CR scaffolding, profiles, effective-value comments,
                              values2platform migration
  bundle/                   Support-bundle collection, redaction, versioned manifest.json
  version/                  Client/operator/platform versions, BOM compat
  config/                   ilmctl context file (reserved; no writes in Phase 1)
```

## Kubernetes client

- `*rest.Config` is built from `genericclioptions.ConfigFlags` so `--kubeconfig`,
  `--context`, `--cluster`, `--user`, and all other kubectl-standard flags behave
  identically to kubectl.
- A typed controller-runtime client covers `otilm.com/v1alpha1`, `corev1`, `appsv1`,
  `policy/v1`, and `networking/v1`.
- Foreign managed-infra CRs (CloudNativePG `Cluster`, RabbitMQ `RabbitmqCluster`,
  Keycloak `Keycloak`) are read via the dynamic client (unstructured, status only),
  so the CLI takes no compile-time dependency on those projects' modules.

## Analyzer engine

`internal/analyze` consumes a **snapshot** (operator state, conditions, phases,
events, workload health, capability results) and emits ordered **findings**
`{severity, title, evidence, remediation, docsURL}`. The engine is data-driven: it
iterates over whatever conditions the operator publishes; named rules add curated
severity and remediation for known condition types (`DatabaseReady`, `MessagingReady`,
etc.). Two snapshot sources feed the same engine:

- **Live** — `internal/k8s` reads the cluster (used by `check` and `status`).
- **Bundle** — `internal/bundle` loads a collected ZIP (used by `diagnostics analyze`).

This guarantees that a live diagnosis and a support engineer's offline bundle analysis
produce identical findings.

## Bundle determinism

Collected bundles carry a versioned `manifest.json` (`schemaVersion: "1"`) that
lists every file, the collection options, the redaction state, and every item skipped
due to insufficient RBAC. Bundles are deterministic: the same cluster state and the
same flags always produce the same bundle content.

## Server-side apply

`init`, `upgrade`, `platform apply`, and `platform edit` use server-side apply with
field manager `ilmctl`. CRDs are applied first, the controller waits for all CRDs to
reach `Established`, then the controller manifest is applied. This ordering ensures a
clean install even on a bare cluster.

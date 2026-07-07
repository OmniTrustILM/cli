# ilmctl / kubectl ilm

`ilmctl` is the command-line interface for OmniTrust ILM — a cloud-native platform
for the lifecycle of certificates, keys, secrets and related cryptographic assets.
The same binary installs as the standalone `ilmctl` and as the kubectl plugin
`kubectl-ilm`; behaviour and context resolution are identical in both modes.

## What Phase 1 covers

Phase 1 is **Kubernetes/operator-only**. It installs and operates the ILM operator
(`otilm.com/v1alpha1`), generates and inspects `Platform`, `Connector` and `Proxy`
custom resources, and diagnoses installs. It does **not** contact the ILM Core REST
API (that arrives in a later phase). Everything runs through the Kubernetes API
server; authentication is the kubeconfig; authorization is cluster RBAC.

## Install

| Channel | Command |
|---|---|
| Go | `go install github.com/OmniTrustILM/cli/cmd/ilmctl@latest` |
| Homebrew tap | `brew install OmniTrustILM/tap/ilmctl` |
| Scoop bucket | `scoop bucket add ilm https://github.com/OmniTrustILM/scoop-bucket && scoop install ilmctl` |
| Binary (signed) | Download from [Releases](https://github.com/OmniTrustILM/cli/releases), verify the checksum and cosign signature |
| kubectl plugin | Place `kubectl-ilm` on `$PATH`; kubectl auto-discovers it as `kubectl ilm` |
| Container | `docker run --rm -v ~/.kube:/home/nonroot/.kube hub.omnitrustregistry.com/ilm/cli:latest version` |

`.deb`/`.rpm` packages and a custom krew index are published under the OmniTrust
namespace. Public indexes (homebrew-core, krew-index) are held pending the trademark
question.

## Documentation map

| Guide | Contents |
|---|---|
| **[Quickstart](quickstart.md)** | Bootstrap a cluster end to end |
| **[Command reference](commands/ilmctl.md)** | Every command and flag (auto-generated) |
| **[Configuration](configuration.md)** | Kubeconfig resolution, context file, output formats, exit codes |
| **[GitOps](gitops.md)** | `generate`/`--dry-run`/apply workflow, server-side apply, field managers |
| **[Upgrades](upgrades.md)** | Operator vs platform upgrade, forward-only invariant, managed-infra guards |
| **[Troubleshooting](troubleshooting.md)** | `check`/`status`/`describe`/`events`/`logs`, common failure table |
| **[Diagnostics](diagnostics.md)** | Support bundles, redaction, offline analysis |
| **[Architecture](architecture.md)** | Two-layer model, dual-invocation, package map |

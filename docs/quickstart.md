# Quickstart

## Use an already-running ILM (no install step)

Point your kubeconfig at the cluster. The CLI discovers the operator and custom
resources from the API server; no local install is required.

```console
$ ilmctl version              # client + operator + platform versions (compat check)
$ ilmctl status -A            # operator, platforms, managed infra, connectors, proxies
$ ilmctl check                # diagnose the running install
$ ilmctl platform get         # list platforms, then describe / logs / events as needed
```

## Bootstrap a fresh cluster

```console
# 1. Check prerequisites and which upstream operators the intended modes need.
$ ilmctl check --pre

# 2. Install upstream operators (opt-in; alternatively pass --with-deps to init).
$ ilmctl deps install --only cert-manager,cnpg

# 3. Install the ILM operator.
#    Default: latest published release (CRDs applied first, then the controller).
$ ilmctl init --version v2.18.0 --wait
#    Operator unreleased? Install from a commit:
$ ilmctl init --ref <commit-sha> --wait

# 4. Generate a Platform CR.
$ ilmctl platform generate \
    --profile managed-ha \
    --db-mode managed \
    --messaging-mode managed \
    --broker-type rabbitmq \
    --keycloak-mode managed \
    > platform.yaml

# 5. Review the file, commit it to Git, then apply.
$ kubectl apply -f platform.yaml
#    Or combine steps 4-5:
$ ilmctl platform generate --profile managed-ha --db-mode managed --apply

# 6. Wait for the platform to become available.
$ ilmctl platform wait ilm --for=condition=Available --timeout 10m

# 7. Verify.
$ ilmctl status
```

## What to read next

- [Configuration](configuration.md) — flags, environment variables, output formats.
- [GitOps](gitops.md) — the generate→commit→sync workflow with Argo/Flux.
- [Upgrades](upgrades.md) — forward-only operator and platform upgrades.
- [Troubleshooting](troubleshooting.md) — `check`, `status`, logs and events.

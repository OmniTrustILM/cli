# ilmctl / kubectl ilm

The command-line interface for OmniTrust ILM — a cloud-native platform for the
lifecycle of certificates, keys, secrets and related cryptographic assets.
One binary, two names (`ilmctl` and `kubectl-ilm`), kubectl-grade UX.

```console
go install github.com/OmniTrustILM/cli/cmd/ilmctl@latest
```

## Documentation

- **[Overview & install](docs/index.md)** — install channels, documentation map.
- **[Quickstart](docs/quickstart.md)** — bootstrap a cluster end to end.
- **[Command reference](docs/commands/ilmctl.md)** — every command and flag (auto-generated).
- **[Configuration](docs/configuration.md)** — kubeconfig, context file, output formats, exit codes.
- **[GitOps](docs/gitops.md)** — generate/`--dry-run`/apply workflow, server-side apply.
- **[Upgrades](docs/upgrades.md)** — forward-only operator and platform upgrades.
- **[Troubleshooting](docs/troubleshooting.md)** — `check`, `status`, logs, events, common failures.
- **[Diagnostics](docs/diagnostics.md)** — support bundles, redaction, offline analysis.
- **[Architecture](docs/architecture.md)** — two-layer model, dual-invocation, package map.

## Contributing

Contributions are welcome. All contributors sign the CLA, handled automatically
by cla-assistant on your first pull request. Maintainers cutting a release
follow [RELEASE.md](RELEASE.md).

## License

[MIT](LICENSE)

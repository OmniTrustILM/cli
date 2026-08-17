# ilmctl / kubectl ilm

The command-line interface for OmniTrust ILM — a cloud-native platform for the
lifecycle of certificates, keys, secrets and related cryptographic assets.
One binary, two names (`ilmctl` and `kubectl-ilm`), kubectl-grade UX.

```console
go install github.com/OmniTrustILM/cli/cmd/ilmctl@latest
```

## Documentation

- **[Overview](docs/site/index.md)** — what `ilmctl` is and where each guide lives.
- **[Quickstart](docs/site/quickstart.md)** — install `ilmctl` and bootstrap a cluster end to end.
- **[Command reference](docs/commands/ilmctl.md)** — every command and flag (auto-generated).
- **[Configuration](docs/site/configuration.md)** — kubeconfig, context file, output formats, exit codes.
- **[GitOps](docs/site/gitops.md)** — generate/`--dry-run`/apply workflow, server-side apply.
- **[Upgrades](docs/site/upgrades.md)** — forward-only operator and platform upgrades.
- **[Troubleshooting](docs/site/troubleshooting.md)** — `check`, `status`, logs, events, common failures.
- **[Diagnostics](docs/site/diagnostics.md)** — support bundles, redaction, offline analysis.
- **[Architecture](docs/architecture.md)** — two-layer model, dual-invocation, package map.

## Contributing

Contributions are welcome. All contributors sign the CLA, handled automatically
by cla-assistant on your first pull request. Maintainers cutting a release
follow [RELEASE.md](RELEASE.md).

The eight guide pages enumerated in [RELEASE.md](RELEASE.md) —
`docs/site/{index,quickstart,configuration,gitops,upgrades,troubleshooting,diagnostics,commands}.md`
— are synced into https://docs.otilm.com. Each carries `sidebar_position` front matter,
and **links may only be same-directory relative links** (`./configuration.md`, `#anchor`)
— every other target is an absolute URL. `docs/site/commands.md` is generated; edit the
command help text, not the page. Everything under `docs/` outside `docs/site/`
(architecture notes, the per-command tree) is GitHub-only.

## License

[MIT](LICENSE)

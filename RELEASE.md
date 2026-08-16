# Releasing ilmctl

The repeatable runbook for cutting an `ilmctl` release. A release is a **tag** —
there is no release branch, and nothing on `main` carries a version number.

## Development model

`main` is the development line and is always releasable: every PR runs the same
`make verify` gate CI runs, and the version a build reports comes from ldflags,
not from a file in the tree. An untagged build calls itself `dev`.

Three version lines meet in this repo and none of them move together:

| Line | Where it lives | Example |
|---|---|---|
| `ilmctl` | the git tag on this repo | `v1.0.0` |
| The operator | the `github.com/OmniTrustILM/operator` pin in `go.mod`, and the manifests `ilmctl init` applies | `v1.0.0` |
| The ILM platform | the operator's BOM (`pkg/bom`), read through the pinned module | `2.19.0` |

The operator pin is an **exact release tag** — never `@latest`, never a pseudo-version
once a release exists, and never a committed `replace` directive. `ilmctl` reads the
supported platform versions out of the pinned module rather than duplicating them, so
bumping the pin is how this repo learns about new platform versions.

## Release sequence

Run everything on a clean, up-to-date `main`.

### 1. Pin the operator release the build is made against

```sh
go get github.com/OmniTrustILM/operator@vX.Y.Z    # an exact tag, never @latest
go mod tidy
```

### 2. Refresh what the change touched

- `make docs` — `docs/commands/` is a **committed** generated artifact; regenerate it
  whenever command help text changed, and commit the result.
- Version examples in the prose docs: [`docs/quickstart.md`](docs/quickstart.md) and
  [`docs/upgrades.md`](docs/upgrades.md) name a concrete operator release in their
  `ilmctl init` / `ilmctl upgrade` examples.
- The operator tag named in [`CLAUDE.md`](CLAUDE.md), which states the current pin.
- `.krew.yaml` is a structural fixture (`go test ./hack/...` validates its shape). The
  krew manifest that ships is rendered by GoReleaser at tag time; the committed file is
  not a release artifact.

### 3. Gate

```sh
make verify    # go mod tidy, fmt, vet, golangci-lint, tests, 80% coverage
```

Optionally rehearse the pipeline itself. This builds the archives, packages and
checksums into `dist/` and renders the same package manifests a real release does —
publishing and signing nothing, and skipping the chocolatey pipe exactly as the release
workflow does, so what you see locally is the set a tag produces:

```sh
make release-snapshot
```

### 4. Tag

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

The tag drives everything: it is the version ldflags inject, the archive names, and the
GitHub release. GoReleaser is configured with `prerelease: auto`, so a plain `vX.Y.Z`
publishes a full release, while a tag carrying a pre-release suffix — `v1.1.0-rc.1`,
`v1.1.0-beta.1` — is marked as a pre-release on GitHub.

**Never move a published tag.** The Go module proxy caches a tag's content
permanently, so re-pointing one leaves `go install …@vX.Y.Z` serving the old code
forever. Fix forward with the next patch tag instead.

## What the tag publishes

The **Release** workflow (`.github/workflows/release.yml`) runs GoReleaser, which
attaches to the GitHub release:

| Asset | Detail |
|---|---|
| `ilmctl_<version>_<os>_<arch>.tar.gz` / `.zip` | linux, darwin, windows × amd64, arm64 (zip on windows); carries `LICENSE`, `README.md` and `docs/` |
| `kubectl-ilm_<version>_<os>_<arch>.tar.gz` / `.zip` | the same build published under the plugin name |
| `ilmctl_<version>_linux_<arch>.deb` / `.rpm` | nfpm packages, installing to `/usr/bin` |
| `checksums.txt` | sha256 over the published archives and packages |
| `checksums.txt.sig` | detached cosign signature over `checksums.txt` — the signature covers the checksum file, not each archive individually |
| `<archive>.sbom.json` | a syft SBOM per archive |

Release notes are GoReleaser's generated changelog for the commits since the previous tag.

The job needs the `COSIGN_PRIVATE_KEY` and `COSIGN_PASSWORD` secrets; signing uses
cosign v3, pinned in the workflow to the release its `sign-blob` flags were verified
against.

Package-manager publishing is **held**, in two different ways:

- The Homebrew tap, scoop bucket and krew index repositories do not exist yet, so those
  three hold their uploads (`skip_upload`). Their manifests are still rendered into
  `dist/`, so the shape stays reviewable.
- Chocolatey is **skipped outright** — by the release workflow and by
  `make release-snapshot` alike — because its pipe shells out to a `choco` binary no
  Linux runner carries, and there is no chocolatey channel to publish to yet. It
  produces nothing, locally or on a tag.

See the comments in [`.goreleaser.yaml`](.goreleaser.yaml) for what to flip once the
targets exist.

The container image is published separately, by the **Publish Docker image** workflow,
which triggers on the same tag.

## After the release

```console
# 1. The module is installable at the tag.
$ go install github.com/OmniTrustILM/cli/cmd/ilmctl@vX.Y.Z
$ ilmctl version
Client Version: vX.Y.Z

# 2. The downloaded artifacts match the signed checksum file. The key is the
#    organisation's cosign public key, the pair of the release signing secret.
$ cosign verify-blob --key cosign.pub --signature checksums.txt.sig checksums.txt
$ sha256sum -c checksums.txt --ignore-missing

# 3. The install path works end to end on a fresh cluster. The tag below is an
#    OPERATOR release — ilmctl's own version is a separate line.
$ ilmctl init --version v1.0.0 --wait
Using operator release v1.0.0 (checksums verified).
$ ilmctl status
```

Step 3 is the one that matters most: it is the only check that exercises the release
manifests, their `checksums.txt` and the ordered apply against a real API server. Run it
against a throwaway cluster (`kind create cluster`) before announcing anything.

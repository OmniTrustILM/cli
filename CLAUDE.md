# CLAUDE.md — conventions for AI-assisted development in this repo

## Module

```
github.com/OmniTrustILM/cli   go 1.26
```

`ilmctl` depends on the operator as an ordinary published Go module, pinned in
`go.mod` to an operator release tag (`v1.0.0` today) and bumped to an exact tag
— never `@latest` — as part of preparing each ilmctl release (see
[RELEASE.md](RELEASE.md)). There is **no `replace` directive committed** — do
not add one. To develop
against a local operator checkout, use a git-ignored `go.work` (or a temporary
local `replace` you never commit), not an edit to `go.mod`.

## License header

Every `.go` file must begin with the MIT copyright block from
`hack/boilerplate.go.txt`. No exceptions. controller-gen and code generators
are configured to prepend it automatically; hand-written files must include it
manually.

## Test-driven development

Write the failing test first, then the implementation. All new packages need at
least one `_test.go` file before the implementation lands.

## Before committing

```sh
make verify    # runs go vet, staticcheck, golangci-lint, and go test ./...
```

CI enforces the same checks; a failing `make verify` will block the PR.

## Operator dependency

`ilmctl` consumes public packages from the operator module
(`api/v1alpha1`, `pkg/...`). The operator is an independent project;
changes to the operator are not driven by CLI requirements.

## Directory layout

```
cmd/ilmctl/          standalone binary entry-point
cmd/kubectl-ilm/     kubectl plugin entry-point
internal/            non-exported packages (buildinfo, etc.)
hack/                tooling scripts and boilerplate
docs/                user guides + generated command reference (make docs)
```

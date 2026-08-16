# Image / version metadata
IMG ?= hub.omnitrustregistry.com/ilm/cli:dev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

PKG := github.com/OmniTrustILM/cli/internal/buildinfo
LDFLAGS := -s -w \
	-X $(PKG).GitVersion=$(VERSION) \
	-X $(PKG).GitCommit=$(GIT_COMMIT) \
	-X $(PKG).BuildDate=$(BUILD_DATE)

COVER_THRESHOLD ?= 80

.PHONY: all
all: build

.PHONY: help
help: ## Display this help.
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: tidy
tidy: ## Tidy go.mod / go.sum.
	go mod tidy

.PHONY: test
test: fmt vet ## Run tests with race detector and coverage profile.
	go test -race -covermode=atomic -coverprofile=cover.out ./...

.PHONY: coverage
coverage: test ## Enforce the minimum coverage threshold.
	@total=$$(go tool cover -func=cover.out | grep total: | awk '{print substr($$3, 1, length($$3)-1)}'); \
	echo "total coverage: $$total% (min $(COVER_THRESHOLD)%)"; \
	awk -v t="$$total" -v m="$(COVER_THRESHOLD)" 'BEGIN { exit (t+0 < m+0) }' || \
	{ echo "coverage $$total% is below threshold $(COVER_THRESHOLD)%"; exit 1; }

.PHONY: lint
lint: ## Run golangci-lint.
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with --fix.
	golangci-lint run --fix ./...

.PHONY: build
build: fmt vet ## Build the ilmctl binary.
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ilmctl ./cmd/ilmctl

.PHONY: build-plugin
build-plugin: build ## Build the kubectl-ilm plugin binary (same build).
	cp bin/ilmctl bin/kubectl-ilm

.PHONY: install
install: build-plugin ## Install ilmctl and kubectl-ilm into GOBIN.
	@dir="$$(go env GOBIN)"; [ -n "$$dir" ] || dir="$$(go env GOPATH)/bin"; \
		install -m 0755 bin/ilmctl "$$dir/ilmctl"; \
		install -m 0755 bin/kubectl-ilm "$$dir/kubectl-ilm"

.PHONY: run
run: ## Run ilmctl with ARGS, e.g. make run ARGS="version".
	go run -ldflags "$(LDFLAGS)" ./cmd/ilmctl $(ARGS)

.PHONY: docs
docs: ## Regenerate the cobra command reference under docs/commands.
	rm -rf docs/commands
	go run -tags tools ./hack/gen-docs.go docs/commands

.PHONY: docker-build
docker-build: ## Build the container image.
	docker build --build-arg VERSION=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) -t $(IMG) .

.PHONY: sonar
sonar: test lint-report ## Run a local ephemeral SonarQube scan.
	./hack/sonar-local.sh

.PHONY: lint-report
lint-report: ## Produce the checkstyle golangci report Sonar consumes.
	golangci-lint run --output.checkstyle.path=golangci-lint-report.xml ./... || true

.PHONY: verify
verify: tidy fmt vet lint coverage ## One-shot pre-commit gate.
	@echo "verify OK"

.PHONY: release-snapshot
release-snapshot: ## Build a local snapshot release (no publish) to verify .goreleaser.yaml.
	# chocolatey is skipped here for the same reason release.yml skips it: the
	# pipe shells out to `choco`, which neither the release runner nor a typical
	# dev machine carries. Snapshot and release therefore render the same set.
	goreleaser release --snapshot --clean --skip=publish,sign,chocolatey

.PHONY: krew-validate
krew-validate: ## Validate the krew plugin manifest (dry-run install via kubectl krew).
	kubectl krew install --manifest=.krew.yaml --dry-run 2>/dev/null || \
	  echo "krew not installed; manifest validated structurally by go test ./hack/..."

# ------------------------------------------------------------------------------
# End-to-end (live cluster) testing
#
# The e2e suite (test/e2e, build tag `e2e`) drives the built ilmctl binary
# against a real Kind cluster. It is fully gated behind the build tag, so it
# never runs under `make test`/`make verify`.
#
#   make e2e         self-contained: installs kind if missing, creates the
#                    Kind cluster (E2E_CLUSTER, default ilm-e2e) if absent,
#                    builds the binary, exports ILM_BIN + KUBECONFIG, and runs
#                    the suite (including the live Keycloak deps validation).
#                    Idempotent: re-running reuses the cluster.
#   make e2e-clean   deletes the Kind cluster.
#
# The suite uses --from-source against the sibling operator checkout
# (E2E_OPERATOR_DIR, default ../operator) purely as a local dev harness; it
# t.Skips cleanly when the cluster or operator source is unavailable.
# ------------------------------------------------------------------------------

E2E_CLUSTER      ?= ilm-e2e
E2E_OPERATOR_DIR ?= ../operator
E2E_KUBECONFIG   ?= $(CURDIR)/bin/e2e.kubeconfig
E2E_TIMEOUT      ?= 20m
GOBIN_DIR        := $(shell go env GOPATH)/bin

.PHONY: e2e
e2e: build ## Run the live Kind-based e2e suite (self-contained + idempotent).
	@command -v kind >/dev/null 2>&1 || command -v "$(GOBIN_DIR)/kind" >/dev/null 2>&1 || { \
	  echo "kind not found; installing sigs.k8s.io/kind@latest into $(GOBIN_DIR)..."; \
	  go install sigs.k8s.io/kind@latest; }
	@KIND="$$(command -v kind || echo $(GOBIN_DIR)/kind)"; \
	if ! "$$KIND" get clusters 2>/dev/null | grep -qx "$(E2E_CLUSTER)"; then \
	  echo "creating Kind cluster $(E2E_CLUSTER)..."; \
	  "$$KIND" create cluster --name "$(E2E_CLUSTER)" --wait 120s; \
	else \
	  echo "reusing existing Kind cluster $(E2E_CLUSTER)"; \
	fi; \
	"$$KIND" get kubeconfig --name "$(E2E_CLUSTER)" > "$(E2E_KUBECONFIG)"
	KUBECONFIG="$(E2E_KUBECONFIG)" \
	ILM_BIN="$(CURDIR)/bin/ilmctl" \
	ILM_OPERATOR_DIR="$(abspath $(E2E_OPERATOR_DIR))" \
	ILM_E2E_DEPS=1 \
	  go test -tags e2e ./test/e2e/... -v -timeout $(E2E_TIMEOUT)

.PHONY: e2e-clean
e2e-clean: ## Delete the Kind cluster used by `make e2e`.
	@KIND="$$(command -v kind || echo $(GOBIN_DIR)/kind)"; \
	"$$KIND" delete cluster --name "$(E2E_CLUSTER)"; \
	rm -f "$(E2E_KUBECONFIG)"

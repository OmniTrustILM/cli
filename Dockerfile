########################
# Build Stage
########################
FROM golang:1.26 AS builder

ARG VERSION=dev
ARG GIT_COMMIT=none
ARG BUILD_DATE=unknown
ENV CGO_ENABLED=0 GOFLAGS="-trimpath"

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "-s -w \
      -X github.com/OmniTrustILM/cli/internal/buildinfo.GitVersion=${VERSION} \
      -X github.com/OmniTrustILM/cli/internal/buildinfo.GitCommit=${GIT_COMMIT} \
      -X github.com/OmniTrustILM/cli/internal/buildinfo.BuildDate=${BUILD_DATE}" \
      -o /out/ilmctl ./cmd/ilmctl

########################
# Run Stage
########################
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.title="ilmctl" \
      org.opencontainers.image.source="https://github.com/OmniTrustILM/cli"

COPY --from=builder /out/ilmctl /ilmctl
USER 65532:65532
ENTRYPOINT ["/ilmctl"]

# Build the manager binary
FROM golang:1.25.8-alpine3.23 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags="-s -w \
      -X github.com/vyrodovalexey/k8s-postgresql-operator/internal/version.Version=${VERSION} \
      -X github.com/vyrodovalexey/k8s-postgresql-operator/internal/version.GitCommit=${GIT_COMMIT} \
      -X github.com/vyrodovalexey/k8s-postgresql-operator/internal/version.BuildTime=${BUILD_TIME}" \
    -o manager github.com/vyrodovalexey/k8s-postgresql-operator/cmd

# Runtime image
FROM alpine:3.23


LABEL org.opencontainers.image.title="k8s-postgresql-operator" \
      org.opencontainers.image.description="Kubernetes operator for managing PostgreSQL resources" \
      org.opencontainers.image.url="https://github.com/vyrodovalexey/k8s-postgresql-operator" \
      org.opencontainers.image.source="https://github.com/vyrodovalexey/k8s-postgresql-operator" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="vyrodovalexey"

RUN apk update \
    && apk upgrade --ignore alpine-baselayout \
    && apk add --no-cache ca-certificates curl jq strace bash tar tzdata wget \
    && addgroup -g 1001 -S web-data \
    && adduser -u 1001 -D -S -G web-data --home /app web-data \
    && chown -R web-data:web-data /app

USER 1001
WORKDIR /app
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/app/manager"]

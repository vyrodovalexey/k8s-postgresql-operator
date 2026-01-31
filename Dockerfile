# Build the manager binary
FROM golang:1.25.6-alpine3.23 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
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
    go build -a -ldflags="-s -w" -trimpath -o manager github.com/vyrodovalexey/k8s-postgresql-operator/cmd

# Final stage - minimal runtime image
FROM alpine:3.23

# OCI Labels
LABEL org.opencontainers.image.title="k8s-postgresql-operator" \
      org.opencontainers.image.description="Kubernetes operator for managing PostgreSQL databases" \
      org.opencontainers.image.url="https://github.com/vyrodovalexey/k8s-postgresql-operator" \
      org.opencontainers.image.source="https://github.com/vyrodovalexey/k8s-postgresql-operator" \
      org.opencontainers.image.vendor="vyrodovalexey" \
      org.opencontainers.image.licenses="Apache-2.0"

# Install only essential runtime dependencies (sorted alphanumerically)
# ca-certificates: Required for TLS connections
# tzdata: Required for timezone support
RUN apk update --no-cache \
    && apk upgrade --no-cache --ignore alpine-baselayout \
    && apk add --no-cache ca-certificates tzdata \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -D -S -G nonroot -H -h /app nonroot \
    && rm -rf /var/cache/apk/*

# Use non-root user (65532 is standard for distroless compatibility)
USER 65532:65532
WORKDIR /app

# Copy the binary from builder
COPY --from=builder --chown=65532:65532 /workspace/manager .

# Note: Health checks are handled by Kubernetes liveness/readiness probes
# defined in the Helm chart deployment template

ENTRYPOINT ["/app/manager"]

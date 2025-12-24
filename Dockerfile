# Build the manager binary
# Build the manager binary
FROM golang:1.24 AS builder
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
RUN CGO_ENABLED=0 GOARCH=arm64 go build -a -o manager github.com/vyrodovalexey/k8s-postgresql-operator/cmd

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.21
RUN apk update \
    && apk upgrade --ignore alpine-baselayout \
    && apk add curl jq ca-certificates strace bash wget tar tzdata \
    && addgroup -g 1001 -S web-data \
    && adduser -u 1001 -D -S -G web-data --home /app web-data \
    && chown -R web-data:web-data /app

USER 1001
WORKDIR /app
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/app/manager"]

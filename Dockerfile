# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.22 AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum* go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build the collector binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ebs-metrics-collector ./cmd/collector

# Final stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /

COPY --from=builder /workspace/ebs-metrics-collector .

# Run as root because we need privileged access to NVMe devices
USER 0

ENTRYPOINT ["/ebs-metrics-collector"]

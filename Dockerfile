# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.22 AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY main.go main.go
COPY config/ config/
COPY pkg/ pkg/

# Build the exporter binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
    -ldflags="-X main.version=${VERSION:-dev} -X main.commit=${COMMIT:-unknown} -X main.buildDate=${BUILD_DATE:-unknown}" \
    -o ebs-metrics-exporter .

# Final stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

WORKDIR /

COPY --from=builder /workspace/ebs-metrics-exporter .

# Run as root because we need privileged access to NVMe devices
USER 0

ENTRYPOINT ["/ebs-metrics-exporter"]

# Building EBS Metrics Exporter

This guide covers building the EBS Metrics Exporter operator and DaemonSet from source.

## Prerequisites

### Required Tools

- **Go**: 1.22 or later
- **Git**: For source control
- **Make**: Build automation
- **Docker or Podman**: For container builds
- **oc CLI**: For OpenShift deployments (optional)

### Environment Setup

```bash
# Verify Go installation
go version  # Should be 1.22+

# Verify make
make --version

# Verify container runtime
docker --version
# or
podman --version
```

## Initial Setup

### 1. Clone the Repository

```bash
git clone https://github.com/nephomaniac/ebs-metrics-exporter.git
cd ebs-metrics-exporter
```

### 2. Initialize Boilerplate

**IMPORTANT**: Run this on first checkout to set up build tooling:

```bash
make boilerplate-update
```

This will:
- Download the OpenShift boilerplate system
- Set up build tools and conventions
- Generate `boilerplate/generated-includes.mk`
- Configure CI/CD integration

### 3. Download Go Dependencies

```bash
go mod download
go mod tidy
```

## Building Locally

### Build the Operator

The operator manages the lifecycle of the exporter DaemonSet:

```bash
# Build operator binary
make go-build

# Output: bin/ebs-metrics-exporter-operator
```

### Build the Exporter DaemonSet

The exporter collects NVMe statistics:

```bash
# Build exporter binary
make build-exporter

# Output: bin/ebs-metrics-exporter
```

### Build Everything

```bash
# Build both binaries
make go-build build-exporter
```

## Container Builds

### Build Operator Container

```bash
# Build operator image
make docker-build-operator

# With custom image name
make docker-build-operator IMG_OPERATOR=quay.io/your-org/ebs-metrics-exporter-operator:v0.1.0
```

### Build Exporter DaemonSet Container

```bash
# Build exporter image
make docker-build-exporter

# With custom image name
make docker-build-exporter IMG_EXPORTER=quay.io/your-org/ebs-metrics-exporter-daemonset:v0.1.0
```

### Build Both Containers

```bash
# Build both images
make docker-build

# With custom registry
export IMAGE_REGISTRY=quay.io/your-org
make docker-build
```

## Push to Registry

### Push Operator Image

```bash
# Push operator
make docker-push-operator IMG_OPERATOR=quay.io/your-org/ebs-metrics-exporter-operator:v0.1.0
```

### Push Exporter Image

```bash
# Push exporter
make docker-push-exporter IMG_EXPORTER=quay.io/your-org/ebs-metrics-exporter-daemonset:v0.1.0
```

### Push Both Images

```bash
# Set registry
export IMAGE_REGISTRY=quay.io/your-org

# Push both
make docker-push
```

## Build Configurations

### FIPS Mode

FIPS mode is **enabled by default**. To build with FIPS-compliant crypto:

```bash
# Build with FIPS (default)
make go-build

# Explicitly enable FIPS
make go-build FIPS_ENABLED=true
```

To disable FIPS:

```bash
# Build without FIPS
make go-build FIPS_ENABLED=false

# Or edit project.mk:
FIPS_ENABLED := false
```

### Debug Build

For development with debug symbols:

```bash
# Build with debug info
go build -gcflags="all=-N -l" -o bin/ebs-metrics-exporter-operator main.go
```

### Static Binary

For fully static binary (no dynamic linking):

```bash
# Build static binary
CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/ebs-metrics-exporter-operator main.go
```

## Multi-Architecture Builds

### Build for ARM64

```bash
# Build ARM64 binary
GOARCH=arm64 make go-build

# Build ARM64 container
docker buildx build --platform linux/arm64 -t ebs-metrics-exporter-operator:arm64 -f Dockerfile.operator .
```

### Build for AMD64

```bash
# Build AMD64 binary
GOARCH=amd64 make go-build

# Build AMD64 container
docker buildx build --platform linux/amd64 -t ebs-metrics-exporter-operator:amd64 -f Dockerfile.operator .
```

### Multi-Arch Manifest

```bash
# Create multi-arch manifest
docker buildx build --platform linux/amd64,linux/arm64 \
  -t quay.io/your-org/ebs-metrics-exporter-operator:latest \
  -f Dockerfile.operator \
  --push .
```

## Testing Builds

### Run Unit Tests

```bash
# Run all tests
make test

# Run with coverage
make coverage

# Run specific package
go test ./pkg/metrics/...
```

### Run Linting

```bash
# Run all linters
make lint

# Format code
make go-fmt

# Vet code
make go-vet
```

### Validate Build

```bash
# Run all validation (tests + lint)
make validate
```

## CI/CD Builds

### Local CI Simulation

Run tests in containers (simulates Prow CI):

```bash
# Run tests in container
make container-test

# Run lint in container
make container-lint
```

### Prow CI

The `.ci-operator.yaml` configures automated builds on:
- Pull requests
- Merges to main
- Release tags

Prow will:
1. Build both containers
2. Run unit tests
3. Run linting
4. Generate coverage reports
5. Publish images (on merge/release)

## Troubleshooting

### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
rm go.sum
go mod download
go mod tidy
```

### Build Fails with "command not found"

```bash
# Ensure boilerplate is initialized
make boilerplate-update

# Check PATH includes $GOBIN
export PATH=$PATH:$(go env GOPATH)/bin
```

### Docker Build Fails

```bash
# Check Docker daemon
docker ps

# Use Podman instead
alias docker=podman

# Clear build cache
docker builder prune
```

### FIPS Build Errors

```bash
# Temporarily disable FIPS
make go-build FIPS_ENABLED=false

# Check Go version (needs 1.19+ for FIPS)
go version
```

### "boilerplate/generated-includes.mk: No such file"

```bash
# Initialize boilerplate
make boilerplate-update

# If that fails, create minimal version
touch boilerplate/generated-includes.mk
make boilerplate-update
```

## Advanced Builds

### Custom Build Tags

```bash
# Build with custom tags
go build -tags "mytag anothertag" -o bin/ebs-metrics-exporter-operator main.go
```

### Vendoring Dependencies

```bash
# Vendor dependencies
go mod vendor

# Build using vendor
go build -mod=vendor -o bin/ebs-metrics-exporter-operator main.go
```

### Cross-Compilation

```bash
# Linux binary on macOS
GOOS=linux GOARCH=amd64 make go-build

# Windows binary
GOOS=windows GOARCH=amd64 go build -o bin/ebs-metrics-exporter-operator.exe main.go
```

## Build Artifacts

After successful builds:

```
bin/
├── ebs-metrics-exporter-operator   # Operator binary
└── ebs-metrics-exporter            # Exporter DaemonSet binary
```

Container images:
- `ebs-metrics-exporter-operator:latest` - Operator
- `ebs-metrics-exporter-daemonset:latest` - Exporter

## Next Steps

After building:

1. **Test locally**: See [DEVELOPMENT.md](DEVELOPMENT.md)
2. **Deploy**: See [QUICKSTART.md](QUICKSTART.md)
3. **Configure CI**: See [BOILERPLATE.md](BOILERPLATE.md)

## Resources

- [Go Build Command](https://golang.org/cmd/go/#hdr-Compile_packages_and_dependencies)
- [Docker Build Reference](https://docs.docker.com/engine/reference/commandline/build/)
- [OpenShift Boilerplate](https://github.com/openshift/boilerplate)

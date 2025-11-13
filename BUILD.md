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

This project has two binaries:
1. **Collector** (`ebs-metrics-collector`) - Standalone binary that queries NVMe devices and exposes Prometheus metrics
2. **Operator** (`ebs-metrics-exporter-operator`) - Kubernetes operator that manages collector DaemonSets

### Quick Build Commands

```bash
# Build collector binary (for local/standalone use)
make build-collector
# Output: bin/ebs-metrics-collector

# Build operator binary (for Kubernetes operator)
make build-operator
# Output: bin/ebs-metrics-exporter-operator

# Build both binaries
make build
```

### Boilerplate Build Commands

The OpenShift boilerplate system provides additional build targets:

```bash
# Build operator with FIPS compliance (for CI/production)
make go-build
# Output: build/_output/bin/ebs-metrics-exporter

# Run linting and static analysis
make go-check

# Run tests
make test

# Generate code coverage
make coverage
```

**Note:** The `go-build` target uses FIPS-compliant crypto and is intended for production/CI builds. For local development, use `make build-collector` or `make build-operator`.

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
└── ebs-metrics-collector            # Exporter DaemonSet binary

build/_output/bin/
└── ebs-metrics-exporter             # FIPS-enabled operator (from go-build)
```

Container images:
- `ebs-metrics-exporter-operator:latest` - Operator
- `ebs-metrics-exporter:latest` - Collector (default)

## Makefile Targets

### Available Build Targets

```bash
make help                    # Show all available targets

# Binary builds
make build                   # Build both collector and operator binaries
make build-collector         # Build the collector binary
make build-operator          # Build the operator binary
make go-build                # Build operator with FIPS (boilerplate)

# Container builds
make docker-build            # Build collector container image (default)
make docker-build-collector  # Build collector container image
make docker-build-operator   # Build operator container image
make docker-build-all        # Build both container images

# Push to registry
make docker-push             # Push collector container image (default)
make docker-push-collector   # Push collector container image
make docker-push-operator    # Push operator container image
make docker-push-all         # Push both container images

# Testing and validation
make test                    # Run unit tests
make coverage                # Generate coverage reports
make lint                    # Run linters
make go-check                # Golang linting and static analysis
make validate                # Run all validation checks

# Boilerplate
make boilerplate-update      # Update boilerplate from upstream
```

## Environment Variables

The following environment variables can be set to customize the build and deployment process:

### Image Configuration

```bash
# Container registry (default: quay.io)
IMAGE_REGISTRY=quay.io

# Image repository/organization (default: app-sre)
IMAGE_REPOSITORY=your-org

# Complete image reference for the collector
IMG=quay.io/your-org/ebs-metrics-exporter:v1.0.0

# Operator image (when using Makefile.operator)
IMG_OPERATOR=quay.io/your-org/ebs-metrics-exporter-operator:latest

# Exporter DaemonSet image (when using Makefile.operator)
IMG_EXPORTER=quay.io/your-org/ebs-metrics-exporter:latest
```

### Project Configuration

```bash
# Application name (default: ebs-metrics-exporter)
APP_NAME=ebs-metrics-exporter

# Target namespace (default: openshift-sre-ebs-metrics)
NAMESPACE=openshift-sre-ebs-metrics

# Operator version for OLM (default: 0.1.0)
OPERATOR_VERSION=0.2.0
```

### Build Configuration

```bash
# Enable FIPS-compliant builds (default: true)
FIPS_ENABLED=true

# Enable Konflux CI/CD builds (default: true)
KONFLUX_BUILDS=true

# Go build packages (default: ./...)
GO_BUILD_PACKAGES=./cmd/...

# Additional Go build flags
GO_BUILD_FLAGS="-v -x"

# Enable Go module vendoring (default: true)
GOMOD_VENDOR=true
```

### Usage Examples

**Build with custom registry:**

```bash
export IMAGE_REGISTRY=quay.io/mycompany
export IMAGE_REPOSITORY=sre-team
make docker-build
```

This produces: `quay.io/mycompany/sre-team/ebs-metrics-exporter:latest`

**Build specific version:**

```bash
export IMG=quay.io/mycompany/ebs-metrics-exporter:v1.2.3
make docker-build
make docker-push
```

**Build without FIPS:**

```bash
make docker-build FIPS_ENABLED=false
```

**Deploy to custom namespace:**

```bash
# Note: You'll need to update the namespace in deploy/*.yaml files
export NAMESPACE=my-custom-namespace
make deploy
```

**Build both operator and exporter with custom images:**

```bash
export IMG_OPERATOR=quay.io/myorg/ebs-operator:v1.0.0
export IMG_EXPORTER=quay.io/myorg/ebs-exporter:v1.0.0
make docker-build-all
make docker-push-all
```

**Override multiple variables:**

```bash
make docker-build \
  IMAGE_REGISTRY=registry.example.com \
  IMAGE_REPOSITORY=team/project \
  APP_NAME=ebs-exporter \
  FIPS_ENABLED=true
```

### Variable Precedence

Variables can be set in multiple ways, with the following precedence (highest to lowest):

1. **Command-line**: `make docker-build IMG=custom-image:tag`
2. **Environment variables**: `export IMG=custom-image:tag && make docker-build`
3. **project.mk**: Edit `project.mk` to set project defaults
4. **Makefile defaults**: Built-in defaults in `Makefile`

### Common Scenarios

**Development Build:**

```bash
# Quick local build without pushing
make build
```

**Production Build:**

```bash
# Build and push versioned image
export IMG=quay.io/production/ebs-metrics-exporter:v1.0.0
make docker-build
make docker-push
```

**Multi-Architecture Build:**

```bash
# Build for ARM64
docker buildx build \
  --platform linux/arm64 \
  -t quay.io/myorg/ebs-metrics-exporter:v1.0.0-arm64 \
  -f Dockerfile .
```

**Testing Different Registries:**

```bash
# Test with local registry
export IMAGE_REGISTRY=localhost:5000
export IMAGE_REPOSITORY=testing
make docker-build
make docker-push
```

## Next Steps

After building:

1. **Test locally**: See [DEVELOPMENT.md](DEVELOPMENT.md)
2. **Deploy**: See [QUICKSTART.md](QUICKSTART.md)
3. **Configure CI**: See [BOILERPLATE.md](BOILERPLATE.md)

## Resources

- [Go Build Command](https://golang.org/cmd/go/#hdr-Compile_packages_and_dependencies)
- [Docker Build Reference](https://docs.docker.com/engine/reference/commandline/build/)
- [OpenShift Boilerplate](https://github.com/openshift/boilerplate)

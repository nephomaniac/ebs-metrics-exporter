# EBS Metrics Exporter

A lightweight Prometheus exporter for Amazon EBS (Elastic Block Store) performance metrics. Collects NVMe device statistics via IOCTLs and exposes them as Prometheus metrics.

**Architecture**: Single Go binary deployed as DaemonSet via Package Operator (PKO).

## 📚 Documentation

### Getting Started
- **[Quick Start Guide](QUICKSTART_DEV.md)** - Get up and running in 5 minutes
- **[Development Setup Script](scripts/dev-setup.sh)** - Interactive environment setup

### Development
- **[Development Workflow](#quick-start-development-workflow)** - Build, deploy, test, iterate (this document)
- **[Build Guide](BUILD.md)** - Detailed build instructions and options
- **[Testing Guide](#testing)** - Unit and integration testing

### Deployment
- **[PKO Build Guide](PKO_BUILD_GUIDE.md)** - Production CI/CD with Konflux and Package Operator
- **[OLM Deployment](OLM.md)** - Legacy OLM deployment (archived)
- **[Deployment Summary](DEPLOYMENT_SUMMARY.md)** - Comprehensive deployment architecture

### Architecture & Design
- **[Refactoring Summary](REFACTORING_SUMMARY.md)** - Architecture decisions and rationale
- **[Architecture Update](ARCHITECTURE_UPDATE.md)** - Technical architecture details
- **[Cost Analysis Plan](COST_ANALYSIS_TODO.md)** - Future cost comparison vs CloudWatch

### Legacy Documentation
- **[README (Refactored)](README.refactored.md)** - Previous iteration documentation
- **[README (Operator)](README.operator.md)** - Operator pattern documentation (archived)
- **[Boilerplate Integration](BOILERPLATE_INTEGRATION_SUMMARY.md)** - Boilerplate framework details
- **[Boilerplate Guide](BOILERPLATE.md)** - Boilerplate system documentation
- **[Legacy Quick Start](QUICKSTART.md)** - Original quick start guide

### Branch History
- **`main`** - Current simplified DaemonSet-only architecture (PKO deployment)
- **`archive/olm-operator-approach`** - Original Operator + DaemonSet pattern (OLM deployment)

---

## Quick Start (Development Workflow)

### Prerequisites

- Go 1.22+
- Docker or Podman
- OpenShift CLI (`oc`) with cluster access
- Quay.io account

### 1. Set Your Image Repository

```bash
# Set your personal Quay.io repository
export QUAY_USER="your-quay-username"
export IMG="quay.io/${QUAY_USER}/ebs-metrics-exporter:test"
export IMG_PKO="quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:test"

# Or set in your shell profile (~/.bashrc, ~/.zshrc)
echo "export QUAY_USER=your-quay-username" >> ~/.zshrc
```

### 2. Build and Push Images

```bash
# Build both images (application + PKO package)
make dev-build

# Push to your Quay.io repository
make dev-push

# Or do both in one command
make dev-build-push
```

<details>
<summary>What this does</summary>

- Builds application image from `Dockerfile` → `quay.io/${QUAY_USER}/ebs-metrics-exporter:test`
- Builds PKO package image from `build/Dockerfile.pko` → `quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:test`
- Pushes both to your Quay.io repository
</details>

### 3. Deploy to Test Cluster

```bash
# Login to your OpenShift cluster
oc login https://api.your-cluster.com

# Deploy using PKO
make dev-deploy

# Or deploy using legacy YAML (non-PKO)
make deploy-legacy IMG=${IMG}
```

<details>
<summary>What this does</summary>

**PKO Deployment** (`make dev-deploy`):
- Creates temporary rendered manifests in `_output/pko/`
- Replaces `{{ .config.image }}` with your `${IMG}`
- Applies all manifests to cluster
- Creates namespace, RBAC, DaemonSet, ServiceMonitor

**Legacy Deployment** (`make deploy-legacy`):
- Uses static YAML from `deploy/` directory
- Replaces `REPLACE_IMAGE` placeholder with your image
- Useful for quick testing without PKO
</details>

### 4. Verify Deployment

```bash
# Check DaemonSet status
make dev-status

# View logs from all pods
make dev-logs

# Test metrics endpoint
make dev-metrics
```

### 5. Make Changes and Iterate

```bash
# Edit code
vim main.go

# Rebuild, push, and redeploy
make dev-rebuild

# Or do it step by step
make dev-build
make dev-push
make dev-restart  # Restarts DaemonSet to pull new image
```

### 6. Clean Up

```bash
# Remove from cluster
make dev-undeploy

# Remove local build artifacts
make clean
```

## Development Workflow Details

### Building Locally

```bash
# Build Go binary (local architecture)
make build

# Build Go binary for Linux/amd64 (OpenShift target)
make build-linux

# Run locally (requires sudo for NVMe device access)
sudo ./bin/ebs-metrics-exporter --device /dev/nvme1n1 --port 8090
```

### Container Images

Two images are built:

**1. Application Image** (`ebs-metrics-exporter:test`)
- Contains the Go binary
- Built from `Dockerfile`
- Runs on cluster nodes as DaemonSet pods

**2. PKO Package Image** (`ebs-metrics-exporter-pko:test`)
- Contains deployment manifests
- Built from `build/Dockerfile.pko`
- Used by Package Operator to deploy the application

### Image Repositories

**Development** (your personal repo):
```bash
quay.io/${QUAY_USER}/ebs-metrics-exporter:test
quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:test
```

**Production** (app-sre managed):
```bash
quay.io/app-sre/ebs-metrics-exporter:v1.0.0
quay.io/app-sre/ebs-metrics-exporter-pko:v1.0.0
```

### Deployment Methods

#### Option 1: PKO Deployment (Recommended)

Uses Package Operator to deploy from `deploy_pko/` manifests.

```bash
# Deploy
make dev-deploy

# Check deployment
oc get packagedeployment -A  # If using PKO operator
oc get daemonset -n openshift-sre-ebs-metrics

# Undeploy
make dev-undeploy
```

#### Option 2: Legacy YAML Deployment

Uses static manifests from `deploy/` directory.

```bash
# Deploy
make deploy-legacy IMG=quay.io/${QUAY_USER}/ebs-metrics-exporter:test

# Update image
oc set image daemonset/ebs-metrics-exporter \
  ebs-metrics-exporter=quay.io/${QUAY_USER}/ebs-metrics-exporter:test \
  -n openshift-sre-ebs-metrics

# Undeploy
make undeploy
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# View coverage report
open coverage.html
```

### Integration Testing

```bash
# Deploy to cluster
make dev-deploy

# Check pod is running
oc get pods -n openshift-sre-ebs-metrics

# Get a pod name
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_

# Check for specific metrics
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ebs_volume_queue_length

# View logs
oc logs -n openshift-sre-ebs-metrics $POD

# Port-forward for local testing
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090
curl http://localhost:8090/metrics
```

### Prometheus Integration

```bash
# Check ServiceMonitor is created
oc get servicemonitor -n openshift-sre-ebs-metrics

# Verify Prometheus is scraping
# Navigate to OpenShift Console → Observe → Metrics
# Query: {__name__=~"ebs_.*"}
```

## Makefile Targets Reference

### Development Targets

| Target | Description |
|--------|-------------|
| `make dev-build` | Build both application and PKO package images |
| `make dev-push` | Push both images to your Quay.io repository |
| `make dev-build-push` | Build and push both images |
| `make dev-deploy` | Deploy to cluster using PKO manifests |
| `make dev-undeploy` | Remove from cluster |
| `make dev-status` | Show deployment status |
| `make dev-logs` | View logs from all pods |
| `make dev-metrics` | Test metrics endpoint from a pod |
| `make dev-restart` | Restart DaemonSet (pulls latest image) |
| `make dev-rebuild` | Build, push, and restart (full iteration) |

### Build Targets

| Target | Description |
|--------|-------------|
| `make build` | Build Go binary for local architecture |
| `make build-linux` | Build Go binary for Linux/amd64 |
| `make docker-build` | Build application container image |
| `make docker-push` | Push application image to registry |
| `make podman-build` | Build using podman (macOS) |
| `make podman-push` | Push using podman |

### Testing Targets

| Target | Description |
|--------|-------------|
| `make test` | Run unit tests |
| `make test-coverage` | Run tests with coverage report |
| `make fmt` | Format Go code |
| `make vet` | Run go vet |
| `make tidy` | Run go mod tidy |
| `make go-check` | Run linter (golangci-lint) |

### Deployment Targets

| Target | Description |
|--------|-------------|
| `make deploy-legacy` | Deploy using legacy YAML (non-PKO) |
| `make undeploy` | Remove legacy deployment |
| `make pko-validate` | Validate PKO manifests |

### Cleanup Targets

| Target | Description |
|--------|-------------|
| `make clean` | Remove build artifacts |

## Metrics Exported

### Counter Metrics
- `ebs_volume_performance_exceeded_iops_total` - Volume IOPS limit exceeded (microseconds)
- `ebs_volume_performance_exceeded_throughput_total` - Volume throughput limit exceeded (microseconds)
- `ebs_instance_performance_exceeded_iops_total` - Instance IOPS limit exceeded (microseconds)
- `ebs_instance_performance_exceeded_throughput_total` - Instance throughput limit exceeded (microseconds)
- `ebs_total_read_ops_total` - Total read operations
- `ebs_total_write_ops_total` - Total write operations
- `ebs_total_read_bytes_total` - Total bytes read
- `ebs_total_write_bytes_total` - Total bytes written

### Gauge Metrics
- `ebs_volume_queue_length` - Current volume queue length

All metrics include labels:
- `device` - NVMe device name (e.g., "nvme1n1")
- `volume_id` - EBS volume ID (e.g., "vol-1234567890abcdef0")

## Configuration

### Environment Variables

```bash
# Required
export QUAY_USER="your-quay-username"

# Optional
export IMG="quay.io/${QUAY_USER}/ebs-metrics-exporter:custom-tag"
export IMG_PKO="quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:custom-tag"
```

### DaemonSet Configuration

Edit `deploy_pko/DaemonSet-ebs-metrics-exporter.yaml.gotmpl`:

**Change device path:**
```yaml
args:
- --device=/dev/nvme0n1  # Change this
- --port=8090
```

**Adjust resources:**
```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

**Change scrape interval** (ServiceMonitor):
```yaml
endpoints:
- port: metrics
  interval: 60s  # Default is 30s
```

## Troubleshooting

### Build Issues

**Q: Build fails with "no such file or directory"**
```bash
# Ensure you're in the repo root
cd ~/sandbox/ebs-metrics-exporter

# Check dependencies
go mod tidy
```

**Q: Docker build fails on macOS**
```bash
# Use podman instead
make podman-build
make podman-push
```

### Deployment Issues

**Q: Pods not starting**
```bash
# Check pod status
oc describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# Check events
oc get events -n openshift-sre-ebs-metrics --sort-by='.lastTimestamp'

# Verify SCC assignment
oc get pod -n openshift-sre-ebs-metrics -o yaml | grep 'openshift.io/scc'
```

**Q: ImagePullBackOff errors**
```bash
# Make sure image is public or you have ImagePullSecrets
# Check image exists
podman pull quay.io/${QUAY_USER}/ebs-metrics-exporter:test

# Make repository public in Quay.io:
# quay.io → Repository Settings → Make Public
```

**Q: No metrics appearing**
```bash
# Check device path
oc exec -n openshift-sre-ebs-metrics $POD -- ls -la /dev/nvme*

# Check container logs
oc logs -n openshift-sre-ebs-metrics $POD

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl localhost:8090/metrics
```

**Q: Permission denied errors**
```bash
# Verify SCC grants SYS_ADMIN capability
oc get scc ebs-metrics-exporter -o yaml | grep -A5 allowedCapabilities

# Check pod security context
oc get pod $POD -n openshift-sre-ebs-metrics -o json | jq '.spec.containers[0].securityContext'
```

### Prometheus Issues

**Q: Prometheus not scraping**
```bash
# Verify ServiceMonitor exists
oc get servicemonitor -n openshift-sre-ebs-metrics

# Check Service endpoints
oc get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Verify Prometheus RBAC
oc get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s

# Check Prometheus targets in UI
# OpenShift Console → Observe → Targets
# Look for: openshift-sre-ebs-metrics/ebs-metrics-exporter
```

## Project Structure

```
ebs-metrics-exporter/
├── main.go                          # Exporter entry point (~150 LOC)
├── config/
│   └── config.go                    # Constants (name, namespace)
├── pkg/
│   ├── collector/
│   │   └── collector.go             # Prometheus collector
│   └── nvme/
│       └── nvme.go                  # NVMe IOCTL logic
├── deploy_pko/                      # PKO deployment manifests
│   ├── manifest.yaml                # PKO package definition
│   ├── Namespace-*.yaml
│   ├── ServiceAccount-*.yaml
│   ├── SecurityContextConstraints-*.yaml
│   ├── Role-*.yaml
│   ├── RoleBinding-*.yaml
│   ├── DaemonSet-*.yaml.gotmpl      # Templated DaemonSet
│   ├── Service-*.yaml
│   └── ServiceMonitor-*.yaml
├── deploy/                          # Legacy YAML manifests
├── build/
│   └── Dockerfile.pko               # PKO package Dockerfile
├── .tekton/                         # Konflux CI/CD pipelines
├── Dockerfile                       # Application Dockerfile
├── Makefile                         # Build and deployment targets
└── README.md                        # This file
```

## Production Deployment

For production deployment via app-interface, see:
- [PKO_BUILD_GUIDE.md](PKO_BUILD_GUIDE.md) - Konflux CI/CD integration
- [REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md) - Architecture decisions

Production images are built automatically by Konflux and managed via app-interface GitOps.

## Architecture

### Why DaemonSet-Only?

This project uses a simplified architecture compared to traditional operators:

**Before** (Operator Pattern):
- Operator Deployment (1 pod) + DaemonSet (N pods)
- ~800 LOC
- Centralized metrics aggregation

**Now** (DaemonSet-Only):
- DaemonSet only (N pods)
- ~150 LOC
- Prometheus handles aggregation

**Benefits:**
- 83% less code
- Simpler to understand and maintain
- Standard node-exporter pattern
- Optimized for PKO deployment

For detailed rationale, see [REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md).

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test locally using the development workflow above
5. Run tests: `make test`
6. Run linter: `make go-check`
7. Format code: `make fmt`
8. Commit with descriptive message
9. Push and create a pull request

## License

Licensed under the Apache License 2.0. See LICENSE file for details.

## References

- **JIRA**: [SREP-2082](https://redhat.atlassian.net/browse/SREP-2082)
- **AWS EBS Stats**: https://docs.aws.amazon.com/ebs/latest/userguide/nvme-detailed-performance-stats.html
- **Prometheus Go Client**: https://github.com/prometheus/client_golang
- **Package Operator**: https://package-operator.run/
- **OpenShift Boilerplate**: https://github.com/openshift/boilerplate

## Help

```bash
# Show all make targets
make help

# Get help with oc commands
oc --help

# View cluster resources
oc get all -n openshift-sre-ebs-metrics
```

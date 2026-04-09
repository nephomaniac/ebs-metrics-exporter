# EBS Metrics Exporter

A lightweight Prometheus exporter for Amazon EBS (Elastic Block Store) performance metrics. Collects NVMe device statistics via IOCTLs and exposes them as Prometheus metrics.

**Architecture**: Single Go binary deployed as DaemonSet via Package Operator (PKO).

## 📚 Documentation

### Getting Started
- **[Quick Start (Local Development)](#quick-start-local-development)** - Get up and running in minutes (see below)
- **[Configuration Guide](CONFIG.md)** - Complete configuration reference
- **[Deployment Guide](DEPLOYMENT.md)** - Production deployment procedures

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
- **[Legacy Quick Start](QUICKSTART.md)** - Original quick start guide (archived)
- **[Legacy Dev Quick Start](QUICKSTART_DEV.md)** - Old development workflow (archived)

### Branch History
- **`main`** - Current simplified DaemonSet-only architecture (PKO deployment)
- **`archive/olm-operator-approach`** - Original Operator + DaemonSet pattern (OLM deployment)

---

## Quick Start (Local Development)

This project follows the **OpenShift Boilerplate PKO pattern**. All builds use boilerplate-provided targets that mimic production CI/CD pipelines.

### Prerequisites

- Go 1.23+ (for local builds)
- Podman or Docker
- OpenShift CLI (`oc`) with cluster access
- Quay.io account (for pushing images)
- `IMAGE_REPOSITORY` environment variable set

### 1. Set Your Image Repository

**Option A: Use local config file (recommended)**

```bash
# Create .env.local from template
make init-env

# Edit .env.local and set your Quay.io username
# IMAGE_REPOSITORY=your-quay-username
vim .env.local

# Verify settings
make show-env
```

**Option B: Export environment variable**

```bash
# Set your personal Quay.io username (used by all boilerplate targets)
export IMAGE_REPOSITORY=your-quay-username

# Add to your shell profile for persistence
echo 'export IMAGE_REPOSITORY=your-quay-username' >> ~/.zshrc
source ~/.zshrc
```

**What this controls:**
- Application image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`
- PKO package image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

**Tip:** The `.env.local` file is git-ignored and safe for personal settings. It's automatically loaded by all `make` commands.

### 2. Local CI - Build and Test (No Cluster Required)

```bash
# Build both images and run all tests (mimics Konflux CI)
make local-ci ALLOW_DIRTY_CHECKOUT=true
```

<details>
<summary>What this does (mimics production CI)</summary>

1. **Build application image** (boilerplate `docker-build`):
   - Uses `build/Dockerfile` with FIPS/BoringCrypto
   - Multi-stage build with Go 1.23+
   - Output: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`

2. **Build PKO package image**:
   - Uses `build/Dockerfile.pko` (scratch + YAML manifests)
   - Context: `deploy_pko/` directory
   - Output: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

3. **Run tests**:
   - `go-test` - Unit tests
   - `validate-pko-fixtures` - PKO template validation

**No cluster access or image push required** - perfect for pre-commit validation.
</details>

### 3. Full Workflow - Build, Test, Push, Deploy

```bash
# Login to your OpenShift cluster
oc login https://api.your-cluster.com

# Ensure you're logged into Quay.io
podman login quay.io

# Full workflow: build → test → push → deploy
make local ALLOW_DIRTY_CHECKOUT=true
```

<details>
<summary>What this does (complete local pipeline)</summary>

**Step 1: Build and Test** (via `local-ci`):
- Build both images
- Run all tests

**Step 2: Push Images** (via `local-push-all`):
- Push application image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`
- Push PKO package: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

**Step 3: Deploy via PKO Operator** (via `local-deploy`):
- Process `hack/pko/clusterpackage-direct.yaml` template
- Create `ClusterPackage` resource
- PKO operator deploys in phases: namespace → rbac → deploy

**Step 4: Show Status**:
- Display ClusterPackage, Namespace, DaemonSet, Pods
</details>

### 4. Verify Deployment

```bash
# Check deployment status
make local-status

# View logs from all pods
make local-logs
```

<details>
<summary>Expected output</summary>

```
=== ClusterPackage ===
NAME                   PHASE       STATUS
ebs-metrics-exporter   Available   True

=== Namespace ===
NAME                        STATUS   AGE
openshift-sre-ebs-metrics   Active   2m

=== DaemonSet ===
NAME                   DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
ebs-metrics-exporter   3         3         3       3            3

=== Pods ===
NAME                         READY   STATUS    RESTARTS   AGE
ebs-metrics-exporter-abc123  1/1     Running   0          2m
ebs-metrics-exporter-def456  1/1     Running   0          2m
ebs-metrics-exporter-ghi789  1/1     Running   0          2m
```
</details>

### 5. Clean Up

```bash
# Remove ClusterPackage (PKO operator cleans up everything)
make local-undeploy

# Remove local build artifacts
make clean
```

---

## Advanced: Step-by-Step Workflow

For more control, you can run individual steps:

### Build Only

```bash
# Build application image only
make docker-build ALLOW_DIRTY_CHECKOUT=true

# Build PKO package image only
make local-build-pko ALLOW_DIRTY_CHECKOUT=true

# Build both
make local-build-all ALLOW_DIRTY_CHECKOUT=true
```

### Test Only

```bash
# Run Go tests
make go-test

# Validate PKO templates
make validate-pko-fixtures

# Run all tests
make local-test-all
```

### Push Only

```bash
# Push application image
make docker-push

# Push PKO package image
make local-push-pko

# Push both
make local-push-all
```

### Deploy Only

```bash
# Deploy to cluster (images must exist in registry)
make local-deploy

# Check status
make local-status

# View logs
make local-logs

# Remove deployment
make local-undeploy
```

---

## Deploying with Custom Images

You can deploy images from **any location** (not just your personal Quay.io repo):

### Method 1: Using Environment Variables

```bash
# Set custom image locations
export IMAGE_REGISTRY=quay.io
export IMAGE_REPOSITORY=team-name  # or app-sre, or any repo
export OPERATOR_IMAGE_TAG=v1.2.3    # or latest, or custom tag

# Deploy using those images
make local-deploy
```

**Example - Deploy from app-sre production**:
```bash
export IMAGE_REPOSITORY=app-sre
export OPERATOR_IMAGE_TAG=v1.0.0
make local-deploy
```

### Method 2: Manually Process Template

For complete control over image sources:

```bash
# Process template with custom parameters
oc process -f hack/pko/clusterpackage-direct.yaml \
  -p OPERATOR_IMAGE=quay.io/custom-org/ebs-metrics-exporter \
  -p PKO_IMAGE=quay.io/custom-org/ebs-metrics-exporter-pko \
  -p IMAGE_TAG=custom-tag \
  -p DEVICE=/dev/nvme2n1 \
  | oc apply -f -
```

### Method 3: Edit ClusterPackage Directly

```bash
# Create and edit ClusterPackage resource
cat <<EOF | oc apply -f -
apiVersion: package-operator.run/v1alpha1
kind: ClusterPackage
metadata:
  name: ebs-metrics-exporter
spec:
  image: quay.io/any-registry/ebs-metrics-exporter-pko:any-tag
  config:
    image: quay.io/any-registry/ebs-metrics-exporter:any-tag
    device: /dev/nvme1n1
EOF
```

**Use cases**:
- Testing pre-release versions
- Using images from CI builds
- Testing with different NVMe device paths
- Deploying production images to test cluster

## Development Workflow Details

### Container Images

Two images are built (mimics Konflux CI/CD):

**1. Application Image** - Go binary with FIPS crypto
- **Built from**: `build/Dockerfile`
- **Builder**: `quay.io/redhat-services-prod/openshift/boilerplate:image-v8.3.4`
- **Features**: Go 1.23+, FIPS/BoringCrypto enabled
- **Size**: ~120 MB (UBI9 minimal + binary)
- **Make target**: `docker-build` (boilerplate)
- **Tekton pipeline**: `.tekton/ebs-metrics-exporter-push.yaml`

**2. PKO Package Image** - YAML manifests wrapper
- **Built from**: `build/Dockerfile.pko`
- **Context**: `deploy_pko/` directory
- **Size**: ~21 KB (scratch + YAML files)
- **Make target**: `local-build-pko` (custom)
- **Tekton pipeline**: `.tekton/ebs-metrics-exporter-pko-push.yaml`

### Image Naming

Images use boilerplate variables:

**Development** (your personal Quay.io repo):
```bash
# Set IMAGE_REPOSITORY to your username
export IMAGE_REPOSITORY=your-quay-username

# Images will be:
quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest
quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest
```

**Production** (app-sre managed via Konflux):
```bash
quay.io/app-sre/ebs-metrics-exporter:v1.0.0
quay.io/app-sre/ebs-metrics-exporter-pko:v1.0.0
```

### Boilerplate Variables

These variables are provided by boilerplate and control image names:

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_REGISTRY` | `quay.io` | Container registry |
| `IMAGE_REPOSITORY` | *(required)* | Your Quay username/org |
| `OPERATOR_NAME` | `ebs-metrics-exporter` | From config/config.go |
| `OPERATOR_NAMESPACE` | `openshift-sre-ebs-metrics` | From config/config.go |
| `OPERATOR_IMAGE_TAG` | `latest` | Image tag |
| `OPERATOR_IMAGE` | `$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME):$(OPERATOR_IMAGE_TAG)` | Full image URI |
| `OPERATOR_IMAGE_URI_LATEST` | Same as above with `:latest` | Latest image URI |

**Example**:
```bash
export IMAGE_REPOSITORY=your-quay-username
# Results in: quay.io/your-quay-username/ebs-metrics-exporter:latest
```

## Testing

### Pre-Commit Testing (No Cluster Required)

```bash
# Run all tests locally (mimics CI checks)
make local-ci ALLOW_DIRTY_CHECKOUT=true
```

This runs:
- `docker-build` - Build with FIPS crypto
- `local-build-pko` - Build PKO package  
- `go-test` - Unit tests
- `validate-pko-fixtures` - Template validation

### Unit Tests Only

```bash
# Run Go tests
make go-test

# Run linter
make go-check
```

### Integration Testing (Requires Cluster)

```bash
# Deploy to cluster
make local-deploy

# Check deployment status
make local-status

# View logs
make local-logs

# Get a pod name
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_

# Check for specific metrics
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ebs_volume_queue_length

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

All targets follow the **OpenShift Boilerplate pattern**. Targets are either:
- ✅ **Boilerplate targets** - Provided by `boilerplate/openshift/golang-osd-operator/`
- 🔧 **Local development targets** - Custom targets that call boilerplate (prefixed with `local-`)

### Top-Level Workflows (Recommended)

| Target | Description | Source |
|--------|-------------|--------|
| `make local-ci` | Build and test everything (no push/deploy) | 🔧 Custom |
| `make local` | Full workflow: build → test → push → deploy | 🔧 Custom |

**Use these for most development work**

### Build Targets

| Target | Description | Source |
|--------|-------------|--------|
| `make docker-build` | Build application image (FIPS/BoringCrypto) | ✅ Boilerplate |
| `make docker-push` | Push application image to registry | ✅ Boilerplate |
| `make go-build` | Build Go binary to build/_output/bin/ | ✅ Boilerplate |
| `make local-build-pko` | Build PKO package image | 🔧 Custom |
| `make local-build-all` | Build both images (docker-build + pko) | 🔧 Custom |

### Push Targets

| Target | Description | Source |
|--------|-------------|--------|
| `make docker-push` | Push application image | ✅ Boilerplate |
| `make local-push-pko` | Push PKO package image | 🔧 Custom |
| `make local-push-all` | Push both images | 🔧 Custom |

### Testing Targets

| Target | Description | Source |
|--------|-------------|--------|
| `make go-test` | Run unit tests | ✅ Boilerplate |
| `make go-check` | Run linter (golangci-lint) | ✅ Boilerplate |
| `make validate-pko-fixtures` | Validate PKO templates | ✅ Boilerplate |
| `make generate-pko-fixtures` | Regenerate PKO test fixtures | ✅ Boilerplate |
| `make local-test-all` | Run all tests (go-test + validate-pko) | 🔧 Custom |

### Deployment Targets

| Target | Description | Source |
|--------|-------------|--------|
| `make local-deploy` | Deploy via PKO operator | 🔧 Custom |
| `make local-undeploy` | Remove ClusterPackage | 🔧 Custom |
| `make local-status` | Show deployment status | 🔧 Custom |
| `make local-logs` | View pod logs | 🔧 Custom |

### Environment Management

| Target | Description | Source |
|--------|-------------|--------|
| `make init-env` | Create .env.local from template | 🔧 Custom |
| `make show-env` | Show current environment configuration | 🔧 Custom |
| `make check-cluster` | Verify cluster identity (safety check) | 🔧 Custom |

### Utility Targets

| Target | Description | Source |
|--------|-------------|--------|
| `make clean` | Remove build artifacts | ✅ Boilerplate |
| `make isclean` | Check if git is clean | ✅ Boilerplate |
| `make boilerplate-update` | Update boilerplate framework | 🔧 Custom |
| `make docker-login` | Login to container registry | ✅ Boilerplate |

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

**Metric Labels:**

For root volumes:
- `volume_id` - EBS volume ID (e.g., "vol-1234567890abcdef0")
- `volume_type` - Always "root" for root volumes

For PVC-backed volumes:
- `volume_id` - EBS volume ID
- `volume_type` - Always "pvc" for PersistentVolumeClaim volumes
- `pvc_namespace` - Kubernetes namespace of the PVC
- `pvc_name` - Name of the PersistentVolumeClaim

## Configuration

The exporter uses **ConfigMap-based configuration** for all runtime settings. See:
- **[CONFIG.md](CONFIG.md)** - Complete configuration reference
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Deployment and update procedures

### Quick Configuration Examples

**Default behavior** (no ConfigMap required):
- Auto-discovers all EBS volumes on the node
- Exports all available metrics
- Maps PVC-backed volumes to namespace/name labels

**Custom configuration** via ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ebs-metrics-exporter-config
  namespace: openshift-sre-ebs-metrics
data:
  config.yaml: |
    deviceDiscovery:
      mode: auto
      skipPVCMapping: false  # Set to true to skip PVC lookups
      autoFilter:
        excludeDevices:
          - /dev/nvme0n1  # Exclude root volume
    metrics:
      pollingIntervalSeconds: 30
```

For all configuration options and examples, see **[CONFIG.md](CONFIG.md)**.

### Resource Limits

Default resource requests/limits (defined in DaemonSet):
```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

### Prometheus Scrape Interval

Default scrape interval is 30 seconds (defined in ServiceMonitor). To change:
```yaml
endpoints:
- port: metrics
  interval: 60s  # Adjust as needed
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
podman pull quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest

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

## Quick Reference

### Common Workflows

**Pre-commit validation** (no cluster):
```bash
export IMAGE_REPOSITORY=your-quay-username
make local-ci ALLOW_DIRTY_CHECKOUT=true
```

**Full end-to-end test** (with cluster):
```bash
export IMAGE_REPOSITORY=your-quay-username
make local ALLOW_DIRTY_CHECKOUT=true
```

**Deploy specific image version**:
```bash
export IMAGE_REPOSITORY=app-sre
export OPERATOR_IMAGE_TAG=v1.0.0
make local-deploy
```

**Deploy from any registry**:
```bash
oc process -f hack/pko/clusterpackage-direct.yaml \
  -p OPERATOR_IMAGE=quay.io/custom/ebs-metrics-exporter \
  -p PKO_IMAGE=quay.io/custom/ebs-metrics-exporter-pko \
  -p IMAGE_TAG=custom-tag \
  | oc apply -f -
```

### Essential Commands

```bash
# Build
make local-build-all ALLOW_DIRTY_CHECKOUT=true

# Test
make local-test-all

# Deploy
make local-deploy

# Check status
make local-status

# View logs
make local-logs

# Clean up
make local-undeploy
```

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

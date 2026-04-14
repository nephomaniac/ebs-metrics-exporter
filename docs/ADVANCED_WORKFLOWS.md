# Advanced Workflows

For more control over the build and deployment process, you can run individual steps instead of using the top-level `make local` target.

## Step-by-Step Workflow

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

---

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

---

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

---

## Related Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get started quickly
- **[Makefile Targets](MAKEFILE.md)** - Complete reference of all make targets
- **[Testing Guide](TESTING.md)** - Testing procedures
- **[Configuration Guide](../CONFIG.md)** - Runtime configuration options

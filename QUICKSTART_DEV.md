# Quick Start - Development Workflow

Get up and running in 5 minutes.

## Prerequisites

- OpenShift CLI (`oc`) with cluster access
- Docker or Podman
- Go 1.22+ (optional, for local builds)
- Quay.io account

## Step 1: Setup

Run the setup script to check prerequisites:

```bash
./scripts/dev-setup.sh
```

Or manually set your Quay.io username:

```bash
export QUAY_USER="your-quay-username"

# Add to shell profile for persistence
echo 'export QUAY_USER="your-quay-username"' >> ~/.zshrc
```

## Step 2: Build and Push

```bash
# Build both images (app + PKO package)
make dev-build

# Push to your Quay.io repository
make dev-push

# Or do both in one command
make dev-build-push
```

**What gets built:**
- `quay.io/${QUAY_USER}/ebs-metrics-exporter:test` - Application image
- `quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:test` - PKO package image

## Step 3: Deploy to Cluster

Make sure you're logged into your OpenShift cluster:

```bash
oc login https://api.your-cluster.com
```

Deploy:

```bash
make dev-deploy
```

This will:
1. Render PKO manifests with your image
2. Create namespace `openshift-sre-ebs-metrics`
3. Deploy DaemonSet, RBAC, ServiceMonitor

## Step 4: Verify

```bash
# Check deployment status
make dev-status

# View logs from all pods
make dev-logs

# Test metrics endpoint
make dev-metrics
```

Expected output from `make dev-metrics`:
```
ebs_volume_performance_exceeded_iops_total{device="nvme1n1",volume_id="vol-abc123"} 0
ebs_volume_queue_length{device="nvme1n1",volume_id="vol-abc123"} 0
ebs_total_read_ops_total{device="nvme1n1",volume_id="vol-abc123"} 12345
...
```

## Step 5: Make Changes and Iterate

Edit the code:

```bash
vim main.go
```

Rebuild and redeploy:

```bash
# Full iteration: build, push, restart DaemonSet
make dev-rebuild
```

Or step-by-step:

```bash
make dev-build      # Build images
make dev-push       # Push to registry
make dev-restart    # Restart DaemonSet to pull new image
```

## Step 6: Test in Prometheus

1. Navigate to OpenShift Console → **Observe** → **Metrics**
2. Query: `{__name__=~"ebs_.*"}`
3. See all EBS metrics

Example queries:
```promql
# Show all EBS metrics
{__name__=~"ebs_.*"}

# Volume queue length
ebs_volume_queue_length

# Read operations rate
rate(ebs_total_read_ops_total[5m])

# IOPS exceeded time
ebs_volume_performance_exceeded_iops_total
```

## Step 7: Clean Up

```bash
# Remove from cluster
make dev-undeploy

# Remove local build artifacts
make clean
```

## Quick Reference

### One-Line Commands

```bash
# Full workflow
make dev-build-push && make dev-deploy && make dev-status

# Rebuild after code changes
make dev-rebuild

# Check if it's working
make dev-metrics
```

### Troubleshooting

**Problem**: `ImagePullBackOff` errors

**Solution**: Make your Quay.io repositories public:
1. Go to https://quay.io
2. Find `ebs-metrics-exporter` repository
3. Settings → Make Public
4. Repeat for `ebs-metrics-exporter-pko`

**Problem**: No pods running

**Solution**: Check events and logs:
```bash
oc get events -n openshift-sre-ebs-metrics --sort-by='.lastTimestamp' | tail -20
oc describe daemonset ebs-metrics-exporter -n openshift-sre-ebs-metrics
```

**Problem**: No metrics appearing

**Solution**: Check device path:
```bash
POD=$(oc get pods -n openshift-sre-ebs-metrics -o name | head -1)
oc exec -n openshift-sre-ebs-metrics $POD -- ls -la /dev/nvme*
```

### Advanced Usage

**Use different device:**

Edit `deploy_pko/DaemonSet-ebs-metrics-exporter.yaml.gotmpl`:
```yaml
args:
- --device=/dev/nvme0n1  # Change this
```

Then redeploy:
```bash
make dev-deploy
```

**Use custom image tag:**

```bash
export DEV_IMG="quay.io/${QUAY_USER}/ebs-metrics-exporter:my-feature"
export DEV_IMG_PKO="quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:my-feature"

make dev-build-push
make dev-deploy
```

**Deploy without PKO (legacy):**

```bash
make deploy-legacy IMG=quay.io/${QUAY_USER}/ebs-metrics-exporter:test
```

### Make Targets Cheat Sheet

| Command | Description |
|---------|-------------|
| `make dev-build` | Build both images |
| `make dev-push` | Push to Quay.io |
| `make dev-build-push` | Build and push |
| `make dev-deploy` | Deploy to cluster |
| `make dev-undeploy` | Remove from cluster |
| `make dev-status` | Show deployment status |
| `make dev-logs` | View pod logs |
| `make dev-metrics` | Test metrics endpoint |
| `make dev-restart` | Restart DaemonSet |
| `make dev-rebuild` | Full iteration (build+push+restart) |
| `make help` | Show all targets |

## Next Steps

- Read [README.md](README.md) for full documentation
- Check [PKO_BUILD_GUIDE.md](PKO_BUILD_GUIDE.md) for production builds
- See [REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md) for architecture details

## Getting Help

```bash
# Show all make targets
make help

# Check OpenShift resources
oc get all -n openshift-sre-ebs-metrics

# View pod details
POD=$(oc get pods -n openshift-sre-ebs-metrics -o name | head -1)
oc describe -n openshift-sre-ebs-metrics $POD
```

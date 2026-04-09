# Deployment Guide

## Image Tagging Strategy

### Production Requirements

**Always use immutable image tags for production deployments.**

The build system automatically generates immutable tags based on git commits:

```
Format: v{MAJOR}.{MINOR}.{COMMIT_NUMBER}-g{GIT_SHA}
Example: v0.1.21-g43e24d6
```

Where:
- `v0.1.21` - Semantic version from `boilerplate/_lib/version.txt`
- `g43e24d6` - Git commit short SHA (immutable identifier)

### Why Immutable Tags Matter

**❌ Bad (mutable tags):**
```yaml
image: quay.io/app-sre/ebs-metrics-exporter:latest
imagePullPolicy: Always
```
**Problem:** Pushing a new image with `latest` tag causes uncontrolled rollouts on pod restarts

**✅ Good (immutable tags):**
```yaml
image: quay.io/app-sre/ebs-metrics-exporter:v0.1.21-g43e24d6
imagePullPolicy: IfNotPresent
```
**Benefit:** Explicit, controlled updates only when ClusterPackage manifest changes

### Image Pull Policy

The DaemonSet uses `imagePullPolicy: IfNotPresent`:
- Pulls image only if not already present on the node
- With immutable tags, ensures consistent versions across all pods
- To update: change the tag in ClusterPackage, don't push new image to same tag

## Building Images

### Local Development

Build both application and PKO images:

```bash
# Set your image repository
export IMAGE_REPOSITORY=your-quay-username

# Build images (creates immutable git-based tag)
GOOS=linux GOARCH=amd64 ALLOW_DIRTY_CHECKOUT=true make docker-build
make local-build-pko

# Push to quay.io
podman push quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:v0.1.21-g43e24d6
podman push quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:v0.1.21-g43e24d6
```

### Production (Konflux)

Konflux CI automatically:
1. Builds on every commit
2. Generates immutable tag based on git SHA
3. Pushes to `quay.io/app-sre/ebs-metrics-exporter:v{VERSION}-g{SHA}`
4. Creates PKO package with matching tag

## Deploying

### Prerequisites

1. **Cluster access** - Must be logged into correct cluster
2. **Cluster verification** - Safety check prevents wrong-cluster deployments
3. **Images pushed** - Both application and PKO images in registry

### Deployment Commands

```bash
# Verify you're on the correct cluster
make check-cluster

# Deploy (will verify cluster first)
IMAGE_REPOSITORY=your-quay-username make local-deploy

# Check status
make local-status

# View logs
make local-logs

# Remove deployment
make local-undeploy
```

### Manual Deployment

If make targets don't work, deploy directly:

```bash
oc process -f hack/pko/clusterpackage-direct.yaml \
  --local=true \
  -p OPERATOR_IMAGE=quay.io/app-sre/ebs-metrics-exporter \
  -p PKO_IMAGE=quay.io/app-sre/ebs-metrics-exporter-pko \
  -p IMAGE_TAG=v0.1.21-g43e24d6 \
  | oc apply -f -
```

## Updating Production

### Rolling Update Process

1. **Make code changes** and commit
2. **Wait for CI** to build new images with new git SHA tag
3. **Update ClusterPackage** with new image tag:
   ```bash
   # New tag after commit will be different, e.g., v0.1.22-g7a8b9c0
   oc patch clusterpackage ebs-metrics-exporter --type=merge \
     -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.22-g7a8b9c0","config":{"image":"quay.io/app-sre/ebs-metrics-exporter:v0.1.22-g7a8b9c0"}}}'
   ```
4. **PKO operator** performs rolling update with `maxUnavailable: 1`
5. **Monitor rollout:**
   ```bash
   oc get daemonset -n openshift-sre-ebs-metrics ebs-metrics-exporter -w
   ```

### Rollback

If issues occur, rollback to previous tag:

```bash
oc patch clusterpackage ebs-metrics-exporter --type=merge \
  -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.21-g43e24d6","config":{"image":"quay.io/app-sre/ebs-metrics-exporter:v0.1.21-g43e24d6"}}}'
```

## Cluster Safety Checks

The Makefile includes cluster verification to prevent accidental deployments:

**Set expected cluster:**
```bash
# Export as environment variable (recommended)
export EXPECTED_CLUSTER=my-test-cluster

# Or set inline for each command
EXPECTED_CLUSTER=my-test-cluster make local-deploy
```

**Extract cluster ID from current cluster:**
```bash
# Get cluster identifier from server URL
oc whoami --show-server
# Example output: https://api.my-test-cluster.example.com:6443
# Use: my-test-cluster
```

**Targets with safety checks:**
- `make local-deploy` - Deployment (write operation)
- `make local-undeploy` - Removal (write operation)

**Read-only targets** (no check required):
- `make local-status` - View deployment status
- `make local-logs` - View pod logs
- `make check-cluster` - Verify cluster identity

## Troubleshooting

### Wrong image pulled

**Symptom:** Pods running old code despite new deployment

**Solution:**
```bash
# Check what image pods are actually running
oc get pods -n openshift-sre-ebs-metrics -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\n"}{end}'

# Force restart all pods
oc delete pods -n openshift-sre-ebs-metrics --all
```

### Image pull errors

**Symptom:** `ImagePullBackOff` or `ErrImagePull`

**Check:**
```bash
# Verify image exists in registry
podman search quay.io/app-sre/ebs-metrics-exporter --limit 100

# Check image pull secrets
oc get secrets -n openshift-sre-ebs-metrics

# Check pod events
oc describe pod -n openshift-sre-ebs-metrics <pod-name>
```

### ClusterPackage stuck

**Symptom:** `AVAILABLE=False`, `PROGRESSING=True` for extended time

**Debug:**
```bash
# Check ClusterPackage status
oc get clusterpackage ebs-metrics-exporter -o yaml

# Check PKO operator logs
oc logs -n package-operator-system deployment/package-operator-manager

# Check phase progression
oc get clusterpackage ebs-metrics-exporter -o jsonpath='{.status.phase}'
```

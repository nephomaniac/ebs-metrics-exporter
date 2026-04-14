# Troubleshooting Guide

## Build Issues

### Build fails with "no such file or directory"

```bash
# Ensure you're in the repo root
cd ~/sandbox/ebs-metrics-exporter

# Check dependencies
go mod tidy
```

### Docker build fails on macOS

```bash
# Use podman instead
make podman-build
make podman-push
```

---

## Deployment Issues

### Pods not starting

```bash
# Check pod status
oc describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# Check events
oc get events -n openshift-sre-ebs-metrics --sort-by='.lastTimestamp'

# Verify SCC assignment
oc get pod -n openshift-sre-ebs-metrics -o yaml | grep 'openshift.io/scc'
```

### ImagePullBackOff errors

```bash
# Make sure image is public or you have ImagePullSecrets
# Check image exists
podman pull quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest

# Make repository public in Quay.io:
# quay.io → Repository Settings → Make Public
```

### No metrics appearing

```bash
# Check device path
oc exec -n openshift-sre-ebs-metrics $POD -- ls -la /dev/nvme*

# Check container logs
oc logs -n openshift-sre-ebs-metrics $POD

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl localhost:8090/metrics
```

### Permission denied errors

```bash
# Verify SCC grants SYS_ADMIN capability
oc get scc ebs-metrics-exporter -o yaml | grep -A5 allowedCapabilities

# Check pod security context
oc get pod $POD -n openshift-sre-ebs-metrics -o json | jq '.spec.containers[0].securityContext'
```

---

## Prometheus Issues

### Prometheus not scraping

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

---

## Configuration Issues

### ConfigMap changes not applied

**Problem**: Edited ConfigMap but pods still using old config.

**Solution**: Pods don't automatically restart when ConfigMap changes. Manually restart them:

```bash
# Option 1: Use make target
make local-restart

# Option 2: Manual deletion
oc delete pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter
```

See **[Operations Guide](../OPERATIONS.md)** for detailed configuration management procedures.

### Wrong device path

**Problem**: Exporter can't find EBS volumes.

**Symptoms:**
- No metrics exported
- Logs show "no devices found"

**Solution**: Check NVMe device paths on your cluster nodes:

```bash
# Get a node name
NODE=$(oc get nodes -o jsonpath='{.items[0].metadata.name}')

# Check devices on node
oc debug node/$NODE -- chroot /host ls -la /dev/nvme*

# Update ConfigMap with correct device pattern
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# Restart pods to apply
make local-restart
```

---

## PKO Deployment Issues

### ClusterPackage stuck in "Progressing"

```bash
# Check ClusterPackage status
oc get clusterpackage ebs-metrics-exporter -o yaml

# Check ClusterObjectSet (PKO's internal state)
oc get clusterobjectset -l package-operator.run/package=ebs-metrics-exporter

# View PKO operator logs
oc logs -n package-operator-system deployment/package-operator-manager
```

### Phase failure in PKO deployment

**Problem**: PKO deployment fails at specific phase (e.g., "deploy" phase).

**Solution**: Check which resources failed:

```bash
# Get ClusterPackage conditions
oc get clusterpackage ebs-metrics-exporter -o jsonpath='{.status.conditions}' | jq

# Check specific phase resources
# Phase order: crds → namespace → rbac → deploy
oc get all,sa,role,rolebinding,scc -n openshift-sre-ebs-metrics

# Check for events
oc get events -n openshift-sre-ebs-metrics --sort-by='.lastTimestamp'
```

---

## Image Registry Issues

### Not logged into Quay.io

```bash
# Login to Quay.io
podman login quay.io
# Enter your username and password (or robot token)

# Or use make target
make docker-login
```

### Repository not public

**Problem**: ImagePullBackOff on cluster because Quay.io repo is private.

**Solution**: Make repository public:
1. Go to https://quay.io
2. Navigate to your repository
3. Settings → Repository Visibility → Make Public
4. Confirm

**Alternative**: Use ImagePullSecrets (for private repos).

---

## Development Environment Issues

### IMAGE_REPOSITORY not set

**Problem**: Build or deploy fails with "IMAGE_REPOSITORY is required".

**Solution**: Set your Quay.io username:

```bash
# Option 1: Environment variable
export IMAGE_REPOSITORY=your-quay-username

# Option 2: .env.local file (recommended)
make init-env
vim .env.local  # Set IMAGE_REPOSITORY=your-quay-username

# Verify
make show-env
```

### Wrong cluster deployment

**Problem**: Accidentally deployed to wrong cluster.

**Solution**: Use cluster safety checks:

```bash
# Set expected cluster ID
export EXPECTED_CLUSTER=my-test-cluster-id

# Add to .env.local for persistence
echo 'EXPECTED_CLUSTER=my-test-cluster-id' >> .env.local

# Now local-deploy will verify cluster before deploying
make local-deploy
```

---

## Metrics Collection Issues

### Metrics showing zero values

**Problem**: All metrics exist but show 0.

**Possible causes:**
1. **No I/O activity** - Normal if cluster is idle
2. **Device not under load** - Performance limits not being exceeded
3. **Wrong device** - Monitoring non-EBS device

**Verification:**
```bash
# Check if EBS volumes are actually being used
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep -E 'ebs_total_(read|write)_ops_total'

# Generate some I/O to test
oc run test-io --image=busybox --restart=Never -- sh -c "dd if=/dev/zero of=/tmp/test bs=1M count=100"
```

### Missing PVC labels

**Problem**: PVC-backed volumes show `volume_type="pvc"` but no `pvc_namespace` or `pvc_name` labels.

**Possible causes:**
1. Exporter lacks RBAC to list PVs/PVCs
2. Volume not actually backed by PVC (e.g., ephemeral)

**Verification:**
```bash
# Check if ServiceAccount has PV/PVC read permissions
oc get role ebs-metrics-exporter -n openshift-sre-ebs-metrics -o yaml

# Check exporter logs for RBAC errors
oc logs -n openshift-sre-ebs-metrics $POD | grep -i "forbidden\|permission"
```

---

## Related Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get started with local development
- **[Testing Guide](TESTING.md)** - Testing procedures
- **[Configuration Guide](../CONFIG.md)** - Configuration reference
- **[Operations Guide](../OPERATIONS.md)** - Day-2 operations and config updates

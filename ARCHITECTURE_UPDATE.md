# Architecture Update: DaemonSet-Only Deployment

## Overview

The EBS Metrics Exporter has been simplified from a two-tier operator architecture to a direct DaemonSet deployment.

## What Changed

### Previous Architecture (Removed)
```
┌─────────────────────────────────────────┐
│  Operator (Deployment)                  │
│  - Manages DaemonSet lifecycle          │
│  - Aggregates metrics from all nodes    │
│  - Exposes on port 8383                 │
└─────────────────────────────────────────┘
              │
              ├─ manages
              ▼
┌─────────────────────────────────────────┐
│  Exporter (DaemonSet)                   │
│  - Runs on every node                   │
│  - Collects NVMe statistics             │
│  - Exposes on port 8090                 │
└─────────────────────────────────────────┘
```

### New Architecture (Current)
```
┌─────────────────────────────────────────┐
│  EBS Metrics Exporter (DaemonSet)       │
│  - Runs on every node                   │
│  - Collects NVMe statistics via IOCTLs  │
│  - Exposes metrics on port 8090         │
│  - Scraped directly by Prometheus       │
└─────────────────────────────────────────┘
```

## Deployment Structure

All resources are now in the `deploy/` directory:

```
deploy/
├── 10_ebs-metrics-exporter.ServiceAccount.yaml
├── 10_prometheus-k8s_openshift-sre-ebs-metrics.Role.yaml
├── 20_ebs-metrics-exporter.SecurityContextConstraints.yaml
├── 20_prometheus-k8s_openshift-sre-ebs-metrics.RoleBinding.yaml
├── 30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml
├── 40_ebs-metrics-exporter_openshift-sre-ebs-metrics.Service.yaml
└── 50_ebs-metrics-exporter_openshift-sre-ebs-metrics.ServiceMonitor.yaml
```

### Resource Descriptions

1. **ServiceAccount** (10_)
   - Name: `ebs-metrics-exporter`
   - Used by the DaemonSet pods

2. **Role for Prometheus** (10_)
   - Namespace: `openshift-sre-ebs-metrics`
   - Allows prometheus-k8s to get/list/watch services, endpoints, pods

3. **SecurityContextConstraints** (20_)
   - Name: `ebs-metrics-exporter`
   - Allows privileged containers
   - Grants SYS_ADMIN capability
   - Permits hostNetwork, hostPID, hostPath

4. **RoleBinding for Prometheus** (20_)
   - Binds prometheus-k8s SA (from openshift-monitoring) to the Role

5. **DaemonSet** (30_)
   - Runs ebs-metrics-exporter on every node
   - Requires privileged access to /dev
   - Exposes metrics on port 8090
   - Uses hostNetwork for device access

6. **Service** (40_)
   - Headless service (clusterIP: None)
   - Selects DaemonSet pods
   - Exposes port 8090

7. **ServiceMonitor** (50_)
   - Configures Prometheus to scrape the Service
   - Scrape interval: 30 seconds
   - Endpoint: /metrics

## Benefits of New Architecture

### Simplicity
- ✅ Single DaemonSet instead of Deployment + DaemonSet
- ✅ Direct Prometheus scraping (no aggregation layer)
- ✅ Fewer moving parts
- ✅ Easier to understand and maintain

### Resource Efficiency
- ✅ No operator pod overhead
- ✅ Metrics collected directly from each node
- ✅ No inter-pod communication needed

### Operational Benefits
- ✅ Simpler deployment (fewer manifests)
- ✅ Direct metric access per node
- ✅ Standard Prometheus ServiceMonitor pattern
- ✅ No need for custom aggregation logic

## Deployment

### Quick Deploy

```bash
# Deploy everything
kubectl apply -f deploy/

# Verify
kubectl get daemonset -n openshift-sre-ebs-metrics
kubectl get pods -n openshift-sre-ebs-metrics
```

### Build and Deploy

```bash
# Build image
make docker-build IMG=quay.io/your-org/ebs-metrics-exporter:latest

# Push image
make docker-push IMG=quay.io/your-org/ebs-metrics-exporter:latest

# Update image in DaemonSet
sed -i 's|REPLACE_IMAGE|quay.io/your-org/ebs-metrics-exporter:latest|g' \
  deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml

# Deploy
make deploy
```

## Metrics Access

### Via Prometheus

Prometheus automatically scrapes all DaemonSet pods via the ServiceMonitor.

Query examples:
```promql
# All EBS metrics
{__name__=~"ebs_.*"}

# Metrics from specific node
ebs_volume_queue_length{node="worker-1"}

# IOPS exceeded across all nodes
sum(ebs_volume_performance_exceeded_iops_percent) by (volume_id)
```

### Via Port Forward

```bash
# Get a pod
POD=$(kubectl get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')

# Port forward
kubectl port-forward -n openshift-sre-ebs-metrics $POD 8090:8090

# Query metrics
curl http://localhost:8090/metrics
```

## Removed Components

The following are **no longer needed**:

### Files Removed
- `main.go` (operator code)
- `controllers/` directory
- `pkg/metrics/` directory (aggregator)
- `config/` directory
- `Dockerfile.operator`
- Operator deployment manifest
- Operator RBAC (ClusterRole, ClusterRoleBinding, operator Role)

### Directories Cleaned Up
- `k8s/` - Deprecated (all manifests moved to `deploy/`)
- `deploy/` - Now contains only DaemonSet-related resources

## Migration Notes

If upgrading from the operator-based deployment:

1. **Undeploy operator**:
   ```bash
   kubectl delete deployment ebs-metrics-collector-operator -n openshift-sre-ebs-metrics
   ```

2. **Clean up old DaemonSet** (if exists):
   ```bash
   kubectl delete -f k8s/ --ignore-not-found=true
   ```

3. **Deploy new version**:
   ```bash
   kubectl apply -f deploy/
   ```

4. **Verify**:
   ```bash
   kubectl get daemonset,service,servicemonitor -n openshift-sre-ebs-metrics
   ```

## Makefile Changes

### Old Targets (Removed)
- `deploy-operator`
- `undeploy-operator`
- `deploy-exporter`
- `undeploy-exporter`
- `deploy-all`
- `undeploy-all`
- `run-operator`
- `docker-build-operator`
- `docker-push-operator`

### New Targets
- `make deploy` - Deploy DaemonSet
- `make undeploy` - Remove DaemonSet
- `make build` - Build exporter binary
- `make docker-build` - Build container image
- `make docker-push` - Push container image
- `make run` - Run exporter locally (with sudo)

## Configuration

### Environment Variables

Configure the DaemonSet by editing `deploy/30_*.DaemonSet.yaml`:

```yaml
env:
- name: EBS_DEVICE
  value: "/dev/nvme1n1"  # Change device path
```

### Scrape Interval

Edit `deploy/50_*.ServiceMonitor.yaml`:

```yaml
endpoints:
- port: metrics
  interval: 30s  # Change scrape interval
```

### Resource Limits

Edit `deploy/30_*.DaemonSet.yaml`:

```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

## Troubleshooting

### Pods Not Running

```bash
# Check pod status
kubectl describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# Check SCC
kubectl get pod <pod-name> -n openshift-sre-ebs-metrics -o yaml | grep openshift.io/scc
```

### No Metrics

```bash
# Check device access
kubectl exec -n openshift-sre-ebs-metrics <pod-name> -- ls -la /dev/nvme*

# Check logs
kubectl logs -n openshift-sre-ebs-metrics <pod-name>

# Test metrics endpoint
kubectl exec -n openshift-sre-ebs-metrics <pod-name> -- curl localhost:8090/metrics
```

### Prometheus Not Scraping

```bash
# Check ServiceMonitor
kubectl get servicemonitor -n openshift-sre-ebs-metrics ebs-metrics-exporter -o yaml

# Check Service endpoints
kubectl get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Verify Prometheus RBAC
kubectl get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s
```

## Next Steps

1. **Update CI/CD** to build only the DaemonSet image
2. **Update documentation** to reflect simplified architecture
3. **Remove deprecated** k8s/ directory
4. **Clean up** operator-related files

## Questions?

- Deployment: See [QUICKSTART.md](QUICKSTART.md)
- Building: See [BUILD.md](BUILD.md)
- Metrics: See [README.md](README.md)

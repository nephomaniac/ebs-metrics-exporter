# EBS Metrics Exporter - Deployment Summary

## Current Architecture

The EBS Metrics Exporter runs as a **DaemonSet** that:
- Runs on every node in the cluster
- Collects NVMe device statistics via IOCTLs
- Exposes metrics on port 8090
- Gets scraped directly by Prometheus

## Deployment Files

All deployment resources are in the `deploy/` directory:

```
deploy/
├── 10_ebs-metrics-exporter.ServiceAccount.yaml              # ServiceAccount for DaemonSet
├── 10_prometheus-k8s_openshift-sre-ebs-metrics.Role.yaml    # Role for Prometheus access
├── 20_ebs-metrics-exporter.SecurityContextConstraints.yaml  # SCC for privileged access
├── 20_prometheus-k8s_openshift-sre-ebs-metrics.RoleBinding.yaml  # Bind Prometheus Role
├── 30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml  # The DaemonSet
├── 40_ebs-metrics-exporter_openshift-sre-ebs-metrics.Service.yaml    # Headless Service
└── 50_ebs-metrics-exporter_openshift-sre-ebs-metrics.ServiceMonitor.yaml  # Prometheus scraping
```

## Quick Deployment

### Prerequisites
- OpenShift cluster
- Container image built and pushed to registry
- `oc` or `kubectl` CLI

### Steps

1. **Build and push image**:
   ```bash
   make docker-build IMG=quay.io/your-org/ebs-metrics-exporter:latest
   make docker-push IMG=quay.io/your-org/ebs-metrics-exporter:latest
   ```

2. **Update image reference**:
   ```bash
   sed -i 's|REPLACE_IMAGE|quay.io/your-org/ebs-metrics-exporter:latest|g' \
     deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml
   ```

3. **Deploy**:
   ```bash
   make deploy
   # or
   kubectl apply -f deploy/
   ```

4. **Verify**:
   ```bash
   kubectl get all -n openshift-sre-ebs-metrics
   ```

## Namespace

- **Name**: `openshift-sre-ebs-metrics`
- **Labels**:
  - `pod-security.kubernetes.io/enforce: privileged`
  - `openshift.io/cluster-monitoring: "true"`

The namespace is created as part of k8s/01-namespace.yaml (kept separate for cluster-wide config).

## Service Account

- **Name**: `ebs-metrics-exporter`
- **Namespace**: `openshift-sre-ebs-metrics`
- **Used by**: DaemonSet pods

## RBAC

### Prometheus Access
- **Role**: `prometheus-k8s` (in openshift-sre-ebs-metrics)
  - Allows: get, list, watch on services, endpoints, pods
- **RoleBinding**: Binds prometheus-k8s SA from openshift-monitoring namespace

### DaemonSet Permissions
- **SCC**: `ebs-metrics-exporter`
  - Allows privileged containers
  - Grants SYS_ADMIN capability
  - Permits hostNetwork, hostPID, hostPath

## DaemonSet Configuration

### Image
```yaml
image: REPLACE_IMAGE  # Update this before deployment
```

### Environment Variables
```yaml
env:
- name: EBS_DEVICE
  value: "/dev/nvme1n1"  # Default NVMe device
```

### Resources
```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

### Security Context
```yaml
securityContext:
  privileged: true
  runAsUser: 0
  capabilities:
    add:
    - SYS_ADMIN
```

### Volume Mounts
```yaml
volumeMounts:
- name: dev
  mountPath: /dev
  readOnly: true
```

### Node Selection
```yaml
nodeSelector:
  kubernetes.io/os: linux
tolerations:
- operator: Exists  # Run on all nodes
```

## Service

- **Type**: Headless (clusterIP: None)
- **Port**: 8090
- **Selector**: Matches DaemonSet pods
- **Purpose**: Endpoint discovery for ServiceMonitor

## ServiceMonitor

- **Name**: `ebs-metrics-exporter`
- **Namespace**: `openshift-sre-ebs-metrics`
- **Scrape interval**: 30 seconds
- **Endpoint**: `/metrics` on port 8090
- **Purpose**: Configure Prometheus to scrape metrics

## Metrics Exposed

All metrics include labels:
- `cluster_id` - OpenShift cluster ID
- `node` - Node name
- `device` - NVMe device (e.g., nvme1n1)
- `volume_id` - EBS volume ID

### Counter Metrics
- `ebs_volume_performance_exceeded_iops_total`
- `ebs_volume_performance_exceeded_throughput_total`
- `ebs_instance_performance_exceeded_iops_total`
- `ebs_instance_performance_exceeded_throughput_total`
- `ebs_total_read_ops_total`
- `ebs_total_write_ops_total`
- `ebs_total_read_bytes_total`
- `ebs_total_write_bytes_total`

### Gauge Metrics
- `ebs_volume_iops_exceeded_check`
- `ebs_volume_throughput_exceeded_check`
- `ebs_volume_queue_length`
- `ebs_volume_performance_exceeded_iops_percent`
- `ebs_volume_performance_exceeded_throughput_percent`
- `ebs_instance_performance_exceeded_iops_percent`
- `ebs_instance_performance_exceeded_throughput_percent`

## Accessing Metrics

### Via Prometheus

Metrics are automatically scraped by OpenShift's Prometheus via the ServiceMonitor.

Query in Prometheus:
```promql
# All EBS metrics
{__name__=~"ebs_.*"}

# Per-node metrics
ebs_volume_queue_length{node="worker-1"}

# Aggregate across nodes
sum(ebs_total_read_ops_total) by (volume_id)
```

### Via Port Forward

```bash
# Get a pod name
POD=$(kubectl get pods -n openshift-sre-ebs-metrics \
  -l app.kubernetes.io/component=ebs-metrics-exporter \
  -o jsonpath='{.items[0].metadata.name}')

# Port forward
kubectl port-forward -n openshift-sre-ebs-metrics $POD 8090:8090

# Query metrics
curl http://localhost:8090/metrics | grep ^ebs_
```

### Via Service (from within cluster)

```bash
# From any pod in the cluster
curl http://ebs-metrics-exporter.openshift-sre-ebs-metrics.svc:8090/metrics
```

## Configuration

### Change NVMe Device

Edit `deploy/30_*.DaemonSet.yaml`:
```yaml
env:
- name: EBS_DEVICE
  value: "/dev/nvme0n1"  # Change to your device
```

### Change Scrape Interval

Edit `deploy/50_*.ServiceMonitor.yaml`:
```yaml
endpoints:
- port: metrics
  interval: 60s  # Change from 30s to 60s
```

### Adjust Resources

Edit `deploy/30_*.DaemonSet.yaml`:
```yaml
resources:
  requests:
    cpu: 20m      # Increase if needed
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

## Makefile Targets

```bash
make build          # Build exporter binary
make docker-build   # Build container image
make docker-push    # Push container image
make deploy         # Deploy to cluster
make undeploy       # Remove from cluster
make run            # Run locally (requires sudo)
make help           # Show all targets
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
kubectl describe pod -n openshift-sre-ebs-metrics

# Check events
kubectl get events -n openshift-sre-ebs-metrics --sort-by='.lastTimestamp'

# Check SCC assignment
kubectl get pod <pod-name> -n openshift-sre-ebs-metrics -o yaml | grep scc
```

### No Metrics

```bash
# Check if pod can access device
kubectl exec -n openshift-sre-ebs-metrics <pod-name> -- ls -la /dev/nvme*

# Check logs
kubectl logs -n openshift-sre-ebs-metrics <pod-name>

# Test metrics endpoint
kubectl exec -n openshift-sre-ebs-metrics <pod-name> -- \
  curl -s localhost:8090/metrics | head -20
```

### Prometheus Not Scraping

```bash
# Check ServiceMonitor exists
kubectl get servicemonitor -n openshift-sre-ebs-metrics

# Check Service has endpoints
kubectl get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Check Prometheus can access namespace
kubectl get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s

# Check ServiceMonitor configuration
kubectl get servicemonitor ebs-metrics-exporter -n openshift-sre-ebs-metrics -o yaml
```

### Image Pull Issues

```bash
# Check image exists
docker pull <your-image>

# Check image in DaemonSet
kubectl get daemonset ebs-metrics-exporter -n openshift-sre-ebs-metrics -o yaml | grep image:

# Check ImagePullBackOff
kubectl describe pod -n openshift-sre-ebs-metrics | grep -A5 "Events:"
```

## Uninstallation

```bash
# Remove all resources
make undeploy
# or
kubectl delete -f deploy/

# Verify removal
kubectl get all -n openshift-sre-ebs-metrics
```

## References

- [Architecture Update](ARCHITECTURE_UPDATE.md) - Detailed architecture changes
- [Quick Start](QUICKSTART.md) - Quick deployment guide
- [Build Guide](BUILD.md) - Build instructions
- [README](README.md) - Main documentation

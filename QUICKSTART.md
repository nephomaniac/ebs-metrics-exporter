# EBS Metrics Exporter - Quick Start Guide

This guide will help you quickly deploy the EBS Metrics Exporter operator and DaemonSet.

## Prerequisites

- OpenShift cluster (4.x+)
- `oc` CLI installed and logged in with cluster-admin privileges
- Container registry access for pushing images
- Go 1.22+ (for building from source)

## Quick Deploy (Using Pre-built Images)

If you have pre-built images available:

### Step 1: Update Image References

Edit the following files to use your container images:

1. **Operator image** - `deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.Deployment.yaml`:
   ```yaml
   image: quay.io/your-org/ebs-metrics-collector-operator:latest
   ```

2. **Exporter image** - `k8s/07-daemonset.yaml`:
   ```yaml
   image: quay.io/your-org/ebs-metrics-exporter:latest
   ```

### Step 2: Deploy Everything

```bash
# Deploy operator
oc apply -f deploy/

# Deploy exporter DaemonSet
oc apply -f k8s/

# Wait for pods to be ready
oc wait --for=condition=ready pod -l app.kubernetes.io/name=ebs-metrics-collector-operator -n openshift-sre-ebs-metrics --timeout=120s
oc wait --for=condition=ready pod -l app.kubernetes.io/component=ebs-metrics-exporter -n openshift-sre-ebs-metrics --timeout=120s
```

### Step 3: Verify Deployment

```bash
# Check operator
oc get deployment -n openshift-sre-ebs-metrics ebs-metrics-collector-operator
oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator --tail=50

# Check DaemonSet
oc get daemonset -n openshift-sre-ebs-metrics ebs-metrics-exporter
oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# Test metrics endpoint
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_
```

## Build and Deploy from Source

### Step 1: Initialize Boilerplate (First Time Only)

**IMPORTANT**: On first checkout, initialize the build system:

```bash
# Initialize OpenShift boilerplate build system
make boilerplate-update

# Download Go dependencies
go mod download
```

This sets up standardized build tooling, CI configuration, and development workflows. See [BOILERPLATE.md](BOILERPLATE.md) for details.

### Step 2: Set Your Registry

```bash
export REGISTRY=quay.io/your-org
export OPERATOR_IMAGE=${REGISTRY}/ebs-metrics-collector-operator:latest
export EXPORTER_IMAGE=${REGISTRY}/ebs-metrics-exporter-daemonset:latest
```

### Step 3: Build Images

```bash
# Build and push operator image
make docker-build-operator IMG_OPERATOR=${OPERATOR_IMAGE}
make docker-push-operator IMG_OPERATOR=${OPERATOR_IMAGE}

# Build and push exporter image
make docker-build-exporter IMG_EXPORTER=${EXPORTER_IMAGE}
make docker-push-exporter IMG_EXPORTER=${EXPORTER_IMAGE}
```

### Step 4: Update Manifests

```bash
# Update operator image
sed -i "s|REPLACE_IMAGE|${OPERATOR_IMAGE}|g" deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.Deployment.yaml

# Update exporter image
sed -i "s|<IMAGE_REGISTRY>/<IMAGE_NAME>:<IMAGE_TAG>|${EXPORTER_IMAGE}|g" k8s/07-daemonset.yaml
```

### Step 5: Deploy

```bash
# Deploy operator
oc apply -f deploy/

# Deploy exporter
oc apply -f k8s/

# Verify
oc get all -n openshift-sre-ebs-metrics
```

## Configuration

### Customize EBS Device

By default, the exporter monitors `/dev/nvme1n1`. To change this:

Edit `k8s/07-daemonset.yaml`:
```yaml
env:
- name: EBS_DEVICE
  value: "/dev/nvme0n1"  # Change to your device
```

### Customize Scrape Interval

Edit `k8s/06-servicemonitor.yaml`:
```yaml
endpoints:
- port: metrics
  interval: 30s  # Change scrape interval
```

## Accessing Metrics

### Via Port Forward

```bash
# Operator metrics
oc port-forward -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator 8383:8383
curl http://localhost:8383/metrics

# Exporter pod metrics  
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090
curl http://localhost:8090/metrics
```

### Via Prometheus

Access the OpenShift Prometheus UI and query:

```promql
# Show all EBS metrics
{__name__=~"ebs_.*"}

# Volume IOPS exceeded percentage
ebs_volume_performance_exceeded_iops_percent

# Total read operations
sum(ebs_total_read_ops_total) by (volume_id)
```

## Uninstall

```bash
# Remove exporter DaemonSet
oc delete -f k8s/

# Remove operator
oc delete -f deploy/
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status
oc describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# Check SCC
oc get pod <pod-name> -n openshift-sre-ebs-metrics -o yaml | grep scc

# Manually test SCC
oc adm policy who-can use scc ebs-metrics-exporter
```

### No Metrics Appearing

```bash
# Check if exporter can access device
oc exec -n openshift-sre-ebs-metrics <pod-name> -- ls -la /dev/nvme*

# Check exporter logs
oc logs -n openshift-sre-ebs-metrics <pod-name>

# Test IOCTL access
oc exec -n openshift-sre-ebs-metrics <pod-name> -- /ebs-metrics-collector --device /dev/nvme1n1
```

### Prometheus Not Scraping

```bash
# Check ServiceMonitor
oc get servicemonitor -n openshift-sre-ebs-metrics -o yaml

# Check if Prometheus has RBAC
oc get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s

# Check Service endpoints
oc get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter
```

## Next Steps

- Review [README.operator.md](README.operator.md) for detailed architecture
- Read [BOILERPLATE.md](BOILERPLATE.md) for build system documentation
- See [BUILD.md](BUILD.md) for detailed build instructions
- Configure alerting rules based on metrics
- Set up Grafana dashboards for visualization
- Customize the operator for your specific needs

## Getting Help

- Check operator logs: `oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator`
- Check exporter logs: `oc logs -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter`
- Verify RBAC: `oc auth can-i --list --as=system:serviceaccount:openshift-sre-ebs-metrics:ebs-metrics-collector-operator`

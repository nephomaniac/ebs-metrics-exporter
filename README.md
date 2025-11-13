# EBS Metrics Exporter

A Prometheus exporter for Amazon EBS (Elastic Block Store) performance metrics. This Go application queries EBS NVMe device statistics via IOCTLs and exposes them as Prometheus metrics through an HTTP server.

This is a Go port of the Python `ebs_script.py` with integrated HTTP server for real-time metrics collection.

## Features

- Queries EBS volume performance metrics directly from NVMe devices
- Exposes metrics in Prometheus format via HTTP endpoint
- Tracks volume and instance IOPS/throughput limits
- Monitors read/write operations, bytes, and queue length
- Calculates percentage of time limits were exceeded
- Compatible with Prometheus scraping

## Metrics Exported

### Counter Metrics
- `ebs_volume_performance_exceeded_iops_total` - Total time (microseconds) volume IOPS limit was exceeded
- `ebs_volume_performance_exceeded_throughput_total` - Total time (microseconds) volume throughput limit was exceeded
- `ebs_instance_performance_exceeded_iops_total` - Total time (microseconds) instance IOPS limit was exceeded
- `ebs_instance_performance_exceeded_throughput_total` - Total time (microseconds) instance throughput limit was exceeded
- `ebs_total_read_ops_total` - Total number of read operations
- `ebs_total_write_ops_total` - Total number of write operations
- `ebs_total_read_bytes_total` - Total bytes read
- `ebs_total_write_bytes_total` - Total bytes written

### Gauge Metrics
- `ebs_volume_iops_exceeded_check` - Whether IOPS limit was exceeded (0 or 1)
- `ebs_volume_throughput_exceeded_check` - Whether throughput limit was exceeded (0 or 1)
- `ebs_volume_queue_length` - Current volume queue length
- `ebs_volume_performance_exceeded_iops_percent` - Percentage of time IOPS limit was exceeded in last interval
- `ebs_volume_performance_exceeded_throughput_percent` - Percentage of time throughput limit was exceeded in last interval
- `ebs_instance_performance_exceeded_iops_percent` - Percentage of time instance IOPS limit was exceeded
- `ebs_instance_performance_exceeded_throughput_percent` - Percentage of time instance throughput limit was exceeded

All metrics include labels:
- `device` - NVMe device name (e.g., "nvme1n1")
- `volume_id` - EBS volume ID (e.g., "vol-1234567890abcdef0")

## Deployment Options

This exporter can be deployed in two ways:

1. **Standalone Binary** - Run directly on EC2 instances (see below)
2. **OpenShift Operator** - Deploy as a DaemonSet in OpenShift clusters (see [OpenShift Deployment](#openshift-deployment))

## Building

For detailed build instructions, environment variables, and advanced build options, see **[BUILD.md](BUILD.md)**.

### Quick Build

```bash
# Build collector binary (standalone)
make build-collector

# Build operator binary (Kubernetes)
make build-operator

# Build both
make build
```

## Standalone Deployment

### Requirements

- Go 1.22 or later
- Linux system with NVMe EBS volumes
- Root/sudo access (required for NVMe IOCTLs)
- Amazon EC2 instance with EBS volumes

## Usage

```bash
# Run the exporter (requires root access for IOCTL operations)
sudo ./ebs-metrics-collector --device /dev/nvme1n1 --port 8090
```

### Command-line Flags

- `--device` - NVMe device to monitor (required, e.g., `/dev/nvme1n1`)
- `--port` - Port to listen on (default: `8090`)

### Example

```bash
# Start the exporter for /dev/nvme1n1 on port 9100
sudo ./ebs-metrics-collector --device /dev/nvme1n1 --port 9100
```

The exporter will start an HTTP server with two endpoints:
- `http://localhost:9100/` - Landing page with basic info
- `http://localhost:9100/metrics` - Prometheus metrics endpoint

## Prometheus Configuration

Add this job to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'ebs'
    static_configs:
      - targets: ['localhost:9100']
```

## OpenShift Deployment

This project supports two deployment architectures:

| Feature | DaemonSet (Direct) | Operator-Based |
|---------|-------------------|----------------|
| **Complexity** | Simple | Moderate |
| **Components** | DaemonSet only | Operator + DaemonSet |
| **Metrics Scope** | Per-node | Per-node + Cluster-wide |
| **Use Case** | Quick deployment, simple monitoring | Advanced monitoring, lifecycle management |
| **Recommended For** | Most deployments | Large clusters, centralized monitoring |

### Deployment Option 1: DaemonSet (Direct)

**Recommended for most users** - Simple, direct deployment of collector pods.

This deploys the collector as a DaemonSet that runs on every node with EBS volumes.

#### Prerequisites

- **OpenShift cluster** (4.x or later)
- **oc CLI** installed and logged in with cluster-admin privileges
- **Container registry** access (e.g., Quay.io)
- **Go 1.22+** (for building from source)
- **Docker or Podman** for container builds

#### 1. Build Container Images

For detailed build instructions, see **[BUILD.md](BUILD.md)**.

```bash
# Quick build
export IMG=quay.io/your-org/ebs-metrics-exporter:latest
make docker-build
make docker-push
```

#### 2. Update Image Reference

```bash
# Update the DaemonSet to use your image
sed -i "s|REPLACE_IMAGE|${IMG}|g" deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml
```

#### 3. Deploy to OpenShift

```bash
# Deploy all resources (ServiceAccount, SCC, DaemonSet, Service, ServiceMonitor)
make deploy

# Or manually:
oc apply -f deploy/
```

#### 4. Verify Deployment

```bash
# Check DaemonSet status
oc get daemonset -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Check running pods
oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# View logs
oc logs -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter --tail=50

# Test metrics endpoint
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_
```

### Deployment Option 2: Operator-Based

The operator-based deployment provides a Kubernetes operator that manages the DaemonSet lifecycle and exposes aggregated cluster-wide metrics.

#### Architecture

```
┌─────────────────────────────────────────┐
│         OpenShift Cluster                │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  Operator Deployment               │ │
│  │  - Manages DaemonSet lifecycle     │ │
│  │  - Exposes aggregated metrics      │ │
│  │  - Port 8383                       │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  EBS Collector DaemonSet           │ │
│  │  - Runs on every node              │ │
│  │  - Collects NVMe stats             │ │
│  │  - Port 8090                       │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

#### Prerequisites

- Same as DaemonSet deployment
- Operator image built and pushed to registry
- Collector (DaemonSet) image built and pushed to registry

#### Installation Steps

**1. Build Images**

```bash
# Build operator image
export IMG_OPERATOR=quay.io/your-org/ebs-metrics-exporter-operator:latest
make docker-build-operator
make docker-push-operator

# Build collector image
export IMG=quay.io/your-org/ebs-metrics-exporter:latest
make docker-build-collector
make docker-push-collector
```

**2. Create Namespace and RBAC**

```bash
# Create namespace
oc create namespace openshift-sre-ebs-metrics

# Label namespace for monitoring
oc label namespace openshift-sre-ebs-metrics openshift.io/cluster-monitoring=true
```

**3. Deploy the Operator**

Create operator deployment manifest `operator-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ebs-metrics-exporter-operator
  namespace: openshift-sre-ebs-metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ebs-metrics-exporter-operator
  template:
    metadata:
      labels:
        app: ebs-metrics-exporter-operator
    spec:
      serviceAccountName: ebs-metrics-exporter
      containers:
      - name: operator
        image: quay.io/your-org/ebs-metrics-exporter-operator:latest
        ports:
        - containerPort: 8383
          name: metrics
        - containerPort: 8081
          name: health
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

Deploy:
```bash
# Deploy ServiceAccount and RBAC
oc apply -f deploy/10_ebs-metrics-exporter.ServiceAccount.yaml
oc apply -f deploy/10_prometheus-k8s_openshift-sre-ebs-metrics.Role.yaml
oc apply -f deploy/20_prometheus-k8s_openshift-sre-ebs-metrics.RoleBinding.yaml

# Deploy operator
oc apply -f operator-deployment.yaml

# Verify operator is running
oc get deployment -n openshift-sre-ebs-metrics ebs-metrics-exporter-operator
oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-exporter-operator
```

**4. Deploy the Collector DaemonSet**

Update the DaemonSet image and deploy:

```bash
# Update image reference
sed -i "s|REPLACE_IMAGE|${IMG}|g" deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml

# Deploy remaining resources
oc apply -f deploy/20_ebs-metrics-exporter.SecurityContextConstraints.yaml
oc apply -f deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml
oc apply -f deploy/40_ebs-metrics-exporter_openshift-sre-ebs-metrics.Service.yaml
oc apply -f deploy/50_ebs-metrics-exporter_openshift-sre-ebs-metrics.ServiceMonitor.yaml

# Verify DaemonSet is running
oc get daemonset -n openshift-sre-ebs-metrics
oc get pods -n openshift-sre-ebs-metrics
```

**5. Verify Deployment**

```bash
# Check operator metrics (aggregated cluster-wide)
oc port-forward -n openshift-sre-ebs-metrics deployment/ebs-metrics-exporter-operator 8383:8383
curl http://localhost:8383/metrics

# Check collector pod metrics (per-node)
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090
curl http://localhost:8090/metrics
```

#### Operator Metrics vs DaemonSet Metrics

**Operator Metrics (Port 8383):**
- Aggregated cluster-wide EBS metrics
- Includes `cluster_id` label
- Single scrape endpoint for entire cluster
- Recommended for cluster-level monitoring

**DaemonSet Pod Metrics (Port 8090):**
- Per-node EBS metrics
- Includes `node`, `device`, `volume_id` labels
- Multiple endpoints (one per node)
- Useful for node-level debugging

For detailed operator architecture and development information, see **[README.operator.md](README.operator.md)**.

### Deployment Architecture (DaemonSet)

The OpenShift deployment creates:

- **Namespace**: `openshift-sre-ebs-metrics` with cluster monitoring enabled
- **ServiceAccount**: `ebs-metrics-exporter` for pod identity
- **SecurityContextConstraints**: Custom SCC for privileged access to NVMe devices
- **DaemonSet**: Runs exporter pod on every Linux node
- **Service**: Headless service for endpoint discovery
- **ServiceMonitor**: Configures Prometheus to scrape metrics automatically
- **RBAC**: Role and RoleBinding for Prometheus access

### Configuration

#### Customize NVMe Device

Edit `deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.DaemonSet.yaml`:

```yaml
env:
- name: EBS_DEVICE
  value: "/dev/nvme0n1"  # Change to your device
```

#### Adjust Resource Limits

```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

#### Change Scrape Interval

Edit `deploy/50_ebs-metrics-exporter_openshift-sre-ebs-metrics.ServiceMonitor.yaml`:

```yaml
endpoints:
- port: metrics
  interval: 60s  # Default is 30s
```

### Accessing Metrics

#### Via OpenShift Prometheus

Metrics are automatically scraped by OpenShift's cluster Prometheus. Access via the OpenShift Console:

1. Navigate to **Observe** → **Metrics**
2. Query examples:

```promql
# Show all EBS metrics
{__name__=~"ebs_.*"}

# Volume IOPS exceeded percentage by node
ebs_volume_performance_exceeded_iops_percent

# Total read operations per volume
sum(rate(ebs_total_read_ops_total[5m])) by (volume_id)

# Nodes with high queue length
ebs_volume_queue_length > 100
```

#### Via Port Forward

```bash
# Forward metrics port from a pod
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090

# Query metrics
curl http://localhost:8090/metrics
```

### Troubleshooting

#### Pods Not Starting

```bash
# Check pod status and events
oc describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# Verify SCC assignment
oc get pod -n openshift-sre-ebs-metrics -o yaml | grep -A5 scc

# Check SCC permissions
oc adm policy who-can use scc ebs-metrics-exporter
```

#### No Metrics Appearing

```bash
# Check device access
oc exec -n openshift-sre-ebs-metrics $POD -- ls -la /dev/nvme*

# View detailed logs
oc logs -n openshift-sre-ebs-metrics $POD -f

# Test IOCTL access manually
oc exec -n openshift-sre-ebs-metrics $POD -- /ebs-metrics-collector --device /dev/nvme1n1
```

#### Prometheus Not Scraping

```bash
# Verify ServiceMonitor exists
oc get servicemonitor -n openshift-sre-ebs-metrics

# Check Service endpoints
oc get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Verify Prometheus RBAC
oc get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s

# Check ServiceMonitor configuration
oc get servicemonitor ebs-metrics-exporter -n openshift-sre-ebs-metrics -o yaml
```

### Uninstall

```bash
# Remove all resources
make undeploy

# Or manually:
oc delete -f deploy/
```

### Additional Documentation

- **[BUILD.md](BUILD.md)** - Detailed build instructions, multi-arch builds, FIPS mode
- **[QUICKSTART.md](QUICKSTART.md)** - Quick deployment guide with step-by-step instructions
- **[DEPLOYMENT_SUMMARY.md](DEPLOYMENT_SUMMARY.md)** - Comprehensive deployment architecture reference
- **[BOILERPLATE.md](BOILERPLATE.md)** - OpenShift boilerplate system documentation

### Build Configuration

For detailed information about:
- Available Makefile targets
- Environment variables
- Build customization options
- Multi-architecture builds
- FIPS mode configuration

See **[BUILD.md](BUILD.md)**.

## Architecture

The exporter follows the Prometheus instrumentation best practices:

1. **Collector Pattern**: Implements `prometheus.Collector` interface
2. **On-Demand Collection**: Stats are queried when `/metrics` is scraped
3. **Thread-Safe**: Uses mutex for concurrent scrape safety
4. **Efficient**: Minimal overhead between scrapes

## License

Licensed under the MIT License. See LICENSE file for details.

## References

- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Instrumenting HTTP Server Tutorial](https://prometheus.io/docs/tutorials/instrumenting_http_server_in_go/)
- [AWS EBS Volume Metrics](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-cloudwatch-metrics.html)

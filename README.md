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

For production use in OpenShift clusters, deploy as a DaemonSet that runs on every node with EBS volumes.

### Prerequisites

- **OpenShift cluster** (4.x or later)
- **oc CLI** installed and logged in with cluster-admin privileges
- **Container registry** access (e.g., Quay.io)
- **Go 1.22+** (for building from source)
- **Docker or Podman** for container builds

### Quick Start

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

### Deployment Architecture

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

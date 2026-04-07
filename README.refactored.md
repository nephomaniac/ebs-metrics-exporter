# EBS Metrics Exporter (Simplified DaemonSet Architecture)

A lightweight Prometheus exporter for Amazon EBS (Elastic Block Store) performance metrics. Collects NVMe device statistics via IOCTLs and exposes them as Prometheus metrics.

**Architecture**: Single Go binary deployed as DaemonSet via Package Operator (PKO).

## Overview

This exporter provides visibility into EBS volume performance without using AWS CloudWatch APIs, reducing costs and network overhead. Each pod collects metrics from its node's NVMe devices via direct IOCTL calls.

### Key Features

- **Zero AWS API Calls** - Direct IOCTL access to NVMe device statistics
- **Low Resource Overhead** - ~10m CPU, ~32Mi memory per node
- **Simple Architecture** - Single binary, no operator reconciliation logic
- **PKO Deployment** - Managed via Package Operator for fleet-wide rollout
- **Per-Node Metrics** - Each pod exposes metrics for its node's EBS volumes
- **Prometheus Integration** - ServiceMonitor for automatic scraping

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

All metrics include labels:
- `device` - NVMe device name (e.g., "nvme1n1")
- `volume_id` - EBS volume ID (e.g., "vol-1234567890abcdef0")

## Architecture

### DaemonSet-Only Design

```
┌─────────────────────────────────────────┐
│         OpenShift Cluster                │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  EBS Metrics Exporter DaemonSet    │ │
│  │  - Runs on every node              │ │
│  │  - Collects NVMe stats via IOCTL   │ │
│  │  - Serves metrics on port 8090     │ │
│  │  - Scraped by Prometheus           │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  Prometheus (OpenShift Monitoring) │ │
│  │  - Scrapes each pod via            │ │
│  │    ServiceMonitor                  │ │
│  │  - Aggregates metrics cluster-wide │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### Why DaemonSet-Only (vs Operator Pattern)?

**Advantages:**
1. **Simpler** - One component (~150 LOC) vs two (~600 LOC)
2. **Direct metrics** - Prometheus handles aggregation
3. **Standard pattern** - Follows node-exporter model
4. **PKO Compatible** - No deployment advantage for operators with PKO
5. **Easier maintenance** - Less code, fewer failure modes

**Trade-offs:**
- No centralized cluster-wide metrics endpoint (Prometheus aggregates instead)
- No dynamic device discovery (uses static device path configuration)
- No custom reconciliation logic (relies on DaemonSet controller)

For this use case, the trade-offs are acceptable and the simplicity wins.

## Deployment

### Prerequisites

- OpenShift cluster with Package Operator (PKO) installed
- Container registry access (e.g., Quay.io)
- AWS EC2 instances with EBS volumes attached as NVMe devices

### PKO Deployment (Recommended)

Package Operator provides GitOps-based deployment across the fleet.

#### 1. Build and Push Image

```bash
export IMG=quay.io/your-org/ebs-metrics-exporter:v1.0.0
make docker-build
make docker-push
```

#### 2. Deploy via PKO

PKO deployment manifests are in `deploy_pko/`:

```bash
# PKO manifests structure
deploy_pko/
├── manifest.yaml                                    # Package definition
├── Namespace-openshift-sre-ebs-metrics.yaml        # Namespace with monitoring labels
├── ServiceAccount-ebs-metrics-exporter.yaml        # Service account
├── SecurityContextConstraints-ebs-metrics.yaml     # SCC with SYS_ADMIN capability
├── Role-prometheus-k8s.yaml                        # Prometheus RBAC
├── RoleBinding-prometheus-k8s.yaml                 # Prometheus access
├── DaemonSet-ebs-metrics-exporter.yaml.gotmpl      # Main workload (templated)
├── Service-ebs-metrics-exporter.yaml               # Headless service
└── ServiceMonitor-ebs-metrics-exporter.yaml        # Prometheus scrape config
```

**Configuration via PKO:**

The DaemonSet template supports PKO configuration:

```yaml
config:
  image: quay.io/your-org/ebs-metrics-exporter:v1.0.0
  device: /dev/nvme1n1  # NVMe device to monitor
```

Deploy through your PKO workflow (typically via app-interface GitOps).

#### 3. Verify Deployment

```bash
# Check DaemonSet status
oc get daemonset -n openshift-sre-ebs-metrics

# Check running pods
oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# View logs from a pod
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-sre-ebs-metrics $POD

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_
```

### Legacy YAML Deployment

For testing or non-PKO clusters:

```bash
# Deploy all resources
oc apply -f deploy/

# Verify
oc get daemonset -n openshift-sre-ebs-metrics ebs-metrics-exporter
```

## Local Development

### Building

```bash
# Build binary
make build

# Build for Linux (from macOS)
make build-linux

# Build container image
make docker-build

# Or using podman (macOS)
make podman-build
```

### Running Locally

**Note:** Requires root access for NVMe IOCTL operations.

```bash
# Build first
make build

# Run (requires sudo)
sudo ./bin/ebs-metrics-exporter --device /dev/nvme1n1 --port 8090

# In another terminal, test metrics
curl http://localhost:8090/metrics
```

### Testing

```bash
# Run unit tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Run linter
make vet

# Clean up dependencies
make tidy
```

## Configuration

### Command-Line Flags

```bash
./ebs-metrics-exporter --help
```

- `--device` - NVMe device path to monitor (default: `/dev/nvme1n1`)
- `--port` - Port to serve metrics on (default: `8090`)

### DaemonSet Configuration

Edit `deploy_pko/DaemonSet-ebs-metrics-exporter.yaml.gotmpl`:

**Change device path:**
```yaml
args:
- --device={{ .config.device }}  # Set via PKO config
```

**Adjust resources:**
```yaml
resources:
  requests:
    cpu: 10m
    memory: 32Mi
  limits:
    cpu: 100m
    memory: 128Mi
```

**Scrape interval (ServiceMonitor):**
```yaml
endpoints:
- port: metrics
  interval: 30s  # Adjust as needed
```

## Accessing Metrics

### Via Prometheus UI

Metrics are automatically scraped by OpenShift cluster Prometheus.

Access via OpenShift Console: **Observe** → **Metrics**

**Example queries:**

```promql
# Show all EBS metrics
{__name__=~"ebs_.*"}

# Volume IOPS exceeded by node
ebs_volume_performance_exceeded_iops_total

# Total read operations rate per volume
rate(ebs_total_read_ops_total[5m])

# Nodes with high queue length
ebs_volume_queue_length > 100

# Throughput exceeded percentage (derived)
rate(ebs_volume_performance_exceeded_throughput_total[5m]) / 1000000 * 100
```

### Via Port Forward

```bash
# Forward metrics port from a pod
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090

# Query metrics
curl http://localhost:8090/metrics

# View landing page
open http://localhost:8090
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod status and events
oc describe pod -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# Verify SCC assignment
oc get pod -n openshift-sre-ebs-metrics -o yaml | grep -A5 'openshift.io/scc'

# Check if SCC allows SYS_ADMIN capability
oc get scc ebs-metrics-exporter -o yaml | grep -A5 allowedCapabilities
```

### No Metrics / Device Access Errors

```bash
# Check device exists
oc exec -n openshift-sre-ebs-metrics $POD -- ls -la /dev/nvme*

# View detailed logs
oc logs -n openshift-sre-ebs-metrics $POD -f

# Check security context
oc get pod $POD -n openshift-sre-ebs-metrics -o json | jq '.spec.containers[0].securityContext'
```

### Prometheus Not Scraping

```bash
# Verify ServiceMonitor exists
oc get servicemonitor -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Check Service endpoints
oc get endpoints -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Verify Prometheus RBAC
oc get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s

# Check Prometheus targets (from Prometheus UI)
# Navigate to: Status → Targets → openshift-sre-ebs-metrics/ebs-metrics-exporter
```

### Device Path Not Found

If your EBS volumes use a different device path:

```bash
# List all NVMe devices on a node
oc debug node/<node-name>
chroot /host
ls -la /dev/nvme*

# Update PKO config with correct device path
# Or set via DaemonSet env variable
```

## Project Structure

```
ebs-metrics-exporter/
├── main.go                          # Simple exporter (150 LOC)
├── config/
│   └── config.go                    # Constants (operator name, namespace)
├── pkg/
│   ├── collector/
│   │   └── collector.go             # Prometheus collector interface
│   └── nvme/
│       └── nvme.go                  # NVMe IOCTL logic
├── deploy_pko/                      # PKO deployment manifests
│   ├── manifest.yaml                # Package definition
│   ├── Namespace-*.yaml
│   ├── ServiceAccount-*.yaml
│   ├── SecurityContextConstraints-*.yaml
│   ├── Role-*.yaml
│   ├── RoleBinding-*.yaml
│   ├── DaemonSet-*.yaml.gotmpl      # Templated DaemonSet
│   ├── Service-*.yaml
│   └── ServiceMonitor-*.yaml
├── deploy/                          # Legacy YAML manifests
├── Dockerfile                       # Container build
├── Makefile                         # Build targets
├── go.mod / go.sum                  # Go dependencies
└── boilerplate/                     # OpenShift boilerplate
```

## Cost Analysis

See [COST_ANALYSIS_TODO.md](COST_ANALYSIS_TODO.md) for planned cost comparison vs CloudWatch-based collection.

**Expected savings:**
- Zero AWS CloudWatch API costs
- Zero CloudWatch metrics storage costs
- Reduced network egress (metrics aggregated locally before forwarding)

## Comparison: Previous vs Refactored Architecture

| Aspect | Previous (Operator + DaemonSet) | Refactored (DaemonSet-Only) |
|--------|----------------------------------|------------------------------|
| **Components** | Operator Deployment + DaemonSet | DaemonSet only |
| **Lines of Code** | ~600 (operator) + ~200 (collector) | ~150 total |
| **Metrics Endpoints** | 2 (operator port 8383 + pods port 8090) | 1 (pods port 8090) |
| **Cluster-wide Metrics** | Operator aggregates | Prometheus aggregates |
| **Resource Overhead** | +1 operator pod per cluster | 0 extra pods |
| **Complexity** | Medium (reconciliation loops) | Low (static DaemonSet) |
| **PKO Deployment** | No advantage | Standard PKO pattern |
| **Maintenance** | Higher (more code) | Lower (less code) |

## Contributing

- Code follows standard Go formatting (`make fmt`)
- Run tests before committing (`make test`)
- Update boilerplate periodically (`make boilerplate-update`)

## License

Licensed under the Apache License 2.0. See LICENSE file for details.

## References

- [AWS EBS NVMe Statistics](https://docs.aws.amazon.com/ebs/latest/userguide/nvme-detailed-performance-stats.html)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [OpenShift Boilerplate](https://github.com/openshift/boilerplate)
- [Package Operator (PKO)](https://github.com/package-operator/package-operator)

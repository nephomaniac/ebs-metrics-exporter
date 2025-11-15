# EBS Metrics Exporter Operator

A Kubernetes operator that manages the deployment and lifecycle of the EBS Metrics Exporter DaemonSet in OpenShift clusters.

## Architecture

This project follows the operator pattern inspired by [osd-metrics-exporter](https://github.com/openshift/osd-metrics-exporter):

```
┌─────────────────────────────────────────────────────────────────┐
│                   OpenShift Cluster                              │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  openshift-sre-ebs-metrics namespace                       │  │
│  │                                                            │  │
│  │  ┌──────────────────────────────┐                         │  │
│  │  │  Operator Deployment         │                         │  │
│  │  │  ┌────────────────────────┐  │                         │  │
│  │  │  │ Controllers:           │  │                         │  │
│  │  │  │  - DaemonSet Controller│  │                         │  │
│  │  │  │    (watches DS pods)   │  │                         │  │
│  │  │  │                        │  │                         │  │
│  │  │  │ Metrics Aggregator:    │  │      ┌──────────────┐  │  │
│  │  │  │  - Exposes /metrics    │◄─┼──────┤ Prometheus   │  │  │
│  │  │  │  - Port 8383           │  │      │ (scrapes)    │  │  │
│  │  │  └────────────────────────┘  │      └──────────────┘  │  │
│  │  └──────────────────────────────┘                         │  │
│  │                                                            │  │
│  │  ┌──────────────────────────────┐                         │  │
│  │  │  EBS Exporter DaemonSet      │                         │  │
│  │  │  ┌────────────┐ ┌──────────┐ │                         │  │
│  │  │  │ Pod (Node1)│ │Pod (N...)│ │                         │  │
│  │  │  │ - Collects │ │- Collects│ │                         │  │
│  │  │  │   NVMe     │ │  NVMe    │ │                         │  │
│  │  │  │   stats    │ │  stats   │ │                         │  │
│  │  │  │ - Port 8090│ │- Port    │ │                         │  │
│  │  │  └────────────┘ └──────────┘ │                         │  │
│  │  └──────────────────────────────┘                         │  │
│  └────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Components

1. **Operator Deployment** (`deploy/` directory)
   - Manages the EBS metrics exporter DaemonSet lifecycle
   - Watches DaemonSet pods and monitors their health
   - Exposes aggregated metrics on port 8383
   - Single replica deployment

2. **EBS Exporter DaemonSet** (`k8s/` directory)
   - Runs on every node in the cluster
   - Collects NVMe device statistics using IOCTLs
   - Exposes per-node metrics on port 8090
   - Requires privileged access to `/dev`

3. **Metrics Aggregator** (`pkg/metrics/`)
   - Singleton pattern for collecting metrics
   - Thread-safe metric updates
   - Exposes Prometheus-compatible metrics endpoint

## Project Structure

```
ebs-metrics-exporter/
├── main.go                          # Original NVMe exporter (DaemonSet)
├── config/
│   └── config.go                    # Operator configuration constants
├── controllers/
│   └── daemonset/
│       └── daemonset_controller.go  # DaemonSet lifecycle controller
├── pkg/
│   └── metrics/
│       ├── metrics.go               # Metrics singleton
│       └── aggregator.go            # EBS metrics aggregation
├── deploy/                          # Operator manifests
│   ├── 10_*.ServiceAccount.yaml
│   ├── 10_*.Role.yaml
│   ├── 10_*.ClusterRole.yaml
│   ├── 20_*.RoleBinding.yaml
│   ├── 20_*.ClusterRoleBinding.yaml
│   └── 30_*.Deployment.yaml
├── k8s/                             # Exporter DaemonSet manifests
│   ├── 01-namespace.yaml
│   ├── 02-securitycontextconstraints.yaml
│   ├── 03-serviceaccount.yaml
│   ├── 04-prometheus-role.yaml
│   ├── 05-service.yaml
│   ├── 06-servicemonitor.yaml
│   └── 07-daemonset.yaml
├── Dockerfile                       # DaemonSet exporter image
├── Dockerfile.operator              # Operator image
├── Makefile.operator                # Build automation
└── go.mod                           # Go dependencies
```

## Metrics Exposed

All metrics include labels: `cluster_id`, `node`, `device`, `volume_id`

### Counter Metrics
- `ebs_volume_performance_exceeded_iops_total` - Volume IOPS limit exceeded (μs)
- `ebs_volume_performance_exceeded_throughput_total` - Volume throughput limit exceeded (μs)
- `ebs_instance_performance_exceeded_iops_total` - Instance IOPS limit exceeded (μs)
- `ebs_instance_performance_exceeded_throughput_total` - Instance throughput limit exceeded (μs)
- `ebs_total_read_ops_total` - Total read operations
- `ebs_total_write_ops_total` - Total write operations
- `ebs_total_read_bytes_total` - Total bytes read
- `ebs_total_write_bytes_total` - Total bytes written

### Gauge Metrics
- `ebs_volume_iops_exceeded_check` - IOPS limit exceeded in interval (0/1)
- `ebs_volume_throughput_exceeded_check` - Throughput limit exceeded in interval (0/1)
- `ebs_volume_queue_length` - Current volume queue length
- `ebs_volume_performance_exceeded_iops_percent` - IOPS exceeded percentage
- `ebs_volume_performance_exceeded_throughput_percent` - Throughput exceeded percentage
- `ebs_instance_performance_exceeded_iops_percent` - Instance IOPS exceeded percentage
- `ebs_instance_performance_exceeded_throughput_percent` - Instance throughput exceeded percentage

## Building

### Build the Operator

```bash
# Build operator binary
make -f Makefile.operator build

# Build operator container image
make -f Makefile.operator docker-build-operator IMG_OPERATOR=<your-registry>/ebs-metrics-collector-operator:latest

# Push operator image
make -f Makefile.operator docker-push-operator IMG_OPERATOR=<your-registry>/ebs-metrics-collector-operator:latest
```

### Build the Exporter DaemonSet

```bash
# Build exporter binary
go build -o ebs-metrics-exporter main.go

# Build exporter container image
make -f Makefile.operator docker-build-exporter IMG_EXPORTER=<your-registry>/ebs-metrics-exporter:latest

# Push exporter image
make -f Makefile.operator docker-push-exporter IMG_EXPORTER=<your-registry>/ebs-metrics-exporter:latest
```

### Build Both

```bash
make -f Makefile.operator docker-build-all IMG_OPERATOR=<operator-image> IMG_EXPORTER=<exporter-image>
make -f Makefile.operator docker-push-all IMG_OPERATOR=<operator-image> IMG_EXPORTER=<exporter-image>
```

## Deployment

### Prerequisites

1. OpenShift cluster with cluster-admin access
2. Container images built and pushed to a registry
3. Nodes with EBS volumes attached as NVMe devices

### Deploy the Operator

1. Update the operator image in `deploy/30_ebs-metrics-exporter_openshift-sre-ebs-metrics.Deployment.yaml`:
   ```yaml
   image: <your-registry>/ebs-metrics-collector-operator:latest
   ```

2. Deploy the operator:
   ```bash
   oc apply -f deploy/
   ```

3. Verify the operator is running:
   ```bash
   oc get deployment -n openshift-sre-ebs-metrics
   oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator
   ```

### Deploy the Exporter DaemonSet

1. Update the exporter image in `k8s/07-daemonset.yaml`:
   ```yaml
   image: <your-registry>/ebs-metrics-exporter:latest
   ```

2. Update the `EBS_DEVICE` environment variable if needed (default: `/dev/nvme1n1`)

3. Deploy the DaemonSet:
   ```bash
   oc apply -f k8s/
   ```

4. Verify the DaemonSet is running:
   ```bash
   oc get daemonset -n openshift-sre-ebs-metrics
   oc get pods -n openshift-sre-ebs-metrics
   ```

## Accessing Metrics

### Operator Metrics
```bash
# Port-forward to operator pod
oc port-forward -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator 8383:8383

# Query metrics
curl http://localhost:8383/metrics
```

### DaemonSet Pod Metrics
```bash
# Port-forward to a specific pod
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090

# Query metrics
curl http://localhost:8090/metrics
```

### Prometheus Integration

The ServiceMonitor automatically configures Prometheus to scrape both:
1. Operator metrics endpoint (port 8383)
2. DaemonSet pod metrics endpoints (port 8090)

View in Prometheus:
```
https://<prometheus-url>/graph
```

Query examples:
```promql
# Volume IOPS exceeded
ebs_volume_performance_exceeded_iops_percent{cluster_id="<cluster-id>"}

# Total read operations across all volumes
sum(ebs_total_read_ops_total)

# Volumes exceeding throughput limits
ebs_volume_throughput_exceeded_check == 1
```

## Development

### Running Locally

```bash
# Ensure you have access to an OpenShift cluster
oc login <cluster-url>

# Run the operator locally
make -f Makefile.operator run
```

### Testing

```bash
# Run tests
make -f Makefile.operator test

# Run with verbose output
go test -v ./...
```

### Code Formatting

```bash
# Format code
make -f Makefile.operator fmt

# Run linter
make -f Makefile.operator vet
```

## RBAC Permissions

### Operator Permissions

The operator requires:
- **ClusterRole**: Read access to `ClusterVersion` (for cluster ID)
- **Role** (in `openshift-sre-ebs-metrics`):
  - Full access to pods, services, endpoints, configmaps, secrets
  - Full access to daemonsets, deployments, replicasets
  - Create/update access to servicemonitors

### Prometheus Permissions

Prometheus requires:
- **Role** (in `openshift-sre-ebs-metrics`):
  - Read access to services, endpoints, pods (for scraping)

## Troubleshooting

### Operator not starting
```bash
# Check operator logs
oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-collector-operator

# Check for RBAC issues
oc auth can-i get clusterversions --as=system:serviceaccount:openshift-sre-ebs-metrics:ebs-metrics-collector-operator
```

### DaemonSet pods failing
```bash
# Check pod logs
oc logs -n openshift-sre-ebs-metrics -l app.kubernetes.io/component=ebs-metrics-exporter

# Check SCC assignment
oc get pod <pod-name> -n openshift-sre-ebs-metrics -o yaml | grep scc

# Verify device access
oc debug node/<node-name>
chroot /host
ls -la /dev/nvme*
```

### Metrics not appearing in Prometheus
```bash
# Check ServiceMonitor
oc get servicemonitor -n openshift-sre-ebs-metrics

# Check Service endpoints
oc get endpoints -n openshift-sre-ebs-metrics

# Verify Prometheus can access the namespace
oc get rolebinding -n openshift-sre-ebs-metrics prometheus-k8s
```

## References

- [osd-metrics-exporter](https://github.com/openshift/osd-metrics-exporter) - Inspiration for operator pattern
- [Operator SDK](https://sdk.operatorframework.io/) - Operator development framework
- [operator-custom-metrics](https://github.com/openshift/operator-custom-metrics) - Metrics library
- [AWS EBS Performance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-io-characteristics.html) - EBS I/O characteristics

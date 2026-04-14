# EBS Metrics Exporter

A lightweight Prometheus exporter for Amazon EBS (Elastic Block Store) performance metrics. Collects NVMe device statistics via IOCTLs and exposes them as Prometheus metrics.

**Architecture**: Single Go binary deployed as DaemonSet via Package Operator (PKO).

## Exported Metrics

### Raw Metrics (Direct from NVMe Device)

| Metric Name | Type | Description | Labels |
|-------------|------|-------------|--------|
| `ebs_volume_performance_exceeded_iops_total` | Counter | Cumulative microseconds volume exceeded IOPS limits | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_volume_performance_exceeded_throughput_total` | Counter | Cumulative microseconds volume exceeded throughput limits | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_instance_performance_exceeded_iops_total` | Counter | Cumulative microseconds instance exceeded IOPS limits | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_instance_performance_exceeded_throughput_total` | Counter | Cumulative microseconds instance exceeded throughput limits | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_read_ops_total` | Counter | Total read operations | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_write_ops_total` | Counter | Total write operations | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_read_bytes_total` | Counter | Total bytes read | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_write_bytes_total` | Counter | Total bytes written | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_read_time_total` | Counter | Cumulative microseconds spent reading | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_total_write_time_total` | Counter | Cumulative microseconds spent writing | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |
| `ebs_volume_queue_length` | Gauge | Current volume queue length | `volume_id`, `volume_type`, `pvc_namespace`*, `pvc_name`* |

\* Only present for PVC-backed volumes (`volume_type="pvc"`). Root volumes have `volume_type="root"`.

### CloudWatch-Compatible Derived Metrics

Automatically generated via Prometheus recording rules for CloudWatch migration and simple alerting:

| Metric Name | Type | Description | CloudWatch Equivalent |
|-------------|------|-------------|----------------------|
| `ebs_volume_throughput_exceeded_check` | Binary (0/1) | 1 if volume exceeded throughput >30s in last minute | `VolumeThroughputExceededCheck` (Oct 2024) |
| `ebs_volume_iops_exceeded_check` | Binary (0/1) | 1 if volume exceeded IOPS >30s in last minute | `VolumeIOPSExceededCheck` (Oct 2024) |
| `ebs_instance_throughput_exceeded_check` | Binary (0/1) | 1 if instance exceeded throughput >30s in last minute | `InstanceEBSThroughputExceededCheck` (Oct 2025) |
| `ebs_instance_iops_exceeded_check` | Binary (0/1) | 1 if instance exceeded IOPS >30s in last minute | `InstanceEBSIOPSExceededCheck` (Oct 2025) |
| `ebs_volume_throughput_exceeded_percent` | Percentage | % of time exceeding throughput (5-min avg) | — |
| `ebs_volume_iops_exceeded_percent` | Percentage | % of time exceeding IOPS (5-min avg) | — |
| `ebs_instance_throughput_exceeded_percent` | Percentage | % of time exceeding instance throughput (5-min avg) | — |
| `ebs_instance_iops_exceeded_percent` | Percentage | % of time exceeding instance IOPS (1-min avg) | — |

See **[CloudWatch-Compatible Metrics Guide](docs/cloudwatch-compatible-metrics.md)** for detailed comparison and migration instructions.

**AWS CloudWatch Documentation:**
- [Volume-level metrics (Oct 2024)](https://aws.amazon.com/about-aws/whats-new/2024/10/amazon-cloudwatch-ebs-volumes-exceeding-performance/) - `VolumeThroughputExceededCheck`, `VolumeIOPSExceededCheck`
- [Instance-level metrics (Oct 2025)](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-cloudwatch-metrics-monitor-ec2-instances-i-o-performance/) - `InstanceEBSThroughputExceededCheck`, `InstanceEBSIOPSExceededCheck`
- [CloudWatch metrics for Amazon EBS](https://docs.aws.amazon.com/ebs/latest/userguide/using_cloudwatch_ebs.html) - Complete reference

---

## 📚 Documentation

### Getting Started
- **[Quick Start Guide](docs/QUICKSTART.md)** - Get up and running in minutes (local development)
- **[Configuration Guide](CONFIG.md)** - Complete configuration reference
- **[Deployment Guide](DEPLOYMENT.md)** - Production deployment procedures

### Development
- **[Advanced Workflows](docs/ADVANCED_WORKFLOWS.md)** - Step-by-step build and deployment workflows
- **[Testing Guide](docs/TESTING.md)** - Unit and integration testing procedures
- **[Makefile Targets](docs/MAKEFILE.md)** - Complete reference of all make targets
- **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions

### Operations
- **[Operations Guide](OPERATIONS.md)** - Day-2 operations, config updates, PKO behavior
- **[Drift Prevention](DRIFT-PREVENTION.md)** - Immutable deployments, GitOps enforcement

### Deployment & Build
- **[PKO Build Guide](PKO_BUILD_GUIDE.md)** - Production CI/CD with Konflux and Package Operator
- **[Build Guide](BUILD.md)** - Detailed build instructions and options
- **[Deployment Summary](DEPLOYMENT_SUMMARY.md)** - Comprehensive deployment architecture

### Architecture & Design
- **[Refactoring Summary](REFACTORING_SUMMARY.md)** - Architecture decisions and rationale
- **[Architecture Update](ARCHITECTURE_UPDATE.md)** - Technical architecture details

### Legacy Documentation
- **[README (Refactored)](README.refactored.md)** - Previous iteration documentation
- **[README (Operator)](README.operator.md)** - Operator pattern documentation (archived)
- **[OLM Deployment](OLM.md)** - Legacy OLM deployment (archived)
- **[Boilerplate Integration](BOILERPLATE_INTEGRATION_SUMMARY.md)** - Boilerplate framework details
- **[Boilerplate Guide](BOILERPLATE.md)** - Boilerplate system documentation

---

## Quick Start

```bash
# 1. Set your Quay.io username
export IMAGE_REPOSITORY=your-quay-username

# 2. Build and test (no cluster required)
make local-ci ALLOW_DIRTY_CHECKOUT=true

# 3. Full workflow: build → test → push → deploy
oc login https://api.your-cluster.com
podman login quay.io
make local ALLOW_DIRTY_CHECKOUT=true

# 4. Verify deployment
make local-status
make local-logs

# 5. Clean up
make local-undeploy
```

See **[Quick Start Guide](docs/QUICKSTART.md)** for detailed instructions and prerequisites.

---

## Architecture

### Why DaemonSet-Only?

This project uses a simplified architecture compared to traditional operators:

**Before** (Operator Pattern):
- Operator Deployment (1 pod) + DaemonSet (N pods)
- ~800 LOC
- Centralized metrics aggregation

**Now** (DaemonSet-Only):
- DaemonSet only (N pods)
- ~150 LOC
- Prometheus handles aggregation

**Benefits:**
- 83% less code
- Simpler to understand and maintain
- Standard node-exporter pattern
- Optimized for PKO deployment

For detailed rationale, see [REFACTORING_SUMMARY.md](REFACTORING_SUMMARY.md).

### Project Structure

```
ebs-metrics-exporter/
├── main.go                          # Exporter entry point (~150 LOC)
├── config/
│   └── config.go                    # Constants (name, namespace)
├── pkg/
│   ├── collector/
│   │   └── collector.go             # Prometheus collector
│   └── nvme/
│       └── nvme.go                  # NVMe IOCTL logic
├── deploy_pko/                      # PKO deployment manifests
│   ├── manifest.yaml                # PKO package definition
│   ├── Namespace-*.yaml
│   ├── ServiceAccount-*.yaml
│   ├── SecurityContextConstraints-*.yaml
│   ├── Role-*.yaml
│   ├── RoleBinding-*.yaml
│   ├── DaemonSet-*.yaml.gotmpl      # Templated DaemonSet
│   ├── Service-*.yaml
│   └── ServiceMonitor-*.yaml
├── deploy/                          # Legacy YAML manifests
├── build/
│   └── Dockerfile.pko               # PKO package Dockerfile
├── .tekton/                         # Konflux CI/CD pipelines
├── Dockerfile                       # Application Dockerfile
├── Makefile                         # Build and deployment targets
└── README.md                        # This file
```

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test locally: `make local-ci ALLOW_DIRTY_CHECKOUT=true`
5. Run linter: `make go-check`
6. Commit with descriptive message
7. Push and create a pull request

---

## License

Licensed under the Apache License 2.0. See LICENSE file for details.

---

## References

- **JIRA**: [SREP-2082](https://redhat.atlassian.net/browse/SREP-2082)
- **AWS EBS Stats**: https://docs.aws.amazon.com/ebs/latest/userguide/nvme-detailed-performance-stats.html
- **Prometheus Go Client**: https://github.com/prometheus/client_golang
- **Package Operator**: https://package-operator.run/
- **OpenShift Boilerplate**: https://github.com/openshift/boilerplate

---

## Help

```bash
# Show all make targets
make help

# Get help with oc commands
oc --help

# View cluster resources
oc get all -n openshift-sre-ebs-metrics
```

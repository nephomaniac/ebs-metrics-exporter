# EBS Metrics Exporter Configuration

The EBS Metrics Exporter uses a ConfigMap for runtime configuration. All settings are optional - the exporter provides sensible defaults for auto-discovery and monitoring.

## Configuration File Location

The exporter reads configuration from `/etc/ebs-exporter/config.yaml` inside the container, mounted from the ConfigMap.

## Default Behavior (No ConfigMap)

If no ConfigMap is provided, the exporter:
- Auto-discovers all Amazon EBS volumes on the node via NVMe vendor ID (0x1D0F)
- Monitors all discovered EBS volumes
- Exports all available metrics
- Polls devices every 30 seconds

## Configuration Schema

### Device Discovery

Controls how EBS volumes are discovered and which devices to monitor.

```yaml
deviceDiscovery:
  mode: auto  # auto | explicit | disabled
  
  autoFilter:
    includeVolumeIDs: []
    excludeVolumeIDs: []
    includeDevices: []
    excludeDevices: []
  
  explicitDevices: []
```

**Discovery Modes:**

- **`auto`** (default): Automatically discover EBS volumes by:
  1. Enumerating `/dev/nvme*n*` block devices
  2. Checking NVMe vendor ID (0x1D0F = Amazon)
  3. Verifying model name contains "Amazon Elastic Block Store"
  4. Extracting volume ID from vendor-specific data

- **`explicit`**: Only monitor devices listed in `explicitDevices` (no auto-discovery)

- **`disabled`**: Do not monitor any devices (for testing/troubleshooting)

### Metric Collection

Controls which metrics are exported and collection frequency.

```yaml
metrics:
  include: []  # Whitelist (empty = all metrics)
  exclude: []  # Blacklist
  pollingIntervalSeconds: 30
```

**Metric Filtering:**
- Cannot specify both `include` and `exclude` (validation error)
- Empty `include` and `exclude` = export all metrics (default)
- Glob patterns supported: `ebs_volume_*`, `ebs_instance_*`

**Available Metrics:**
- `ebs_volume_performance_exceeded_iops_total`
- `ebs_volume_performance_exceeded_throughput_total`
- `ebs_instance_performance_exceeded_iops_total`
- `ebs_instance_performance_exceeded_throughput_total`
- `ebs_total_read_ops_total`
- `ebs_total_write_ops_total`
- `ebs_total_read_bytes_total`
- `ebs_total_write_bytes_total`
- `ebs_volume_queue_length`

### Advanced Settings

```yaml
advanced:
  logLevel: info  # debug | info | warn | error
  maxDevices: 20  # Safety limit (1-100)
  deviceOpenTimeoutSeconds: 5
```

## Configuration Examples

### Example 1: Default Auto-Discovery

Monitor all EBS volumes with default settings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ebs-metrics-exporter-config
  namespace: openshift-sre-ebs-metrics
data:
  config.yaml: |
    deviceDiscovery:
      mode: auto
    metrics:
      pollingIntervalSeconds: 30
```

### Example 2: Exclude Root Volume

Monitor all EBS volumes except the root volume:

```yaml
data:
  config.yaml: |
    deviceDiscovery:
      mode: auto
      autoFilter:
        excludeDevices:
          - /dev/nvme0n1  # Root volume
```

### Example 3: Monitor Only Specific Volume IDs

Only monitor data volumes by volume ID:

```yaml
data:
  config.yaml: |
    deviceDiscovery:
      mode: auto
      autoFilter:
        includeVolumeIDs:
          - vol-abc123456789  # Data volume 1
          - vol-def987654321  # Data volume 2
```

### Example 4: Explicit Device List

Manually specify devices (bypass auto-discovery):

```yaml
data:
  config.yaml: |
    deviceDiscovery:
      mode: explicit
      explicitDevices:
        - devicePath: /dev/nvme1n1
          volumeID: vol-abc123  # Optional
        - devicePath: /dev/nvme2n1
```

### Example 5: Export Only Throttling Metrics

Only export IOPS and throughput exceeded metrics:

```yaml
data:
  config.yaml: |
    deviceDiscovery:
      mode: auto
    metrics:
      include:
        - ebs_volume_performance_exceeded_*
        - ebs_instance_performance_exceeded_*
```

### Example 6: Exclude Instance-Level Metrics

Export all metrics except instance-level throttling:

```yaml
data:
  config.yaml: |
    metrics:
      exclude:
        - ebs_instance_*
```

### Example 7: High-Frequency Polling for Debugging

Poll every 10 seconds with debug logging:

```yaml
data:
  config.yaml: |
    metrics:
      pollingIntervalSeconds: 10
    advanced:
      logLevel: debug
```

## Applying Configuration Changes

1. **Create or update the ConfigMap:**

```bash
kubectl apply -f configmap.yaml -n openshift-sre-ebs-metrics
```

2. **Restart the DaemonSet to pick up changes:**

```bash
kubectl rollout restart daemonset/ebs-metrics-exporter -n openshift-sre-ebs-metrics
```

Or delete pods to force recreation:

```bash
kubectl delete pods -l app.kubernetes.io/name=ebs-metrics-exporter -n openshift-sre-ebs-metrics
```

## Validation

The exporter validates configuration on startup and will fail to start if:
- Invalid discovery mode
- Both `include` and `exclude` metrics specified
- Polling interval < 1 or > 3600 seconds
- Invalid log level
- maxDevices < 1 or > 100

Check logs for validation errors:

```bash
kubectl logs -n openshift-sre-ebs-metrics daemonset/ebs-metrics-exporter
```

## Troubleshooting

### No devices found

Check logs for discovery details:
```bash
kubectl logs -n openshift-sre-ebs-metrics <pod-name> | grep discovered
```

Verify NVMe devices exist on node:
```bash
kubectl debug node/<node-name> -- chroot /host ls -la /dev/nvme*
```

### Metrics not appearing

1. Check metric filtering configuration
2. Verify devices are being monitored: `kubectl logs <pod>`
3. Check `/metrics` endpoint: `kubectl exec <pod> -- curl localhost:8090/metrics`

### Configuration not applied

1. Verify ConfigMap exists: `kubectl get cm -n openshift-sre-ebs-metrics`
2. Verify ConfigMap is mounted: `kubectl describe pod <pod-name>`
3. Check pod was restarted after ConfigMap change
4. Check logs for config validation errors

## Migration from --device Flag

**Legacy approach (deprecated):**
```yaml
args:
  - --device=/dev/nvme1n1
```

**New approach (ConfigMap):**
```yaml
data:
  config.yaml: |
    deviceDiscovery:
      mode: explicit
      explicitDevices:
        - devicePath: /dev/nvme1n1
```

The `--device` flag still works but will be removed in v1.0. Use ConfigMap instead.

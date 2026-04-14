# CloudWatch-Compatible Metrics

The EBS Metrics Exporter provides both **precise raw metrics** and **CloudWatch-compatible derived metrics** via Prometheus recording rules.

## Overview

| Metric Type | Precision | Use Case |
|-------------|-----------|----------|
| **Raw Metrics** | Microseconds (cumulative) | Detailed analysis, capacity planning, trending |
| **Compatible Metrics** | Binary (0 or 1) | Simple alerting, CloudWatch migration |

---

## CloudWatch-Compatible Recording Rules

These metrics are automatically generated every 60 seconds via PrometheusRule:

### Volume-Level Metrics

| Metric Name | CloudWatch Equivalent | Description |
|-------------|----------------------|-------------|
| `ebs_volume_throughput_exceeded_check` | `VolumeThroughputExceededCheck` | 1 if volume exceeded throughput >30s in last minute |
| `ebs_volume_iops_exceeded_check` | `VolumeIOPSExceededCheck` | 1 if volume exceeded IOPS >30s in last minute |

### Instance-Level Metrics (Not in CloudWatch)

| Metric Name | CloudWatch Equivalent | Description |
|-------------|----------------------|-------------|
| `ebs_instance_throughput_exceeded_check` | ❌ Not available | 1 if instance exceeded throughput >30s in last minute |
| `ebs_instance_iops_exceeded_check` | ❌ Not available | 1 if instance exceeded IOPS >30s in last minute |

### Percentage Metrics (Bonus)

| Metric Name | Description |
|-------------|-------------|
| `ebs_volume_throughput_exceeded_percent` | % of time exceeding throughput (5-minute avg) |
| `ebs_volume_iops_exceeded_percent` | % of time exceeding IOPS (5-minute avg) |
| `ebs_instance_throughput_exceeded_percent` | % of time exceeding instance throughput (5-minute avg) |
| `ebs_instance_iops_exceeded_percent` | % of time exceeding instance IOPS (1-minute avg) |

---

## Key Differences: CloudWatch vs Our Metrics

### CloudWatch Metrics

**Advantages:**
- ✅ Simple binary check (0 or 1)
- ✅ Familiar for AWS users
- ✅ Easy alerting: `metric == 1`

**Limitations:**
- ❌ Binary only - can't tell *how much* you exceeded
- ❌ Misses brief bursts (<30s per minute)
- ❌ 5-10 minute data lag
- ❌ Volume-level only (no instance metrics)
- ❌ Sparse - only publishes when exceeded
- ❌ Costs money ($500+/year for API calls)

### Our Raw Metrics

**Advantages:**
- ✅ Precise cumulative microseconds
- ✅ Captures all exceedance (even 1-2 second bursts)
- ✅ Real-time (30-second collection)
- ✅ Both volume-level AND instance-level
- ✅ Always present (even at 0)
- ✅ Enables % calculations and trending
- ✅ Free (no AWS API costs)

**Considerations:**
- Requires rate() calculations for time-based queries
- Steeper learning curve for CloudWatch users

---

## Example Queries

### Using Compatible Metrics (Simple)

```promql
# Alert when volume exceeds throughput limits
ebs_volume_throughput_exceeded_check == 1

# Count how many volumes are currently exceeding limits
sum(ebs_volume_throughput_exceeded_check)

# Volumes exceeding throughput with details
ebs_volume_throughput_exceeded_check{volume_id=~".+"}
```

### Using Raw Metrics (Advanced)

```promql
# Seconds per minute exceeding throughput
rate(ebs_volume_performance_exceeded_throughput_total[1m]) * 60

# Percentage of time exceeding limits (last 5 minutes)
rate(ebs_volume_performance_exceeded_throughput_total[5m]) / 1000000 * 100

# Total minutes exceeded in last hour
increase(ebs_volume_performance_exceeded_throughput_total[1h]) / 60000000

# Volumes with ANY exceedance (even brief bursts)
rate(ebs_volume_performance_exceeded_throughput_total[5m]) > 0
```

### Using Percentage Metrics (Best of Both)

```promql
# Alert on sustained high exceedance
ebs_volume_throughput_exceeded_percent > 10

# Volumes spending >5% of time at limits
ebs_volume_throughput_exceeded_percent > 5
```

---

## Migration from CloudWatch

If you have existing CloudWatch-based alerts, you can migrate them easily:

### CloudWatch Alert (AWS)

```yaml
# CloudWatch Alarm
MetricName: VolumeThroughputExceededCheck
Statistic: Maximum
ComparisonOperator: GreaterThanThreshold
Threshold: 0
EvaluationPeriods: 2
```

### Equivalent Prometheus Alert (Our Exporter)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: ebs-throughput-alert
spec:
  groups:
  - name: ebs_alerts
    rules:
    - alert: EBSVolumeThroughputExceeded
      expr: ebs_volume_throughput_exceeded_check == 1
      for: 2m
      annotations:
        summary: "Volume {{ $labels.volume_id }} exceeding throughput limits"
        description: "Volume has exceeded provisioned throughput for 2 minutes"
```

---

## Real-World Example

During cluster creation, we observed:

**CloudWatch Metrics:**
```
Example Volume:
  Time 00:13-00:17: VolumeThroughputExceededCheck = 1 (5 minutes sustained)
  Time 00:20: VolumeThroughputExceededCheck = 1 (1 minute)
  Total: 6 minutes of "consistent" exceedance detected
```

**Our Raw Metrics (Same Volume):**
```
Example Volume:
  Total exceeded: 109,258,299 microseconds = 109 seconds cumulative
  Over 90 minutes runtime = 2.0% of time
  Average: 1.2 seconds per minute
```

**Our Compatible Metric:**
```
ebs_volume_throughput_exceeded_check:
  During sustained periods (00:13-00:17): 1
  During normal operation: 0
  Matches CloudWatch behavior
```

**What This Tells Us:**
- CloudWatch caught the **sustained** exceedance during cluster creation (5+ minutes)
- Our raw metrics also caught **brief intermittent bursts** (109 sec total, 1-2 sec/min average)
- Our compatible metric matches CloudWatch for sustained periods
- Our raw metric provides **additional visibility** into brief bursts CloudWatch misses

---

## Recommendation

**Use both approaches:**

1. **Compatible metrics** for simple alerting:
   ```promql
   ebs_volume_throughput_exceeded_check == 1
   ```

2. **Raw metrics** for detailed analysis:
   ```promql
   rate(ebs_volume_performance_exceeded_throughput_total[5m]) > 0
   ```

3. **Percentage metrics** for capacity planning:
   ```promql
   ebs_volume_throughput_exceeded_percent > 5
   ```

This gives you the simplicity of CloudWatch with the precision of direct NVMe access.

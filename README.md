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

## Requirements

- Go 1.16 or later
- Linux system with NVMe EBS volumes
- Root/sudo access (required for NVMe IOCTLs)
- Amazon EC2 instance with EBS volumes

## Building

```bash
go build -o ebs-metrics-exporter
```

## Usage

```bash
# Run the exporter (requires root access for IOCTL operations)
sudo ./ebs-metrics-exporter --device /dev/nvme1n1 --port 8090
```

### Command-line Flags

- `--device` - NVMe device to monitor (required, e.g., `/dev/nvme1n1`)
- `--port` - Port to listen on (default: `8090`)

### Example

```bash
# Start the exporter for /dev/nvme1n1 on port 9100
sudo ./ebs-metrics-exporter --device /dev/nvme1n1 --port 9100
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

## Comparison with Python Version

The original Python script (`ebs_script.py`) writes metrics to a text file for the node_exporter textfile collector. This Go version:

1. **HTTP Server**: Runs a standalone HTTP server instead of writing to files
2. **Real-time**: Metrics are collected on-demand when Prometheus scrapes
3. **No State Files**: Calculates intervals between scrapes automatically
4. **Simplified Deployment**: Single binary, no need for node_exporter textfile collector
5. **Better Performance**: Go's compiled nature and efficient goroutines

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

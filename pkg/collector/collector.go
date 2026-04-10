package collector

import (
	"fmt"
	"log"
	"sync"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/nvme"
	"github.com/prometheus/client_golang/prometheus"
)

// EBSCollector collects EBS volume performance metrics
type EBSCollector struct {
	device       *nvme.Device
	mutex        sync.Mutex
	metricFilter *MetricFilter // Optional metric filtering

	// Volume metadata for labels
	volumeType   string
	pvcNamespace string
	pvcName      string

	// Counter metrics
	volumePerformanceExceededIOPS       *prometheus.Desc
	volumePerformanceExceededThroughput *prometheus.Desc
	instancePerformanceExceededIOPS     *prometheus.Desc
	instancePerformanceExceededThroughput *prometheus.Desc
	totalReadOps                        *prometheus.Desc
	totalWriteOps                       *prometheus.Desc
	totalReadBytes                      *prometheus.Desc
	totalWriteBytes                     *prometheus.Desc
	totalReadTime                       *prometheus.Desc
	totalWriteTime                      *prometheus.Desc

	// Gauge metrics
	volumeQueueLength                   *prometheus.Desc
}

// NewEBSCollector creates a new EBS collector with volume metadata
func NewEBSCollector(devicePath, volumeType, pvcNamespace, pvcName string) (*EBSCollector, error) {
	device, err := nvme.OpenDevice(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %w", err)
	}

	// Labels: volume_id, volume_type, and optionally pvc_namespace/pvc_name
	// For root volumes: volume_id, volume_type
	// For PVC volumes: volume_id, volume_type, pvc_namespace, pvc_name
	var labels []string
	if volumeType == "pvc" {
		labels = []string{"volume_id", "volume_type", "pvc_namespace", "pvc_name"}
	} else {
		labels = []string{"volume_id", "volume_type"}
	}

	return &EBSCollector{
		device:       device,
		volumeType:   volumeType,
		pvcNamespace: pvcNamespace,
		pvcName:      pvcName,
		volumePerformanceExceededIOPS: prometheus.NewDesc(
			"ebs_volume_performance_exceeded_iops",
			"Total time in microseconds that the EBS volume IOPS limit was exceeded",
			labels,
			nil,
		),
		volumePerformanceExceededThroughput: prometheus.NewDesc(
			"ebs_volume_performance_exceeded_throughput",
			"Total time in microseconds that the EBS volume throughput limit was exceeded",
			labels,
			nil,
		),
		instancePerformanceExceededIOPS: prometheus.NewDesc(
			"ebs_instance_performance_exceeded_iops",
			"Total time in microseconds that the EC2 instance EBS IOPS limit was exceeded",
			labels,
			nil,
		),
		instancePerformanceExceededThroughput: prometheus.NewDesc(
			"ebs_instance_performance_exceeded_throughput",
			"Total time in microseconds that the EC2 instance EBS throughput limit was exceeded",
			labels,
			nil,
		),
		totalReadOps: prometheus.NewDesc(
			"ebs_total_read_ops",
			"Total number of read operations",
			labels,
			nil,
		),
		totalWriteOps: prometheus.NewDesc(
			"ebs_total_write_ops",
			"Total number of write operations",
			labels,
			nil,
		),
		totalReadBytes: prometheus.NewDesc(
			"ebs_total_read_bytes",
			"Total bytes read",
			labels,
			nil,
		),
		totalWriteBytes: prometheus.NewDesc(
			"ebs_total_write_bytes",
			"Total bytes written",
			labels,
			nil,
		),
		totalReadTime: prometheus.NewDesc(
			"ebs_total_read_time",
			"Total time in microseconds spent on read operations",
			labels,
			nil,
		),
		totalWriteTime: prometheus.NewDesc(
			"ebs_total_write_time",
			"Total time in microseconds spent on write operations",
			labels,
			nil,
		),
		volumeQueueLength: prometheus.NewDesc(
			"ebs_volume_queue_length",
			"Current volume queue length",
			labels,
			nil,
		),
	}, nil
}

// Describe implements the prometheus.Collector interface
func (c *EBSCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.volumePerformanceExceededIOPS
	ch <- c.volumePerformanceExceededThroughput
	ch <- c.instancePerformanceExceededIOPS
	ch <- c.instancePerformanceExceededThroughput
	ch <- c.totalReadOps
	ch <- c.totalWriteOps
	ch <- c.totalReadBytes
	ch <- c.totalWriteBytes
	ch <- c.totalReadTime
	ch <- c.totalWriteTime
	ch <- c.volumeQueueLength
}

// Collect implements the prometheus.Collector interface
func (c *EBSCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stats, err := c.device.QueryStats()
	if err != nil {
		log.Printf("Error querying stats: %v", err)
		return
	}

	// Build label values based on volume type
	var labels []string
	if c.volumeType == "pvc" {
		labels = []string{c.device.VolumeID, c.volumeType, c.pvcNamespace, c.pvcName}
	} else {
		labels = []string{c.device.VolumeID, c.volumeType}
	}

	// Counter metrics
	if c.shouldExportMetric("ebs_volume_performance_exceeded_iops") {
		ch <- prometheus.MustNewConstMetric(
			c.volumePerformanceExceededIOPS,
			prometheus.CounterValue,
			float64(stats.EBSVolumePerformanceExceededIOPS),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_volume_performance_exceeded_throughput") {
		ch <- prometheus.MustNewConstMetric(
			c.volumePerformanceExceededThroughput,
			prometheus.CounterValue,
			float64(stats.EBSVolumePerformanceExceededTP),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_instance_performance_exceeded_iops") {
		ch <- prometheus.MustNewConstMetric(
			c.instancePerformanceExceededIOPS,
			prometheus.CounterValue,
			float64(stats.EBSInstancePerformanceExceededIOPS),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_instance_performance_exceeded_throughput") {
		ch <- prometheus.MustNewConstMetric(
			c.instancePerformanceExceededThroughput,
			prometheus.CounterValue,
			float64(stats.EBSInstancePerformanceExceededTP),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_read_ops") {
		ch <- prometheus.MustNewConstMetric(
			c.totalReadOps,
			prometheus.CounterValue,
			float64(stats.TotalReadOps),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_write_ops") {
		ch <- prometheus.MustNewConstMetric(
			c.totalWriteOps,
			prometheus.CounterValue,
			float64(stats.TotalWriteOps),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_read_bytes") {
		ch <- prometheus.MustNewConstMetric(
			c.totalReadBytes,
			prometheus.CounterValue,
			float64(stats.TotalReadBytes),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_write_bytes") {
		ch <- prometheus.MustNewConstMetric(
			c.totalWriteBytes,
			prometheus.CounterValue,
			float64(stats.TotalWriteBytes),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_read_time") {
		ch <- prometheus.MustNewConstMetric(
			c.totalReadTime,
			prometheus.CounterValue,
			float64(stats.TotalReadTime),
			labels...,
		)
	}

	if c.shouldExportMetric("ebs_total_write_time") {
		ch <- prometheus.MustNewConstMetric(
			c.totalWriteTime,
			prometheus.CounterValue,
			float64(stats.TotalWriteTime),
			labels...,
		)
	}

	// Gauge metrics
	if c.shouldExportMetric("ebs_volume_queue_length") {
		ch <- prometheus.MustNewConstMetric(
			c.volumeQueueLength,
			prometheus.GaugeValue,
			float64(stats.VolumeQueueLength),
			labels...,
		)
	}
}

// GetDevice returns the device path
func (c *EBSCollector) GetDevice() string {
	return c.device.Path
}

// GetVolumeID returns the volume ID
func (c *EBSCollector) GetVolumeID() string {
	return c.device.VolumeID
}

// SetMetricFilter sets the metric filter for this collector
func (c *EBSCollector) SetMetricFilter(filter *MetricFilter) {
	c.metricFilter = filter
}

// shouldExportMetric checks if a metric should be exported based on the filter
func (c *EBSCollector) shouldExportMetric(metricName string) bool {
	if c.metricFilter == nil {
		return true // No filter, export everything
	}
	return c.metricFilter.ShouldExport(metricName)
}

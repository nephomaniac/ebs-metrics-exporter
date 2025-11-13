package collector

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/nvme"
	"github.com/prometheus/client_golang/prometheus"
)

// EBSCollector collects EBS volume performance metrics
type EBSCollector struct {
	device   *nvme.Device
	mutex    sync.Mutex

	// Counter metrics
	volumePerformanceExceededIOPSTotal       *prometheus.Desc
	volumePerformanceExceededThroughputTotal *prometheus.Desc
	instancePerformanceExceededIOPSTotal     *prometheus.Desc
	instancePerformanceExceededThroughputTotal *prometheus.Desc
	totalReadOpsTotal                        *prometheus.Desc
	totalWriteOpsTotal                       *prometheus.Desc
	totalReadBytesTotal                      *prometheus.Desc
	totalWriteBytesTotal                     *prometheus.Desc

	// Gauge metrics
	volumeQueueLength                        *prometheus.Desc
}

// NewEBSCollector creates a new EBS collector
func NewEBSCollector(devicePath string) (*EBSCollector, error) {
	device, err := nvme.OpenDevice(devicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %w", err)
	}

	labels := []string{"device", "volume_id"}

	return &EBSCollector{
		device: device,
		volumePerformanceExceededIOPSTotal: prometheus.NewDesc(
			"ebs_volume_performance_exceeded_iops_total",
			"Total time in microseconds that the EBS volume IOPS limit was exceeded",
			labels,
			nil,
		),
		volumePerformanceExceededThroughputTotal: prometheus.NewDesc(
			"ebs_volume_performance_exceeded_throughput_total",
			"Total time in microseconds that the EBS volume throughput limit was exceeded",
			labels,
			nil,
		),
		instancePerformanceExceededIOPSTotal: prometheus.NewDesc(
			"ebs_instance_performance_exceeded_iops_total",
			"Total time in microseconds that the EC2 instance EBS IOPS limit was exceeded",
			labels,
			nil,
		),
		instancePerformanceExceededThroughputTotal: prometheus.NewDesc(
			"ebs_instance_performance_exceeded_throughput_total",
			"Total time in microseconds that the EC2 instance EBS throughput limit was exceeded",
			labels,
			nil,
		),
		totalReadOpsTotal: prometheus.NewDesc(
			"ebs_total_read_ops_total",
			"Total number of read operations",
			labels,
			nil,
		),
		totalWriteOpsTotal: prometheus.NewDesc(
			"ebs_total_write_ops_total",
			"Total number of write operations",
			labels,
			nil,
		),
		totalReadBytesTotal: prometheus.NewDesc(
			"ebs_total_read_bytes_total",
			"Total bytes read",
			labels,
			nil,
		),
		totalWriteBytesTotal: prometheus.NewDesc(
			"ebs_total_write_bytes_total",
			"Total bytes written",
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
	ch <- c.volumePerformanceExceededIOPSTotal
	ch <- c.volumePerformanceExceededThroughputTotal
	ch <- c.instancePerformanceExceededIOPSTotal
	ch <- c.instancePerformanceExceededThroughputTotal
	ch <- c.totalReadOpsTotal
	ch <- c.totalWriteOpsTotal
	ch <- c.totalReadBytesTotal
	ch <- c.totalWriteBytesTotal
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

	deviceName := strings.TrimPrefix(c.device.Path, "/dev/")
	labels := []string{deviceName, c.device.VolumeID}

	// Counter metrics
	ch <- prometheus.MustNewConstMetric(
		c.volumePerformanceExceededIOPSTotal,
		prometheus.CounterValue,
		float64(stats.EBSVolumePerformanceExceededIOPS),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.volumePerformanceExceededThroughputTotal,
		prometheus.CounterValue,
		float64(stats.EBSVolumePerformanceExceededTP),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.instancePerformanceExceededIOPSTotal,
		prometheus.CounterValue,
		float64(stats.EBSInstancePerformanceExceededIOPS),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.instancePerformanceExceededThroughputTotal,
		prometheus.CounterValue,
		float64(stats.EBSInstancePerformanceExceededTP),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.totalReadOpsTotal,
		prometheus.CounterValue,
		float64(stats.TotalReadOps),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.totalWriteOpsTotal,
		prometheus.CounterValue,
		float64(stats.TotalWriteOps),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.totalReadBytesTotal,
		prometheus.CounterValue,
		float64(stats.TotalReadBytes),
		labels...,
	)

	ch <- prometheus.MustNewConstMetric(
		c.totalWriteBytesTotal,
		prometheus.CounterValue,
		float64(stats.TotalWriteBytes),
		labels...,
	)

	// Gauge metrics
	ch <- prometheus.MustNewConstMetric(
		c.volumeQueueLength,
		prometheus.GaugeValue,
		float64(stats.VolumeQueueLength),
		labels...,
	)
}

// GetDevice returns the device path
func (c *EBSCollector) GetDevice() string {
	return c.device.Path
}

// GetVolumeID returns the volume ID
func (c *EBSCollector) GetVolumeID() string {
	return c.device.VolumeID
}

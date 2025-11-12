package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	clusterIDLabel = "cluster_id"
	deviceLabel    = "device"
	volumeIDLabel  = "volume_id"
	nodeLabel      = "node"
	
	ebsExporterValue = "ebs-metrics-exporter"
)

// EBSMetricsAggregator collects and aggregates EBS performance metrics
type EBSMetricsAggregator struct {
	volumeIOPSExceededTotal            *prometheus.GaugeVec
	volumeThroughputExceededTotal      *prometheus.GaugeVec
	volumeIOPSExceededCheck            *prometheus.GaugeVec
	volumeThroughputExceededCheck      *prometheus.GaugeVec
	instanceIOPSExceededTotal          *prometheus.GaugeVec
	instanceThroughputExceededTotal    *prometheus.GaugeVec
	totalReadOpsTotal                  *prometheus.GaugeVec
	totalWriteOpsTotal                 *prometheus.GaugeVec
	totalReadBytesTotal                *prometheus.GaugeVec
	totalWriteBytesTotal               *prometheus.GaugeVec
	volumeQueueLength                  *prometheus.GaugeVec
	volumeIOPSExceededPercent          *prometheus.GaugeVec
	volumeThroughputExceededPercent    *prometheus.GaugeVec
	instanceIOPSExceededPercent        *prometheus.GaugeVec
	instanceThroughputExceededPercent  *prometheus.GaugeVec
	
	mutex               sync.Mutex
	aggregationInterval time.Duration
	clusterId           string
}

// NewMetricsAggregator creates a new EBS metrics aggregator
func NewMetricsAggregator(aggregationInterval time.Duration, clusterId string) *EBSMetricsAggregator {
	return &EBSMetricsAggregator{
		volumeIOPSExceededTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_performance_exceeded_iops_total",
			Help:        "Total time in microseconds that the EBS volume IOPS limit was exceeded",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeThroughputExceededTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_performance_exceeded_throughput_total",
			Help:        "Total time in microseconds that the EBS volume throughput limit was exceeded",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeIOPSExceededCheck: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_iops_exceeded_check",
			Help:        "Reports whether an application consistently attempted to drive IOPS that exceeds the volume's provisioned IOPS performance within the last collection interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeThroughputExceededCheck: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_throughput_exceeded_check",
			Help:        "Reports whether an application consistently attempted to drive throughput that exceeds the volume's provisioned throughput performance within the last collection interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		instanceIOPSExceededTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_instance_performance_exceeded_iops_total",
			Help:        "Total time in microseconds that the EC2 instance EBS IOPS limit was exceeded",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		instanceThroughputExceededTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_instance_performance_exceeded_throughput_total",
			Help:        "Total time in microseconds that the EC2 instance EBS throughput limit was exceeded",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		totalReadOpsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_total_read_ops_total",
			Help:        "Total number of read operations",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		totalWriteOpsTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_total_write_ops_total",
			Help:        "Total number of write operations",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		totalReadBytesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_total_read_bytes_total",
			Help:        "Total bytes read",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		totalWriteBytesTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_total_write_bytes_total",
			Help:        "Total bytes written",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeQueueLength: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_queue_length",
			Help:        "Current volume queue length",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeIOPSExceededPercent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_performance_exceeded_iops_percent",
			Help:        "Percentage of time that the EBS volume IOPS limit was exceeded during the last interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		volumeThroughputExceededPercent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_volume_performance_exceeded_throughput_percent",
			Help:        "Percentage of time that the EBS volume throughput limit was exceeded during the last interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		instanceIOPSExceededPercent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_instance_performance_exceeded_iops_percent",
			Help:        "Percentage of time that the EC2 instance EBS IOPS limit was exceeded during the last interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		instanceThroughputExceededPercent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "ebs_instance_performance_exceeded_throughput_percent",
			Help:        "Percentage of time that the EC2 instance EBS throughput limit was exceeded during the last interval",
			ConstLabels: map[string]string{"name": ebsExporterValue},
		}, []string{clusterIDLabel, nodeLabel, deviceLabel, volumeIDLabel}),
		
		aggregationInterval: aggregationInterval,
		clusterId:           clusterId,
	}
}

// GetMetrics returns all Prometheus collectors
func (a *EBSMetricsAggregator) GetMetrics() []prometheus.Collector {
	return []prometheus.Collector{
		a.volumeIOPSExceededTotal,
		a.volumeThroughputExceededTotal,
		a.volumeIOPSExceededCheck,
		a.volumeThroughputExceededCheck,
		a.instanceIOPSExceededTotal,
		a.instanceThroughputExceededTotal,
		a.totalReadOpsTotal,
		a.totalWriteOpsTotal,
		a.totalReadBytesTotal,
		a.totalWriteBytesTotal,
		a.volumeQueueLength,
		a.volumeIOPSExceededPercent,
		a.volumeThroughputExceededPercent,
		a.instanceIOPSExceededPercent,
		a.instanceThroughputExceededPercent,
	}
}

// SetVolumeMetrics updates all metrics for a specific volume
// This will be called from the DaemonSet pods that collect the actual NVMe stats
func (a *EBSMetricsAggregator) SetVolumeMetrics(node, device, volumeID string, metrics map[string]float64) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	labels := prometheus.Labels{
		clusterIDLabel: a.clusterId,
		nodeLabel:      node,
		deviceLabel:    device,
		volumeIDLabel:  volumeID,
	}
	
	if val, ok := metrics["volume_iops_exceeded_total"]; ok {
		a.volumeIOPSExceededTotal.With(labels).Set(val)
	}
	if val, ok := metrics["volume_throughput_exceeded_total"]; ok {
		a.volumeThroughputExceededTotal.With(labels).Set(val)
	}
	if val, ok := metrics["volume_iops_exceeded_check"]; ok {
		a.volumeIOPSExceededCheck.With(labels).Set(val)
	}
	if val, ok := metrics["volume_throughput_exceeded_check"]; ok {
		a.volumeThroughputExceededCheck.With(labels).Set(val)
	}
	if val, ok := metrics["instance_iops_exceeded_total"]; ok {
		a.instanceIOPSExceededTotal.With(labels).Set(val)
	}
	if val, ok := metrics["instance_throughput_exceeded_total"]; ok {
		a.instanceThroughputExceededTotal.With(labels).Set(val)
	}
	if val, ok := metrics["total_read_ops_total"]; ok {
		a.totalReadOpsTotal.With(labels).Set(val)
	}
	if val, ok := metrics["total_write_ops_total"]; ok {
		a.totalWriteOpsTotal.With(labels).Set(val)
	}
	if val, ok := metrics["total_read_bytes_total"]; ok {
		a.totalReadBytesTotal.With(labels).Set(val)
	}
	if val, ok := metrics["total_write_bytes_total"]; ok {
		a.totalWriteBytesTotal.With(labels).Set(val)
	}
	if val, ok := metrics["volume_queue_length"]; ok {
		a.volumeQueueLength.With(labels).Set(val)
	}
	if val, ok := metrics["volume_iops_exceeded_percent"]; ok {
		a.volumeIOPSExceededPercent.With(labels).Set(val)
	}
	if val, ok := metrics["volume_throughput_exceeded_percent"]; ok {
		a.volumeThroughputExceededPercent.With(labels).Set(val)
	}
	if val, ok := metrics["instance_iops_exceeded_percent"]; ok {
		a.instanceIOPSExceededPercent.With(labels).Set(val)
	}
	if val, ok := metrics["instance_throughput_exceeded_percent"]; ok {
		a.instanceThroughputExceededPercent.With(labels).Set(val)
	}
}

// RemoveVolumeMetrics removes metrics for a specific volume when the pod is deleted
func (a *EBSMetricsAggregator) RemoveVolumeMetrics(node, device, volumeID string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	labels := prometheus.Labels{
		clusterIDLabel: a.clusterId,
		nodeLabel:      node,
		deviceLabel:    device,
		volumeIDLabel:  volumeID,
	}
	
	a.volumeIOPSExceededTotal.Delete(labels)
	a.volumeThroughputExceededTotal.Delete(labels)
	a.volumeIOPSExceededCheck.Delete(labels)
	a.volumeThroughputExceededCheck.Delete(labels)
	a.instanceIOPSExceededTotal.Delete(labels)
	a.instanceThroughputExceededTotal.Delete(labels)
	a.totalReadOpsTotal.Delete(labels)
	a.totalWriteOpsTotal.Delete(labels)
	a.totalReadBytesTotal.Delete(labels)
	a.totalWriteBytesTotal.Delete(labels)
	a.volumeQueueLength.Delete(labels)
	a.volumeIOPSExceededPercent.Delete(labels)
	a.volumeThroughputExceededPercent.Delete(labels)
	a.instanceIOPSExceededPercent.Delete(labels)
	a.instanceThroughputExceededPercent.Delete(labels)
}

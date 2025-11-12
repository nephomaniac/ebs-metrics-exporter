package config

const (
	OperatorName       = "ebs-metrics-exporter"
	OperatorNamespace  = "openshift-sre-ebs-metrics"
	MetricsPort        = "8383"
	HealthProbeAddress = ":8081"
	DaemonSetName      = "ebs-metrics-exporter"
)

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ConfigValidationStatus tracks whether the current ConfigMap is valid (1) or invalid (0)
	ConfigValidationStatus = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ebs_operator_config_validation_status",
		Help: "Current configuration validation status (1=valid, 0=invalid)",
	})

	// ReconciliationErrors counts failed reconciliation attempts
	ReconciliationErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ebs_operator_reconciliation_errors_total",
		Help: "Total number of reconciliation errors by resource type",
	}, []string{"resource_type"})

	// LastSuccessfulReconciliation tracks the last successful reconciliation timestamp
	LastSuccessfulReconciliation = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ebs_operator_last_successful_reconciliation_timestamp",
		Help: "Timestamp of last successful reconciliation by resource type",
	}, []string{"resource_type"})

	// DaemonSetPodsReady tracks the number of ready pods vs desired
	DaemonSetPodsReady = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ebs_operator_daemonset_pods_ready",
		Help: "Number of ready DaemonSet pods",
	})

	// DaemonSetPodsDesired tracks the desired number of DaemonSet pods
	DaemonSetPodsDesired = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ebs_operator_daemonset_pods_desired",
		Help: "Desired number of DaemonSet pods",
	})

	// ConfigMapChanges counts ConfigMap update events
	ConfigMapChanges = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ebs_operator_configmap_changes_total",
		Help: "Total number of ConfigMap change events",
	})

	// PodRestartTriggers counts how many times we triggered pod restarts
	PodRestartTriggers = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ebs_operator_pod_restart_triggers_total",
		Help: "Total number of pod restart triggers due to config changes",
	})

	// ValidationErrors tracks specific validation error types
	ValidationErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ebs_operator_validation_errors_total",
		Help: "Total number of validation errors by error type",
	}, []string{"error_type"})
)

func init() {
	// Register custom metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(
		ConfigValidationStatus,
		ReconciliationErrors,
		LastSuccessfulReconciliation,
		DaemonSetPodsReady,
		DaemonSetPodsDesired,
		ConfigMapChanges,
		PodRestartTriggers,
		ValidationErrors,
	)
}

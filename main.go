package main

import (
	"context"
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	customMetrics "github.com/openshift/operator-custom-metrics/pkg/metrics"

	operatorConfig "github.com/nephomaniac/ebs-metrics-exporter/config"
	"github.com/nephomaniac/ebs-metrics-exporter/controllers/daemonset"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	configv1 "github.com/openshift/api/config/v1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":"+operatorConfig.MetricsPort, "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", operatorConfig.HealthProbeAddress, "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Get cluster ID from ClusterVersion
	clusterId := getClusterID()
	if clusterId == "" {
		setupLog.Info("Warning: Could not retrieve cluster ID, using 'unknown'")
		clusterId = "unknown"
	}
	setupLog.Info("Retrieved cluster ID", "clusterId", clusterId)

	// Define namespaces to watch
	watchNamespaces := map[string]cache.Config{
		operatorConfig.OperatorNamespace: {},
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0", // Disable default metrics, we'll use custom metrics server
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "ebs-metrics-exporter-lock",
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize metrics aggregator
	metricsAggregator := metrics.GetMetricsAggregator(clusterId)

	// Setup DaemonSet controller
	if err = (&daemonset.DaemonSetReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		MetricsAggregator: metricsAggregator,
		ClusterId:         clusterId,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DaemonSet")
		os.Exit(1)
	}

	// Configure custom metrics server
	setupLog.Info("Configuring custom metrics server", "port", operatorConfig.MetricsPort)
	metricsConfig := customMetrics.NewBuilder(operatorConfig.OperatorNamespace, operatorConfig.OperatorName).
		WithPath("/metrics").
		WithPort(operatorConfig.MetricsPort).
		WithServiceMonitor().
		WithCollectors(metricsAggregator.GetMetrics()).
		GetConfig()

	if err := customMetrics.ConfigureMetrics(context.TODO(), *metricsConfig); err != nil {
		setupLog.Error(err, "unable to configure custom metrics")
		os.Exit(1)
	}

	// Add health and readiness probes
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// getClusterID retrieves the cluster ID from the ClusterVersion resource
func getClusterID() string {
	_, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to get kubeconfig")
		return ""
	}

	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to create client")
		return ""
	}

	clusterVersion := &configv1.ClusterVersion{}
	err = c.Get(context.TODO(), client.ObjectKey{Name: "version"}, clusterVersion)
	if err != nil {
		setupLog.Error(err, "unable to get ClusterVersion")
		return ""
	}

	return string(clusterVersion.Spec.ClusterID)
}

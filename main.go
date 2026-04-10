package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/collector"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/reconciler"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	mode                = flag.String("mode", "exporter", "Run mode: exporter (default) or reconciler")
	configPath          = flag.String("config", "/etc/ebs-exporter/config.yaml", "Path to configuration file")
	port                = flag.Int("port", 8090, "Port to serve metrics on")
	reconcilerNamespace = flag.String("reconciler-namespace", "openshift-sre-ebs-metrics", "Namespace to watch (reconciler mode only)")
	version             = "dev"
	commit              = "unknown"
	buildDate           = "unknown"
)

func main() {
	flag.Parse()

	// Print FIPS crypto status (set at build time via boilerplate)
	fmt.Println("***** Starting with FIPS crypto enabled *****")

	log.Printf("EBS Metrics Exporter starting")
	log.Printf("Version: %s, Commit: %s, BuildDate: %s", version, commit, buildDate)
	log.Printf("Mode: %s", *mode)

	// Route to appropriate mode
	switch *mode {
	case "exporter":
		runExporter()
	case "reconciler":
		runReconciler()
	default:
		log.Fatalf("Invalid mode: %s (valid modes: exporter, reconciler)", *mode)
	}
}

func runExporter() {
	log.Printf("Config file: %s", *configPath)

	// Load configuration
	// IMPORTANT: Config validation enforces package-as-source-of-truth
	// Invalid config causes pod to fail startup, preventing rollout of bad config
	cfg, err := config.Load(*configPath)
	if err != nil {
		// Check if config file exists but is invalid
		if _, statErr := os.Stat(*configPath); statErr == nil {
			// Config file exists but failed validation
			log.Printf("ERROR: Configuration validation failed: %v", err)
			log.Printf("Pod will exit to prevent rollout of invalid configuration")
			log.Printf("Fix the ConfigMap and PKO will reconcile")
			os.Exit(1) // Exit with error - pod enters CrashLoopBackOff
		}
		// Config file doesn't exist - use defaults (ConfigMap optional: true)
		log.Printf("Config file not found, using defaults: %v", err)
		cfg = config.DefaultConfig()
	} else {
		log.Printf("Configuration loaded and validated successfully")
		log.Printf("Discovery mode: %s", cfg.DeviceDiscovery.Mode)
		log.Printf("Polling interval: %d seconds", cfg.Metrics.PollingIntervalSeconds)
		log.Printf("Skip PVC mapping: %t", cfg.DeviceDiscovery.SkipPVCMapping)
	}

	// Create multi-device collector with auto-discovery
	multiCollector, err := collector.NewMultiDeviceCollector(cfg)
	if err != nil {
		log.Fatalf("Failed to create multi-device collector: %v", err)
	}

	// Log discovered devices
	devices := multiCollector.GetDevices()
	if len(devices) == 0 {
		log.Println("Warning: No devices to monitor")
	} else {
		log.Printf("Monitoring %d device(s):", len(devices))
		collectorInfo := multiCollector.GetCollectorInfo()
		for devicePath, volumeID := range collectorInfo {
			log.Printf("  - %s (volume: %s)", devicePath, volumeID)
		}
	}

	// Register collector with Prometheus
	registry := prometheus.NewRegistry()
	registry.MustRegister(multiCollector)

	// Create HTTP server
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// Landing page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		collectorInfo := multiCollector.GetCollectorInfo()
		devicesHTML := ""
		for devicePath, volumeID := range collectorInfo {
			devicesHTML += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", devicePath, volumeID)
		}

		fmt.Fprintf(w, `<html>
<head><title>EBS Metrics Exporter</title></head>
<body>
<h1>EBS Metrics Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
<dl>
<dt>Version</dt><dd>%s</dd>
<dt>Discovery Mode</dt><dd>%s</dd>
<dt>Devices Monitored</dt><dd>%d</dd>
</dl>
<h2>Monitored Devices</h2>
<table border="1">
<tr><th>Device</th><th>Volume ID</th></tr>
%s
</table>
</body>
</html>
`, version, cfg.DeviceDiscovery.Mode, len(collectorInfo), devicesHTML)
	})

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Readiness check endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Consider ready if we have at least one device OR discovery mode is disabled
		devices := multiCollector.GetDevices()
		if len(devices) > 0 || cfg.DeviceDiscovery.Mode == "disabled" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "ready")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "no devices available")
		}
	})

	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gracefully...")

	// Graceful shutdown with 10 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Exporter stopped")
}

func runReconciler() {
	log.Printf("Namespace: %s", *reconcilerNamespace)
	log.Println("EBS Metrics Exporter Operator - event-driven configuration validation and health monitoring")

	// Set up logger for controller-runtime
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create Kubernetes runtime scheme
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		log.Fatalf("Failed to add scheme: %v", err)
	}

	// Create controller manager with operator metrics on port 8081
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// Expose operator metrics on port 8081
		Metrics: metricsserver.Options{
			BindAddress: ":8081",
		},
		// Health probe on port 8082
		HealthProbeBindAddress: ":8082",
	})
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}

	// Create and register reconciler
	r := &reconciler.Reconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Namespace: *reconcilerNamespace,
	}

	if err := r.SetupWithManager(mgr); err != nil {
		log.Fatalf("Failed to setup reconciler with manager: %v", err)
	}

	// Add health check endpoints
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatalf("Failed to add healthz check: %v", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatalf("Failed to add readyz check: %v", err)
	}

	log.Println("Operator started - functions:")
	log.Println("  ✓ Configuration validation (invalid config → alert via metrics)")
	log.Println("  ✓ Pod restart coordination (config change → rolling restart)")
	log.Println("  ✓ DaemonSet health monitoring (pod readiness → metrics)")
	log.Printf("  ✓ Watching: %s/%s (ConfigMap)", *reconcilerNamespace, reconciler.ConfigMapName)
	log.Printf("  ✓ Watching: %s/%s (DaemonSet)", *reconcilerNamespace, reconciler.DaemonSetName)
	log.Println("  ✓ Operator metrics: http://localhost:8081/metrics")
	log.Println("  ✓ Health probe: http://localhost:8082/healthz")

	// Start manager (blocks until signal)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("Manager exited with error: %v", err)
	}

	log.Println("Operator stopped")
}

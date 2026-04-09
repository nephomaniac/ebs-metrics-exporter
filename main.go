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
)

var (
	mode       = flag.String("mode", "exporter", "Run mode: exporter (default) or reconciler")
	configPath = flag.String("config", "/etc/ebs-exporter/config.yaml", "Path to configuration file")
	port       = flag.Int("port", 8090, "Port to serve metrics on")
	reconcilerNamespace = flag.String("reconciler-namespace", "openshift-sre-ebs-metrics", "Namespace to watch (reconciler mode only)")
	reconcilerInterval  = flag.Int("reconciler-interval", 30, "Reconciliation interval in seconds (reconciler mode only)")
	version    = "dev"
	commit     = "unknown"
	buildDate  = "unknown"
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
	log.Printf("Interval: %ds", *reconcilerInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Create reconciler
	r, err := reconciler.New(*reconcilerNamespace)
	if err != nil {
		log.Fatalf("Failed to create reconciler: %v", err)
	}

	// Start reconciliation loop
	ticker := time.NewTicker(time.Duration(*reconcilerInterval) * time.Second)
	defer ticker.Stop()

	log.Println("Reconciler started, watching for drift...")

	// Run initial reconciliation
	if err := r.Reconcile(ctx); err != nil {
		log.Printf("ERROR: Initial reconciliation failed: %v", err)
	}

	// Periodic reconciliation
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down reconciler")
			return
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				log.Printf("ERROR: Reconciliation failed: %v", err)
			}
		}
	}
}

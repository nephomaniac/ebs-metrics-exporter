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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	configPath = flag.String("config", "/etc/ebs-exporter/config.yaml", "Path to configuration file")
	port       = flag.Int("port", 8090, "Port to serve metrics on")
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
	log.Printf("Config file: %s", *configPath)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		log.Printf("Using default configuration")
		cfg = config.DefaultConfig()
	} else {
		log.Printf("Configuration loaded successfully")
		log.Printf("Discovery mode: %s", cfg.DeviceDiscovery.Mode)
		log.Printf("Polling interval: %d seconds", cfg.Metrics.PollingIntervalSeconds)
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

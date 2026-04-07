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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	devicePath = flag.String("device", "/dev/nvme1n1", "NVMe device path to monitor")
	port       = flag.Int("port", 8090, "Port to serve metrics on")
	version    = "dev"
	commit     = "unknown"
	buildDate  = "unknown"
)

func main() {
	flag.Parse()

	log.Printf("EBS Metrics Exporter starting")
	log.Printf("Version: %s, Commit: %s, BuildDate: %s", version, commit, buildDate)
	log.Printf("Monitoring device: %s", *devicePath)

	// Create EBS collector
	ebsCollector, err := collector.NewEBSCollector(*devicePath)
	if err != nil {
		log.Fatalf("Failed to create EBS collector: %v", err)
	}

	log.Printf("Initialized collector for volume: %s", ebsCollector.GetVolumeID())

	// Register collector with Prometheus
	registry := prometheus.NewRegistry()
	registry.MustRegister(ebsCollector)

	// Create HTTP server
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	// Landing page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<html>
<head><title>EBS Metrics Exporter</title></head>
<body>
<h1>EBS Metrics Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
<dl>
<dt>Version</dt><dd>%s</dd>
<dt>Device</dt><dd>%s</dd>
<dt>Volume ID</dt><dd>%s</dd>
</dl>
</body>
</html>
`, version, ebsCollector.GetDevice(), ebsCollector.GetVolumeID())
	})

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Readiness check endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ready")
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

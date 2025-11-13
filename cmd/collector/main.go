package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	devicePath = flag.String("device", "", "NVMe device to monitor (e.g., /dev/nvme1n1)")
	port       = flag.String("port", "8090", "Port to listen on")
)

func main() {
	flag.Parse()

	if *devicePath == "" {
		fmt.Fprintf(os.Stderr, "Error: --device flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Create the EBS collector
	ebsCollector, err := collector.NewEBSCollector(*devicePath)
	if err != nil {
		log.Fatalf("Failed to create EBS collector: %v", err)
	}

	// Register the collector with Prometheus
	prometheus.MustRegister(ebsCollector)

	// Set up HTTP handlers
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>
<head><title>EBS Metrics Exporter</title></head>
<body>
<h1>EBS Metrics Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
<p>Device: %s</p>
<p>Volume ID: %s</p>
</body>
</html>`, ebsCollector.GetDevice(), ebsCollector.GetVolumeID())
	})

	addr := ":" + *port
	log.Printf("Starting EBS metrics exporter on %s", addr)
	log.Printf("Monitoring device: %s", *devicePath)
	log.Printf("Volume ID: %s", ebsCollector.GetVolumeID())
	log.Printf("Metrics available at http://localhost:%s/metrics", *port)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

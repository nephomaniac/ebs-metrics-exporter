package collector

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/discovery"
	"github.com/prometheus/client_golang/prometheus"
)

// MultiDeviceCollector manages multiple EBS device collectors
type MultiDeviceCollector struct {
	collectors   map[string]*EBSCollector // key is clean device path (e.g., /dev/nvme0n1)
	mutex        sync.RWMutex
	config       *config.Config
	metricFilter *MetricFilter
	stopCh       chan struct{} // Signal channel to stop rediscovery goroutine
}

// NewMultiDeviceCollector creates a collector that auto-discovers and monitors multiple devices
func NewMultiDeviceCollector(cfg *config.Config) (*MultiDeviceCollector, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	mdc := &MultiDeviceCollector{
		collectors:   make(map[string]*EBSCollector),
		config:       cfg,
		metricFilter: NewMetricFilter(cfg),
		stopCh:       make(chan struct{}),
	}

	// Discover devices based on configuration
	if err := mdc.discoverDevices(); err != nil {
		return nil, fmt.Errorf("failed to discover devices: %w", err)
	}

	// Start periodic rediscovery if configured
	if cfg.Metrics.RediscoveryIntervalSeconds > 0 {
		go mdc.periodicRediscovery()
	}

	return mdc, nil
}

// discoverDevices discovers and initializes collectors for all configured devices
func (mdc *MultiDeviceCollector) discoverDevices() error {
	// Discover devices with configuration filtering
	devices, err := discovery.DiscoverWithFilter(mdc.config)
	if err != nil {
		return fmt.Errorf("device discovery failed: %w", err)
	}

	if len(devices) == 0 {
		log.Println("Warning: No devices to monitor (discovery returned 0 devices)")
		return nil
	}

	log.Printf("Initializing collectors for %d device(s)...", len(devices))

	// Create collector for each discovered device
	for _, dev := range devices {
		collector, err := NewEBSCollector(dev.DevicePath, dev.VolumeType, dev.PVCNamespace, dev.PVCName)
		if err != nil {
			log.Printf("Warning: Failed to create collector for %s: %v", dev.CleanDevicePath, err)
			continue
		}

		// Set metric filter on collector
		collector.SetMetricFilter(mdc.metricFilter)

		mdc.collectors[dev.VolumeID] = collector // Use volume_id as key instead of device path
		if dev.VolumeType == "pvc" {
			log.Printf("Initialized collector for %s (PVC: %s/%s)", dev.VolumeID, dev.PVCNamespace, dev.PVCName)
		} else {
			log.Printf("Initialized collector for %s (root volume)", dev.VolumeID)
		}
	}

	if len(mdc.collectors) == 0 {
		return fmt.Errorf("no collectors initialized successfully")
	}

	log.Printf("Successfully initialized %d collector(s)", len(mdc.collectors))
	return nil
}

// Describe implements prometheus.Collector interface
// Sends metric descriptions from all device collectors
func (mdc *MultiDeviceCollector) Describe(ch chan<- *prometheus.Desc) {
	mdc.mutex.RLock()
	defer mdc.mutex.RUnlock()

	// If no collectors, return empty (this can happen if discovery found nothing)
	if len(mdc.collectors) == 0 {
		return
	}

	// All collectors share the same metric descriptors (same metric names with different labels)
	// We only need to describe once from any collector
	for _, collector := range mdc.collectors {
		collector.Describe(ch)
		break // Only describe once since all descriptors are identical
	}
}

// Collect implements prometheus.Collector interface
// Collects metrics from all device collectors
func (mdc *MultiDeviceCollector) Collect(ch chan<- prometheus.Metric) {
	mdc.mutex.RLock()
	defer mdc.mutex.RUnlock()

	// Collect from each device collector
	for devicePath, collector := range mdc.collectors {
		log.Printf("Collecting metrics from %s", devicePath)
		collector.Collect(ch)
	}
}

// GetDevices returns the list of monitored device paths
func (mdc *MultiDeviceCollector) GetDevices() []string {
	mdc.mutex.RLock()
	defer mdc.mutex.RUnlock()

	devices := make([]string, 0, len(mdc.collectors))
	for devicePath := range mdc.collectors {
		devices = append(devices, devicePath)
	}
	return devices
}

// GetCollectorInfo returns information about all collectors for debugging/status
func (mdc *MultiDeviceCollector) GetCollectorInfo() map[string]string {
	mdc.mutex.RLock()
	defer mdc.mutex.RUnlock()

	info := make(map[string]string)
	for devicePath, collector := range mdc.collectors {
		info[devicePath] = collector.GetVolumeID()
	}
	return info
}

// Stop gracefully stops the multi-device collector and its background goroutines
func (mdc *MultiDeviceCollector) Stop() {
	close(mdc.stopCh)
}

// periodicRediscovery runs in background to detect newly attached/detached volumes
func (mdc *MultiDeviceCollector) periodicRediscovery() {
	interval := time.Duration(mdc.config.Metrics.RediscoveryIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting periodic volume rediscovery (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			if err := mdc.rediscover(); err != nil {
				log.Printf("Rediscovery error: %v", err)
			}
		case <-mdc.stopCh:
			log.Println("Stopping periodic rediscovery")
			return
		}
	}
}

// rediscover checks for new/removed devices and updates collectors accordingly
func (mdc *MultiDeviceCollector) rediscover() error {
	// Discover current devices
	devices, err := discovery.DiscoverWithFilter(mdc.config)
	if err != nil {
		return fmt.Errorf("rediscovery failed: %w", err)
	}

	// Build set of discovered volume IDs for quick lookup
	discoveredVolumes := make(map[string]discovery.DiscoveredDevice)
	for _, dev := range devices {
		discoveredVolumes[dev.VolumeID] = dev
	}

	mdc.mutex.Lock()
	defer mdc.mutex.Unlock()

	// Get current collector volume IDs
	currentVolumes := make(map[string]bool)
	for volumeID := range mdc.collectors {
		currentVolumes[volumeID] = true
	}

	// Find newly attached devices (in discovered but not in current)
	for volumeID, dev := range discoveredVolumes {
		if !currentVolumes[volumeID] {
			log.Printf("Detected newly attached volume: %s", volumeID)
			if err := mdc.addCollector(dev); err != nil {
				log.Printf("Failed to add collector for %s: %v", volumeID, err)
			}
		}
	}

	// Find detached devices (in current but not in discovered)
	for volumeID := range currentVolumes {
		if _, found := discoveredVolumes[volumeID]; !found {
			log.Printf("Detected detached volume: %s", volumeID)
			mdc.removeCollector(volumeID)
		}
	}

	return nil
}

// addCollector adds a new collector for a discovered device (caller must hold mutex)
func (mdc *MultiDeviceCollector) addCollector(dev discovery.DiscoveredDevice) error {
	collector, err := NewEBSCollector(dev.DevicePath, dev.VolumeType, dev.PVCNamespace, dev.PVCName)
	if err != nil {
		return fmt.Errorf("failed to create collector: %w", err)
	}

	// Set metric filter
	collector.SetMetricFilter(mdc.metricFilter)

	mdc.collectors[dev.VolumeID] = collector // Use volume_id as key
	if dev.VolumeType == "pvc" {
		log.Printf("Added collector for %s (PVC: %s/%s)", dev.VolumeID, dev.PVCNamespace, dev.PVCName)
	} else {
		log.Printf("Added collector for %s (root volume)", dev.VolumeID)
	}
	return nil
}

// removeCollector removes a collector for a detached device (caller must hold mutex)
func (mdc *MultiDeviceCollector) removeCollector(cleanPath string) {
	if collector, exists := mdc.collectors[cleanPath]; exists {
		volumeID := collector.GetVolumeID()
		delete(mdc.collectors, cleanPath)
		log.Printf("Removed collector for %s (volume: %s)", cleanPath, volumeID)
	}
}

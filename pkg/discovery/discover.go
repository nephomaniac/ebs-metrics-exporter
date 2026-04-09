package discovery

import (
	"fmt"
	"log"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/k8s"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/nvme"
)

// DiscoveredDevice represents an auto-discovered EBS device
type DiscoveredDevice struct {
	// DevicePath is the full path to the device (e.g., /host/dev/nvme0n1)
	DevicePath string

	// VolumeID is the AWS EBS volume ID (e.g., vol-abc123)
	VolumeID string

	// CleanDevicePath is the device path without /host prefix (e.g., /dev/nvme0n1)
	// DEPRECATED: Use VolumeType/PVC labels instead
	CleanDevicePath string

	// VolumeType indicates if this is a "root" or "pvc" volume
	VolumeType string

	// PVCNamespace is the namespace of the PVC (empty for root volumes)
	PVCNamespace string

	// PVCName is the name of the PVC (empty for root volumes)
	PVCName string
}

// DiscoverEBSDevices automatically discovers all Amazon EBS volumes
// by enumerating NVMe devices and checking vendor ID / model name.
// Returns a list of discovered devices with their volume IDs and PVC metadata.
// If skipPVCMapping is true, all volumes are labeled as "root" without querying k8s API.
func DiscoverEBSDevices(skipPVCMapping bool) ([]DiscoveredDevice, error) {
	// Enumerate all NVMe block devices (excludes partitions)
	devicePaths, err := EnumerateNVMeDevices()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate NVMe devices: %w", err)
	}

	if len(devicePaths) == 0 {
		log.Println("Warning: No NVMe devices found on this node")
		return []DiscoveredDevice{}, nil
	}

	log.Printf("Found %d NVMe block device(s), checking for EBS volumes...", len(devicePaths))

	// Initialize PVC mapper for enriching volume metadata (unless disabled)
	var mapper *k8s.PVCMapper
	if skipPVCMapping {
		log.Println("PVC mapping disabled by configuration (skipPVCMapping=true)")
		mapper = nil
	} else {
		var err error
		mapper, err = k8s.NewPVCMapper()
		if err != nil {
			log.Printf("Warning: Failed to initialize PVC mapper: %v (will label all volumes as root)", err)
			mapper = nil
		}
	}

	var discovered []DiscoveredDevice
	for _, devicePath := range devicePaths {
		// Try to open device and identify as EBS volume
		device, err := nvme.OpenDevice(devicePath)
		if err != nil {
			// Not an EBS device (wrong vendor ID, model name, or I/O error)
			// This is expected for non-EBS NVMe devices (instance store, etc.)
			log.Printf("Skipping %s: %v", devicePath, err)
			continue
		}

		// Successfully identified as EBS volume
		log.Printf("Discovered EBS volume: %s -> %s", devicePath, device.VolumeID)

		// Enrich with PVC metadata
		volumeType := "root"
		pvcNamespace := ""
		pvcName := ""

		if mapper != nil {
			metadata, err := mapper.GetVolumeMetadata(device.VolumeID)
			if err != nil {
				log.Printf("Warning: Failed to get metadata for %s: %v", device.VolumeID, err)
			} else {
				volumeType = metadata.VolumeType
				pvcNamespace = metadata.PVCNamespace
				pvcName = metadata.PVCName

				if volumeType == "pvc" {
					log.Printf("  Mapped to PVC: %s/%s", pvcNamespace, pvcName)
				} else {
					log.Printf("  Identified as root volume")
				}
			}
		}

		discovered = append(discovered, DiscoveredDevice{
			DevicePath:      devicePath,
			VolumeID:        device.VolumeID,
			CleanDevicePath: StripHostPrefix(devicePath),
			VolumeType:      volumeType,
			PVCNamespace:    pvcNamespace,
			PVCName:         pvcName,
		})
	}

	if len(discovered) == 0 {
		log.Println("Warning: No EBS volumes found (NVMe devices present but not Amazon EBS)")
	} else {
		log.Printf("Successfully discovered %d EBS volume(s)", len(discovered))
	}

	return discovered, nil
}

// DiscoverWithFilter discovers EBS devices and applies ConfigMap filters
func DiscoverWithFilter(cfg *config.Config) ([]DiscoveredDevice, error) {
	// Check discovery mode
	switch cfg.DeviceDiscovery.Mode {
	case "disabled":
		log.Println("Device discovery disabled by configuration")
		return []DiscoveredDevice{}, nil

	case "explicit":
		return discoverExplicit(cfg)

	case "auto":
		return discoverAuto(cfg)

	default:
		return nil, fmt.Errorf("invalid discovery mode: %s", cfg.DeviceDiscovery.Mode)
	}
}

// discoverAuto performs auto-discovery with filtering
func discoverAuto(cfg *config.Config) ([]DiscoveredDevice, error) {
	// Discover all EBS devices
	devices, err := DiscoverEBSDevices(cfg.DeviceDiscovery.SkipPVCMapping)
	if err != nil {
		return nil, err
	}

	// Apply filters
	filtered := applyFilters(devices, cfg.DeviceDiscovery.AutoFilter)

	// Apply max devices limit
	if len(filtered) > cfg.Advanced.MaxDevices {
		log.Printf("Warning: Discovered %d devices but maxDevices is %d, limiting...",
			len(filtered), cfg.Advanced.MaxDevices)
		filtered = filtered[:cfg.Advanced.MaxDevices]
	}

	return filtered, nil
}

// discoverExplicit uses explicitly configured devices
func discoverExplicit(cfg *config.Config) ([]DiscoveredDevice, error) {
	if len(cfg.DeviceDiscovery.ExplicitDevices) == 0 {
		log.Println("Explicit mode but no devices configured")
		return []DiscoveredDevice{}, nil
	}

	// Initialize PVC mapper (unless disabled)
	var mapper *k8s.PVCMapper
	if cfg.DeviceDiscovery.SkipPVCMapping {
		log.Println("PVC mapping disabled by configuration (skipPVCMapping=true)")
		mapper = nil
	} else {
		var err error
		mapper, err = k8s.NewPVCMapper()
		if err != nil {
			log.Printf("Warning: Failed to initialize PVC mapper: %v (will label all volumes as root)", err)
			mapper = nil
		}
	}

	var devices []DiscoveredDevice
	for _, explicitDev := range cfg.DeviceDiscovery.ExplicitDevices {
		devicePath := AddHostPrefix(explicitDev.DevicePath)

		// Try to open device
		device, err := nvme.OpenDevice(devicePath)
		if err != nil {
			log.Printf("Warning: Explicit device %s failed: %v", explicitDev.DevicePath, err)
			continue
		}

		// Use configured volume ID if provided, otherwise use discovered
		volumeID := explicitDev.VolumeID
		if volumeID == "" {
			volumeID = device.VolumeID
		}

		// Enrich with PVC metadata
		volumeType := "root"
		pvcNamespace := ""
		pvcName := ""

		if mapper != nil {
			metadata, err := mapper.GetVolumeMetadata(volumeID)
			if err != nil {
				log.Printf("Warning: Failed to get metadata for %s: %v", volumeID, err)
			} else {
				volumeType = metadata.VolumeType
				pvcNamespace = metadata.PVCNamespace
				pvcName = metadata.PVCName
			}
		}

		devices = append(devices, DiscoveredDevice{
			DevicePath:      devicePath,
			VolumeID:        volumeID,
			CleanDevicePath: explicitDev.DevicePath,
			VolumeType:      volumeType,
			PVCNamespace:    pvcNamespace,
			PVCName:         pvcName,
		})

		log.Printf("Explicit device configured: %s -> %s", explicitDev.DevicePath, volumeID)
	}

	return devices, nil
}

// applyFilters applies include/exclude filters to discovered devices
func applyFilters(devices []DiscoveredDevice, filter config.AutoFilterConfig) []DiscoveredDevice {
	var filtered []DiscoveredDevice

	for _, dev := range devices {
		// Check volume ID filters
		if len(filter.IncludeVolumeIDs) > 0 {
			if !contains(filter.IncludeVolumeIDs, dev.VolumeID) {
				log.Printf("Filtering out %s (volume ID %s not in include list)",
					dev.CleanDevicePath, dev.VolumeID)
				continue
			}
		}

		if contains(filter.ExcludeVolumeIDs, dev.VolumeID) {
			log.Printf("Filtering out %s (volume ID %s in exclude list)",
				dev.CleanDevicePath, dev.VolumeID)
			continue
		}

		// Check device path filters
		if len(filter.IncludeDevices) > 0 {
			if !contains(filter.IncludeDevices, dev.CleanDevicePath) {
				log.Printf("Filtering out %s (device not in include list)", dev.CleanDevicePath)
				continue
			}
		}

		if contains(filter.ExcludeDevices, dev.CleanDevicePath) {
			log.Printf("Filtering out %s (device in exclude list)", dev.CleanDevicePath)
			continue
		}

		// Device passed all filters
		filtered = append(filtered, dev)
	}

	return filtered
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

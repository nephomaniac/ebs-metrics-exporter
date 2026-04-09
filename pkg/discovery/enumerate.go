package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	// DevicePathPrefix is the directory containing NVMe devices
	DevicePathPrefix = "/host/dev"
)

var (
	// nvmeBlockDevicePattern matches NVMe block devices (not partitions)
	// Format: nvmeXnY where X is controller number, Y is namespace
	// Examples: nvme0n1, nvme1n1, nvme10n1
	// Does NOT match: nvme0n1p1, nvme1n1p2 (partitions have 'p' suffix)
	nvmeBlockDevicePattern = regexp.MustCompile(`^nvme[0-9]+n[0-9]+$`)
)

// EnumerateNVMeDevices finds all NVMe block devices (excluding partitions)
// Returns full device paths (e.g., /host/dev/nvme0n1)
func EnumerateNVMeDevices() ([]string, error) {
	// Read all entries in /host/dev
	entries, err := os.ReadDir(DevicePathPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to read device directory %s: %w", DevicePathPrefix, err)
	}

	var devices []string
	for _, entry := range entries {
		name := entry.Name()

		// Check if this matches the NVMe block device pattern
		if nvmeBlockDevicePattern.MatchString(name) {
			// Build full path
			fullPath := filepath.Join(DevicePathPrefix, name)

			// Verify it's a block device (not a regular file or directory)
			info, err := entry.Info()
			if err != nil {
				// Skip if we can't stat it
				continue
			}

			// Check if it's a device file (block or character device)
			// In Go, Mode().Type() & os.ModeDevice checks for device files
			if info.Mode()&os.ModeDevice != 0 {
				devices = append(devices, fullPath)
			}
		}
	}

	return devices, nil
}

// IsNVMeBlockDevice checks if a device path is a valid NVMe block device
// (not a partition). Accepts both /dev/nvmeXnY and /host/dev/nvmeXnY formats.
func IsNVMeBlockDevice(devicePath string) bool {
	// Extract just the device name
	name := filepath.Base(devicePath)

	// Check against pattern
	return nvmeBlockDevicePattern.MatchString(name)
}

// StripHostPrefix removes /host prefix from device paths if present
// Example: /host/dev/nvme0n1 -> /dev/nvme0n1
func StripHostPrefix(devicePath string) string {
	// If path starts with /host, remove it
	if len(devicePath) > 5 && devicePath[:5] == "/host" {
		return devicePath[5:]
	}
	return devicePath
}

// AddHostPrefix adds /host prefix to device paths if not present
// Example: /dev/nvme0n1 -> /host/dev/nvme0n1
func AddHostPrefix(devicePath string) string {
	// If path doesn't start with /host, add it
	if len(devicePath) < 5 || devicePath[:5] != "/host" {
		return "/host" + devicePath
	}
	return devicePath
}

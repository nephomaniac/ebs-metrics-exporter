package discovery

import (
	"testing"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
)

func TestApplyFilters_IncludeVolumeIDs(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-abc123", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-def456", CleanDevicePath: "/dev/nvme1n1"},
		{DevicePath: "/host/dev/nvme2n1", VolumeID: "vol-ghi789", CleanDevicePath: "/dev/nvme2n1"},
	}

	filter := config.AutoFilterConfig{
		IncludeVolumeIDs: []string{"vol-abc123", "vol-ghi789"},
	}

	filtered := applyFilters(devices, filter)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices after include filter, got %d", len(filtered))
	}

	// Check we got the right devices
	foundAbc := false
	foundGhi := false
	for _, dev := range filtered {
		if dev.VolumeID == "vol-abc123" {
			foundAbc = true
		}
		if dev.VolumeID == "vol-ghi789" {
			foundGhi = true
		}
	}

	if !foundAbc || !foundGhi {
		t.Error("Include filter did not return expected volume IDs")
	}
}

func TestApplyFilters_ExcludeVolumeIDs(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-abc123", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-def456", CleanDevicePath: "/dev/nvme1n1"},
		{DevicePath: "/host/dev/nvme2n1", VolumeID: "vol-ghi789", CleanDevicePath: "/dev/nvme2n1"},
	}

	filter := config.AutoFilterConfig{
		ExcludeVolumeIDs: []string{"vol-abc123"}, // Exclude root volume
	}

	filtered := applyFilters(devices, filter)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices after exclude filter, got %d", len(filtered))
	}

	// Make sure vol-abc123 was excluded
	for _, dev := range filtered {
		if dev.VolumeID == "vol-abc123" {
			t.Error("Exclude filter failed to exclude vol-abc123")
		}
	}
}

func TestApplyFilters_IncludeDevices(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-abc123", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-def456", CleanDevicePath: "/dev/nvme1n1"},
		{DevicePath: "/host/dev/nvme2n1", VolumeID: "vol-ghi789", CleanDevicePath: "/dev/nvme2n1"},
	}

	filter := config.AutoFilterConfig{
		IncludeDevices: []string{"/dev/nvme1n1", "/dev/nvme2n1"},
	}

	filtered := applyFilters(devices, filter)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices after include devices filter, got %d", len(filtered))
	}

	// Make sure nvme0n1 was excluded
	for _, dev := range filtered {
		if dev.CleanDevicePath == "/dev/nvme0n1" {
			t.Error("Include devices filter should have excluded /dev/nvme0n1")
		}
	}
}

func TestApplyFilters_ExcludeDevices(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-abc123", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-def456", CleanDevicePath: "/dev/nvme1n1"},
	}

	filter := config.AutoFilterConfig{
		ExcludeDevices: []string{"/dev/nvme0n1"}, // Exclude root volume device
	}

	filtered := applyFilters(devices, filter)

	if len(filtered) != 1 {
		t.Errorf("Expected 1 device after exclude devices filter, got %d", len(filtered))
	}

	if filtered[0].CleanDevicePath != "/dev/nvme1n1" {
		t.Errorf("Expected /dev/nvme1n1, got %s", filtered[0].CleanDevicePath)
	}
}

func TestApplyFilters_CombinedFilters(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-root", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-data1", CleanDevicePath: "/dev/nvme1n1"},
		{DevicePath: "/host/dev/nvme2n1", VolumeID: "vol-data2", CleanDevicePath: "/dev/nvme2n1"},
		{DevicePath: "/host/dev/nvme3n1", VolumeID: "vol-temp", CleanDevicePath: "/dev/nvme3n1"},
	}

	filter := config.AutoFilterConfig{
		ExcludeVolumeIDs: []string{"vol-root"},   // Exclude root volume
		ExcludeDevices:   []string{"/dev/nvme3n1"}, // Exclude temp device
	}

	filtered := applyFilters(devices, filter)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 devices after combined filters, got %d", len(filtered))
	}

	// Should have vol-data1 and vol-data2
	for _, dev := range filtered {
		if dev.VolumeID != "vol-data1" && dev.VolumeID != "vol-data2" {
			t.Errorf("Unexpected volume after filter: %s", dev.VolumeID)
		}
	}
}

func TestApplyFilters_NoFilters(t *testing.T) {
	devices := []DiscoveredDevice{
		{DevicePath: "/host/dev/nvme0n1", VolumeID: "vol-abc123", CleanDevicePath: "/dev/nvme0n1"},
		{DevicePath: "/host/dev/nvme1n1", VolumeID: "vol-def456", CleanDevicePath: "/dev/nvme1n1"},
	}

	filter := config.AutoFilterConfig{} // Empty filter

	filtered := applyFilters(devices, filter)

	if len(filtered) != len(devices) {
		t.Errorf("No filters should return all devices, got %d, want %d", len(filtered), len(devices))
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		str   string
		want  bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not_found", []string{"a", "b", "c"}, "d", false},
		{"empty_slice", []string{}, "a", false},
		{"exact_match", []string{"vol-abc123"}, "vol-abc123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.str)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

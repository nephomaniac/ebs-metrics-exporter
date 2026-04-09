package nvme

import (
	"fmt"
	"os"
)

// MockDevice represents a mock NVMe device for testing
type MockDevice struct {
	path     string
	volumeID string
	stats    *EBSNVMEStats
	queryErr error
}

// MockDeviceBuilder helps build mock devices
type MockDeviceBuilder struct {
	devices map[string]*MockDevice
}

// NewMockDeviceBuilder creates a new mock device builder
func NewMockDeviceBuilder() *MockDeviceBuilder {
	return &MockDeviceBuilder{
		devices: make(map[string]*MockDevice),
	}
}

// AddDevice adds a mock device that will be "discovered"
func (b *MockDeviceBuilder) AddDevice(path, volumeID string) *MockDeviceBuilder {
	b.devices[path] = &MockDevice{
		path:     path,
		volumeID: volumeID,
		stats: &EBSNVMEStats{
			Magic:                             AmznNVMEStatsMagic,
			TotalReadOps:                      1000,
			TotalWriteOps:                     2000,
			TotalReadBytes:                    1024000,
			TotalWriteBytes:                   2048000,
			EBSVolumePerformanceExceededIOPS:  0,
			EBSVolumePerformanceExceededTP:    0,
			EBSInstancePerformanceExceededIOPS: 0,
			EBSInstancePerformanceExceededTP:   0,
			VolumeQueueLength:                 0,
		},
	}
	return b
}

// AddDeviceWithStats adds a mock device with custom stats
func (b *MockDeviceBuilder) AddDeviceWithStats(path, volumeID string, stats *EBSNVMEStats) *MockDeviceBuilder {
	b.devices[path] = &MockDevice{
		path:     path,
		volumeID: volumeID,
		stats:    stats,
	}
	return b
}

// SetDeviceError sets an error to be returned when querying this device
func (b *MockDeviceBuilder) SetDeviceError(path string, err error) *MockDeviceBuilder {
	if dev, exists := b.devices[path]; exists {
		dev.queryErr = err
	}
	return b
}

// MockOpenDevice is a mock version of OpenDevice for testing
func (b *MockDeviceBuilder) MockOpenDevice(devicePath string) (*Device, error) {
	mockDev, exists := b.devices[devicePath]
	if !exists {
		return nil, fmt.Errorf("not an Amazon NVMe device (VID: 0x%x)", 0x0000)
	}

	return &Device{
		Path:     devicePath,
		VolumeID: mockDev.volumeID,
	}, nil
}

// MockQueryStats returns mock stats for testing
func (d *MockDevice) MockQueryStats() (*EBSNVMEStats, error) {
	if d.queryErr != nil {
		return nil, d.queryErr
	}
	return d.stats, nil
}

// IsTestEnvironment returns true if running in test environment
// This can be checked to avoid real device operations during tests
func IsTestEnvironment() bool {
	return os.Getenv("GO_TEST") == "1" || os.Getenv("TESTING") == "true"
}

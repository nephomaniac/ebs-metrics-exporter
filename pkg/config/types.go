package config

import "time"

// Config represents the complete exporter configuration
type Config struct {
	DeviceDiscovery DeviceDiscoveryConfig `yaml:"deviceDiscovery"`
	Metrics         MetricsConfig         `yaml:"metrics"`
	Advanced        AdvancedConfig        `yaml:"advanced"`
}

// DeviceDiscoveryConfig controls how EBS volumes are discovered and filtered
type DeviceDiscoveryConfig struct {
	// Mode determines the discovery strategy: "auto", "explicit", or "disabled"
	Mode string `yaml:"mode"`

	// SkipPVCMapping disables PVC lookups via Kubernetes API
	// When true: all volumes labeled as volume_type="root", no k8s API queries, no ClusterRole needed
	// When false: queries k8s API to map volumes to PVCs (default)
	SkipPVCMapping bool `yaml:"skipPVCMapping"`

	// AutoFilter applies when Mode is "auto"
	AutoFilter AutoFilterConfig `yaml:"autoFilter"`

	// ExplicitDevices lists devices to monitor when Mode is "explicit"
	ExplicitDevices []ExplicitDevice `yaml:"explicitDevices"`
}

// AutoFilterConfig filters auto-discovered devices
type AutoFilterConfig struct {
	// IncludeVolumeIDs whitelists volume IDs (empty = all)
	IncludeVolumeIDs []string `yaml:"includeVolumeIDs"`

	// ExcludeVolumeIDs blacklists volume IDs
	ExcludeVolumeIDs []string `yaml:"excludeVolumeIDs"`

	// IncludeDevices whitelists device paths (empty = all)
	IncludeDevices []string `yaml:"includeDevices"`

	// ExcludeDevices blacklists device paths
	ExcludeDevices []string `yaml:"excludeDevices"`
}

// ExplicitDevice represents a manually specified device
type ExplicitDevice struct {
	// DevicePath is the device file path (e.g., /dev/nvme1n1)
	DevicePath string `yaml:"devicePath"`

	// VolumeID is optional (will be discovered if not provided)
	VolumeID string `yaml:"volumeID,omitempty"`
}

// MetricsConfig controls metric collection and filtering
type MetricsConfig struct {
	// Include whitelists metrics (glob patterns supported, empty = all)
	Include []string `yaml:"include"`

	// Exclude blacklists metrics (glob patterns supported)
	Exclude []string `yaml:"exclude"`

	// PollingIntervalSeconds sets collection frequency
	PollingIntervalSeconds int `yaml:"pollingIntervalSeconds"`

	// RediscoveryIntervalSeconds controls how often to check for new/removed volumes
	// Set to 0 to disable dynamic discovery (discover only at startup)
	RediscoveryIntervalSeconds int `yaml:"rediscoveryIntervalSeconds"`
}

// AdvancedConfig provides additional tuning options
type AdvancedConfig struct {
	// LogLevel sets logging verbosity: debug, info, warn, error
	LogLevel string `yaml:"logLevel"`

	// MaxDevices limits the number of devices to monitor
	MaxDevices int `yaml:"maxDevices"`

	// DeviceOpenTimeoutSeconds sets device open timeout
	DeviceOpenTimeoutSeconds int `yaml:"deviceOpenTimeoutSeconds"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DeviceDiscovery: DeviceDiscoveryConfig{
			Mode: "auto",
			AutoFilter: AutoFilterConfig{
				IncludeVolumeIDs: []string{},
				ExcludeVolumeIDs: []string{},
				IncludeDevices:   []string{},
				ExcludeDevices:   []string{},
			},
			ExplicitDevices: []ExplicitDevice{},
		},
		Metrics: MetricsConfig{
			Include:                    []string{},
			Exclude:                    []string{},
			PollingIntervalSeconds:     30,
			RediscoveryIntervalSeconds: 60,
		},
		Advanced: AdvancedConfig{
			LogLevel:                 "info",
			MaxDevices:               20,
			DeviceOpenTimeoutSeconds: 5,
		},
	}
}

// GetPollingInterval returns the polling interval as a time.Duration
func (c *Config) GetPollingInterval() time.Duration {
	if c.Metrics.PollingIntervalSeconds <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.Metrics.PollingIntervalSeconds) * time.Second
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	// Validate discovery mode
	switch c.DeviceDiscovery.Mode {
	case "auto", "explicit", "disabled":
		// Valid modes
	default:
		return ErrInvalidDiscoveryMode
	}

	// Validate metric filtering (can't have both include and exclude)
	if len(c.Metrics.Include) > 0 && len(c.Metrics.Exclude) > 0 {
		return ErrBothIncludeExclude
	}

	// Validate polling interval
	if c.Metrics.PollingIntervalSeconds < 1 || c.Metrics.PollingIntervalSeconds > 3600 {
		return ErrInvalidPollingInterval
	}

	// Validate log level
	switch c.Advanced.LogLevel {
	case "debug", "info", "warn", "error":
		// Valid log levels
	default:
		return ErrInvalidLogLevel
	}

	// Validate max devices
	if c.Advanced.MaxDevices < 1 || c.Advanced.MaxDevices > 100 {
		return ErrInvalidMaxDevices
	}

	return nil
}

package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DeviceDiscovery.Mode != "auto" {
		t.Errorf("Default discovery mode = %s, want auto", cfg.DeviceDiscovery.Mode)
	}

	if cfg.Metrics.PollingIntervalSeconds != 30 {
		t.Errorf("Default polling interval = %d, want 30", cfg.Metrics.PollingIntervalSeconds)
	}

	if cfg.Metrics.RediscoveryIntervalSeconds != 60 {
		t.Errorf("Default rediscovery interval = %d, want 60", cfg.Metrics.RediscoveryIntervalSeconds)
	}

	if cfg.Advanced.LogLevel != "info" {
		t.Errorf("Default log level = %s, want info", cfg.Advanced.LogLevel)
	}

	if cfg.Advanced.MaxDevices != 20 {
		t.Errorf("Default max devices = %d, want 20", cfg.Advanced.MaxDevices)
	}
}

func TestGetPollingInterval(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected time.Duration
	}{
		{"default", 30, 30 * time.Second},
		{"custom", 60, 60 * time.Second},
		{"zero_uses_default", 0, 30 * time.Second},
		{"negative_uses_default", -5, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Metrics: MetricsConfig{
					PollingIntervalSeconds: tt.seconds,
				},
			}
			got := cfg.GetPollingInterval()
			if got != tt.expected {
				t.Errorf("GetPollingInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidate_DiscoveryMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{"auto_valid", "auto", false},
		{"explicit_valid", "explicit", false},
		{"disabled_valid", "disabled", false},
		{"invalid_mode", "unknown", true},
		{"empty_invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.DeviceDiscovery.Mode = tt.mode
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != ErrInvalidDiscoveryMode {
				t.Errorf("Expected ErrInvalidDiscoveryMode, got %v", err)
			}
		})
	}
}

func TestValidate_BothIncludeExclude(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Metrics.Include = []string{"ebs_volume_*"}
	cfg.Metrics.Exclude = []string{"ebs_instance_*"}

	err := cfg.Validate()
	if err != ErrBothIncludeExclude {
		t.Errorf("Validate() with both include/exclude should return ErrBothIncludeExclude, got %v", err)
	}
}

func TestValidate_PollingInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		wantErr  bool
	}{
		{"valid_30", 30, false},
		{"valid_1", 1, false},
		{"valid_3600", 3600, false},
		{"invalid_0", 0, true},
		{"invalid_negative", -10, true},
		{"invalid_too_large", 3601, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Metrics.PollingIntervalSeconds = tt.interval
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_LogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{"debug", "debug", false},
		{"info", "info", false},
		{"warn", "warn", false},
		{"error", "error", false},
		{"invalid", "trace", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Advanced.LogLevel = tt.level
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_MaxDevices(t *testing.T) {
	tests := []struct {
		name       string
		maxDevices int
		wantErr    bool
	}{
		{"valid_1", 1, false},
		{"valid_20", 20, false},
		{"valid_100", 100, false},
		{"invalid_0", 0, true},
		{"invalid_negative", -5, true},
		{"invalid_too_large", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Advanced.MaxDevices = tt.maxDevices
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

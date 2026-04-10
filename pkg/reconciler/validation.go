package reconciler

import (
	"fmt"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
	"github.com/nephomaniac/ebs-metrics-exporter/pkg/metrics"
	"gopkg.in/yaml.v3"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidateConfigYAML validates the structure and values of config.yaml content
// Returns nil if valid, or a ValidationError describing the problem
func ValidateConfigYAML(configYAML string) error {
	// Parse YAML to ensure it's well-formed
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil {
		metrics.ValidationErrors.WithLabelValues("yaml_parse_error").Inc()
		return ValidationError{
			Field:   "config.yaml",
			Message: fmt.Sprintf("invalid YAML syntax: %v", err),
		}
	}

	// Validate discovery mode
	validModes := map[string]bool{
		"auto":     true,
		"explicit": true,
		"disabled": true,
	}
	if !validModes[cfg.DeviceDiscovery.Mode] {
		metrics.ValidationErrors.WithLabelValues("invalid_discovery_mode").Inc()
		return ValidationError{
			Field:   "deviceDiscovery.mode",
			Message: fmt.Sprintf("invalid mode '%s', must be one of: auto, explicit, disabled", cfg.DeviceDiscovery.Mode),
		}
	}

	// Validate polling interval (must be positive, reasonable range)
	if cfg.Metrics.PollingIntervalSeconds < 1 {
		metrics.ValidationErrors.WithLabelValues("invalid_polling_interval").Inc()
		return ValidationError{
			Field:   "metrics.pollingIntervalSeconds",
			Message: fmt.Sprintf("must be at least 1, got %d", cfg.Metrics.PollingIntervalSeconds),
		}
	}
	if cfg.Metrics.PollingIntervalSeconds > 3600 {
		metrics.ValidationErrors.WithLabelValues("invalid_polling_interval").Inc()
		return ValidationError{
			Field:   "metrics.pollingIntervalSeconds",
			Message: fmt.Sprintf("must be at most 3600 (1 hour), got %d", cfg.Metrics.PollingIntervalSeconds),
		}
	}

	// Validate rediscovery interval (0 = disabled, or positive)
	if cfg.Metrics.RediscoveryIntervalSeconds < 0 {
		metrics.ValidationErrors.WithLabelValues("invalid_rediscovery_interval").Inc()
		return ValidationError{
			Field:   "metrics.rediscoveryIntervalSeconds",
			Message: fmt.Sprintf("must be 0 (disabled) or positive, got %d", cfg.Metrics.RediscoveryIntervalSeconds),
		}
	}

	// Validate maxDevices (safety limit)
	if cfg.Advanced.MaxDevices < 1 {
		metrics.ValidationErrors.WithLabelValues("invalid_max_devices").Inc()
		return ValidationError{
			Field:   "advanced.maxDevices",
			Message: fmt.Sprintf("must be at least 1, got %d", cfg.Advanced.MaxDevices),
		}
	}
	if cfg.Advanced.MaxDevices > 100 {
		metrics.ValidationErrors.WithLabelValues("invalid_max_devices").Inc()
		return ValidationError{
			Field:   "advanced.maxDevices",
			Message: fmt.Sprintf("must be at most 100, got %d", cfg.Advanced.MaxDevices),
		}
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if cfg.Advanced.LogLevel != "" && !validLogLevels[cfg.Advanced.LogLevel] {
		metrics.ValidationErrors.WithLabelValues("invalid_log_level").Inc()
		return ValidationError{
			Field:   "advanced.logLevel",
			Message: fmt.Sprintf("invalid level '%s', must be one of: debug, info, warn, error", cfg.Advanced.LogLevel),
		}
	}

	// Validate explicit mode has devices if mode is "explicit"
	if cfg.DeviceDiscovery.Mode == "explicit" && len(cfg.DeviceDiscovery.ExplicitDevices) == 0 {
		metrics.ValidationErrors.WithLabelValues("explicit_mode_no_devices").Inc()
		return ValidationError{
			Field:   "deviceDiscovery.explicitDevices",
			Message: "mode is 'explicit' but no devices are specified",
		}
	}

	// All validations passed
	return nil
}

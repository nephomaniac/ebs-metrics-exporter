package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigPath is the default location for the config file
	DefaultConfigPath = "/etc/ebs-exporter/config.yaml"
)

// Load reads and parses the configuration from the specified path.
// If the path is empty or the file doesn't exist, returns default configuration.
func Load(path string) (*Config, error) {
	// Use default path if not specified
	if path == "" {
		path = DefaultConfigPath
	}

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Config file doesn't exist - use defaults
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	// Parse YAML
	cfg := DefaultConfig() // Start with defaults
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadOrDefault is like Load but returns default config on any error.
// Use this when you want to be permissive about config errors.
func LoadOrDefault(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		// Log error but continue with defaults
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config from %s: %v\n", path, err)
		fmt.Fprintf(os.Stderr, "Using default configuration\n")
		return DefaultConfig()
	}
	return cfg
}

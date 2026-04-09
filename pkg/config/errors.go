package config

import "errors"

var (
	// ErrInvalidDiscoveryMode indicates an invalid discovery mode
	ErrInvalidDiscoveryMode = errors.New("invalid discovery mode: must be 'auto', 'explicit', or 'disabled'")

	// ErrBothIncludeExclude indicates both include and exclude metrics are specified
	ErrBothIncludeExclude = errors.New("cannot specify both metrics.include and metrics.exclude")

	// ErrInvalidPollingInterval indicates polling interval is out of range
	ErrInvalidPollingInterval = errors.New("polling interval must be between 1 and 3600 seconds")

	// ErrInvalidLogLevel indicates an invalid log level
	ErrInvalidLogLevel = errors.New("invalid log level: must be 'debug', 'info', 'warn', or 'error'")

	// ErrInvalidMaxDevices indicates max devices is out of range
	ErrInvalidMaxDevices = errors.New("max devices must be between 1 and 100")
)

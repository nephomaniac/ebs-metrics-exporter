package collector

import (
	"path/filepath"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
)

// MetricFilter determines which metrics should be exported
type MetricFilter struct {
	include []string
	exclude []string
}

// NewMetricFilter creates a filter from configuration
func NewMetricFilter(cfg *config.Config) *MetricFilter {
	return &MetricFilter{
		include: cfg.Metrics.Include,
		exclude: cfg.Metrics.Exclude,
	}
}

// ShouldExport returns true if the metric should be exported
func (f *MetricFilter) ShouldExport(metricName string) bool {
	// If both lists are empty, export everything (default)
	if len(f.include) == 0 && len(f.exclude) == 0 {
		return true
	}

	// If include list is present, metric must match at least one pattern
	if len(f.include) > 0 {
		for _, pattern := range f.include {
			if matchGlob(pattern, metricName) {
				return true
			}
		}
		// Didn't match any include pattern
		return false
	}

	// If exclude list is present, metric must NOT match any pattern
	if len(f.exclude) > 0 {
		for _, pattern := range f.exclude {
			if matchGlob(pattern, metricName) {
				return false // Matched exclude pattern
			}
		}
		// Didn't match any exclude pattern
		return true
	}

	// Default: export
	return true
}

// matchGlob performs glob pattern matching
// Supports wildcards like "ebs_volume_*" or "ebs_instance_*"
func matchGlob(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		// Invalid pattern - treat as no match
		return false
	}
	return matched
}

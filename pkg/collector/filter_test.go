package collector

import (
	"testing"

	"github.com/nephomaniac/ebs-metrics-exporter/pkg/config"
)

func TestMetricFilter_ShouldExport_NoFilter(t *testing.T) {
	// Default config has empty include/exclude - should export everything
	cfg := config.DefaultConfig()
	filter := NewMetricFilter(cfg)

	metrics := []string{
		"ebs_volume_performance_exceeded_iops",
		"ebs_instance_performance_exceeded_iops",
		"ebs_total_read_ops",
		"ebs_volume_queue_length",
	}

	for _, metric := range metrics {
		if !filter.ShouldExport(metric) {
			t.Errorf("No filter: %s should be exported", metric)
		}
	}
}

func TestMetricFilter_ShouldExport_IncludeList(t *testing.T) {
	cfg := &config.Config{
		Metrics: config.MetricsConfig{
			Include: []string{"ebs_volume_*", "ebs_total_read_*"},
			Exclude: []string{},
		},
	}
	filter := NewMetricFilter(cfg)

	tests := []struct {
		metric string
		want   bool
	}{
		{"ebs_volume_performance_exceeded_iops", true},  // matches ebs_volume_*
		{"ebs_volume_queue_length", true},                     // matches ebs_volume_*
		{"ebs_total_read_ops", true},                    // matches ebs_total_read_*
		{"ebs_total_read_bytes", true},                  // matches ebs_total_read_*
		{"ebs_instance_performance_exceeded_iops", false}, // not in include list
		{"ebs_total_write_ops", false},                  // not in include list
	}

	for _, tt := range tests {
		got := filter.ShouldExport(tt.metric)
		if got != tt.want {
			t.Errorf("Include filter: %s should be %v, got %v", tt.metric, tt.want, got)
		}
	}
}

func TestMetricFilter_ShouldExport_ExcludeList(t *testing.T) {
	cfg := &config.Config{
		Metrics: config.MetricsConfig{
			Include: []string{},
			Exclude: []string{"ebs_instance_*"},
		},
	}
	filter := NewMetricFilter(cfg)

	tests := []struct {
		metric string
		want   bool
	}{
		{"ebs_volume_performance_exceeded_iops", true},     // not excluded
		{"ebs_total_read_ops", true},                       // not excluded
		{"ebs_instance_performance_exceeded_iops", false},  // matches exclude pattern
		{"ebs_instance_performance_exceeded_throughput", false}, // matches exclude pattern
	}

	for _, tt := range tests {
		got := filter.ShouldExport(tt.metric)
		if got != tt.want {
			t.Errorf("Exclude filter: %s should be %v, got %v", tt.metric, tt.want, got)
		}
	}
}

func TestMetricFilter_ShouldExport_ExactMatch(t *testing.T) {
	cfg := &config.Config{
		Metrics: config.MetricsConfig{
			Include: []string{"ebs_volume_queue_length"}, // exact match, no wildcard
			Exclude: []string{},
		},
	}
	filter := NewMetricFilter(cfg)

	tests := []struct {
		metric string
		want   bool
	}{
		{"ebs_volume_queue_length", true},                      // exact match
		{"ebs_volume_performance_exceeded_iops", false},  // not in include list
	}

	for _, tt := range tests {
		got := filter.ShouldExport(tt.metric)
		if got != tt.want {
			t.Errorf("Exact match filter: %s should be %v, got %v", tt.metric, tt.want, got)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"ebs_volume_*", "ebs_volume_queue_length", true},
		{"ebs_volume_*", "ebs_instance_queue_length", false},
		{"ebs_total_*", "ebs_total_read_ops", true},
		{"ebs_total_*", "ebs_volume_queue_length", false},
		{"ebs_volume_queue_length", "ebs_volume_queue_length", true}, // exact match
		{"*", "ebs_anything", true},                                  // match all
		{"ebs_volume_performance_exceeded_*", "ebs_volume_performance_exceeded_iops", true},
		{"ebs_volume_performance_exceeded_*", "ebs_volume_queue_length", false},
	}

	for _, tt := range tests {
		got := matchGlob(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

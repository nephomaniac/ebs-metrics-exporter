package metrics

import "time"

const (
	aggregatorResyncInterval = 5 * time.Minute
)

var aggregator *EBSMetricsAggregator

// GetMetricsAggregator returns the singleton metrics aggregator instance
func GetMetricsAggregator(clusterId string) *EBSMetricsAggregator {
	if aggregator == nil {
		aggregator = NewMetricsAggregator(aggregatorResyncInterval, clusterId)
	}
	return aggregator
}

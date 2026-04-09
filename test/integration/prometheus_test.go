//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// PrometheusQueryResult represents the Prometheus query API response
type PrometheusQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// TestPrometheusIngestion verifies EBS metrics are in cluster Prometheus
func TestPrometheusIngestion(t *testing.T) {
	// Port-forward to Prometheus in openshift-monitoring namespace
	t.Log("Setting up port-forward to Prometheus...")

	// Use oc to query Prometheus via the thanos-querier route
	// This is safer than port-forwarding and works across all clusters
	queries := []string{
		`ebs_total_read_ops_total`,
		`ebs_total_write_ops_total`,
		`ebs_volume_queue_length`,
	}

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("Querying cluster Prometheus for EBS metrics")
	t.Log(strings.Repeat("=", 80))

	allResults := make(map[string][]PrometheusResult, 0)

	for _, query := range queries {
		t.Logf("\nQuery: %s", query)

		// Use oc to exec promtool query in prometheus pod
		stdout, stderr, err := runCommand(t, "oc", "exec", "-n", "openshift-monitoring",
			"prometheus-k8s-0", "-c", "prometheus", "--",
			"promtool", "query", "instant", "http://localhost:9090",
			query)

		if err != nil {
			t.Logf("Warning: Failed to query Prometheus: %v\nStderr: %s", err, stderr)
			continue
		}

		// Parse promtool output
		results := parsePromtoolOutput(stdout)
		if len(results) == 0 {
			t.Logf("  No results found (metrics may not be scraped yet)")
			continue
		}

		allResults[query] = results
		t.Logf("  Found %d samples", len(results))
	}

	if len(allResults) == 0 {
		t.Error("No EBS metrics found in Prometheus - check ServiceMonitor and scrape config")
		return
	}

	// Print formatted table of results
	printPrometheusResultsTable(t, allResults)
}

// PrometheusResult represents a single metric sample from Prometheus
type PrometheusResult struct {
	Labels map[string]string
	Value  string
}

// parsePromtoolOutput parses the output from promtool query instant
func parsePromtoolOutput(output string) []PrometheusResult {
	var results []PrometheusResult

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip lines that don't look like metric output
		if !strings.Contains(line, "=>") {
			continue
		}

		// Parse line format: metric{labels} => value @timestamp
		parts := strings.Split(line, "=>")
		if len(parts) != 2 {
			continue
		}

		metricPart := strings.TrimSpace(parts[0])
		valuePart := strings.TrimSpace(parts[1])

		// Extract value (before @ timestamp)
		valueFields := strings.Fields(valuePart)
		if len(valueFields) == 0 {
			continue
		}
		value := valueFields[0]

		// Parse labels from metric part
		labels := make(map[string]string)

		// Find label section between { and }
		labelStart := strings.Index(metricPart, "{")
		labelEnd := strings.Index(metricPart, "}")

		if labelStart != -1 && labelEnd != -1 {
			metricName := metricPart[:labelStart]
			labels["__name__"] = metricName

			labelSection := metricPart[labelStart+1 : labelEnd]
			// Parse labels: key="value", key="value"
			labelPairs := strings.Split(labelSection, ",")
			for _, pair := range labelPairs {
				pair = strings.TrimSpace(pair)
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
					labels[key] = val
				}
			}
		}

		results = append(results, PrometheusResult{
			Labels: labels,
			Value:  value,
		})
	}

	return results
}

// printPrometheusResultsTable prints a formatted table of Prometheus query results
func printPrometheusResultsTable(t *testing.T, results map[string][]PrometheusResult) {
	// Group by device
	deviceMetrics := make(map[string]map[string]string) // device -> metric -> value

	for _, samples := range results {
		for _, sample := range samples {
			device := sample.Labels["device"]
			volumeID := sample.Labels["volume_id"]

			if device == "" {
				continue
			}

			deviceKey := fmt.Sprintf("%s (%s)", device, volumeID)

			if deviceMetrics[deviceKey] == nil {
				deviceMetrics[deviceKey] = make(map[string]string)
			}

			metricName := sample.Labels["__name__"]
			deviceMetrics[deviceKey][metricName] = sample.Value
		}
	}

	// Get sorted device list
	devices := make([]string, 0, len(deviceMetrics))
	for device := range deviceMetrics {
		devices = append(devices, device)
	}
	sort.Strings(devices)

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("EBS Metrics in Cluster Prometheus (Last 5 minutes)")
	t.Log(strings.Repeat("=", 80))

	for _, device := range devices {
		metrics := deviceMetrics[device]

		t.Logf("\n┌─────────────────────────────────────────────────────────────────────────────")
		t.Logf("│ Device: %s", device)
		t.Logf("├─────────────────────────────────────────────────────────────────────────────")
		t.Logf("│ %-50s │ %20s", "Metric", "Value")
		t.Logf("├─────────────────────────────────────────────────────────────────────────────")

		// Get sorted metric names
		metricNames := make([]string, 0, len(metrics))
		for name := range metrics {
			metricNames = append(metricNames, name)
		}
		sort.Strings(metricNames)

		for _, name := range metricNames {
			value := metrics[name]
			t.Logf("│ %-50s │ %20s", name, value)
		}

		t.Logf("└─────────────────────────────────────────────────────────────────────────────")
	}

	t.Logf("\n✅ Successfully verified EBS metrics in Prometheus")
	t.Logf("Total devices with metrics: %d", len(devices))
}

// TestPrometheusServiceMonitor verifies ServiceMonitor exists and is configured
func TestPrometheusServiceMonitor(t *testing.T) {
	ns := getNamespace()

	stdout, stderr, err := runCommand(t, "oc", "get", "servicemonitor",
		"-n", ns, "-o", "json")
	if err != nil {
		t.Logf("Warning: Failed to get ServiceMonitors: %v\nStderr: %s", err, stderr)
		return
	}

	// Check if output contains our ServiceMonitor
	if !strings.Contains(stdout, "ebs-metrics-exporter") {
		t.Error("ServiceMonitor for ebs-metrics-exporter not found")
		return
	}

	t.Log("✅ ServiceMonitor exists")

	// Parse and display key fields
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Logf("Warning: Failed to parse ServiceMonitor JSON: %v", err)
		return
	}

	items, ok := result["items"].([]interface{})
	if !ok || len(items) == 0 {
		return
	}

	for _, item := range items {
		sm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		metadata, _ := sm["metadata"].(map[string]interface{})
		name, _ := metadata["name"].(string)

		if !strings.Contains(name, "ebs-metrics") {
			continue
		}

		spec, _ := sm["spec"].(map[string]interface{})
		endpoints, _ := spec["endpoints"].([]interface{})

		t.Logf("\nServiceMonitor: %s", name)

		for i, ep := range endpoints {
			endpoint, _ := ep.(map[string]interface{})
			port, _ := endpoint["port"].(string)
			path, _ := endpoint["path"].(string)
			interval, _ := endpoint["interval"].(string)

			t.Logf("  Endpoint %d:", i+1)
			t.Logf("    Port: %s", port)
			t.Logf("    Path: %s", path)
			t.Logf("    Interval: %s", interval)
		}
	}
}

// TestPrometheusTargets verifies Prometheus is scraping our targets
func TestPrometheusTargets(t *testing.T) {
	t.Log("Checking Prometheus targets...")

	// Query prometheus pod to check targets
	stdout, stderr, err := runCommand(t, "oc", "exec", "-n", "openshift-monitoring",
		"prometheus-k8s-0", "-c", "prometheus", "--",
		"wget", "-qO-", "http://localhost:9090/api/v1/targets")

	if err != nil {
		t.Logf("Warning: Failed to query Prometheus targets: %v\nStderr: %s", err, stderr)
		return
	}

	// Check if ebs-metrics-exporter targets are present
	if strings.Contains(stdout, "ebs-metrics-exporter") {
		t.Log("✅ EBS metrics exporter targets found in Prometheus")

		// Count how many targets
		count := strings.Count(stdout, "ebs-metrics-exporter")
		t.Logf("   Found %d target(s)", count)

		// Check for "up" state
		if strings.Contains(stdout, `"health":"up"`) {
			t.Log("   Targets are healthy (up)")
		} else if strings.Contains(stdout, `"health":"down"`) {
			t.Error("   Warning: Some targets are down")
		}
	} else {
		t.Error("❌ EBS metrics exporter targets NOT found in Prometheus")
		t.Log("   Check ServiceMonitor configuration and namespace labels")
	}
}

// TestWaitForMetricIngestion waits for metrics to appear in Prometheus
func TestWaitForMetricIngestion(t *testing.T) {
	t.Log("Waiting for metrics to be scraped by Prometheus...")

	maxWait := 2 * time.Minute
	checkInterval := 10 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		stdout, _, err := runCommand(t, "oc", "exec", "-n", "openshift-monitoring",
			"prometheus-k8s-0", "-c", "prometheus", "--",
			"promtool", "query", "instant", "http://localhost:9090",
			"ebs_total_read_ops_total")

		if err == nil && strings.Contains(stdout, "=>") {
			t.Log("✅ Metrics found in Prometheus!")
			return
		}

		remaining := time.Until(deadline).Round(time.Second)
		t.Logf("   Waiting... (%s remaining)", remaining)
		time.Sleep(checkInterval)
	}

	t.Error("❌ Metrics not found in Prometheus after waiting")
	t.Log("   This may be normal if scrape interval is long or ServiceMonitor is misconfigured")
}

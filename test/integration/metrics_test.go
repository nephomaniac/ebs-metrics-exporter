//go:build integration

package integration

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// MetricSample represents a single metric sample with labels and value
type MetricSample struct {
	Name     string
	Device   string
	VolumeID string
	Labels   map[string]string
	Value    float64
}

// TestMetricsTable scrapes metrics endpoint from all pods and displays a formatted table
func TestMetricsTable(t *testing.T) {
	ns := getNamespace()

	// Get all running pods
	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName,
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		t.Fatalf("Failed to get pod names: %v\nStderr: %s", err, stderr)
	}

	podNames := strings.Fields(strings.TrimSpace(stdout))
	if len(podNames) == 0 {
		t.Skip("No pods found to test metrics endpoint")
	}

	t.Logf("Found %d pod(s) to scrape metrics from", len(podNames))

	// Collect all samples from all pods
	allSamples := make([]MetricSample, 0)
	nodeMap := make(map[string]string) // device -> node name

	for _, podName := range podNames {
		t.Logf("Scraping metrics from pod: %s", podName)

		// Get node name for this pod
		nodeStdout, _, nodeErr := runCommand(t, "oc", "get", "pod", podName, "-n", ns,
			"-o", "jsonpath={.spec.nodeName}")
		nodeName := "unknown"
		if nodeErr == nil {
			nodeName = strings.TrimSpace(nodeStdout)
		}

		// Curl metrics endpoint
		stdout, stderr, err = runCommand(t, "oc", "exec", "-n", ns, podName, "--",
			"curl", "-s", "http://localhost:8090/metrics")
		if err != nil {
			t.Logf("  Warning: Failed to curl metrics from %s: %v", podName, err)
			continue
		}

		// Parse Prometheus metrics
		samples, err := parsePrometheusMetrics(stdout)
		if err != nil {
			t.Logf("  Warning: Failed to parse metrics from %s: %v", podName, err)
			continue
		}

		// Track which node each device is on
		for _, sample := range samples {
			if sample.Device != "" {
				nodeMap[sample.Device] = nodeName
			}
		}

		allSamples = append(allSamples, samples...)
		t.Logf("  Collected %d samples", len(samples))
	}

	if len(allSamples) == 0 {
		t.Error("No metrics found from any pod")
		return
	}

	// Print formatted table with node information
	printMetricsTableWithNodes(t, allSamples, nodeMap)

	// Count unique devices and EBS metrics
	deviceSet := make(map[string]bool)
	ebsMetricCount := 0
	for _, sample := range allSamples {
		if sample.Device != "" {
			deviceSet[sample.Device] = true
		}
		if strings.HasPrefix(sample.Name, "ebs_") {
			ebsMetricCount++
		}
	}

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("Summary:")
	t.Logf("  Total metric samples: %d", len(allSamples))
	t.Logf("  EBS metric samples: %d", ebsMetricCount)
	t.Logf("  Unique devices monitored: %d", len(deviceSet))
	t.Logf("  Pods scraped: %d", len(podNames))
	t.Log(strings.Repeat("=", 80))

	if ebsMetricCount == 0 {
		t.Error("No EBS metrics found")
	}
}

// parsePrometheusMetrics parses Prometheus text format into MetricSample slice
// Uses simple line-by-line parsing to avoid version compatibility issues
func parsePrometheusMetrics(input string) ([]MetricSample, error) {
	var samples []MetricSample
	scanner := bufio.NewScanner(strings.NewReader(input))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line: metric_name{label="value",...} value
		sample, err := parseMetricLine(line)
		if err != nil {
			// Skip malformed lines
			continue
		}

		samples = append(samples, sample)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading metrics: %w", err)
	}

	return samples, nil
}

// parseMetricLine parses a single Prometheus metric line
func parseMetricLine(line string) (MetricSample, error) {
	sample := MetricSample{
		Labels: make(map[string]string),
	}

	// Find the metric name (everything before { or space)
	nameEnd := strings.IndexAny(line, "{ ")
	if nameEnd == -1 {
		return sample, fmt.Errorf("invalid metric line")
	}

	sample.Name = line[:nameEnd]

	// Check if there are labels
	if line[nameEnd] == '{' {
		// Find the closing brace
		labelEnd := strings.Index(line, "}")
		if labelEnd == -1 {
			return sample, fmt.Errorf("unclosed label section")
		}

		// Parse labels
		labelSection := line[nameEnd+1 : labelEnd]
		labels := parseLabels(labelSection)
		sample.Labels = labels

		// Extract device and volume_id for easy access
		if device, ok := labels["device"]; ok {
			sample.Device = device
		}
		if volumeID, ok := labels["volume_id"]; ok {
			sample.VolumeID = volumeID
		}

		// Value is after the closing brace
		valueStr := strings.TrimSpace(line[labelEnd+1:])
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return sample, fmt.Errorf("invalid value: %w", err)
		}
		sample.Value = value
	} else {
		// No labels, value is after the space
		valueStr := strings.TrimSpace(line[nameEnd:])
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return sample, fmt.Errorf("invalid value: %w", err)
		}
		sample.Value = value
	}

	return sample, nil
}

// parseLabels parses label section: key1="value1",key2="value2"
func parseLabels(labelSection string) map[string]string {
	labels := make(map[string]string)

	// Simple state machine to handle quoted values with commas
	var currentKey, currentValue strings.Builder
	inValue := false
	inQuote := false

	for i := 0; i < len(labelSection); i++ {
		ch := labelSection[i]

		if ch == '=' && !inQuote {
			inValue = true
			continue
		}

		if ch == '"' {
			inQuote = !inQuote
			continue
		}

		if ch == ',' && !inQuote {
			// End of label pair
			if currentKey.Len() > 0 && currentValue.Len() > 0 {
				labels[currentKey.String()] = currentValue.String()
			}
			currentKey.Reset()
			currentValue.Reset()
			inValue = false
			continue
		}

		if inValue {
			currentValue.WriteByte(ch)
		} else if ch != ' ' {
			currentKey.WriteByte(ch)
		}
	}

	// Add the last label pair
	if currentKey.Len() > 0 && currentValue.Len() > 0 {
		labels[currentKey.String()] = currentValue.String()
	}

	return labels
}

// printMetricsTableWithNodes prints a formatted table of metrics with node information
func printMetricsTableWithNodes(t *testing.T, samples []MetricSample, nodeMap map[string]string) {
	// Group samples by device
	deviceGroups := make(map[string][]MetricSample)
	for _, sample := range samples {
		if sample.Device == "" {
			continue // Skip non-device metrics
		}
		deviceGroups[sample.Device] = append(deviceGroups[sample.Device], sample)
	}

	// Get sorted device list
	devices := make([]string, 0, len(deviceGroups))
	for device := range deviceGroups {
		devices = append(devices, device)
	}
	sort.Strings(devices)

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("EBS Metrics by Device (from all pods)")
	t.Log(strings.Repeat("=", 80))

	// Print table for each device
	for _, device := range devices {
		deviceSamples := deviceGroups[device]

		// Get volume ID and node (should be same for all metrics on this device)
		volumeID := ""
		nodeName := nodeMap[device]
		if len(deviceSamples) > 0 {
			volumeID = deviceSamples[0].VolumeID
		}

		// Shorten node name for display
		shortNode := nodeName
		if strings.HasPrefix(nodeName, "ip-") {
			// Extract just the IP part
			parts := strings.Split(nodeName, ".")
			if len(parts) > 0 {
				shortNode = parts[0]
			}
		}

		t.Logf("\n┌─────────────────────────────────────────────────────────────────────────────")
		t.Logf("│ Device: %-20s  Node: %s", device, shortNode)
		if volumeID != "" {
			t.Logf("│ Volume ID: %s", volumeID)
		}
		t.Logf("├─────────────────────────────────────────────────────────────────────────────")
		t.Logf("│ %-50s │ %20s", "Metric", "Value")
		t.Logf("├─────────────────────────────────────────────────────────────────────────────")

		// Sort metrics by name
		sort.Slice(deviceSamples, func(i, j int) bool {
			return deviceSamples[i].Name < deviceSamples[j].Name
		})

		for _, sample := range deviceSamples {
			t.Logf("│ %-50s │ %20.0f", sample.Name, sample.Value)
		}

		t.Logf("└─────────────────────────────────────────────────────────────────────────────")
	}
}

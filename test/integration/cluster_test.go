//go:build integration

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	defaultNamespace = "openshift-sre-ebs-metrics"
	daemonSetName    = "ebs-metrics-exporter"
)

// TestMain ensures integration tests are only run when explicitly enabled
func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		os.Exit(0) // Skip all tests in this package
	}
	os.Exit(m.Run())
}

// getNamespace returns the test namespace from env or default
func getNamespace() string {
	if ns := os.Getenv("TEST_NAMESPACE"); ns != "" {
		return ns
	}
	return defaultNamespace
}

// runCommand executes a shell command and returns stdout, stderr, and error
func runCommand(t *testing.T, command string, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestClusterConnectivity verifies we can connect to the cluster
func TestClusterConnectivity(t *testing.T) {
	stdout, stderr, err := runCommand(t, "oc", "whoami")
	if err != nil {
		t.Fatalf("Failed to connect to cluster: %v\nStderr: %s", err, stderr)
	}

	username := strings.TrimSpace(stdout)
	if username == "" {
		t.Fatal("oc whoami returned empty username")
	}

	t.Logf("Connected to cluster as: %s", username)

	// Verify we can get cluster version
	stdout, stderr, err = runCommand(t, "oc", "version", "--short")
	if err != nil {
		t.Logf("Warning: Failed to get cluster version: %v", err)
	} else {
		t.Logf("Cluster version info:\n%s", stdout)
	}
}

// TestNamespaceExists verifies the namespace exists
func TestNamespaceExists(t *testing.T) {
	ns := getNamespace()

	stdout, stderr, err := runCommand(t, "oc", "get", "namespace", ns)
	if err != nil {
		t.Fatalf("Namespace %s does not exist: %v\nStderr: %s", ns, err, stderr)
	}

	if !strings.Contains(stdout, ns) {
		t.Fatalf("Namespace %s not found in output", ns)
	}

	t.Logf("Namespace %s exists", ns)
}

// TestDaemonSetDeployed verifies the DaemonSet is deployed
func TestDaemonSetDeployed(t *testing.T) {
	ns := getNamespace()

	stdout, stderr, err := runCommand(t, "oc", "get", "daemonset", daemonSetName, "-n", ns, "-o", "json")
	if err != nil {
		t.Fatalf("DaemonSet %s not found in namespace %s: %v\nStderr: %s", daemonSetName, ns, err, stderr)
	}

	if !strings.Contains(stdout, daemonSetName) {
		t.Fatalf("DaemonSet %s not found in output", daemonSetName)
	}

	t.Logf("DaemonSet %s exists in namespace %s", daemonSetName, ns)

	// Get DaemonSet status
	stdout, _, err = runCommand(t, "oc", "get", "daemonset", daemonSetName, "-n", ns,
		"-o", "jsonpath={.status.numberReady}/{.status.desiredNumberScheduled}")
	if err == nil {
		t.Logf("DaemonSet status: %s pods ready", stdout)
	}
}

// TestPodsRunning verifies pods are running
func TestPodsRunning(t *testing.T) {
	ns := getNamespace()

	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName, "-o", "wide")
	if err != nil {
		t.Fatalf("Failed to get pods in namespace %s: %v\nStderr: %s", ns, err, stderr)
	}

	if !strings.Contains(stdout, daemonSetName) {
		t.Fatalf("No pods found for DaemonSet %s", daemonSetName)
	}

	t.Logf("Pods found:\n%s", stdout)

	// Count Running pods
	lines := strings.Split(stdout, "\n")
	runningCount := 0
	for _, line := range lines {
		if strings.Contains(line, "Running") {
			runningCount++
		}
	}

	if runningCount == 0 {
		t.Error("No pods in Running state found")
	} else {
		t.Logf("Found %d pod(s) in Running state", runningCount)
	}
}

// TestServiceExists verifies the Service is created
func TestServiceExists(t *testing.T) {
	ns := getNamespace()

	stdout, stderr, err := runCommand(t, "oc", "get", "service", daemonSetName, "-n", ns)
	if err != nil {
		t.Fatalf("Service %s not found: %v\nStderr: %s", daemonSetName, err, stderr)
	}

	if !strings.Contains(stdout, daemonSetName) {
		t.Fatalf("Service %s not found in output", daemonSetName)
	}

	t.Logf("Service %s exists", daemonSetName)
}

// TestMetricsEndpoint verifies metrics are being exposed
func TestMetricsEndpoint(t *testing.T) {
	ns := getNamespace()

	// Get first running pod
	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		t.Fatalf("Failed to get pod name: %v\nStderr: %s", err, stderr)
	}

	podName := strings.TrimSpace(stdout)
	if podName == "" {
		t.Fatal("No pod found to test metrics endpoint")
	}

	t.Logf("Testing metrics endpoint on pod: %s", podName)

	// Curl metrics endpoint from within the pod
	stdout, stderr, err = runCommand(t, "oc", "exec", "-n", ns, podName, "--",
		"curl", "-s", "http://localhost:8090/metrics")
	if err != nil {
		t.Fatalf("Failed to curl metrics endpoint: %v\nStderr: %s", err, stderr)
	}

	// Verify we got Prometheus metrics
	if !strings.Contains(stdout, "ebs_") {
		t.Error("Metrics output does not contain expected ebs_ metrics")
	}

	if !strings.Contains(stdout, "HELP") || !strings.Contains(stdout, "TYPE") {
		t.Error("Metrics output does not contain Prometheus format (HELP/TYPE)")
	}

	// Count how many ebs_ metrics we found
	ebsMetricCount := strings.Count(stdout, "ebs_")
	t.Logf("Found %d ebs_ metric samples", ebsMetricCount)

	if ebsMetricCount == 0 {
		t.Error("No EBS metrics found in output")
	}
}

// TestHealthEndpoints verifies health check endpoints work
func TestHealthEndpoints(t *testing.T) {
	ns := getNamespace()

	// Get first running pod
	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		t.Fatalf("Failed to get pod name: %v\nStderr: %s", err, stderr)
	}

	podName := strings.TrimSpace(stdout)
	if podName == "" {
		t.Skip("No pod found to test health endpoints")
	}

	// Test /healthz
	stdout, stderr, err = runCommand(t, "oc", "exec", "-n", ns, podName, "--",
		"curl", "-s", "http://localhost:8090/healthz")
	if err != nil {
		t.Errorf("/healthz endpoint failed: %v\nStderr: %s", err, stderr)
	} else if !strings.Contains(stdout, "ok") {
		t.Errorf("/healthz did not return 'ok', got: %s", stdout)
	} else {
		t.Log("/healthz endpoint returned: ok")
	}

	// Test /readyz
	stdout, stderr, err = runCommand(t, "oc", "exec", "-n", ns, podName, "--",
		"curl", "-s", "http://localhost:8090/readyz")
	if err != nil {
		t.Errorf("/readyz endpoint failed: %v\nStderr: %s", err, stderr)
	} else if !strings.Contains(stdout, "ready") {
		t.Errorf("/readyz did not return 'ready', got: %s", stdout)
	} else {
		t.Log("/readyz endpoint returned: ready")
	}
}

// TestDeviceDiscovery verifies devices are discovered on nodes
func TestDeviceDiscovery(t *testing.T) {
	ns := getNamespace()

	// Get logs from first pod
	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		t.Fatalf("Failed to get pod name: %v\nStderr: %s", err, stderr)
	}

	podName := strings.TrimSpace(stdout)
	if podName == "" {
		t.Skip("No pod found to check logs")
	}

	// Get pod logs
	stdout, stderr, err = runCommand(t, "oc", "logs", "-n", ns, podName, "--tail=100")
	if err != nil {
		t.Fatalf("Failed to get pod logs: %v\nStderr: %s", err, stderr)
	}

	t.Logf("Pod logs (last 100 lines):\n%s", stdout)

	// Check for device discovery messages
	if strings.Contains(stdout, "Discovered EBS volume") ||
		strings.Contains(stdout, "Monitoring") {
		t.Log("Device discovery messages found in logs")
	} else {
		t.Log("Warning: No explicit device discovery messages in logs")
	}

	// Check for errors
	if strings.Contains(stdout, "Error") || strings.Contains(stdout, "Failed") {
		t.Log("Warning: Error messages found in logs (may be expected if no EBS volumes)")
	}
}

// TestPodSecurityContext verifies pods are running with correct security context
func TestPodSecurityContext(t *testing.T) {
	ns := getNamespace()

	// Get pod security context
	stdout, stderr, err := runCommand(t, "oc", "get", "pods", "-n", ns,
		"-l", "app.kubernetes.io/name="+daemonSetName,
		"-o", "jsonpath={.items[0].spec.containers[0].securityContext}")
	if err != nil {
		t.Fatalf("Failed to get pod security context: %v\nStderr: %s", err, stderr)
	}

	t.Logf("Pod security context: %s", stdout)

	// Check for privileged
	if !strings.Contains(stdout, "privileged") {
		t.Error("Pod security context does not contain 'privileged' setting")
	}
}

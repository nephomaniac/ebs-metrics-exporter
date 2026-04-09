# Integration Tests

This directory contains integration tests that require a live OpenShift cluster with EBS volumes attached.

## ⚠️ Important

**Integration tests are NOT run as part of `go test` or `make go-test`.**

These tests require:
- Access to a live OpenShift/Kubernetes cluster
- Nodes with actual EBS volumes attached
- Appropriate RBAC permissions
- Real NVMe devices accessible via hostPath mounts

## Running Integration Tests

Integration tests must be run manually against a test cluster:

```bash
# Set up cluster access
oc login https://api.your-test-cluster.com

# Run integration tests (manual execution only)
go test -v ./test/integration/ -args -cluster-test
```

## Test Categories

### Read-Only Tests
Tests that only observe cluster state without making changes:
- Verify DaemonSet deployment exists
- Check pods are running on all nodes
- Query metrics endpoints
- Verify device discovery on actual nodes

### Write Tests (Use with Caution)
Tests that modify cluster state:
- Deploy/undeploy test DaemonSets
- Create/delete test ConfigMaps
- Test dynamic volume attachment/detachment

## Writing Integration Tests

Integration tests should:
1. **Check for cluster connectivity** before running
2. **Skip gracefully** if no cluster is available
3. **Use dedicated test namespaces** (e.g., `ebs-metrics-test`)
4. **Clean up all resources** after completion
5. **Be idempotent** (safe to run multiple times)

Example:

```go
func TestIntegration_PodDeployment(t *testing.T) {
    if os.Getenv("INTEGRATION_TEST") != "true" {
        t.Skip("Skipping integration test (set INTEGRATION_TEST=true to run)")
    }
    
    // Test code here
}
```

## Environment Variables

- `INTEGRATION_TEST=true` - Enable integration tests
- `KUBECONFIG` - Path to kubeconfig file
- `TEST_NAMESPACE` - Namespace for test resources (default: `ebs-metrics-test`)

## Safety

**NEVER run integration tests against production clusters.**

Always use:
- Dedicated test clusters
- Non-production namespaces
- Read-only operations where possible
- Proper cleanup in defer statements

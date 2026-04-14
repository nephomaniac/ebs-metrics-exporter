# Testing Guide

## Pre-Commit Testing (No Cluster Required)

```bash
# Run all tests locally (mimics CI checks)
make local-ci ALLOW_DIRTY_CHECKOUT=true
```

This runs:
- `docker-build` - Build with FIPS crypto
- `local-build-pko` - Build PKO package  
- `go-test` - Unit tests
- `validate-pko-fixtures` - Template validation

---

## Unit Tests Only

```bash
# Run Go tests
make go-test

# Run linter
make go-check
```

---

## Integration Testing (Requires Cluster)

```bash
# Deploy to cluster
make local-deploy

# Check deployment status
make local-status

# View logs
make local-logs

# Get a pod name
POD=$(oc get pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter -o jsonpath='{.items[0].metadata.name}')

# Test metrics endpoint
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ^ebs_

# Check for specific metrics
oc exec -n openshift-sre-ebs-metrics $POD -- curl -s localhost:8090/metrics | grep ebs_volume_queue_length

# Port-forward for local testing
oc port-forward -n openshift-sre-ebs-metrics $POD 8090:8090
curl http://localhost:8090/metrics
```

---

## Prometheus Integration

```bash
# Check ServiceMonitor is created
oc get servicemonitor -n openshift-sre-ebs-metrics

# Verify Prometheus is scraping
# Navigate to OpenShift Console → Observe → Metrics
# Query: {__name__=~"ebs_.*"}
```

---

## Make Targets for Testing

### `make go-test`
Run Go unit tests.

**When to use:**
- After code changes
- Pre-commit validation

**What it does:**
- Runs all `*_test.go` files
- Excludes integration tests (no `-tags integration`)

**Example:**
```bash
make go-test
```

### `make go-check`
Run golangci-lint linter.

**When to use:**
- Pre-commit validation
- Code quality checks

**Example:**
```bash
make go-check
```

### `make validate-pko-fixtures`
Validate PKO YAML templates against schemas.

**When to use:**
- After modifying deploy_pko/ files
- Pre-commit validation

**Example:**
```bash
make validate-pko-fixtures
```

### `make local-test-all`
Run all tests (unit + PKO validation).

**When to use:**
- Pre-commit comprehensive testing
- Part of `local-ci`

**Example:**
```bash
make local-test-all
```

### `make integration-test`
Run integration tests against live cluster.

**When to use:**
- After deployment
- Testing with real EBS volumes

**Requirements:**
- Cluster access
- Deployed DaemonSet

**Example:**
```bash
make integration-test
```

### `make metrics-table`
Display formatted metrics table from running pods.

**When to use:**
- Verify metrics are being collected
- Debug metric labels

**Example:**
```bash
make metrics-table
```

### `make verify-prometheus`
Verify metrics are ingested into cluster Prometheus.

**When to use:**
- Confirm ServiceMonitor is working
- Verify Prometheus scrape configuration

**Example:**
```bash
make verify-prometheus
```

---

## Related Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get started with local development
- **[Troubleshooting](TROUBLESHOOTING.md)** - Common issues and solutions
- **[Makefile Targets](MAKEFILE.md)** - Complete reference of all make targets

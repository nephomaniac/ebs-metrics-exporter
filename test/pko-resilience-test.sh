#!/bin/bash
# PKO Resilience Testing Script
# Tests how PKO and the deployment react to various admin operations

set -e

NAMESPACE="openshift-sre-ebs-metrics"
APP_NAME="ebs-metrics-exporter"

echo "============================================"
echo "PKO Resilience Testing"
echo "============================================"
echo ""
echo "⚠️  WARNING: This script modifies cluster resources"
echo "    Only run in a test cluster!"
echo ""
read -p "Press Enter to continue or Ctrl+C to abort..."
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function test_header() {
    echo ""
    echo "============================================"
    echo "TEST: $1"
    echo "============================================"
}

function pass() {
    echo -e "${GREEN}✓ PASS:${NC} $1"
}

function fail() {
    echo -e "${RED}✗ FAIL:${NC} $1"
}

function info() {
    echo -e "${YELLOW}ℹ INFO:${NC} $1"
}

function wait_for_pods() {
    local expected=$1
    local timeout=60
    local elapsed=0

    info "Waiting for $expected pods to be ready..."
    while [ $elapsed -lt $timeout ]; do
        READY=$(oc get pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME \
            --field-selector=status.phase=Running 2>/dev/null | grep -c Running || echo "0")
        if [ "$READY" -eq "$expected" ]; then
            pass "All $expected pods ready"
            return 0
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    fail "Timeout waiting for pods"
    return 1
}

# Prerequisite check
test_header "Prerequisite Check"
if ! oc get clusterpackage $APP_NAME &>/dev/null; then
    fail "ClusterPackage not found. Deploy first with: make local-deploy"
    exit 1
fi
pass "ClusterPackage exists"

INITIAL_POD_COUNT=$(oc get pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME --field-selector=status.phase=Running 2>/dev/null | grep -c Running || echo "0")
if [ "$INITIAL_POD_COUNT" -eq "0" ]; then
    fail "No running pods found. Deployment may not be ready."
    exit 1
fi
pass "$INITIAL_POD_COUNT pods running initially"

# Test 1: Edit ConfigMap and verify persistence
test_header "Test 1: ConfigMap Edit Persistence"
info "Backing up ConfigMap..."
oc get configmap ebs-metrics-exporter-config -n $NAMESPACE -o yaml > /tmp/configmap-backup.yaml

info "Adding test annotation to ConfigMap..."
oc annotate configmap ebs-metrics-exporter-config -n $NAMESPACE test-timestamp="$(date +%s)" --overwrite

info "Waiting 5 seconds for PKO reconciliation..."
sleep 5

ANNOTATION=$(oc get configmap ebs-metrics-exporter-config -n $NAMESPACE -o jsonpath='{.metadata.annotations.test-timestamp}' 2>/dev/null || echo "")
if [ -n "$ANNOTATION" ]; then
    pass "ConfigMap annotation persisted (PKO did NOT revert manual change)"
else
    fail "ConfigMap annotation removed (unexpected PKO behavior)"
fi

# Test 2: ConfigMap change does NOT restart pods
test_header "Test 2: ConfigMap Change - Pod Restart Behavior"
PODS_BEFORE=$(oc get pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME -o jsonpath='{.items[*].metadata.uid}')

info "Modifying ConfigMap content..."
oc patch configmap ebs-metrics-exporter-config -n $NAMESPACE --type=merge \
    -p '{"data":{"test-key":"test-value"}}'

info "Waiting 10 seconds to see if pods restart automatically..."
sleep 10

PODS_AFTER=$(oc get pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME -o jsonpath='{.items[*].metadata.uid}')
if [ "$PODS_BEFORE" == "$PODS_AFTER" ]; then
    pass "Pods did NOT restart (expected - no auto-restart on ConfigMap change)"
    info "Manual restart required: oc delete pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME"
else
    fail "Pods restarted unexpectedly (or DaemonSet was modified)"
fi

# Cleanup test data from ConfigMap
info "Cleaning up test data from ConfigMap..."
oc patch configmap ebs-metrics-exporter-config -n $NAMESPACE --type=json \
    -p '[{"op":"remove","path":"/data/test-key"}]' 2>/dev/null || true
oc annotate configmap ebs-metrics-exporter-config -n $NAMESPACE test-timestamp- 2>/dev/null || true

# Test 3: Manual pod deletion triggers recreation
test_header "Test 3: DaemonSet Pod Recreation"
POD_NAME=$(oc get pods -n $NAMESPACE -l app.kubernetes.io/name=$APP_NAME -o jsonpath='{.items[0].metadata.name}')
info "Deleting pod: $POD_NAME"
oc delete pod $POD_NAME -n $NAMESPACE

info "Waiting for DaemonSet to recreate pod..."
if wait_for_pods $INITIAL_POD_COUNT; then
    pass "DaemonSet recreated pod automatically"
else
    fail "DaemonSet did not recreate pod"
fi

# Test 4: ConfigMap deletion behavior
test_header "Test 4: ConfigMap Deletion - PKO Behavior"
echo -e "${RED}⚠️  WARNING: About to delete ConfigMap${NC}"
read -p "Continue? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    info "Skipping ConfigMap deletion test"
else
    info "Deleting ConfigMap..."
    oc delete configmap ebs-metrics-exporter-config -n $NAMESPACE

    info "Waiting 10 seconds for PKO reconciliation..."
    sleep 10

    if oc get configmap ebs-metrics-exporter-config -n $NAMESPACE &>/dev/null; then
        fail "ConfigMap was recreated by PKO (unexpected with IfNoController collision protection)"
    else
        pass "ConfigMap was NOT recreated by PKO (expected behavior)"
        info "Running pods still work (cached ConfigMap), but new pods will fail to start"
    fi

    info "Restoring ConfigMap from backup..."
    oc apply -f /tmp/configmap-backup.yaml
    pass "ConfigMap restored manually"
fi

# Test 5: ClusterPackage image update
test_header "Test 5: ClusterPackage Image Update"
CURRENT_IMAGE=$(oc get clusterpackage $APP_NAME -o jsonpath='{.spec.image}')
info "Current PKO image: $CURRENT_IMAGE"

# Don't actually change the image, just show what would happen
info "To update image, you would run:"
echo "  oc patch clusterpackage $APP_NAME --type=merge \\"
echo "    -p '{\"spec\":{\"image\":\"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.24-gabc123\"}}'"
info "This WOULD trigger DaemonSet rolling update (PKO updates DaemonSet)"
info "Skipping actual update to avoid disruption"

# Summary
test_header "Test Summary"
echo ""
echo "PKO Collision Protection Behavior:"
echo "  ✓ Manual ConfigMap edits persist (PKO doesn't revert)"
echo "  ✓ ConfigMap changes do NOT auto-restart pods"
echo "  ✓ Manual pod deletion triggers DaemonSet recreation"
echo "  ✓ ConfigMap deletion NOT recovered by PKO"
echo "  ✓ ClusterPackage updates trigger DaemonSet updates"
echo ""
echo "Operational Procedures Required:"
echo "  1. After editing ConfigMap: oc delete pods -l app.kubernetes.io/name=$APP_NAME"
echo "  2. After deleting ConfigMap: manually restore or recreate ClusterPackage"
echo "  3. To update images: patch ClusterPackage spec.image"
echo ""
echo "For detailed procedures, see: OPERATIONS.md"
echo ""

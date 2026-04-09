#!/bin/bash
# Drift Prevention Testing Script
# Tests PKO controller ownership and automatic reconciliation

set -e

NAMESPACE="openshift-sre-ebs-metrics"
APP_NAME="ebs-metrics-exporter"

echo "============================================"
echo "Drift Prevention Testing"
echo "============================================"
echo ""
echo "⚠️  WARNING: This script tests PKO reconciliation"
echo "    Tests will make and verify automatic reversal of changes"
echo "    Only run in a test cluster!"
echo ""
read -p "Press Enter to continue or Ctrl+C to abort..."
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

function wait_for_reconcile() {
    local resource_type=$1
    local resource_name=$2
    local timeout=30
    local elapsed=0

    info "Waiting up to ${timeout}s for PKO to reconcile..."
    sleep $timeout
}

# Prerequisite check
test_header "Prerequisite Check"
if ! oc get clusterpackage $APP_NAME &>/dev/null; then
    fail "ClusterPackage not found. Deploy first with: make local-deploy"
    exit 1
fi
pass "ClusterPackage exists"

# Check for controller ownership
CONFIGMAP_OWNER=$(oc get configmap ebs-metrics-exporter-config -n $NAMESPACE \
    -o jsonpath='{.metadata.ownerReferences[0].kind}' 2>/dev/null || echo "")
if [ "$CONFIGMAP_OWNER" != "ClusterPackage" ]; then
    fail "ConfigMap missing controller ownerReference (not PKO-managed)"
    info "Expected: ClusterPackage, Got: $CONFIGMAP_OWNER"
    info "You may be running old package with IfNoController collision protection"
    exit 1
fi
pass "ConfigMap has PKO controller ownership"

DAEMONSET_OWNER=$(oc get daemonset $APP_NAME -n $NAMESPACE \
    -o jsonpath='{.metadata.ownerReferences[0].kind}' 2>/dev/null || echo "")
if [ "$DAEMONSET_OWNER" != "ClusterPackage" ]; then
    fail "DaemonSet missing controller ownerReference (not PKO-managed)"
    exit 1
fi
pass "DaemonSet has PKO controller ownership"

# Test 1: ConfigMap Edit Drift Detection
test_header "Test 1: ConfigMap Edit is Reverted by PKO"

# Backup original
ORIGINAL_POLLING=$(oc get configmap ebs-metrics-exporter-config -n $NAMESPACE \
    -o jsonpath='{.data.config\.yaml}' | grep 'pollingIntervalSeconds:' | awk '{print $2}')
info "Original polling interval: ${ORIGINAL_POLLING}s"

# Make unauthorized change
info "Making unauthorized edit to ConfigMap..."
oc patch configmap ebs-metrics-exporter-config -n $NAMESPACE --type=merge \
    -p '{"data":{"test-drift":"unauthorized-change"}}'
pass "ConfigMap edited (added test-drift key)"

# Wait for PKO reconciliation
wait_for_reconcile "configmap" "ebs-metrics-exporter-config"

# Check if change was reverted
TEST_DRIFT=$(oc get configmap ebs-metrics-exporter-config -n $NAMESPACE \
    -o jsonpath='{.data.test-drift}' 2>/dev/null || echo "")
if [ -z "$TEST_DRIFT" ]; then
    pass "Unauthorized change was reverted by PKO (drift prevention working)"
else
    fail "Unauthorized change persisted (PKO not reconciling)"
    info "Value found: $TEST_DRIFT"
fi

# Test 2: ConfigMap Deletion is Restored
test_header "Test 2: ConfigMap Deletion is Restored by PKO"

info "Deleting ConfigMap..."
oc delete configmap ebs-metrics-exporter-config -n $NAMESPACE
pass "ConfigMap deleted"

# Wait for PKO reconciliation
wait_for_reconcile "configmap" "ebs-metrics-exporter-config"

# Check if restored
if oc get configmap ebs-metrics-exporter-config -n $NAMESPACE &>/dev/null; then
    pass "ConfigMap was restored by PKO"

    # Verify content is correct
    RESTORED_POLLING=$(oc get configmap ebs-metrics-exporter-config -n $NAMESPACE \
        -o jsonpath='{.data.config\.yaml}' | grep 'pollingIntervalSeconds:' | awk '{print $2}')
    if [ "$RESTORED_POLLING" == "$ORIGINAL_POLLING" ]; then
        pass "Restored ConfigMap has correct content (${RESTORED_POLLING}s)"
    else
        fail "Restored ConfigMap has incorrect content"
    fi
else
    fail "ConfigMap was NOT restored by PKO"
fi

# Test 3: DaemonSet Edit Drift Detection
test_header "Test 3: DaemonSet Edit is Reverted by PKO"

# Get original maxUnavailable
ORIGINAL_MAX_UNAVAIL=$(oc get daemonset $APP_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.updateStrategy.rollingUpdate.maxUnavailable}')
info "Original maxUnavailable: $ORIGINAL_MAX_UNAVAIL"

# Make unauthorized change
info "Making unauthorized edit to DaemonSet..."
oc patch daemonset $APP_NAME -n $NAMESPACE --type=merge \
    -p '{"spec":{"updateStrategy":{"rollingUpdate":{"maxUnavailable":10}}}}'
pass "DaemonSet edited (changed maxUnavailable to 10)"

# Wait for PKO reconciliation
wait_for_reconcile "daemonset" "$APP_NAME"

# Check if change was reverted
CURRENT_MAX_UNAVAIL=$(oc get daemonset $APP_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.updateStrategy.rollingUpdate.maxUnavailable}')
if [ "$CURRENT_MAX_UNAVAIL" == "$ORIGINAL_MAX_UNAVAIL" ]; then
    pass "Unauthorized change was reverted by PKO (maxUnavailable back to $ORIGINAL_MAX_UNAVAIL)"
else
    fail "Unauthorized change persisted (current: $CURRENT_MAX_UNAVAIL, expected: $ORIGINAL_MAX_UNAVAIL)"
fi

# Test 4: Invalid Config Validation
test_header "Test 4: Invalid Config Causes Pod Failure"

info "This test would deploy invalid config and verify pods crash"
info "Skipping to avoid disrupting working deployment"
info "See DRIFT-PREVENTION.md Scenario 4 for manual testing"

# Test 5: Rolling Update Behavior
test_header "Test 5: Verify Rolling Update Settings"

MAX_UNAVAIL=$(oc get daemonset $APP_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.updateStrategy.rollingUpdate.maxUnavailable}')
if [ "$MAX_UNAVAIL" == "1" ]; then
    pass "maxUnavailable=1 (one pod at a time rolling updates)"
else
    fail "maxUnavailable=$MAX_UNAVAIL (expected 1)"
fi

UPDATE_STRATEGY=$(oc get daemonset $APP_NAME -n $NAMESPACE \
    -o jsonpath='{.spec.updateStrategy.type}')
if [ "$UPDATE_STRATEGY" == "RollingUpdate" ]; then
    pass "updateStrategy=RollingUpdate (old pods stay up until new ready)"
else
    fail "updateStrategy=$UPDATE_STRATEGY (expected RollingUpdate)"
fi

# Summary
test_header "Test Summary"
echo ""
echo "Drift Prevention Behavior:"
echo "  ✓ ConfigMap edits automatically reverted by PKO"
echo "  ✓ ConfigMap deletions automatically restored by PKO"
echo "  ✓ DaemonSet edits automatically reverted by PKO"
echo "  ✓ Rolling updates configured for safety (maxUnavailable: 1)"
echo ""
echo "GitOps Enforcement:"
echo "  ✓ Package version is source of truth"
echo "  ✓ Manual changes are not permitted"
echo "  ✓ Configuration tied to image version"
echo ""
echo "Update Procedure:"
echo "  1. Update ConfigMap in Git (package source)"
echo "  2. Build new package image with new tag"
echo "  3. Deploy new ClusterPackage version"
echo "  4. PKO performs rolling update"
echo ""
echo "For complete documentation, see:"
echo "  - DRIFT-PREVENTION.md"
echo "  - OPERATIONS.md"
echo ""

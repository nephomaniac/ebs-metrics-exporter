#!/usr/bin/env bash
#
# Development setup helper script
# Sets up environment for local development and testing
#

set -e

echo "=== EBS Metrics Exporter Development Setup ==="
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check oc
if ! command -v oc &> /dev/null; then
    echo "❌ oc (OpenShift CLI) not found. Install from: https://mirror.openshift.com/pub/openshift-v4/clients/ocp/"
    exit 1
fi
echo "✓ oc found: $(oc version --client -o yaml | grep gitVersion | awk '{print $2}')"

# Check docker or podman
if command -v docker &> /dev/null; then
    CONTAINER_ENGINE="docker"
    echo "✓ docker found: $(docker --version)"
elif command -v podman &> /dev/null; then
    CONTAINER_ENGINE="podman"
    echo "✓ podman found: $(podman --version)"
else
    echo "❌ Neither docker nor podman found. Please install one of them."
    exit 1
fi

# Check go
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Install from: https://go.dev/dl/"
    exit 1
fi
echo "✓ go found: $(go version)"

echo ""
echo "=== Configuration ==="
echo ""

# Get Quay.io username
if [ -z "$QUAY_USER" ]; then
    echo "QUAY_USER environment variable not set."
    echo ""
    read -p "Enter your Quay.io username: " QUAY_USER

    if [ -z "$QUAY_USER" ]; then
        echo "❌ Quay username required"
        exit 1
    fi

    echo ""
    echo "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo ""
    echo "  export QUAY_USER=$QUAY_USER"
    echo ""
    read -p "Press Enter to continue..."
fi

echo "✓ QUAY_USER: $QUAY_USER"

# Set image variables
export DEV_IMG="quay.io/${QUAY_USER}/ebs-metrics-exporter:test"
export DEV_IMG_PKO="quay.io/${QUAY_USER}/ebs-metrics-exporter-pko:test"

echo "✓ Application image: $DEV_IMG"
echo "✓ PKO package image: $DEV_IMG_PKO"

echo ""
echo "=== Cluster Connection ==="
echo ""

# Check cluster connection
if oc whoami &> /dev/null; then
    echo "✓ Connected to: $(oc whoami --show-server)"
    echo "✓ Logged in as: $(oc whoami)"
else
    echo "⚠ Not connected to an OpenShift cluster"
    echo ""
    read -p "Enter cluster API URL (e.g., https://api.cluster.example.com:6443): " CLUSTER_URL

    if [ -n "$CLUSTER_URL" ]; then
        echo "Logging in to $CLUSTER_URL..."
        oc login "$CLUSTER_URL"
    else
        echo "⚠ Skipping cluster login. You can login later with: oc login <url>"
    fi
fi

echo ""
echo "=== Ready to Start Development ==="
echo ""
echo "Next steps:"
echo ""
echo "  1. Build images:"
echo "     make dev-build"
echo ""
echo "  2. Push to Quay.io:"
echo "     make dev-push"
echo ""
echo "  3. Deploy to cluster:"
echo "     make dev-deploy"
echo ""
echo "  4. Check status:"
echo "     make dev-status"
echo ""
echo "  5. View logs:"
echo "     make dev-logs"
echo ""
echo "  6. Make changes and rebuild:"
echo "     make dev-rebuild"
echo ""
echo "For more help: make help"
echo ""
echo "✅ Setup complete!"

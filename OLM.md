# OLM (Operator Lifecycle Manager) Deployment

This guide explains how to deploy the EBS Metrics Exporter Operator using OLM (Operator Lifecycle Manager) in OpenShift.

## Overview

OLM provides a declarative way to install, manage, and upgrade operators in Kubernetes/OpenShift clusters. This operator is packaged as an OLM bundle that can be:

1. Published to a catalog and installed via OperatorHub
2. Installed directly using a CatalogSource
3. Installed for development/testing using operator-sdk

## Prerequisites

### Required Tools

- **OpenShift 4.10+** (OLM is pre-installed)
- **oc CLI** with cluster-admin access
- **operator-sdk** v1.28.0+ (for bundle validation)
- **opm** (Operator Package Manager) for catalog builds
- **Docker or Podman** for building bundle/catalog images

### Install operator-sdk

```bash
# macOS
brew install operator-sdk

# Linux
export ARCH=$(case $(uname -m) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(uname -m) ;; esac)
export OS=$(uname | awk '{print tolower($0)}')
export OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/v1.28.0
curl -LO ${OPERATOR_SDK_DL_URL}/operator-sdk_${OS}_${ARCH}
chmod +x operator-sdk_${OS}_${ARCH} && sudo mv operator-sdk_${OS}_${ARCH} /usr/local/bin/operator-sdk
```

### Install opm

```bash
# macOS
brew install operator-framework/tap/opm

# Linux
export ARCH=$(case $(uname -m) in x86_64) echo -n amd64 ;; aarch64) echo -n arm64 ;; *) echo -n $(uname -m) ;; esac)
export OS=$(uname | awk '{print tolower($0)}')
export OPM_DL_URL=https://github.com/operator-framework/operator-registry/releases/download/v1.28.0
curl -LO ${OPM_DL_URL}/${OS}-${ARCH}-opm
chmod +x ${OS}-${ARCH}-opm && sudo mv ${OS}-${ARCH}-opm /usr/local/bin/opm
```

## Bundle Structure

The OLM bundle is located in the `bundle/` directory:

```
bundle/
├── bundle.Dockerfile              # Dockerfile for bundle image
├── manifests/
│   └── ebs-metrics-exporter.clusterserviceversion.yaml  # CSV manifest
└── metadata/
    └── annotations.yaml           # Bundle metadata
```

### ClusterServiceVersion (CSV)

The CSV (`bundle/manifests/ebs-metrics-exporter.clusterserviceversion.yaml`) contains:
- Operator metadata (name, version, description)
- Install strategy (deployment spec, RBAC)
- Owned CRDs (none for this operator)
- Required permissions
- Related images

### Bundle Annotations

The `bundle/metadata/annotations.yaml` file contains:
- Bundle format version
- Package name and channels
- OpenShift version compatibility

## Deployment Methods

### Method 1: Via Custom CatalogSource (Recommended)

This method creates a custom catalog containing your operator bundle.

#### 1. Build and Push Images

```bash
# Set your registry
export IMAGE_REGISTRY=quay.io/your-org
export OPERATOR_VERSION=0.1.0

# Build operator image
make docker-build-operator IMG_OPERATOR=${IMAGE_REGISTRY}/ebs-metrics-exporter-operator:v${OPERATOR_VERSION}
make docker-push-operator IMG_OPERATOR=${IMAGE_REGISTRY}/ebs-metrics-exporter-operator:v${OPERATOR_VERSION}

# Build collector image
make docker-build-collector IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter:v${OPERATOR_VERSION}
make docker-push-collector IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter:v${OPERATOR_VERSION}
```

#### 2. Update CSV with Image References

Edit `bundle/manifests/ebs-metrics-exporter.clusterserviceversion.yaml`:

```yaml
spec:
  install:
    spec:
      deployments:
      - name: ebs-metrics-exporter-operator
        spec:
          template:
            spec:
              containers:
              - image: quay.io/your-org/ebs-metrics-exporter-operator:v0.1.0
  relatedImages:
  - name: ebs-metrics-exporter-operator
    image: quay.io/your-org/ebs-metrics-exporter-operator:v0.1.0
  - name: ebs-metrics-exporter
    image: quay.io/your-org/ebs-metrics-exporter:v0.1.0
```

#### 3. Build and Push Bundle

```bash
# Build bundle image
make bundle-build BUNDLE_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION}

# Push bundle image
make bundle-push BUNDLE_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION}
```

#### 4. Build and Push Catalog

```bash
# Build catalog image
make catalog-build \
  BUNDLE_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION} \
  CATALOG_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-catalog:v${OPERATOR_VERSION}

# Push catalog image
make catalog-push CATALOG_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-catalog:v${OPERATOR_VERSION}
```

#### 5. Create CatalogSource

```bash
# Deploy catalog source
make olm-deploy CATALOG_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-catalog:v${OPERATOR_VERSION}

# Or manually:
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: ebs-metrics-exporter-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: ${IMAGE_REGISTRY}/ebs-metrics-exporter-catalog:v${OPERATOR_VERSION}
  displayName: EBS Metrics Exporter
  publisher: Red Hat
  updateStrategy:
    registryPoll:
      interval: 10m
EOF
```

#### 6. Verify CatalogSource

```bash
# Check catalog is ready
oc get catalogsource -n openshift-marketplace ebs-metrics-exporter-catalog

# Check packages available
oc get packagemanifests -n openshift-marketplace | grep ebs-metrics
```

#### 7. Install via OperatorHub UI

1. Navigate to **OperatorHub** in OpenShift Console
2. Search for "EBS Metrics Exporter"
3. Click **Install**
4. Select:
   - **Update channel**: alpha
   - **Installation mode**: A specific namespace on the cluster
   - **Installed Namespace**: openshift-sre-ebs-metrics (or create new)
   - **Update approval**: Automatic
5. Click **Install**

#### 8. Install via CLI (Subscription)

```bash
# Create namespace
oc create namespace openshift-sre-ebs-metrics

# Label namespace for monitoring
oc label namespace openshift-sre-ebs-metrics openshift.io/cluster-monitoring=true

# Create Subscription
cat <<EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ebs-metrics-exporter
  namespace: openshift-sre-ebs-metrics
spec:
  channel: alpha
  name: ebs-metrics-exporter
  source: ebs-metrics-exporter-catalog
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic
EOF
```

#### 9. Verify Installation

```bash
# Check subscription
oc get subscription -n openshift-sre-ebs-metrics

# Check install plan
oc get installplan -n openshift-sre-ebs-metrics

# Check operator pod
oc get pods -n openshift-sre-ebs-metrics

# Check CSV
oc get csv -n openshift-sre-ebs-metrics
```

### Method 2: operator-sdk run bundle (Development/Testing)

For quick testing without building a catalog:

```bash
# Build and push bundle
make bundle-build BUNDLE_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION}
make bundle-push BUNDLE_IMG=${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION}

# Install operator using operator-sdk
operator-sdk run bundle ${IMAGE_REGISTRY}/ebs-metrics-exporter-bundle:v${OPERATOR_VERSION} \
  --namespace openshift-sre-ebs-metrics

# Verify
oc get csv -n openshift-sre-ebs-metrics
oc get pods -n openshift-sre-ebs-metrics

# Cleanup
operator-sdk cleanup ebs-metrics-exporter --namespace openshift-sre-ebs-metrics
```

## Bundle Validation

### Validate Bundle Format

```bash
# Validate bundle structure and manifests
make bundle-validate

# Or manually:
operator-sdk bundle validate ./bundle --select-optional suite=operatorframework
```

### Common Validation Checks

The validator checks for:
- ✅ Valid CSV format
- ✅ Required annotations present
- ✅ RBAC permissions defined
- ✅ Valid install modes
- ✅ Image references in relatedImages
- ✅ Proper version format

### Fix Common Validation Errors

**Error: Missing relatedImages**
```yaml
# Add to CSV spec:
spec:
  relatedImages:
  - name: operator
    image: quay.io/your-org/ebs-metrics-exporter-operator:v0.1.0
```

**Error: Invalid channel**
```yaml
# In bundle/metadata/annotations.yaml:
operators.operatorframework.io.bundle.channels.v1: alpha
operators.operatorframework.io.bundle.channel.default.v1: alpha
```

## Upgrading the Operator

### Create New Bundle Version

1. **Update version in project.mk:**
   ```makefile
   OPERATOR_VERSION ?= 0.2.0
   ```

2. **Update CSV:**
   ```bash
   cp bundle/manifests/ebs-metrics-exporter.clusterserviceversion.yaml \
      bundle/manifests/ebs-metrics-exporter.v0.2.0.clusterserviceversion.yaml
   ```

3. **Edit new CSV:**
   - Update `metadata.name` to `ebs-metrics-exporter.v0.2.0`
   - Update `spec.version` to `0.2.0`
   - Add `spec.replaces: ebs-metrics-exporter.v0.1.0`
   - Update image tags

4. **Build and push:**
   ```bash
   make bundle-build OPERATOR_VERSION=0.2.0
   make bundle-push OPERATOR_VERSION=0.2.0
   make catalog-build OPERATOR_VERSION=0.2.0
   make catalog-push OPERATOR_VERSION=0.2.0
   ```

5. **Update CatalogSource** with new catalog image

## Uninstalling

### Remove Operator Installation

```bash
# Delete subscription
oc delete subscription ebs-metrics-exporter -n openshift-sre-ebs-metrics

# Delete CSV (if automatic cleanup doesn't work)
oc delete csv -n openshift-sre-ebs-metrics -l operators.coreos.com/ebs-metrics-exporter.openshift-sre-ebs-metrics=

# Remove catalog source
make olm-undeploy

# Or manually:
oc delete catalogsource ebs-metrics-exporter-catalog -n openshift-marketplace
```

### Remove Operator Resources

```bash
# Remove namespace
oc delete namespace openshift-sre-ebs-metrics
```

## Troubleshooting

### Operator Not Appearing in OperatorHub

```bash
# Check CatalogSource status
oc get catalogsource -n openshift-marketplace ebs-metrics-exporter-catalog -o yaml

# Check catalog pod logs
POD=$(oc get pods -n openshift-marketplace -l olm.catalogSource=ebs-metrics-exporter-catalog -o jsonpath='{.items[0].metadata.name}')
oc logs -n openshift-marketplace $POD

# Check package manifest
oc get packagemanifest ebs-metrics-exporter -o yaml
```

### Subscription in Pending State

```bash
# Check subscription status
oc describe subscription -n openshift-sre-ebs-metrics ebs-metrics-exporter

# Check install plan
oc get installplan -n openshift-sre-ebs-metrics
oc describe installplan -n openshift-sre-ebs-metrics <install-plan-name>

# Approve manual install plan (if needed)
oc patch installplan <install-plan-name> \
  -n openshift-sre-ebs-metrics \
  --type merge \
  --patch '{"spec":{"approved":true}}'
```

### CSV Installation Failed

```bash
# Check CSV status
oc get csv -n openshift-sre-ebs-metrics
oc describe csv -n openshift-sre-ebs-metrics <csv-name>

# Check operator pod logs
oc logs -n openshift-sre-ebs-metrics deployment/ebs-metrics-exporter-operator

# Check OLM operator logs
oc logs -n openshift-operator-lifecycle-manager deployment/olm-operator
```

### Bundle Validation Failures

```bash
# Run validation with verbose output
operator-sdk bundle validate ./bundle -b docker --verbose

# Check specific validation issues
operator-sdk bundle validate ./bundle --select-optional suite=operatorframework --verbose
```

## Publishing to OperatorHub

To publish to the official OperatorHub (community-operators or certified-operators):

1. Fork the appropriate repository:
   - Community: https://github.com/k8s-operatorhub/community-operators
   - Certified: https://github.com/redhat-openshift-ecosystem/certified-operators

2. Create bundle in operators/<operator-name>/<version>/ directory

3. Submit pull request

4. Address review feedback

See: https://operator-framework.github.io/community-operators/

## References

- [OLM Documentation](https://olm.operatorframework.io/)
- [operator-sdk Bundle Guide](https://sdk.operatorframework.io/docs/olm-integration/tutorial-bundle/)
- [OpenShift Operator Certification](https://connect.redhat.com/en/partner-with-us/red-hat-openshift-operator-certification)
- [Bundle Format Specification](https://olm.operatorframework.io/docs/tasks/creating-operator-bundle/)

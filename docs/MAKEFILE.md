# Makefile Targets Reference

All targets follow the **OpenShift Boilerplate pattern**. Targets are either:
- ✅ **Boilerplate targets** - Provided by `boilerplate/openshift/golang-osd-operator/`
- 🔧 **Local development targets** - Custom targets that call boilerplate (prefixed with `local-`)

---

## Environment Management Targets

### `make init-env`
Create `.env.local` from template for personal development settings.

**When to use:**
- First time setting up the project
- Want to persist environment variables locally

**What it does:**
- Copies `.env.local.template` to `.env.local`
- Provides guidance on required settings
- File is git-ignored (safe for personal settings)

**Example:**
```bash
make init-env
# Edit .env.local and set IMAGE_REPOSITORY
vim .env.local
```

### `make show-env`
Display current environment configuration and computed values.

**When to use:**
- Verify your settings before building/deploying
- Debug why images are going to wrong registry
- Check if EXPECTED_CLUSTER is set

**What it shows:**
- Image configuration (registry, repository, tag)
- Computed image URIs
- Build settings (container engine, FIPS, etc.)
- Cluster safety settings
- Whether .env.local exists

**Example:**
```bash
make show-env
# Shows all current environment variables and their values
```

### `make check-cluster`
Verify you're connected to the expected test cluster.

**When to use:**
- Before deploying to prevent wrong-cluster accidents
- Automatically called by `local-deploy` and `local-undeploy`

**What it does:**
- Extracts cluster ID from `oc whoami --show-server`
- Compares against EXPECTED_CLUSTER variable
- Fails if mismatch (safety check)

**Example:**
```bash
export EXPECTED_CLUSTER=my-test-cluster
make check-cluster
# ✅ Cluster verified: my-test-cluster
```

---

## Top-Level Workflow Targets

### `make local-ci`
Build and test everything (mimics Konflux CI pipeline).

**When to use:**
- Pre-commit validation
- Verify changes before pushing
- Test without deploying to cluster

**What it does:**
1. Build application image (`docker-build`)
2. Build PKO package image (`local-build-pko`)
3. Run unit tests (`go-test`)
4. Validate PKO templates (`validate-pko-fixtures`)

**Requirements:**
- Podman or Docker
- No cluster access required
- No image push required

**Example:**
```bash
make local-ci ALLOW_DIRTY_CHECKOUT=true
# Runs all build and test steps locally
```

### `make local`
Full workflow: build → test → push → deploy → verify.

**When to use:**
- Complete end-to-end testing
- Deploy your changes to test cluster
- After making code changes

**What it does:**
1. Phase 1: Build and test (`local-ci`)
2. Phase 2: Push images (`local-push-all`)
3. Phase 3: Deploy (`local-deploy`)
4. Phase 4: Show status (`local-status`)

**Requirements:**
- Cluster access (`oc login`)
- Registry access (`podman login quay.io`)
- IMAGE_REPOSITORY set

**Example:**
```bash
make local ALLOW_DIRTY_CHECKOUT=true
# Full pipeline from build to deployment
```

---

## Build Targets

### `make docker-build`
Build application image with FIPS/BoringCrypto (boilerplate target).

**When to use:**
- Build the main exporter binary
- Test with FIPS-enabled crypto

**What it does:**
- Uses `build/Dockerfile` with multi-stage build
- Compiles with Go 1.23+ and BoringCrypto
- Tags as `${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}/${OPERATOR_NAME}:${OPERATOR_IMAGE_TAG}`

**Example:**
```bash
make docker-build ALLOW_DIRTY_CHECKOUT=true
# Builds: quay.io/your-username/ebs-metrics-exporter:v0.1.23-g<sha>
```

### `make local-build-pko`
Build PKO package image (YAML manifests wrapper).

**When to use:**
- After changing deploy_pko/ manifests
- Testing PKO deployment changes

**What it does:**
- Uses `build/Dockerfile.pko` (scratch + YAML)
- Context is `deploy_pko/` directory
- Tags as `${OPERATOR_IMAGE_URI_LATEST}-pko`

**Example:**
```bash
make local-build-pko
# Builds: quay.io/your-username/ebs-metrics-exporter-pko:latest
```

### `make local-build-all`
Build both application and PKO images.

**When to use:**
- Building everything at once
- Part of local-ci workflow

**What it does:**
- Calls `docker-build`
- Calls `local-build-pko`

**Example:**
```bash
make local-build-all ALLOW_DIRTY_CHECKOUT=true
```

### `make go-build`
Build Go binary locally (for testing, not containerized).

**When to use:**
- Quick local binary builds
- Testing code changes without Docker

**What it does:**
- Compiles to `build/_output/bin/`
- Uses local Go toolchain (not containerized)

**Example:**
```bash
make go-build
./build/_output/bin/ebs-metrics-exporter --help
```

---

## Push Targets

### `make docker-push`
Push application image to registry (boilerplate target).

**When to use:**
- After successful `docker-build`
- Before deploying

**Requirements:**
- Must be logged into registry (`podman login quay.io`)
- Image must be built first

**Example:**
```bash
make docker-push
```

### `make local-push-pko`
Push PKO package image to registry.

**When to use:**
- After successful `local-build-pko`
- Before deploying

**Example:**
```bash
make local-push-pko
```

### `make local-push-all`
Push both images to registry.

**When to use:**
- After building both images
- Part of `local` workflow

**Example:**
```bash
make local-push-all
```

---

## Testing Targets

### `make go-test`
Run Go unit tests (boilerplate target).

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

## Deployment Targets

### `make local-deploy`
Deploy via PKO operator (with cluster safety check).

**When to use:**
- After pushing images
- Deploy changes to test cluster

**What it does:**
1. Runs `check-cluster` (safety verification)
2. Processes `hack/pko/clusterpackage-direct.yaml`
3. Creates ClusterPackage resource
4. PKO operator deploys in phases

**Requirements:**
- Images pushed to registry
- EXPECTED_CLUSTER set (if using safety checks)
- Cluster access

**Example:**
```bash
make local-deploy
# Deploys using current IMAGE_REPOSITORY and OPERATOR_IMAGE_TAG
```

### `make local-undeploy`
Remove ClusterPackage (PKO cleans up everything).

**When to use:**
- Clean up after testing
- Remove deployment before redeploying

**What it does:**
1. Runs `check-cluster` (safety verification)
2. Deletes ClusterPackage resource
3. PKO operator removes all resources in reverse order

**Example:**
```bash
make local-undeploy
```

### `make local-status`
Show deployment status (read-only, no cluster check).

**When to use:**
- Check if deployment is ready
- Debug deployment issues

**What it shows:**
- ClusterPackage status
- Namespace
- DaemonSet status
- Pod list

**Example:**
```bash
make local-status
```

### `make local-logs`
View logs from all pods.

**When to use:**
- Debug runtime issues
- Verify metrics collection

**Example:**
```bash
make local-logs
# Shows last 50 lines from all pods
```

### `make local-restart`
Restart DaemonSet pods to apply configuration changes.

**When to use:**
- After editing ConfigMap via `oc edit`
- After making manual configuration changes
- To apply new configuration without redeploying

**What it does:**
1. Checks for running pods
2. Deletes all pods (DaemonSet recreates them)
3. Waits for restart
4. Shows deployment status

**Why needed:**
- Pods don't automatically restart when ConfigMap changes
- This is standard Kubernetes behavior
- Manual restart applies new configuration

**Example:**
```bash
# 1. Edit ConfigMap
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# 2. Restart pods to apply changes
make local-restart

# Output shows pods being deleted and recreated
```

**See also:** [OPERATIONS.md](../OPERATIONS.md) for detailed configuration management procedures.

---

## Utility Targets

### `make clean`
Remove build artifacts.

**When to use:**
- Clean build directory
- Force rebuild

**What it removes:**
- `build/_output/`
- `_output/`
- Built binaries

**Example:**
```bash
make clean
```

### `make boilerplate-update`
Update boilerplate framework to latest version.

**When to use:**
- Periodic updates
- Get new boilerplate features

**What it does:**
- Runs `boilerplate/update` script
- Updates make includes and scripts

**Example:**
```bash
make boilerplate-update
```

### `make docker-login`
Login to container registry (prompts for credentials).

**When to use:**
- Before pushing images
- If podman/docker not authenticated

**Example:**
```bash
make docker-login
# Prompts for registry, username, password
```

---

## Related Documentation

- **[Quick Start Guide](QUICKSTART.md)** - Get started quickly
- **[Advanced Workflows](ADVANCED_WORKFLOWS.md)** - Step-by-step workflows
- **[Testing Guide](TESTING.md)** - Testing procedures

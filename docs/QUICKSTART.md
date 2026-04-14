# Quick Start (Local Development)

This project follows the **OpenShift Boilerplate PKO pattern**. All builds use boilerplate-provided targets that mimic production CI/CD pipelines.

## Prerequisites

- Go 1.23+ (for local builds)
- Podman or Docker
- OpenShift CLI (`oc`) with cluster access
- Quay.io account (for pushing images)
- `IMAGE_REPOSITORY` environment variable set

## 1. Set Your Image Repository

**Option A: Use local config file (recommended)**

```bash
# Create .env.local from template
make init-env

# Edit .env.local and set your Quay.io username
# IMAGE_REPOSITORY=your-quay-username
vim .env.local

# Verify settings
make show-env
```

**Option B: Export environment variable**

```bash
# Set your personal Quay.io username (used by all boilerplate targets)
export IMAGE_REPOSITORY=your-quay-username

# Add to your shell profile for persistence
echo 'export IMAGE_REPOSITORY=your-quay-username' >> ~/.zshrc
source ~/.zshrc
```

**What this controls:**
- Application image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`
- PKO package image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

**Tip:** The `.env.local` file is git-ignored and safe for personal settings. It's automatically loaded by all `make` commands.

## 2. Local CI - Build and Test (No Cluster Required)

```bash
# Build both images and run all tests (mimics Konflux CI)
make local-ci ALLOW_DIRTY_CHECKOUT=true
```

<details>
<summary>What this does (mimics production CI)</summary>

1. **Build application image** (boilerplate `docker-build`):
   - Uses `build/Dockerfile` with FIPS/BoringCrypto
   - Multi-stage build with Go 1.23+
   - Output: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`

2. **Build PKO package image**:
   - Uses `build/Dockerfile.pko` (scratch + YAML manifests)
   - Context: `deploy_pko/` directory
   - Output: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

3. **Run tests**:
   - `go-test` - Unit tests
   - `validate-pko-fixtures` - PKO template validation

**No cluster access or image push required** - perfect for pre-commit validation.
</details>

## 3. Full Workflow - Build, Test, Push, Deploy

```bash
# Login to your OpenShift cluster
oc login https://api.your-cluster.com

# Ensure you're logged into Quay.io
podman login quay.io

# Full workflow: build → test → push → deploy
make local ALLOW_DIRTY_CHECKOUT=true
```

<details>
<summary>What this does (complete local pipeline)</summary>

**Step 1: Build and Test** (via `local-ci`):
- Build both images
- Run all tests

**Step 2: Push Images** (via `local-push-all`):
- Push application image: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter:latest`
- Push PKO package: `quay.io/${IMAGE_REPOSITORY}/ebs-metrics-exporter-pko:latest`

**Step 3: Deploy via PKO Operator** (via `local-deploy`):
- Process `hack/pko/clusterpackage-direct.yaml` template
- Create `ClusterPackage` resource
- PKO operator deploys in phases: namespace → rbac → deploy

**Step 4: Show Status**:
- Display ClusterPackage, Namespace, DaemonSet, Pods
</details>

## 4. Verify Deployment

```bash
# Check deployment status
make local-status

# View logs from all pods
make local-logs
```

<details>
<summary>Expected output</summary>

```
=== ClusterPackage ===
NAME                   PHASE       STATUS
ebs-metrics-exporter   Available   True

=== Namespace ===
NAME                        STATUS   AGE
openshift-sre-ebs-metrics   Active   2m

=== DaemonSet ===
NAME                   DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE
ebs-metrics-exporter   3         3         3       3            3

=== Pods ===
NAME                         READY   STATUS    RESTARTS   AGE
ebs-metrics-exporter-abc123  1/1     Running   0          2m
ebs-metrics-exporter-def456  1/1     Running   0          2m
ebs-metrics-exporter-ghi789  1/1     Running   0          2m
```
</details>

## 5. Clean Up

```bash
# Remove ClusterPackage (PKO operator cleans up everything)
make local-undeploy

# Remove local build artifacts
make clean
```

---

## Next Steps

- **[Advanced Workflows](ADVANCED_WORKFLOWS.md)** - Step-by-step control over build/test/deploy
- **[Testing Guide](TESTING.md)** - Unit and integration testing procedures
- **[Configuration Guide](../CONFIG.md)** - Configure device discovery and metrics collection
- **[Makefile Targets](MAKEFILE.md)** - Complete reference of all make targets

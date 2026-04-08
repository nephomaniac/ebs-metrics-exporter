# PKO Build and Deployment Guide

This document explains how boilerplate integrates with Package Operator (PKO) builds for the EBS Metrics Exporter.

## Overview

PKO deployment requires **two container images**:

1. **Application Image** - The actual binary
2. **PKO Package Image** - The deployment manifests

Both are built via Konflux (Red Hat's internal CI/CD) using Tekton pipelines provided by boilerplate.

## Two-Image Architecture

### 1. Application Image: `ebs-metrics-exporter`

**Purpose**: Contains the Go binary that runs on cluster nodes

**Built from**: `Dockerfile`
```dockerfile
FROM registry.access.redhat.com/ubi9/go-toolset:1.22 AS builder
# ... build ebs-metrics-exporter binary
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /workspace/ebs-metrics-exporter .
ENTRYPOINT ["/ebs-metrics-exporter"]
```

**Tekton Pipelines**:
- `.tekton/ebs-metrics-exporter-pull-request.yaml` - PR builds
- `.tekton/ebs-metrics-exporter-push.yaml` - Main branch builds

**Build Parameters**:
```yaml
- name: dockerfile
  value: Dockerfile
- name: path-context
  value: .  # Root of repo
```

**Output**: `quay.io/app-sre/ebs-metrics-exporter:v1.0.0`

### 2. PKO Package Image: `ebs-metrics-exporter-pko`

**Purpose**: Contains YAML manifests for PKO deployment

**Built from**: `build/Dockerfile.pko`
```dockerfile
FROM scratch
COPY * /package/
```

**Tekton Pipelines**:
- `.tekton/ebs-metrics-exporter-pko-pull-request.yaml` - PR builds
- `.tekton/ebs-metrics-exporter-pko-push.yaml` - Main branch builds

**Build Parameters**:
```yaml
- name: dockerfile
  value: build/Dockerfile.pko
- name: path-context
  value: deploy_pko  # Only copy manifests
```

**Output**: `quay.io/app-sre/ebs-metrics-exporter-pko:v1.0.0`

**Contents** (from `deploy_pko/`):
```
/package/
├── manifest.yaml
├── Namespace-openshift-sre-ebs-metrics.yaml
├── ServiceAccount-ebs-metrics-exporter.yaml
├── SecurityContextConstraints-ebs-metrics-exporter.yaml
├── Role-prometheus-k8s.yaml
├── RoleBinding-prometheus-k8s.yaml
├── DaemonSet-ebs-metrics-exporter.yaml.gotmpl
├── Service-ebs-metrics-exporter.yaml
└── ServiceMonitor-ebs-metrics-exporter.yaml
```

## Boilerplate Integration

### Centralized Tekton Pipeline

All `.tekton/*.yaml` files reference boilerplate's centralized pipeline:

```yaml
spec:
  pipelineRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/openshift/boilerplate
    - name: revision
      value: master
    - name: pathInRepo
      value: pipelines/docker-build-oci-ta/pipeline.yaml
```

**Benefits**:
- ✅ Single source of truth for build logic
- ✅ Automatic updates when boilerplate improves
- ✅ OCI trusted artifacts support
- ✅ Multi-arch builds (amd64, arm64)
- ✅ SBOM generation
- ✅ Security scanning

### Konflux/AppStudio Integration

The `.tekton/` pipelines are configured for Konflux:

```yaml
metadata:
  labels:
    appstudio.openshift.io/application: ebs-metrics-exporter
    appstudio.openshift.io/component: ebs-metrics-exporter-pko
  namespace: ebs-metrics-exporter-tenant
```

**Tenant Namespace**: `ebs-metrics-exporter-tenant` (Konflux workspace)

**Image Registry**: `quay.io/redhat-user-workloads/ebs-metrics-exporter-tenant/`

## Build Workflow

### Local Development Builds

For testing locally:

```bash
# Build application image
make docker-build IMG=quay.io/yourusername/ebs-metrics-exporter:test

# Build PKO package image
docker build -f build/Dockerfile.pko -t quay.io/yourusername/ebs-metrics-exporter-pko:test deploy_pko/

# Push both
docker push quay.io/yourusername/ebs-metrics-exporter:test
docker push quay.io/yourusername/ebs-metrics-exporter-pko:test
```

### Konflux Automated Builds

When code is pushed to GitHub:

**1. Pull Request** (e.g., feature branch → main):
- Triggers: `*-pko-pull-request.yaml` and `*-pull-request.yaml`
- Builds both images with tag `on-pr-<commit-sha>`
- Images expire after 3 days
- Registry: `quay.io/redhat-user-workloads/.../on-pr-abc123`

**2. Main Branch Push**:
- Triggers: `*-pko-push.yaml` and `*-push.yaml`
- Builds both images with tag `<commit-sha>`
- Images persist (production builds)
- Registry: `quay.io/redhat-user-workloads/.../abc123`

**3. Release Tag** (manual):
- App-SRE promotes images to production registry
- Final location: `quay.io/app-sre/ebs-metrics-exporter:v1.0.0`

## PKO Deployment Flow

### 1. Build Phase (Konflux)

```
┌─────────────────────────────────────────────┐
│  GitHub Push to main                        │
└──────────────┬──────────────────────────────┘
               │
               ├─► Tekton: ebs-metrics-exporter-push.yaml
               │   └─► Dockerfile (context: .)
               │       └─► Output: quay.io/.../ebs-metrics-exporter:abc123
               │
               └─► Tekton: ebs-metrics-exporter-pko-push.yaml
                   └─► Dockerfile.pko (context: deploy_pko/)
                       └─► Output: quay.io/.../ebs-metrics-exporter-pko:abc123
```

### 2. Deployment Phase (app-interface)

```
┌─────────────────────────────────────────────┐
│  app-interface GitOps repository            │
│  - Defines which clusters get which version │
└──────────────┬──────────────────────────────┘
               │
               ├─► Production Clusters
               │   └─► PKO Package Image: quay.io/app-sre/ebs-metrics-exporter-pko:v1.0.0
               │
               └─► Staging Clusters
                   └─► PKO Package Image: quay.io/app-sre/ebs-metrics-exporter-pko:v1.1.0-rc1
```

### 3. Runtime (OpenShift Cluster)

```
┌─────────────────────────────────────────────┐
│  Package Operator (PKO) on cluster          │
└──────────────┬──────────────────────────────┘
               │
               ├─► Pulls: ebs-metrics-exporter-pko:v1.0.0
               │   └─► Extracts: /package/*.yaml manifests
               │
               ├─► Creates: Namespace, RBAC, DaemonSet, etc.
               │
               └─► DaemonSet pulls: ebs-metrics-exporter:v1.0.0
                   └─► Runs on all nodes
                       └─► Collects EBS metrics via IOCTL
```

## PKO Package Image vs Application Image

| Aspect | PKO Package Image | Application Image |
|--------|------------------|-------------------|
| **Built from** | `build/Dockerfile.pko` | `Dockerfile` |
| **Base** | `FROM scratch` | `FROM ubi9/go-toolset` + `ubi9/ubi-minimal` |
| **Contents** | YAML manifests | Go binary |
| **Size** | ~10 KB | ~11 MB |
| **Purpose** | Deployment config | Runtime application |
| **Context** | `deploy_pko/` | Entire repo |
| **Consumed by** | Package Operator | Kubernetes |
| **Updates** | When manifests change | When code changes |

## Versioning Strategy

Both images use the same version:

```bash
# Git commit triggers builds
git commit -m "Add new metric"
git push

# Konflux builds both:
# - ebs-metrics-exporter:abc123def
# - ebs-metrics-exporter-pko:abc123def

# App-SRE promotes to release:
# - ebs-metrics-exporter:v1.0.0
# - ebs-metrics-exporter-pko:v1.0.0
```

**Important**: The PKO package image references the application image via template:

```yaml
# In deploy_pko/DaemonSet-ebs-metrics-exporter.yaml.gotmpl
containers:
- name: ebs-metrics-exporter
  image: '{{ .config.image }}'  # Injected at deployment time
```

The `{{ .config.image }}` is configured in app-interface:

```yaml
# In app-interface saas file
parameters:
  IMAGE_TAG: v1.0.0
resourceTemplates:
- name: ebs-metrics-exporter
  targets:
  - namespace: openshift-sre-ebs-metrics
    ref: main
    parameters:
      PACKAGE_IMAGE: quay.io/app-sre/ebs-metrics-exporter-pko:v1.0.0
      PACKAGE_IMAGE_TAG: v1.0.0
  parameters:
    IMAGE: quay.io/app-sre/ebs-metrics-exporter
    IMAGE_TAG: v1.0.0
```

## Boilerplate Maintenance

### Updating Pipelines

Boilerplate can regenerate `.tekton/` files:

```bash
# From boilerplate repo
cd ~/sandbox/boilerplate
python3 boilerplate/openshift/golang-osd-operator/olm_pko_migration.py \
  --operator-name ebs-metrics-exporter \
  --github-url https://github.com/nephomaniac/ebs-metrics-exporter \
  --tekton-only
```

This regenerates pipeline files with latest boilerplate standards.

### Boilerplate Update Workflow

When OpenShift standards change:

```bash
cd ~/sandbox/ebs-metrics-exporter
make boilerplate-update
```

This syncs:
- Pipeline definitions
- Build scripts
- Linting configs
- CI configurations

## Troubleshooting

### Build Failures

**PKO package build fails:**
```
Error: COPY failed: no source files were specified
```
→ Check that `deploy_pko/` directory exists and has manifests

**Application build fails:**
```
Error: cannot find package
```
→ Run `go mod tidy` locally and commit

### Image Not Found

**Error**: `ImagePullBackOff` in DaemonSet pods

**Solution**: Check app-interface `saas` file has correct image references:
```yaml
config:
  image: quay.io/app-sre/ebs-metrics-exporter:v1.0.0  # Must match published image
```

### Pipeline Not Triggering

**Problem**: Push to GitHub doesn't trigger Konflux build

**Check**:
1. Pipeline file exists: `.tekton/ebs-metrics-exporter-pko-push.yaml`
2. CEL expression matches branch: `target_branch == "main"`
3. Konflux tenant configured: `ebs-metrics-exporter-tenant` namespace exists

## References

- [Boilerplate Centralized Pipelines](https://github.com/openshift/boilerplate/tree/master/pipelines/docker-build-oci-ta)
- [Package Operator Docs](https://package-operator.run/)
- [Konflux Documentation](https://konflux.pages.redhat.com/docs/)
- [App-SRE GitOps](https://gitlab.cee.redhat.com/service/app-interface)

## Summary

**Boilerplate provides**:
- ✅ Tekton pipeline definitions (centralized)
- ✅ PKO package Dockerfile template
- ✅ Konflux integration
- ✅ FIPS-compliant builds
- ✅ OCI trusted artifacts

**You maintain**:
- Application code (`main.go`, `pkg/`)
- PKO manifests (`deploy_pko/`)
- Dockerfile for application image

**Konflux builds**:
- Application image: `ebs-metrics-exporter:tag`
- PKO package image: `ebs-metrics-exporter-pko:tag`

**App-interface deploys**:
- PKO pulls package image
- PKO applies manifests
- DaemonSet pulls application image
- Metrics collection begins

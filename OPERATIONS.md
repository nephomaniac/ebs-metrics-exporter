# Operations Guide

This document covers operational procedures for managing the EBS Metrics Exporter in production.

## PKO Package Operator Behavior

The deployment uses Package Operator (PKO) with specific lifecycle management characteristics.

### Collision Protection

All resources have this annotation:
```yaml
package-operator.run/collision-protection: IfNoController
```

**What this means:**
- PKO creates resources on initial deployment
- PKO **will NOT overwrite** manual changes if the resource has no controller reference
- Admin edits to ConfigMaps, DaemonSets, etc. **will persist**
- PKO will NOT automatically restore deleted resources

**Implications:**
- ✅ Safe for cluster admins to edit ConfigMaps directly
- ✅ Changes won't be reverted by PKO reconciliation
- ⚠️ If you delete a resource, you must recreate the entire ClusterPackage
- ⚠️ Manual changes will be lost if you delete and recreate the ClusterPackage

### Updating the ClusterPackage

When you update the ClusterPackage itself (e.g., changing image tags):

```bash
# Update ClusterPackage with new image
oc patch clusterpackage ebs-metrics-exporter --type=merge \
  -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.24-gabc123"}}'
```

**PKO will:**
- ✅ Update the DaemonSet with new image
- ✅ Trigger rolling update of pods
- ❌ NOT update the ConfigMap (collision protection)
- ❌ NOT restart pods if only ConfigMap changed externally

## Configuration Management

### Scenario 1: Admin Edits ConfigMap

**Method:**
```bash
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics
```

**What happens:**
1. ✅ ConfigMap is updated with your changes
2. ❌ Pods do NOT automatically restart
3. ❌ PKO does NOT revert your changes
4. ⚠️ Old pods continue running with old config

**To apply changes:**
```bash
# Option 1: Delete pods (DaemonSet recreates them)
oc delete pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# Option 2: Rollout restart (Kubernetes 1.15+)
oc rollout restart daemonset/ebs-metrics-exporter -n openshift-sre-ebs-metrics

# Option 3: Scale down and up (not recommended for DaemonSet)
# Don't use this - DaemonSet will recreate pods immediately
```

**Best practice:**
```bash
# 1. Edit ConfigMap
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# 2. Verify changes
oc get configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics -o yaml

# 3. Restart pods to apply
oc delete pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# 4. Watch rollout
oc get pods -n openshift-sre-ebs-metrics -w
```

### Scenario 2: Admin Deletes ConfigMap

**Method:**
```bash
oc delete configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics
```

**What happens:**
1. ✅ ConfigMap is deleted
2. ❌ PKO does NOT recreate it (collision protection)
3. ⚠️ Running pods continue with mounted config (cached)
4. ❌ New pods will fail to start (missing volume mount)

**Recovery options:**

**Option A: Manually recreate ConfigMap**
```bash
# Recreate from template or backup
oc apply -f configmap-backup.yaml
```

**Option B: Recreate ClusterPackage** (loses all manual edits)
```bash
# Delete ClusterPackage
oc delete clusterpackage ebs-metrics-exporter

# Recreate (use deployment procedure)
oc process -f hack/pko/clusterpackage-direct.yaml ... | oc apply -f -
```

**Best practice:**
- Don't delete the ConfigMap in production
- If needed, edit it instead of deleting
- Keep backups of production ConfigMaps

### Scenario 3: Admin Updates ClusterPackage Config

If the ClusterPackage spec includes `config` parameters:

```bash
# Update ClusterPackage config
oc edit clusterpackage ebs-metrics-exporter
# Change spec.config.* values
```

**What happens:**
1. ✅ PKO processes template with new config values
2. ⚠️ Behavior depends on collision protection
3. ❌ May NOT update existing resources (collision protection)

**Current limitation:**
- Our ClusterPackage uses `config.image` for image overrides
- ConfigMap content is **static template**, not parameterized via ClusterPackage config
- Changing ClusterPackage config won't update ConfigMap

### Scenario 4: Automatic Pod Restart on ConfigMap Change

**Current behavior:**
- ❌ Pods do NOT automatically restart when ConfigMap changes

**Why:**
- DaemonSet template has no ConfigMap checksum annotation
- Kubernetes doesn't watch ConfigMap changes for pod restarts
- This is standard Kubernetes behavior

**To enable automatic restarts:**

We could add a ConfigMap checksum annotation to force pod restarts:

```yaml
# DaemonSet template metadata (not currently implemented)
annotations:
  configmap-checksum: "sha256:abc123..."  # Hash of ConfigMap contents
```

When ConfigMap changes, the checksum annotation would change, triggering rolling update.

**Recommendation:** Document manual restart procedure rather than adding complexity.

## Production Recommendations

### 1. Configuration Change Workflow

**For production clusters:**

```bash
# 1. Backup current config
oc get configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics -o yaml > config-backup-$(date +%Y%m%d).yaml

# 2. Test changes in dev/stage first
oc edit configmap ebs-metrics-exporter-config -n dev-namespace
oc delete pods -n dev-namespace -l app.kubernetes.io/name=ebs-metrics-exporter
# Verify functionality

# 3. Apply to production
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# 4. Rolling restart (one pod at a time)
# DaemonSet maxUnavailable: 1 ensures gradual rollout
oc delete pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter

# 5. Monitor rollout
oc get pods -n openshift-sre-ebs-metrics -w
oc logs -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter --tail=20
```

### 2. GitOps Workflow (Recommended)

For production, manage ConfigMaps via GitOps (app-interface):

1. **Store ConfigMap in Git** (app-interface repo)
2. **Apply via CI/CD** (not manual `oc edit`)
3. **Automated rollout** (app-interface handles pod restart)

This provides:
- ✅ Version control and audit trail
- ✅ Peer review via PRs
- ✅ Rollback capability
- ✅ Consistent across environments

### 3. Monitoring Configuration Changes

**Alert on unexpected changes:**

```yaml
# PrometheusRule example
- alert: EBSExporterConfigChanged
  expr: |
    kube_configmap_info{namespace="openshift-sre-ebs-metrics",configmap="ebs-metrics-exporter-config"}
  annotations:
    summary: "EBS Exporter ConfigMap modified"
    description: "ConfigMap changed - verify if intentional and restart pods"
```

### 4. Config Validation

Before applying config changes:

```bash
# Validate YAML syntax
oc apply --dry-run=client -f new-config.yaml

# Check for invalid config (will fail on pod startup)
# No pre-validation available - exporter validates on startup
```

**Exporter behavior on invalid config:**
- Logs error message and exits
- Pod enters CrashLoopBackOff
- DaemonSet continues retrying
- Check logs: `oc logs <pod-name>`

## Disaster Recovery

### ConfigMap Lost

**Symptoms:**
- Existing pods run normally (cached config)
- New pods fail to start with volume mount errors

**Recovery:**
```bash
# Recreate from backup
oc apply -f config-backup-YYYYMMDD.yaml

# Or recreate ClusterPackage (loses manual edits)
oc delete clusterpackage ebs-metrics-exporter
make local-deploy  # Or app-interface deployment
```

### ClusterPackage Deleted

**Symptoms:**
- All resources deleted (PKO cleanup)
- Namespace gone
- Pods terminated

**Recovery:**
```bash
# Redeploy via ClusterPackage
oc process -f hack/pko/clusterpackage-direct.yaml \
  -p OPERATOR_IMAGE=quay.io/app-sre/ebs-metrics-exporter \
  -p PKO_IMAGE=quay.io/app-sre/ebs-metrics-exporter-pko \
  -p IMAGE_TAG=v0.1.23-gabc123 \
  | oc apply -f -

# Wait for deployment
oc get clusterpackage ebs-metrics-exporter -w
```

### DaemonSet Deleted

**Symptoms:**
- All pods terminated
- DaemonSet gone

**What happens:**
- ❌ PKO does NOT automatically recreate (collision protection)

**Recovery:**
```bash
# Recreate entire ClusterPackage
oc delete clusterpackage ebs-metrics-exporter
# Redeploy
```

## Summary: PKO Collision Protection Behavior

| Action | PKO Behavior | Result |
|--------|-------------|--------|
| Edit ConfigMap | Ignores changes | ✅ Your changes persist |
| Delete ConfigMap | Does NOT recreate | ❌ Must manually restore |
| Edit DaemonSet | Ignores changes | ✅ Your changes persist |
| Delete DaemonSet | Does NOT recreate | ❌ Must recreate ClusterPackage |
| Update ClusterPackage image | Updates DaemonSet | ✅ Rolling update triggered |
| Update ClusterPackage config | May not update resources | ⚠️ Collision protection prevents |

**Key takeaway:** PKO creates resources initially, but doesn't enforce ongoing reconciliation due to `IfNoController` collision protection. Manual changes persist, but deletions aren't recovered.

## References

- **PKO Collision Protection**: https://package-operator.run/docs/concepts/collision-protection/
- **Kubernetes ConfigMap Updates**: https://kubernetes.io/docs/concepts/configuration/configmap/
- **DaemonSet Updates**: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/

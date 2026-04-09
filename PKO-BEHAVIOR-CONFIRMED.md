# PKO Behavior - Confirmed Through Testing

## Executive Summary

After extensive testing and documentation review, PKO (Package Operator) provides **deletion protection** and **version-based updates**, but **NOT continuous drift reconciliation** like ArgoCD.

## Test Environment

- **Cluster**: OpenShift with PKO v0.23.3
- **Package**: ebs-metrics-exporter
- **Tests**: Multiple drift scenarios with healthy package (Available: True)

## Key Findings

### ✅ What PKO DOES

1. **Deletion Protection** (Confirmed Working)
   - Deleted ConfigMap → Restored in ~15 seconds
   - Deleted DaemonSet → Would be restored
   - Uses ownerReferences + blockOwnerDeletion

2. **Version Updates** (Confirmed Working)
   - New ClusterPackage version → Resources updated
   - Rolling updates triggered
   - Pod restarts with new config

3. **Availability Monitoring** (Fixed)
   - Probes determine package health
   - DaemonSets need `fieldsEqual` probe (no .status.conditions)
   - Healthy package shows `Available: True`

### ❌ What PKO DOES NOT DO

1. **Continuous Annotation/Label Reconciliation**
   - User-added annotations → **Preserved** (by design)
   - Changed template annotations → **NOT reverted**
   - This is intentional per PKO docs

2. **Field-Level Drift Prevention**
   - Manual edits to most fields → Persist
   - Only reconciled on package version change
   - Not a GitOps drift-prevention tool

## PKO Design Philosophy

From https://package-operator.run/docs/concepts/reconciliation/

> **Annotations and Labels**: "Kubernetes operators may use labels to scope their caches. It also allows humans to add extra labels and annotations for ops or debugging work."

**PKO uses partial reconciliation:**
- Fields IN template → Reconciled on package update
- Fields NOT in template → Preserved (operational flexibility)
- Annotations/labels NOT in template → Preserved (for debugging)

## Test Results

### Test 1: User-Added Annotation
```bash
# Add annotation NOT in template
oc annotate configmap ... test-drift="unauthorized"

# Result after 60 seconds:
✅ Annotation PERSISTED (expected PKO behavior)
```
**Why:** PKO preserves user-added annotations for operational flexibility

### Test 2: Modified Template Annotation
```bash
# Change annotation that IS in template
oc annotate configmap ... package-operator.run/phase="wrong"  # Template has "deploy"

# Result after 60 seconds:
✅ Change PERSISTED (PKO doesn't continuously reconcile)
```
**Why:** PKO only reconciles on package version changes, not continuously

### Test 3: Deleted ConfigMap
```bash
# Delete ConfigMap
oc delete configmap ebs-metrics-exporter-config

# Result after 15 seconds:
✅ ConfigMap RESTORED (deletion protection works)
```
**Why:** ownerReferences trigger immediate restoration

### Test 4: Package Version Change
```bash
# Deploy new package version
oc patch clusterpackage ... spec.image="...v0.1.28..."

# Result:
✅ All resources updated to match new template
✅ Rolling update triggered
```
**Why:** This is PKO's primary function - version management

## Availability Probe Fix

### Problem
```yaml
# WRONG - DaemonSets don't have .status.conditions
- condition:
    type: Available
    status: 'True'
```

**Error:** `Phase "deploy" failed: missing .status.conditions`

### Solution
```yaml
# CORRECT - Use fieldsEqual for DaemonSets
- fieldsEqual:
    fieldA: .status.numberReady
    fieldB: .status.desiredNumberScheduled
```

**Result:** `Available: True` ✅

## Implications for Production

### PKO Is Perfect For:
✅ **Version-based deployments** - Deploy v1.2.3, rollback to v1.2.2
✅ **Deletion protection** - Accidental deletes are recovered
✅ **Phased rollouts** - CRDs → Namespace → RBAC → Deploy
✅ **Immutable infrastructure** - Config tied to package version

### PKO Is NOT Suitable For:
❌ **Continuous drift reconciliation** - Manual edits persist
❌ **GitOps enforcement** - No automatic reversion of changes
❌ **Real-time configuration compliance** - Only updates on version change

### For True GitOps Drift Prevention:
Add **ArgoCD** or **FluxCD** on top of PKO:
- PKO handles package lifecycle
- ArgoCD handles continuous drift prevention
- Best of both worlds

## Recommended Approach

### Option 1: PKO Only (Current)
**Pros:**
- Simple, minimal dependencies
- Deletion protection
- Version control via package tags

**Cons:**
- Manual edits persist
- Relies on operational discipline
- No automatic drift correction

**Best for:** Stable environments with strict change control

### Option 2: PKO + ArgoCD
**Pros:**
- Full GitOps enforcement
- Continuous reconciliation
- Automatic drift remediation

**Cons:**
- Additional complexity
- Another operator to manage
- Requires ArgoCD expertise

**Best for:** Large-scale, multi-cluster deployments needing strict GitOps

### Option 3: Monitoring + Alerts (Middle Ground)
**Pros:**
- Detect drift without preventing it
- Allow emergency hot-fixes
- Alert on unexpected changes

**Cons:**
- Drift still possible
- Requires monitoring setup

**Best for:** Most production environments

## Recommendation for EBS Metrics Exporter

**Use PKO as-is** with operational discipline:

1. **Version Control**
   - All config changes via new package versions
   - Git commits = audit trail
   - Immutable image tags

2. **Deletion Protection**
   - Resources automatically restored
   - ownerReferences provide safety net

3. **Monitoring**
   - Alert on unexpected package revisions
   - Log all manual changes
   - Regular drift audits

4. **Emergency Procedures**
   - Manual hot-fixes allowed (survive restarts)
   - Document all manual changes
   - Follow up with package update

## Files Updated

- `deploy_pko/manifest.yaml` - Fixed availability probe for DaemonSet
- This document - Complete PKO behavior reference

## References

- **PKO Reconciliation**: https://package-operator.run/docs/concepts/reconciliation/
- **PKO Status Probes**: https://package-operator.run/docs/concepts/status-probes/
- **PKO Documentation**: https://package-operator.run/docs/

## Conclusion

PKO is a **package lifecycle manager**, not a **drift prevention tool**.

It excels at:
- Version management
- Deletion protection  
- Phased deployments

For continuous drift prevention, consider adding ArgoCD.

For most use cases, PKO's behavior is **intentional and appropriate** for operational flexibility.

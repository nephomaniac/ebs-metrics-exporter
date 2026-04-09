# Drift Prevention & Immutable Deployments

## Overview

The EBS Metrics Exporter uses **Package Operator (PKO) controller ownership** to enforce drift prevention and ensure the package version is the source of truth.

## Key Behaviors

### ✅ What PKO Enforces

1. **ConfigMap Drift Prevention**
   - Manual edits are **automatically reverted** to package version
   - Deletions are **automatically restored**
   - Configuration tied to package/image version

2. **DaemonSet Drift Prevention**
   - Manual edits to pod spec are **automatically reverted**
   - Deletions are **automatically restored**
   - Image versions tied to package version

3. **Service/ServiceMonitor Drift Prevention**
   - Application resources managed by PKO
   - Manual changes reverted
   - Deletions restored

### ⚠️ Infrastructure Resources (Not Enforced)

These keep `collision-protection: IfNoController` to avoid conflicts:
- Namespace (may pre-exist, shared)
- ClusterRole/ClusterRoleBinding (cluster-wide)
- ServiceAccount (avoid token churn)
- SecurityContextConstraints (cluster-wide policy)
- RBAC for Prometheus (may be managed externally)

## Configuration Management

### Production Workflow (GitOps)

**ONLY way to update configuration in production:**

1. Update ConfigMap in package source (Git)
2. Build new PKO package with updated config
3. Tag with new version
4. Deploy new ClusterPackage version

**Example:**
```bash
# 1. Update deploy_pko/ConfigMap-ebs-exporter-config.yaml.gotmpl
vim deploy_pko/ConfigMap-ebs-exporter-config.yaml.gotmpl

# 2. Commit and push
git add deploy_pko/ConfigMap-ebs-exporter-config.yaml.gotmpl
git commit -m "Update config: exclude root volumes"
git push origin main

# 3. CI builds new images
# - quay.io/app-sre/ebs-metrics-exporter:v0.1.25-gabc123
# - quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.25-gabc123

# 4. Update ClusterPackage to new version
oc patch clusterpackage ebs-metrics-exporter --type=merge \
  -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.25-gabc123"}}'

# 5. PKO performs rolling update
# - Creates new ConfigMap with updated content
# - Updates DaemonSet with new image
# - Triggers rolling restart (maxUnavailable: 1)
```

### What Happens on Deploy

**Rolling Update Sequence:**

1. PKO updates ConfigMap to new version from package
2. PKO updates DaemonSet with new image tag
3. DaemonSet controller creates 1 new pod with new config
4. New pod starts:
   - Loads new ConfigMap content
   - Validates configuration
   - If valid: pod becomes ready
   - If invalid: pod crashes (CrashLoopBackOff)
5. If new pod is ready, DaemonSet deletes 1 old pod
6. Repeat for all nodes (one at a time)

**Safety guarantees:**
- Old pods keep running until new pod is ready
- `maxUnavailable: 1` ensures gradual rollout
- Invalid config prevents rollout (new pods crash)
- Can rollback by deploying previous package version

## Configuration Validation

### Startup Validation

The exporter **strictly validates** configuration on startup:

```go
// main.go
cfg, err := config.Load(*configPath)
if err != nil {
    if _, statErr := os.Stat(*configPath); statErr == nil {
        // Config exists but is invalid - FAIL
        log.Printf("ERROR: Configuration validation failed: %v", err)
        os.Exit(1)  // Pod enters CrashLoopBackOff
    }
    // Config doesn't exist - use defaults
    cfg = config.DefaultConfig()
}
```

**What gets validated:**
- YAML syntax correctness
- Discovery mode validity (auto, explicit, disabled)
- Metric filtering rules (can't have both include and exclude)
- Polling interval bounds (1-3600 seconds)
- Advanced settings (log level, maxDevices, etc.)

**On validation failure:**
- Pod logs error message with details
- Pod exits with code 1
- Kubernetes marks pod as failed
- DaemonSet keeps old pods running
- Rollout halts (new pods not ready)

**Check validation errors:**
```bash
# Get pod logs
oc logs -n openshift-sre-ebs-metrics <pod-name>

# Look for:
# ERROR: Configuration validation failed: invalid discovery mode "invalidmode"
# Pod will exit to prevent rollout of invalid configuration
```

## Drift Prevention in Action

### Scenario 1: Admin Tries to Edit ConfigMap

```bash
# Admin edits ConfigMap manually
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics
# Changes pollingIntervalSeconds: 10
# Saves and exits
```

**What happens:**
1. ConfigMap updated with manual change
2. Within ~10 seconds, PKO detects drift
3. PKO reverts ConfigMap to package version
4. Manual change is lost

**Output:**
```bash
# Watch ConfigMap being reverted
oc get configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics -w
# Shows: configmap updated → configmap updated (reverted)
```

**Lesson:** Manual edits are futile - use GitOps workflow

### Scenario 2: Admin Deletes ConfigMap

```bash
# Admin deletes ConfigMap
oc delete configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics
```

**What happens:**
1. ConfigMap deleted
2. Within ~10 seconds, PKO detects missing resource
3. PKO recreates ConfigMap from package
4. Running pods unaffected (cached config)
5. New pods work normally

**Output:**
```bash
# Watch ConfigMap being recreated
oc get configmap -n openshift-sre-ebs-metrics -w
# Shows: configmap deleted → configmap created (restored)
```

**Lesson:** Deletions are automatically recovered

### Scenario 3: Admin Edits DaemonSet

```bash
# Admin changes replica count or image
oc edit daemonset ebs-metrics-exporter -n openshift-sre-ebs-metrics
# Changes image to different version
# Saves and exits
```

**What happens:**
1. DaemonSet updated with manual change
2. Within ~10 seconds, PKO detects drift
3. PKO reverts DaemonSet to package version
4. Pods rollback to correct image

**Lesson:** DaemonSet is immutable - deploy new package for changes

### Scenario 4: Deploy Invalid Config

```bash
# Update package with invalid config
# Invalid: both include and exclude filters
data:
  config.yaml: |
    metrics:
      include: ["ebs_volume_*"]
      exclude: ["ebs_instance_*"]  # INVALID: can't have both
```

**What happens:**
1. New ClusterPackage deployed
2. PKO updates ConfigMap with invalid content
3. PKO triggers DaemonSet rolling update
4. New pod starts, loads config
5. Validation fails: "cannot specify both include and exclude"
6. Pod exits with error
7. Pod enters CrashLoopBackOff
8. DaemonSet stops rollout (old pods still running)

**Recovery:**
```bash
# Rollback to previous package version
oc patch clusterpackage ebs-metrics-exporter --type=merge \
  -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.24-gprev123"}}'

# Or fix and deploy corrected version
# Update Git → CI builds → deploy new version
```

## Monitoring & Alerts

### Recommended Alerts

**1. PKO Drift Detection**

```yaml
# Alert when PKO reconciles (drift detected)
- alert: EBSExporterDriftDetected
  expr: |
    increase(package_operator_reconcile_total{
      name="ebs-metrics-exporter"
    }[5m]) > 3
  annotations:
    summary: "PKO detected drift in ebs-metrics-exporter"
    description: "Manual changes are being reverted. Use GitOps workflow."
```

**2. Config Validation Failures**

```yaml
# Alert when pods crash due to config validation
- alert: EBSExporterConfigValidationFailed
  expr: |
    kube_pod_container_status_restarts_total{
      namespace="openshift-sre-ebs-metrics",
      pod=~"ebs-metrics-exporter.*"
    } > 5
  annotations:
    summary: "EBS Exporter pods crashing (likely config validation)"
    description: "Check pod logs for validation errors"
```

**3. ClusterPackage Not Available**

```yaml
# Alert when ClusterPackage is not available
- alert: EBSExporterPackageNotAvailable
  expr: |
    package_operator_package_available{
      name="ebs-metrics-exporter"
    } == 0
  for: 5m
  annotations:
    summary: "EBS Exporter ClusterPackage not available"
    description: "Package deployment may have failed"
```

## Version Management

### Immutable Tags

**Production images use git-based immutable tags:**

```
Format: v{MAJOR}.{MINOR}.{COMMIT_NUMBER}-g{GIT_SHA}
Example: v0.1.25-gabc1234
```

**Properties:**
- ✅ Unique per commit
- ✅ Immutable (can't be overwritten)
- ✅ Traceable to source code
- ✅ Safe for production

**See:** [DEPLOYMENT.md](DEPLOYMENT.md) for complete tagging strategy

### Rollback Procedure

**To rollback to previous version:**

```bash
# 1. Find previous version
oc get clusterpackage ebs-metrics-exporter -o yaml | grep 'image:'
# Current: quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.25-gabc123

# 2. Patch to previous version
oc patch clusterpackage ebs-metrics-exporter --type=merge \
  -p '{"spec":{"image":"quay.io/app-sre/ebs-metrics-exporter-pko:v0.1.24-gprev123"}}'

# 3. Monitor rollback
oc get pods -n openshift-sre-ebs-metrics -w
# Pods will rolling restart to previous version
```

## Testing Drift Prevention

Run the drift prevention test:

```bash
./test/pko-resilience-test.sh
```

**Tests:**
- ConfigMap edit (reverted by PKO)
- ConfigMap delete (restored by PKO)
- DaemonSet edit (reverted by PKO)
- Invalid config (pods fail to start)

## Summary: GitOps-Enforced Immutability

| Action | Result | How to Properly Update |
|--------|--------|------------------------|
| Edit ConfigMap manually | ❌ Reverted by PKO | Update in Git → new package |
| Delete ConfigMap | ✅ Restored by PKO | Use GitOps |
| Edit DaemonSet manually | ❌ Reverted by PKO | Update in Git → new package |
| Delete DaemonSet | ✅ Restored by PKO | Use GitOps |
| Deploy invalid config | ❌ Pods crash, rollout halts | Fix config → new package |
| Deploy new version | ✅ Rolling update (1 pod at time) | GitOps workflow |

**Key principle:** Package version is the **single source of truth**. All updates go through GitOps.

# Optional: Automatic Pod Restart on ConfigMap Changes

## Current Behavior

**Problem:**
- Cluster admin edits ConfigMap via `oc edit`
- Pods continue running with old configuration
- Manual pod restart required

**Current workflow:**
```bash
# 1. Edit ConfigMap
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# 2. Manually restart pods
oc delete pods -n openshift-sre-ebs-metrics -l app.kubernetes.io/name=ebs-metrics-exporter
```

## Proposed Enhancement

Add ConfigMap checksum annotation to DaemonSet template to trigger automatic rolling restart when ConfigMap changes.

### Implementation

**Option 1: Use Reloader (External Controller)**

Deploy [Reloader](https://github.com/stakater/Reloader) to watch ConfigMaps and restart pods:

```yaml
# Add annotation to DaemonSet
metadata:
  annotations:
    reloader.stakater.com/auto: "true"
```

**Pros:**
- ✅ No code changes needed
- ✅ Works across all deployments
- ✅ Supports ConfigMaps and Secrets

**Cons:**
- ❌ Requires external dependency
- ❌ Additional cluster-wide controller
- ❌ May not be available in all clusters

**Option 2: PKO Template with Checksum**

Modify DaemonSet template to include ConfigMap checksum:

```yaml
# deploy_pko/DaemonSet-ebs-metrics-exporter.yaml.gotmpl
spec:
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8090"
        prometheus.io/path: "/metrics"
        # Add checksum of ConfigMap content
        checksum/config: '{{ include "deploy_pko/ConfigMap-ebs-exporter-config.yaml.gotmpl" . | sha256sum }}'
```

**How it works:**
1. PKO processes template and calculates ConfigMap SHA256
2. Annotation added to pod template with hash
3. When ConfigMap changes, hash changes
4. DaemonSet sees pod template changed
5. Triggers rolling update automatically

**Pros:**
- ✅ No external dependencies
- ✅ Native Kubernetes behavior
- ✅ Works with PKO

**Cons:**
- ❌ Only works when updating via ClusterPackage
- ❌ Doesn't detect manual ConfigMap edits
- ❌ Requires PKO templating changes

**Option 3: Wave Annotation (Kubernetes Native)**

Use [Kubernetes Wave](https://github.com/wave-k8s/wave):

```yaml
# Add annotation to DaemonSet
metadata:
  annotations:
    wave.pusher.com/update-on-config-change: "true"
```

**Pros:**
- ✅ Kubernetes native approach
- ✅ Lightweight

**Cons:**
- ❌ Requires Wave controller
- ❌ Additional dependency

## Recommendation

**For production (app-interface managed):**
- Use GitOps workflow - config changes go through app-interface MR
- app-interface CI/CD handles pod restart automatically
- No checksum annotation needed

**For development/testing:**
- Manual restart is acceptable
- Document in OPERATIONS.md
- Add to `make` targets for convenience

**If automatic restart is critical:**
- Use Reloader (if available in cluster)
- Or add to app-interface deployment automation

## Example: Adding Convenience Make Target

Add to Makefile:

```makefile
.PHONY: local-restart
local-restart: ## Restart DaemonSet pods to apply ConfigMap changes
	@echo "Restarting DaemonSet pods..."
	@oc delete pods -n $(OPERATOR_NAMESPACE) -l app.kubernetes.io/name=$(OPERATOR_NAME)
	@echo "Waiting for pods to restart..."
	@sleep 5
	@make local-status
```

**Usage:**
```bash
# Edit ConfigMap
oc edit configmap ebs-metrics-exporter-config -n openshift-sre-ebs-metrics

# Restart pods with one command
make local-restart
```

## Decision

**Current approach:** Document manual restart procedure in OPERATIONS.md

**Reasoning:**
- Production uses GitOps (app-interface handles restarts)
- Development infrequent config changes
- Manual restart is simple and explicit
- Avoids additional dependencies
- No surprising automatic restarts

**Future consideration:** If automatic restart becomes critical, evaluate Reloader or app-interface automation.

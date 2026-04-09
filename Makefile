export KONFLUX_BUILDS=true
FIPS_ENABLED=true

include boilerplate/generated-includes.mk

# Expected test cluster identifier - set via environment variable or modify here
# Extract from: oc whoami --show-server
EXPECTED_CLUSTER ?= maclarkrosa0408

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

##@ Cluster Safety

.PHONY: check-cluster
check-cluster: ## Verify connected to expected test cluster (safety check)
	@echo "Checking cluster identity..."
	@CURRENT_CLUSTER=$$(oc whoami --show-server 2>/dev/null | grep -o '[^.]*\.thib\.s1\.devshift\.org' | cut -d. -f1 || echo "unknown"); \
	if [ "$$CURRENT_CLUSTER" != "$(EXPECTED_CLUSTER)" ]; then \
		echo "❌ ERROR: Wrong cluster!"; \
		echo "   Expected: $(EXPECTED_CLUSTER)"; \
		echo "   Current:  $$CURRENT_CLUSTER"; \
		echo ""; \
		echo "To override: EXPECTED_CLUSTER=$$CURRENT_CLUSTER make <target>"; \
		exit 1; \
	fi; \
	echo "✅ Cluster verified: $(EXPECTED_CLUSTER)"

##@ Local Development (mimics Konflux pipelines)

.PHONY: local-ci
local-ci: ## Build and test everything (mimics CI checks)
	@echo ""
	@echo "============================================"
	@echo "⏺ Local CI Workflow (mimics Konflux CI)"
	@echo "============================================"
	@echo "This will:"
	@echo "  1. Build application image (FIPS/BoringCrypto, Go 1.23+)"
	@echo "  2. Build PKO package image (YAML manifests)"
	@echo "  3. Run unit tests (go-test)"
	@echo "  4. Validate PKO templates (validate-pko-fixtures)"
	@echo ""
	@echo "If this passes, Konflux CI should pass too!"
	@echo "============================================"
	@echo ""
	@$(MAKE) local-build-all
	@$(MAKE) local-test-all
	@echo ""
	@echo "============================================"
	@echo "✅ Local CI complete!"
	@echo "============================================"
	@echo "Built images:"
	@echo "  - $(OPERATOR_IMAGE_URI_LATEST)"
	@echo "  - $(OPERATOR_IMAGE_URI_LATEST)-pko"
	@echo ""
	@echo "Next steps:"
	@echo "  make local-push-all    # Push images to registry"
	@echo "  make local-deploy      # Deploy via PKO operator"
	@echo "============================================"

.PHONY: local
local: ## Full workflow: build, test, push, deploy
	@echo ""
	@echo "============================================"
	@echo "⏺ Full Local Workflow (mimics production)"
	@echo "============================================"
	@echo "This will:"
	@echo "  Phase 1: Build and Test (local-ci)"
	@echo "    - Build both images with FIPS crypto"
	@echo "    - Run all tests"
	@echo "  Phase 2: Push Images (local-push-all)"
	@echo "    - Push to $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/"
	@echo "  Phase 3: Deploy (local-deploy)"
	@echo "    - Create ClusterPackage resource"
	@echo "    - PKO operator deploys in phases"
	@echo "  Phase 4: Verify (local-status)"
	@echo "    - Show deployment status"
	@echo ""
	@echo "Cluster: $$(oc whoami --show-console 2>/dev/null || echo 'Not logged in')"
	@echo "============================================"
	@echo ""
	@$(MAKE) local-ci
	@echo ""
	@echo "Phase 2: Pushing images..."
	@$(MAKE) local-push-all
	@echo ""
	@echo "Phase 3: Deploying via PKO operator..."
	@$(MAKE) local-deploy
	@echo ""
	@echo "Phase 4: Verifying deployment..."
	@$(MAKE) local-status
	@echo ""
	@echo "============================================"
	@echo "✅ Full local deployment complete!"
	@echo "============================================"
	@echo "View logs: make local-logs"
	@echo "Check status: make local-status"
	@echo "Cleanup: make local-undeploy"
	@echo "============================================"

.PHONY: local-build-all
local-build-all: ## Build both images (mimics Konflux)
	@echo "Building application image (mimics .tekton/ebs-metrics-exporter-push.yaml)..."
	@$(MAKE) docker-build
	@echo ""
	@$(MAKE) local-build-pko
	@echo ""
	@echo "✅ Both images built successfully"

.PHONY: local-build-pko
local-build-pko: ## Build PKO package image (mimics Konflux PKO pipeline)
	@echo "Building PKO package image (mimics .tekton/*-pko-push.yaml)..."
	${CONTAINER_ENGINE} build -f build/Dockerfile.pko \
		-t $(OPERATOR_IMAGE_URI_LATEST)-pko \
		deploy_pko/
	@echo "✅ Built: $(OPERATOR_IMAGE_URI_LATEST)-pko"

.PHONY: local-push-all
local-push-all: ## Push both images
	@echo "Pushing application image..."
	@$(MAKE) docker-push
	@echo ""
	@$(MAKE) local-push-pko
	@echo ""
	@echo "✅ Both images pushed to $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/"

.PHONY: local-push-pko
local-push-pko: ## Push PKO package image
	@echo "Pushing PKO package image..."
	${CONTAINER_ENGINE} push $(OPERATOR_IMAGE_URI_LATEST)-pko
	@echo "✅ Pushed: $(OPERATOR_IMAGE_URI_LATEST)-pko"

.PHONY: local-deploy
local-deploy: check-cluster ## Deploy via PKO operator (processes ClusterPackage template)
	@echo "============================================"
	@echo "Deploying via PKO operator..."
	@echo "============================================"
	@echo "Processing ClusterPackage template..."
	@echo "  Template: hack/pko/clusterpackage-direct.yaml"
	@echo "  Application image: $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME):$(OPERATOR_IMAGE_TAG)"
	@echo "  PKO package image: $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-pko:$(OPERATOR_IMAGE_TAG)"
	@echo ""
	@if [ ! -f hack/pko/clusterpackage-direct.yaml ]; then \
		echo "ERROR: hack/pko/clusterpackage-direct.yaml not found"; \
		exit 1; \
	fi
	@oc process -f hack/pko/clusterpackage-direct.yaml \
		--local=true \
		-p OPERATOR_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME) \
		-p PKO_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-pko \
		-p IMAGE_TAG=$(OPERATOR_IMAGE_TAG) \
		| oc apply -f -
	@echo ""
	@echo "✅ ClusterPackage created: $(OPERATOR_NAME)"
	@echo ""
	@echo "PKO operator will now deploy in phases:"
	@echo "  Phase 1: namespace - Create namespace"
	@echo "  Phase 2: rbac - ServiceAccount, SCC, Roles"
	@echo "  Phase 3: deploy - DaemonSet, Service, ServiceMonitor"
	@echo ""
	@echo "Watch deployment: oc get clusterpackage $(OPERATOR_NAME) -w"

.PHONY: local-deploy-custom
local-deploy-custom: ## Deploy with custom image tags (set CUSTOM_TAG variable)
	@echo "============================================"
	@echo "Deploying with custom image tags..."
	@echo "============================================"
	@if [ -z "$(CUSTOM_TAG)" ]; then \
		echo "ERROR: CUSTOM_TAG not set"; \
		echo "Usage: make local-deploy-custom CUSTOM_TAG=v1.0.0"; \
		exit 1; \
	fi
	@echo "Application image: $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME):$(CUSTOM_TAG)"
	@echo "PKO package image: $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-pko:$(CUSTOM_TAG)"
	@echo ""
	@oc process -f hack/pko/clusterpackage-direct.yaml \
		--local=true \
		-p OPERATOR_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME) \
		-p PKO_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-pko \
		-p IMAGE_TAG=$(CUSTOM_TAG) \
		| oc apply -f -
	@echo ""
	@echo "✅ Deployed with custom tag: $(CUSTOM_TAG)"

.PHONY: local-undeploy
local-undeploy: check-cluster ## Remove ClusterPackage (PKO operator cleans up)
	@echo "============================================"
	@echo "Removing deployment..."
	@echo "============================================"
	@echo "Deleting ClusterPackage: $(OPERATOR_NAME)"
	@echo ""
	@echo "PKO operator will clean up in reverse order:"
	@echo "  Phase 3: deploy - DaemonSet, Service, ServiceMonitor"
	@echo "  Phase 2: rbac - ServiceAccount, SCC, Roles"
	@echo "  Phase 1: namespace - Remove namespace"
	@echo ""
	@oc delete clusterpackage $(OPERATOR_NAME) --ignore-not-found=true
	@echo ""
	@echo "✅ ClusterPackage deleted"
	@echo "Note: PKO operator may take a few seconds to clean up resources"

.PHONY: local-status
local-status: ## Show deployment status
	@echo "============================================"
	@echo "Deployment Status"
	@echo "============================================"
	@echo ""
	@echo "=== ClusterPackage (PKO resource) ==="
	@oc get clusterpackage $(OPERATOR_NAME) -o wide 2>/dev/null || echo "Not found"
	@echo ""
	@echo "=== Namespace ==="
	@oc get namespace $(OPERATOR_NAMESPACE) --ignore-not-found || echo "Not found"
	@echo ""
	@echo "=== DaemonSet ==="
	@oc get daemonset -n $(OPERATOR_NAMESPACE) --ignore-not-found || echo "Not found"
	@echo ""
	@echo "=== Pods ==="
	@oc get pods -n $(OPERATOR_NAMESPACE) --ignore-not-found || echo "No pods found"
	@echo ""
	@echo "View logs: make local-logs"

.PHONY: local-logs
local-logs: ## View logs from pods
	@echo "============================================"
	@echo "Pod Logs (last 50 lines)"
	@echo "============================================"
	@echo "Namespace: $(OPERATOR_NAMESPACE)"
	@echo "Label: app.kubernetes.io/name=$(OPERATOR_NAME)"
	@echo ""
	@oc logs -n $(OPERATOR_NAMESPACE) -l app.kubernetes.io/name=$(OPERATOR_NAME) --tail=50 --all-containers=true

.PHONY: local-test-all
local-test-all: ## Run all tests (mimics CI checks)
	@echo "Running unit tests..."
	@$(MAKE) go-test
	@echo ""
	@echo "Validating PKO templates..."
	@$(MAKE) validate-pko-fixtures
	@echo ""
	@echo "✅ All tests passed"

##@ Integration Testing

.PHONY: integration-test
integration-test: ## Run integration tests against live cluster
	@echo "============================================"
	@echo "Running integration tests (requires cluster)"
	@echo "============================================"
	@if ! oc whoami &>/dev/null; then \
		echo "ERROR: Not logged into OpenShift cluster"; \
		echo "Run: oc login <cluster-url>"; \
		exit 1; \
	fi
	@echo "Cluster: $$(oc whoami --show-console 2>/dev/null)"
	@echo "User: $$(oc whoami 2>/dev/null)"
	@echo ""
	INTEGRATION_TEST=true go test -v -tags integration ./test/integration/ -run .
	@echo ""
	@echo "✅ Integration tests complete"

.PHONY: metrics-table
metrics-table: ## Display metrics table from running pods
	@echo "============================================"
	@echo "Metrics Table"
	@echo "============================================"
	@if ! oc whoami &>/dev/null; then \
		echo "ERROR: Not logged into OpenShift cluster"; \
		echo "Run: oc login <cluster-url>"; \
		exit 1; \
	fi
	@echo "Cluster: $$(oc whoami --show-console 2>/dev/null)"
	@echo ""
	INTEGRATION_TEST=true go test -v -tags integration ./test/integration/ -run TestMetricsTable
	@echo ""
	@echo "✅ Metrics table complete"

.PHONY: verify-prometheus
verify-prometheus: ## Verify metrics are in cluster Prometheus
	@echo "============================================"
	@echo "Verifying Prometheus Ingestion"
	@echo "============================================"
	@if ! oc whoami &>/dev/null; then \
		echo "ERROR: Not logged into OpenShift cluster"; \
		echo "Run: oc login <cluster-url>"; \
		exit 1; \
	fi
	@echo "Cluster: $$(oc whoami --show-console 2>/dev/null)"
	@echo ""
	INTEGRATION_TEST=true go test -v -tags integration ./test/integration/ -run 'TestPrometheus'
	@echo ""
	@echo "✅ Prometheus verification complete"

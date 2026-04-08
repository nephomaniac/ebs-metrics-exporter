export KONFLUX_BUILDS=true
FIPS_ENABLED=true

include boilerplate/generated-includes.mk

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

##@ Local Development (mimics Konflux pipelines)

.PHONY: local-ci
local-ci: local-build-all local-test-all ## Build and test everything (mimics CI checks)
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
local: local-ci local-push-all local-deploy local-status ## Full workflow: build, test, push, deploy
	@echo ""
	@echo "============================================"
	@echo "✅ Full local deployment complete!"
	@echo "============================================"
	@echo "View logs: make local-logs"
	@echo "Check status: make local-status"
	@echo "Cleanup: make local-undeploy"
	@echo "============================================"

.PHONY: local-build-all
local-build-all: docker-build local-build-pko ## Build both images (mimics Konflux)

.PHONY: local-build-pko
local-build-pko: ## Build PKO package image (mimics Konflux PKO pipeline)
	@echo "Building PKO package image (mimics .tekton/*-pko-push.yaml)..."
	${CONTAINER_ENGINE} build -f build/Dockerfile.pko \
		-t $(OPERATOR_IMAGE_URI_LATEST)-pko \
		deploy_pko/
	@echo "✅ Built: $(OPERATOR_IMAGE_URI_LATEST)-pko"

.PHONY: local-push-all
local-push-all: docker-push local-push-pko ## Push both images

.PHONY: local-push-pko
local-push-pko: ## Push PKO package image
	@echo "Pushing PKO package image..."
	${CONTAINER_ENGINE} push $(OPERATOR_IMAGE_URI_LATEST)-pko
	@echo "✅ Pushed: $(OPERATOR_IMAGE_URI_LATEST)-pko"

.PHONY: local-deploy
local-deploy: ## Deploy via PKO operator (processes ClusterPackage template)
	@echo "Deploying via PKO operator..."
	@if [ ! -f hack/pko/clusterpackage-direct.yaml ]; then \
		echo "ERROR: hack/pko/clusterpackage-direct.yaml not found"; \
		exit 1; \
	fi
	@oc process -f hack/pko/clusterpackage-direct.yaml \
		-p OPERATOR_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME) \
		-p PKO_IMAGE=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-pko \
		-p IMAGE_TAG=$(OPERATOR_IMAGE_TAG) \
		| oc apply -f -
	@echo "✅ ClusterPackage created: $(OPERATOR_NAME)"
	@echo "Watch deployment: oc get clusterpackage $(OPERATOR_NAME) -w"

.PHONY: local-undeploy
local-undeploy: ## Remove ClusterPackage (PKO operator cleans up)
	@echo "Removing ClusterPackage..."
	@oc delete clusterpackage $(OPERATOR_NAME) --ignore-not-found=true
	@echo "✅ Removed: $(OPERATOR_NAME)"

.PHONY: local-status
local-status: ## Show deployment status
	@echo "=== ClusterPackage ==="
	@oc get clusterpackage $(OPERATOR_NAME) -o wide 2>/dev/null || echo "Not found"
	@echo ""
	@echo "=== Namespace ==="
	@oc get namespace $(OPERATOR_NAMESPACE) --ignore-not-found || echo "Not found"
	@echo ""
	@echo "=== DaemonSet ==="
	@oc get daemonset -n $(OPERATOR_NAMESPACE) --ignore-not-found
	@echo ""
	@echo "=== Pods ==="
	@oc get pods -n $(OPERATOR_NAMESPACE) --ignore-not-found

.PHONY: local-logs
local-logs: ## View logs from pods
	@oc logs -n $(OPERATOR_NAMESPACE) -l app.kubernetes.io/name=$(OPERATOR_NAME) --tail=50 --all-containers=true

.PHONY: local-test-all
local-test-all: go-test validate-pko-fixtures ## Run all tests (mimics CI checks)

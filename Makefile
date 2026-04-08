# Boilerplate configuration
export KONFLUX_BUILDS ?= true
FIPS_ENABLED ?= true

# Include boilerplate
include boilerplate/generated-includes.mk

# Project-specific variables
APP_NAME ?= ebs-metrics-exporter
NAMESPACE ?= openshift-sre-ebs-metrics

# Version information
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

# Image URLs
IMAGE_REGISTRY ?= quay.io
IMAGE_REPOSITORY ?= $(IMAGE_REGISTRY)/app-sre
IMG ?= $(IMAGE_REPOSITORY)/$(APP_NAME):latest

# Development image URLs (use your personal Quay.io repo)
QUAY_USER ?= $(shell echo $$QUAY_USER)
DEV_IMG ?= quay.io/$(QUAY_USER)/$(APP_NAME):test
DEV_IMG_PKO ?= quay.io/$(QUAY_USER)/$(APP_NAME)-pko:test
PKO_OUTPUT_DIR ?= _output/pko

# Boilerplate update
.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

# Build targets
.PHONY: build
build: ## Build the exporter binary
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/ebs-metrics-exporter .

.PHONY: build-linux
build-linux: ## Build the exporter binary for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/ebs-metrics-exporter .

# Container build targets
.PHONY: docker-build
docker-build: ## Build container image
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ${IMG} -f Dockerfile .

.PHONY: docker-push
docker-push: ## Push container image
	docker push ${IMG}

# Podman targets (for macOS)
.PHONY: podman-build
podman-build: ## Build container image using podman
	podman build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ${IMG} -f Dockerfile .

.PHONY: podman-push
podman-push: ## Push container image using podman
	podman push ${IMG}

# Deploy targets (legacy YAML deployment)
.PHONY: deploy
deploy: ## Deploy DaemonSet to the cluster (legacy)
	kubectl apply -f deploy/

.PHONY: undeploy
undeploy: ## Undeploy DaemonSet from the cluster (legacy)
	kubectl delete -f deploy/ --ignore-not-found=true

# PKO targets
.PHONY: pko-validate
pko-validate: ## Validate PKO package manifests
	@echo "Validating PKO manifests..."
	@ls -la deploy_pko/
	@echo "Checking manifest.yaml..."
	@test -f deploy_pko/manifest.yaml || (echo "ERROR: deploy_pko/manifest.yaml not found" && exit 1)

# Development targets
.PHONY: run
run: build ## Run the exporter locally (requires sudo for device access)
	@echo "Note: This requires sudo access to read NVMe devices"
	@echo "Usage: sudo ./bin/ebs-metrics-exporter --device /dev/nvme1n1 --port 8090"
	sudo ./bin/ebs-metrics-exporter --device /dev/nvme1n1 --port 8090

.PHONY: test
test: ## Run tests
	go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: fmt
fmt: ## Run go fmt
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

# Clean targets
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf $(PKO_OUTPUT_DIR)
	rm -f coverage.out

##@ Development Workflow

.PHONY: dev-build
dev-build: ## Build both application and PKO package images for development
	@echo "Building application image: $(DEV_IMG)"
	@if [ -z "$(QUAY_USER)" ]; then \
		echo "ERROR: QUAY_USER not set. Export QUAY_USER=your-quay-username"; \
		exit 1; \
	fi
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(DEV_IMG) -f Dockerfile .
	@echo "Building PKO package image: $(DEV_IMG_PKO)"
	docker build -f build/Dockerfile.pko -t $(DEV_IMG_PKO) deploy_pko/
	@echo "✅ Built: $(DEV_IMG)"
	@echo "✅ Built: $(DEV_IMG_PKO)"

.PHONY: dev-push
dev-push: ## Push both images to your Quay.io repository
	@echo "Pushing $(DEV_IMG)"
	docker push $(DEV_IMG)
	@echo "Pushing $(DEV_IMG_PKO)"
	docker push $(DEV_IMG_PKO)
	@echo "✅ Pushed both images to quay.io/$(QUAY_USER)/"

.PHONY: dev-build-push
dev-build-push: dev-build dev-push ## Build and push both images

.PHONY: dev-render-pko
dev-render-pko: ## Render PKO manifests with development image
	@echo "Rendering PKO manifests to $(PKO_OUTPUT_DIR)/"
	@rm -rf $(PKO_OUTPUT_DIR)
	@mkdir -p $(PKO_OUTPUT_DIR)
	@for file in deploy_pko/*.yaml deploy_pko/*.yaml.gotmpl; do \
		if [ -f "$$file" ]; then \
			filename=$$(basename $$file .gotmpl); \
			sed -e "s|{{ .config.image }}|$(DEV_IMG)|g" \
			    -e "s|{{ .config.device }}|/dev/nvme1n1|g" \
			    "$$file" > "$(PKO_OUTPUT_DIR)/$$filename"; \
			echo "  ✓ $$filename"; \
		fi; \
	done
	@echo "✅ Rendered manifests in $(PKO_OUTPUT_DIR)/"

.PHONY: dev-deploy
dev-deploy: dev-render-pko ## Deploy to cluster using PKO manifests
	@echo "Deploying to OpenShift cluster..."
	@echo "Using image: $(DEV_IMG)"
	oc apply -f $(PKO_OUTPUT_DIR)/
	@echo "✅ Deployed to namespace: $(NAMESPACE)"
	@echo ""
	@echo "Check status with: make dev-status"

.PHONY: dev-undeploy
dev-undeploy: ## Remove deployment from cluster
	@echo "Removing deployment from cluster..."
	oc delete -f $(PKO_OUTPUT_DIR)/ --ignore-not-found=true
	@echo "✅ Removed from namespace: $(NAMESPACE)"

.PHONY: dev-status
dev-status: ## Show deployment status
	@echo "=== Namespace ==="
	@oc get namespace $(NAMESPACE) 2>/dev/null || echo "Namespace not found"
	@echo ""
	@echo "=== DaemonSet ==="
	@oc get daemonset -n $(NAMESPACE) 2>/dev/null || echo "DaemonSet not found"
	@echo ""
	@echo "=== Pods ==="
	@oc get pods -n $(NAMESPACE) -l app.kubernetes.io/name=$(APP_NAME) 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "=== Service ==="
	@oc get service -n $(NAMESPACE) 2>/dev/null || echo "Service not found"
	@echo ""
	@echo "=== ServiceMonitor ==="
	@oc get servicemonitor -n $(NAMESPACE) 2>/dev/null || echo "ServiceMonitor not found"

.PHONY: dev-logs
dev-logs: ## View logs from all pods
	@echo "Fetching logs from all pods in $(NAMESPACE)..."
	@oc logs -n $(NAMESPACE) -l app.kubernetes.io/name=$(APP_NAME) --tail=50 --all-containers=true

.PHONY: dev-metrics
dev-metrics: ## Test metrics endpoint from a pod
	@POD=$$(oc get pods -n $(NAMESPACE) -l app.kubernetes.io/name=$(APP_NAME) -o jsonpath='{.items[0].metadata.name}' 2>/dev/null); \
	if [ -z "$$POD" ]; then \
		echo "ERROR: No pods found in namespace $(NAMESPACE)"; \
		exit 1; \
	fi; \
	echo "Testing metrics from pod: $$POD"; \
	echo ""; \
	oc exec -n $(NAMESPACE) $$POD -- curl -s localhost:8090/metrics | grep ^ebs_ || echo "No EBS metrics found"

.PHONY: dev-restart
dev-restart: ## Restart DaemonSet to pull latest image
	@echo "Restarting DaemonSet..."
	oc rollout restart daemonset/$(APP_NAME) -n $(NAMESPACE)
	@echo "Waiting for rollout to complete..."
	oc rollout status daemonset/$(APP_NAME) -n $(NAMESPACE) --timeout=2m
	@echo "✅ DaemonSet restarted"

.PHONY: dev-rebuild
dev-rebuild: dev-build dev-push dev-restart ## Full iteration: build, push, and restart

.PHONY: deploy-legacy
deploy-legacy: ## Deploy using legacy YAML (non-PKO)
	@echo "Deploying using legacy YAML manifests..."
	@if [ -z "$(IMG)" ]; then \
		echo "ERROR: IMG not set. Usage: make deploy-legacy IMG=quay.io/user/image:tag"; \
		exit 1; \
	fi
	@echo "Using image: $(IMG)"
	@mkdir -p _output/legacy
	@for file in deploy/*.yaml; do \
		if [ -f "$$file" ]; then \
			filename=$$(basename $$file); \
			sed "s|REPLACE_IMAGE|$(IMG)|g" "$$file" > "_output/legacy/$$filename"; \
		fi; \
	done
	oc apply -f _output/legacy/
	@echo "✅ Deployed to namespace: $(NAMESPACE)"

# Help target
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

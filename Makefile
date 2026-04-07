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
	rm -f coverage.out

# Help target
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

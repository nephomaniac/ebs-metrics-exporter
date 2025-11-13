# Boilerplate configuration
export KONFLUX_BUILDS ?= true
FIPS_ENABLED ?= true

# Include boilerplate
include boilerplate/generated-includes.mk

# Project-specific variables
APP_NAME ?= ebs-metrics-exporter
NAMESPACE ?= openshift-sre-ebs-metrics

# Image URLs
IMAGE_REGISTRY ?= quay.io
IMAGE_REPOSITORY ?= $(IMAGE_REGISTRY)/app-sre
IMG ?= $(IMAGE_REPOSITORY)/$(APP_NAME):latest

# Boilerplate update
.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

# Container build targets
.PHONY: docker-build
docker-build: ## Build container image
	docker build -t ${IMG} -f Dockerfile .

.PHONY: docker-push
docker-push: ## Push container image
	docker push ${IMG}

# Deploy targets
.PHONY: deploy
deploy: ## Deploy DaemonSet to the cluster
	kubectl apply -f deploy/

.PHONY: undeploy
undeploy: ## Undeploy DaemonSet from the cluster
	kubectl delete -f deploy/ --ignore-not-found=true

# Development targets
.PHONY: build
build: ## Build the exporter binary
	CGO_ENABLED=0 go build -o bin/ebs-metrics-collector main.go

.PHONY: run
run: ## Run the exporter locally (requires sudo for device access)
	@echo "Note: This requires sudo access to read NVMe devices"
	sudo ./bin/ebs-metrics-collector --device /dev/nvme1n1 --port 8090

# Help target
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

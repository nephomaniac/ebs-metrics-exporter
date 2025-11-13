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
docker-build: docker-build-collector ## Build collector container image (default)

.PHONY: docker-build-collector
docker-build-collector: ## Build collector container image
	docker build -t ${IMG} -f Dockerfile .

.PHONY: docker-build-operator
docker-build-operator: ## Build operator container image
	docker build -t ${IMG}-operator -f Dockerfile.operator .

.PHONY: docker-build-all
docker-build-all: docker-build-collector docker-build-operator ## Build both container images

.PHONY: docker-push
docker-push: docker-push-collector ## Push collector container image (default)

.PHONY: docker-push-collector
docker-push-collector: ## Push collector container image
	docker push ${IMG}

.PHONY: docker-push-operator
docker-push-operator: ## Push operator container image
	docker push ${IMG}-operator

.PHONY: docker-push-all
docker-push-all: docker-push-collector docker-push-operator ## Push both container images

# Deploy targets
.PHONY: deploy
deploy: ## Deploy DaemonSet to the cluster
	kubectl apply -f deploy/

.PHONY: undeploy
undeploy: ## Undeploy DaemonSet from the cluster
	kubectl delete -f deploy/ --ignore-not-found=true

# Development targets
.PHONY: build
build: build-collector build-operator ## Build both collector and operator binaries

.PHONY: build-collector
build-collector: ## Build the collector binary
	CGO_ENABLED=0 go build -o bin/ebs-metrics-collector ./cmd/collector

.PHONY: build-operator
build-operator: ## Build the operator binary
	CGO_ENABLED=0 go build -o bin/ebs-metrics-exporter-operator main.go

.PHONY: run
run: build-collector ## Run the collector locally (requires sudo for device access)
	@echo "Note: This requires sudo access to read NVMe devices"
	sudo ./bin/ebs-metrics-collector --device /dev/nvme1n1 --port 8090

# Help target
.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

# OLM Bundle variables
BUNDLE_IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(APP_NAME)-bundle:v$(OPERATOR_VERSION)
CATALOG_IMG ?= $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(APP_NAME)-catalog:v$(OPERATOR_VERSION)

# OLM Bundle targets
.PHONY: bundle
bundle: ## Generate bundle manifests and metadata
	@echo "Bundle manifests are located in bundle/ directory"
	@echo "Update bundle/manifests/ebs-metrics-exporter.clusterserviceversion.yaml with your images"

.PHONY: bundle-build
bundle-build: ## Build the bundle image
	docker build -f bundle/bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image
	docker push $(BUNDLE_IMG)

.PHONY: bundle-validate
bundle-validate: ## Validate the bundle using operator-sdk
	operator-sdk bundle validate ./bundle --select-optional suite=operatorframework

.PHONY: catalog-build
catalog-build: ## Build a catalog image containing this bundle
	opm index add --bundles $(BUNDLE_IMG) --tag $(CATALOG_IMG) --container-tool docker

.PHONY: catalog-push
catalog-push: ## Push the catalog image
	docker push $(CATALOG_IMG)

.PHONY: olm-deploy
olm-deploy: ## Deploy operator via OLM (requires OLM to be installed)
	@echo "Creating CatalogSource..."
	@cat <<EOF | oc apply -f - \n\
	apiVersion: operators.coreos.com/v1alpha1\n\
	kind: CatalogSource\n\
	metadata:\n\
	  name: ebs-metrics-exporter-catalog\n\
	  namespace: openshift-marketplace\n\
	spec:\n\
	  sourceType: grpc\n\
	  image: $(CATALOG_IMG)\n\
	  displayName: EBS Metrics Exporter\n\
	  publisher: Red Hat\n\
	EOF
	@echo "CatalogSource created. Install operator from OperatorHub."

.PHONY: olm-undeploy
olm-undeploy: ## Remove OLM catalog source
	oc delete catalogsource ebs-metrics-exporter-catalog -n openshift-marketplace --ignore-not-found=true

.DEFAULT_GOAL := help

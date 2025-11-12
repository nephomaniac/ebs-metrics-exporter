# Project-specific Makefile configuration
# This file is sourced by boilerplate/generated-includes.mk

# Operator configuration
OPERATOR_NAME := ebs-metrics-exporter
OPERATOR_NAMESPACE := openshift-sre-ebs-metrics

# Image configuration
IMAGE_REGISTRY ?= quay.io
IMAGE_REPOSITORY ?= app-sre

# Container image names
OPERATOR_IMAGE_URI := $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)
EXPORTER_IMAGE_URI := $(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(OPERATOR_NAME)-daemonset

# OLM configuration
OPERATOR_VERSION ?= 0.1.0
CHANNELS := alpha
DEFAULT_CHANNEL := alpha

# FIPS configuration
FIPS_ENABLED := true

# Additional images to build (for boilerplate multi-image support)
ADDITIONAL_IMAGE_SPECS := ebs-metrics-exporter-daemonset=Dockerfile

# Golang configuration
GO_BUILD_PACKAGES := ./...
GO_BUILD_FLAGS :=

# Test configuration  
TESTTARGETS := $(shell go list -e ./...)

# Coverage configuration
COVERAGE_PACKAGES := $(shell go list ./... | grep -v /vendor/)

# Validation configuration
VALIDATE_SCHEMA ?= true

# Ensure go modules are vendor'd
GOMOD_VENDOR := true

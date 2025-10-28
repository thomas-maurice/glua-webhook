.PHONY: help test test-unit test-integration test-scripts build clean fmt lint docker-build kind-test

# Default target
.DEFAULT_GOAL := test

# Variables
BINARY_NAME=glua-webhook
DOCKER_IMAGE=glua-webhook
DOCKER_TAG=latest
KIND_CLUSTER_NAME=glua-webhook-test

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: test-unit test-scripts ## Run all tests (unit + script tests)

test-unit: ## Run unit tests with race detection and coverage
	@echo "Running unit tests..."
	go test -v -race -cover ./pkg/...

test-scripts: ## Run Lua script tests
	@echo "Running Lua script tests..."
	go test -v -race ./test/script_test.go

test-integration: ## Run Kind-based integration tests
	@echo "Running integration tests with Kind..."
	go test -v -timeout=10m ./test/integration/...

test-all: test test-integration ## Run all tests including integration tests

build: ## Build the glua-webhook binary
	@echo "Building glua-webhook binary..."
	go build -o bin/$(BINARY_NAME) ./cmd/glua-webhook

clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out

fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found, skipping..."; \
	fi

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Kind cluster management
kind-create: ## Create Kind cluster for testing
	@echo "Creating Kind cluster: $(KIND_CLUSTER_NAME)"
	kind create cluster --name $(KIND_CLUSTER_NAME)
	@echo "Cluster created successfully"

kind-delete: ## Delete Kind cluster
	@echo "Deleting Kind cluster: $(KIND_CLUSTER_NAME)"
	kind delete cluster --name $(KIND_CLUSTER_NAME)

kind-load-image: docker-build ## Load Docker image into Kind cluster
	@echo "Loading image into Kind cluster..."
	kind load docker-image $(DOCKER_IMAGE):$(DOCKER_TAG) --name $(KIND_CLUSTER_NAME)

kind-deploy: kind-load-image ## Deploy webhook to Kind cluster
	@echo "Deploying webhook to Kind cluster..."
	kubectl apply -f examples/manifests/00-namespace.yaml
	kubectl apply -f examples/manifests/01-configmaps.yaml
	kubectl apply -f examples/manifests/04-rbac.yaml
	@echo "Waiting for namespace to be ready..."
	sleep 2
	kubectl apply -f examples/manifests/02-deployment.yaml
	kubectl apply -f examples/manifests/03-service.yaml
	@echo "Deployment complete"

kind-test: kind-create kind-deploy test-integration ## Create cluster, deploy, and run integration tests
	@echo "Integration testing complete"

# Development helpers
dev-deps: ## Install development dependencies
	@echo "Installing development dependencies..."
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	go mod tidy

coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Code generation (if needed in future)
generate: ## Run code generators
	@echo "Running code generators..."
	go generate ./...

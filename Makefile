# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=aks-mentions-bot
BINARY_UNIX=$(BINARY_NAME)_unix

# Docker parameters
DOCKER_IMAGE=aks-mentions-bot
DOCKER_TAG=latest

.PHONY: all build clean test coverage deps docker-build docker-run deploy-azure help

all: test build

build: ## Build the binary
	$(GOBUILD) -o bin/$(BINARY_NAME) -v ./cmd/bot

build-linux: ## Build the binary for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) -v ./cmd/bot

clean: ## Remove build artifacts
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)
	rm -f bin/$(BINARY_UNIX)

test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

test-report: ## Run integration test that generates a sample report
	$(GOTEST) -v ./internal/monitoring -run TestReportGeneration

test-report-cli: ## Generate a sample report using the CLI tool
	$(GOCMD) run ./cmd/test-report/main.go

test-apis: ## Test API connectivity with real services
	$(GOCMD) run ./cmd/test-apis/main.go

test-integration: ## Run full integration test with real APIs (no Azure required)
	$(GOCMD) run ./cmd/test-integration/main.go

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

run: ## Run the application locally
	$(GOCMD) run cmd/bot/main.go

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run: ## Run Docker container locally
	docker run -p 8080:8080 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-clean: ## Remove Docker images
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true

azd-up: ## Deploy to Azure using azd
	azd up

azd-down: ## Destroy Azure resources
	azd down

azd-logs: ## View Azure logs
	azd logs --follow

azd-deploy: ## Deploy without provisioning
	azd deploy

lint: ## Run linters
	golangci-lint run

fmt: ## Format Go code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

mod-update: ## Update Go modules
	$(GOGET) -u all
	$(GOMOD) tidy

dev-setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	@if [ ! -f .env ]; then cp .env.example .env; echo "Created .env file from example"; fi
	@echo "Development environment ready!"
	@echo "Please edit .env file with your configuration"

health-check: ## Check if the application is running
	@curl -s http://localhost:8080/health | grep -q "healthy" && echo "✅ Application is healthy" || echo "❌ Application is not responding"

metrics: ## Show application metrics
	@curl -s http://localhost:8080/metrics | jq .

trigger: ## Manually trigger monitoring
	@curl -X POST http://localhost:8080/trigger

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

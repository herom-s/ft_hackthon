.PHONY: help build build-cli build-api build-worker install install-cli install-api install-worker clean test fmt lint run-api run-cli deps docker-build docker-up docker-down docker-restart docker-logs docker-ps docker-clean docker-cli-binary deploy deploy-info deploy-destroy

# Variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_CLI=ft_hackthon
BINARY_API=ft_hackthon-api
BINARY_WORKER=ft_hackthon-worker

# Build directories
CLI_DIR=./cmd/ft_hackthon
API_DIR=./cmd/api
WORKER_DIR=./cmd/worker

# Colors for output
BLUE := $(shell printf '\033[0;34m')
GREEN := $(shell printf '\033[0;32m')
YELLOW := $(shell printf '\033[1;33m')
NC := $(shell printf '\033[0m')

help: ## Display this help screen
	@echo "$(BLUE)ft_hackthon - Hackathon Grading System$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

# Dependencies
deps: ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(NC)"
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "$(GREEN)✓ Dependencies downloaded$(NC)"

# Building
build: build-cli build-api build-worker ## Build all components

build-cli: ## Build CLI tool
	@echo "$(BLUE)Building ft_hackthon...$(NC)"
	$(GOBUILD) -o bin/$(BINARY_CLI) $(CLI_DIR)
	@echo "$(GREEN)✓ CLI built: bin/$(BINARY_CLI)$(NC)"

build-api: ## Build API server
	@echo "$(BLUE)Building API server...$(NC)"
	$(GOBUILD) -o bin/$(BINARY_API) $(API_DIR)
	@echo "$(GREEN)✓ API server built: bin/$(BINARY_API)$(NC)"

build-worker: ## Build worker engine
	@echo "$(BLUE)Building worker engine...$(NC)"
	$(GOBUILD) -o bin/$(BINARY_WORKER) $(WORKER_DIR)
	@echo "$(GREEN)✓ Worker built: bin/$(BINARY_WORKER)$(NC)"

build-dev: ## Build with debug flags
	@echo "$(BLUE)Building with debug flags...$(NC)"
	$(GOBUILD) -gcflags="all=-N -l" -o bin/$(BINARY_CLI)-dev $(CLI_DIR)
	@echo "$(GREEN)✓ Dev CLI built: bin/$(BINARY_CLI)-dev$(NC)"

# Installation
install: install-cli install-api install-worker ## Install all components

install-cli: build-cli ## Install CLI tool
	@echo "$(BLUE)Installing ft_hackthon...$(NC)"
	cp bin/$(BINARY_CLI) $(GOPATH)/bin/
	@echo "$(GREEN)✓ ft_hackthon installed to $(GOPATH)/bin$(NC)"

install-api: build-api ## Install API server
	@echo "$(BLUE)Installing API server...$(NC)"
	cp bin/$(BINARY_API) $(GOPATH)/bin/
	@echo "$(GREEN)✓ API server installed to $(GOPATH)/bin$(NC)"

install-worker: build-worker ## Install worker engine
	@echo "$(BLUE)Installing worker engine...$(NC)"
	cp bin/$(BINARY_WORKER) $(GOPATH)/bin/
	@echo "$(GREEN)✓ Worker installed to $(GOPATH)/bin$(NC)"

# Testing and Quality
test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	$(GOTEST) -v ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

test-coverage: ## Run tests with coverage
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report: coverage.html$(NC)"

fmt: ## Format code
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GOCMD) fmt ./...
	@echo "$(GREEN)✓ Code formatted$(NC)"

lint: ## Run linter
	@echo "$(BLUE)Running linter...$(NC)"
	@which golangci-lint > /dev/null || (echo "$(YELLOW)Installing golangci-lint...$(NC)" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...
	@echo "$(GREEN)✓ Linting completed$(NC)"

vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GOCMD) vet ./...
	@echo "$(GREEN)✓ Vet completed$(NC)"

# Running
run-api: build-api ## Run API server locally (requires TESTSUITES_PATH)
	@echo "$(BLUE)Starting API server...$(NC)"
	TESTSUITES_PATH="$${TESTSUITES_PATH:-$$PWD/testsuites}" ./bin/$(BINARY_API)

run-worker: build-worker ## Run worker engine locally
	@echo "$(BLUE)Starting worker engine...$(NC)"
	TESTSUITES_PATH="$${TESTSUITES_PATH:-$$PWD/testsuites}" ./bin/$(BINARY_WORKER)

run-cli: build-cli ## Run CLI (use with ARGS=..., adds --insecure by default)
	@echo "$(BLUE)Running ft_hackthon...$(NC)"
	./bin/$(BINARY_CLI) --insecure $(ARGS)

# Docker
docker-build: ## Build all Docker images
	@echo "$(BLUE)Building Docker images...$(NC)"
	docker compose build
	@echo "$(GREEN)✓ Docker images built$(NC)"

docker-up: ## Start Docker services (default: detached). Usage: make docker-up or make docker-up ARGS=
	@echo "$(BLUE)Starting Docker services...$(NC)"
	docker compose up -d $(ARGS)
	@echo "$(GREEN)✓ Docker services started$(NC)"

docker-down: ## Stop and remove Docker containers
	@echo "$(BLUE)Stopping Docker services...$(NC)"
	docker compose down
	@echo "$(GREEN)✓ Docker services stopped$(NC)"

docker-restart: docker-down docker-up ## Restart all Docker services

docker-logs: ## View Docker logs (tail with -f: make docker-logs ARGS=-f)
	@echo "$(BLUE)Docker logs:$(NC)"
	docker compose logs $(ARGS)

docker-ps: ## List running Docker containers
	@echo "$(BLUE)Running containers:$(NC)"
	docker compose ps

docker-clean: ## Remove all Docker resources (containers, volumes, images)
	@echo "$(BLUE)Removing all Docker resources...$(NC)"
	docker compose down --volumes --remove-orphans
	@echo "$(GREEN)✓ Docker resources removed$(NC)"

docker-cli-binary: ## Extract standalone CLI binary (outputs to ./bin/ft_hackthon)
	@echo "$(BLUE)Extracting standalone CLI binary...$(NC)"
	docker build --output=bin/ --target=cli-binary .
	@mv bin/ft_hackthon bin/ft_hackthon-cli 2>/dev/null || true
	@echo "$(GREEN)✓ CLI binary extracted to bin/ft_hackthon-cli$(NC)"

# Cleanup
clean: ## Clean build artifacts and workspace
	@echo "$(BLUE)Cleaning...$(NC)"
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "$(YELLOW)Cleaning ft_hackthon data...$(NC)"
	rm -rf ~/ft_hackthon/
	rm -rf ~/.ft_hackthon/
	@echo "$(GREEN)✓ Clean completed$(NC)"

# Development
watch: ## Watch for changes and rebuild
	@echo "$(BLUE)Watching for changes...$(NC)"
	@which entr > /dev/null || (echo "$(YELLOW)Installing entr...$(NC)" && go install github.com/cortesi/entr/cmd/entr@latest)
	find . -name "*.go" | entr -r make build-cli

# Deployment (multi-cloud via OpenTofu)
deploy: ## Deploy to cloud. Set CLOUD_PROVIDER=digitalocean|aws|gcp|azure in .env
	@echo "$(BLUE)Deploying via OpenTofu...$(NC)"
	@if ! command -v tofu >/dev/null 2>&1; then \
		echo "$(YELLOW)OpenTofu not found. Installing...$(NC)" && \
		(brew install opentofu 2>/dev/null || sudo snap install opentofu --classic 2>/dev/null || \
		echo "Install: https://opentofu.org/docs/cli/install/" && exit 1); fi
	@set -a; . $(PWD)/.env 2>/dev/null || true; set +a; \
		PROVIDER="$${CLOUD_PROVIDER:-digitalocean}"; \
		if [ ! -d "terraform/$$PROVIDER" ]; then echo "Unknown provider: $$PROVIDER (choose: digitalocean, aws, gcp, azure)" && exit 1; fi; \
		echo "Provider: $$PROVIDER"; \
		cd terraform/$$PROVIDER && tofu init && tofu apply
	@echo "$(GREEN)✓ Deployed. Run 'make deploy-info' for VM IP.$(NC)"

deploy-info: ## Show deployed VM info
	@set -a; . $(PWD)/.env 2>/dev/null || true; set +a; \
		cd terraform/$${CLOUD_PROVIDER:-digitalocean} && tofu output

deploy-destroy: ## Destroy the VM
	@set -a; . $(PWD)/.env 2>/dev/null || true; set +a; \
		cd terraform/$${CLOUD_PROVIDER:-digitalocean} && tofu destroy
	@echo "$(GREEN)✓ VM destroyed$(NC)"

# Documentation
docs: ## Generate documentation
	@echo "$(BLUE)Generating documentation...$(NC)"
	$(GOCMD) doc ./...

# Setup
setup: ## Setup development environment
	@echo "$(BLUE)Setting up development environment...$(NC)"
	mkdir -p bin
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "$(GREEN)✓ Setup completed$(NC)"

# Info
info: ## Display build and environment info
	@echo "$(BLUE)Build Information$(NC)"
	@echo "Go Version: $$($(GOCMD) version)"
	@echo "OS: $$($(GOCMD) env GOOS)"
	@echo "Arch: $$($(GOCMD) env GOARCH)"
	@echo ""
	@echo "$(BLUE)Project Structure$(NC)"
	@find . -type f -name "*.go" | grep -E "(cmd|internal)" | sort

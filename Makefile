.PHONY: help build build-cli build-api build-worker install install-cli install-api install-worker \
        clean test test-coverage fmt lint vet run-api run-cli run-worker deps \
        docker-build docker-up docker-down docker-restart docker-logs docker-ps docker-clean docker-cli-binary \
        deploy deploy-info deploy-destroy dns setup info watch \
        floci-up floci-down test-deploy test-deploy-aws test-deploy-azure test-deploy-gcp

# ── Go ──
GOCMD     = go
GOBUILD   = $(GOCMD) build
GOCLEAN   = $(GOCMD) clean
GOTEST    = $(GOCMD) test
GOMOD     = $(GOCMD) mod

BINARY_CLI     = ft_hackthon
BINARY_API     = ft_hackthon-api
BINARY_WORKER  = ft_hackthon-worker
CLI_DIR        = ./cmd/ft_hackthon
API_DIR        = ./cmd/api
WORKER_DIR     = ./cmd/worker

# ── Colors ──
BLUE  := $(shell printf '\033[0;34m')
GREEN := $(shell printf '\033[0;32m')
YELLOW := $(shell printf '\033[1;33m')
NC    := $(shell printf '\033[0m')

# ── Env ──
# Usage: $(LOAD_ENV) at the start of any recipe that needs .env vars.
# Must be followed by line continuations (backslash) to stay in the same shell.
LOAD_ENV = set -a; . $(PWD)/.env 2>/dev/null || true; set +a

# ── Help ──
help:
	@echo "$(BLUE)ft_hackthon - Hackathon Grading System$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	  | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

# ── Dependencies ──
deps: ## Download Go dependencies
	$(GOMOD) download
	$(GOMOD) tidy

# ── Build ──
build: build-cli build-api build-worker ## Build all components

build-cli: ## Build CLI binary
	$(GOBUILD) -o bin/$(BINARY_CLI) $(CLI_DIR)

build-api: ## Build API server binary
	$(GOBUILD) -o bin/$(BINARY_API) $(API_DIR)

build-worker: ## Build worker binary
	$(GOBUILD) -o bin/$(BINARY_WORKER) $(WORKER_DIR)

build-dev: ## Build CLI with debug flags
	$(GOBUILD) -gcflags="all=-N -l" -o bin/$(BINARY_CLI)-dev $(CLI_DIR)

# ── Install ──
install: install-cli install-api install-worker ## Install all binaries to GOPATH/bin
install-cli: build-cli
	cp bin/$(BINARY_CLI) $(GOPATH)/bin/
install-api: build-api
	cp bin/$(BINARY_API) $(GOPATH)/bin/
install-worker: build-worker
	cp bin/$(BINARY_WORKER) $(GOPATH)/bin/

# ── Test & Quality ──
test: ## Run all tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage report
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

fmt: ## Format Go code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

lint: ## Run golangci-lint
	@which golangci-lint > /dev/null \
	  || (echo "$(YELLOW)Installing golangci-lint...$(NC)" \
	      && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# ── Run Locally ──
run-api: build-api ## Start API server (requires TESTSUITES_PATH)
	TESTSUITES_PATH="$${TESTSUITES_PATH:-$$PWD/testsuites}" ./bin/$(BINARY_API)

run-worker: build-worker ## Start worker engine
	TESTSUITES_PATH="$${TESTSUITES_PATH:-$$PWD/testsuites}" ./bin/$(BINARY_WORKER)

run-cli: build-cli ## Run CLI (--insecure added by default; override with ARGS=...)
	./bin/$(BINARY_CLI) --insecure $(ARGS)

# ── Docker ──
docker-build: ## Build all Docker images
	docker compose build

docker-up: ## Start services (detached). make docker-up ARGS= for foreground
	docker compose up -d $(ARGS)

docker-down: ## Stop and remove containers
	docker compose down

docker-restart: docker-down docker-up ## Restart all services

docker-logs: ## Tail logs. make docker-logs ARGS=-f for follow
	docker compose logs $(ARGS)

docker-ps: ## List running containers
	docker compose ps

docker-clean: ## Remove containers, volumes, images
	docker compose down --volumes --remove-orphans

docker-cli-binary: ## Extract standalone CLI binary to ./bin/ft_hackthon-cli
	docker build --output=bin/ --target=cli-binary .
	@mv bin/ft_hackthon bin/ft_hackthon-cli 2>/dev/null || true

# ── Clean ──
clean: ## Remove build artifacts
	$(GOCLEAN)
	rm -rf bin/ coverage.out coverage.html

# ── Development ──
watch: ## Rebuild CLI on Go file changes
	@which entr > /dev/null \
	  || (echo "$(YELLOW)Installing entr...$(NC)" \
	      && go install github.com/cortesi/entr/cmd/entr@latest)
	find . -name "*.go" | entr -r make build-cli

setup: ## Initialize development environment
	mkdir -p bin
	$(GOMOD) download
	$(GOMOD) tidy

info: ## Show Go version and project structure
	@echo "$(BLUE)Go Version:$(NC) $$($(GOCMD) version)"
	@echo "$(BLUE)OS / Arch:$(NC) $$($(GOCMD) env GOOS)/$$($(GOCMD) env GOARCH)"
	@echo ""
	@echo "$(BLUE)Project Structure:$(NC)"
	@find . -type f -name "*.go" | grep -E "(cmd|internal)" | sort

# ── Terraform helpers ──
# Exports used by deploy, deploy-destroy, and dns.
# Usage: inline with a recipe using line continuations.
define TOFU_ENV
	$(LOAD_ENV) && \
	PROVIDER="$${CLOUD_PROVIDER:-aws}" && \
	export TF_VAR_domain="$${DOMAIN:-}" && \
	export TF_VAR_ssh_key_name="$${SSH_KEY_NAME:-}" && \
	export TF_VAR_ssh_public_key="$$(cat "$${SSH_PUBLIC_KEY_PATH:-$$HOME/.ssh/id_rsa.pub}" 2>/dev/null)" && \
	true
endef

# ── Deploy ──
deploy: ## Deploy to CLOUD_PROVIDER (aws|gcp|azure). Reads .env
	@$(TOFU_ENV); \
		echo "$(BLUE)Deploying $$PROVIDER...$(NC)"; \
		if ! command -v tofu >/dev/null 2>&1; then \
			echo "Install OpenTofu: https://opentofu.org/docs/cli/install/" >&2; \
			exit 1; \
		fi; \
		if [ ! -d "terraform/$$PROVIDER" ]; then \
			echo "Unknown provider: $$PROVIDER (choose: aws, gcp, azure)" >&2; \
			exit 1; \
		fi; \
		if [ "$$PROVIDER" = "azure" ] && [ -z "$$TF_VAR_ssh_public_key" ]; then \
			echo "$(YELLOW)Azure requires SSH_PUBLIC_KEY_PATH in .env$(NC)" >&2; \
			exit 1; \
		fi; \
		cd terraform/$$PROVIDER && tofu init && tofu apply -auto-approve; \
		EXIT=$$?; \
		if [ "$$PROVIDER" = "azure" ] && [ $$EXIT -ne 0 ]; then \
			echo "$(YELLOW)Azure eventual consistency — retrying...$(NC)"; \
			sleep 10; \
			for i in 1 2 3; do \
				tofu apply -auto-approve && EXIT=0 && break; \
				EXIT=$$?; \
				[ $$i -lt 3 ] && sleep 15; \
			done; \
		fi; \
		exit $$EXIT
	@echo "$(GREEN)✓ Deployed — run 'make deploy-info' for connection details$(NC)"

# ── Deploy integration tests (local cloud emulators, no cloud account) ──
FLOCI_COMPOSE = docker compose -f terraform/test/docker-compose.yml

floci-up: ## Start the floci AWS, Azure and GCP emulators used by test-deploy
	@$(FLOCI_COMPOSE) up -d --wait

floci-down: ## Stop the floci emulators
	@$(FLOCI_COMPOSE) down --volumes --remove-orphans

test-deploy: floci-up ## Deploy and destroy every cloud module against the floci emulators
	@TF_PLUGIN_CACHE_DIR="$${TF_PLUGIN_CACHE_DIR:-$$HOME/.terraform.d/plugin-cache}" \
		$(GOTEST) -count=1 -tags integration -timeout 30m -v ./terraform/test/

test-deploy-aws: ## Deploy test for the AWS module only (floci)
	@$(MAKE) --no-print-directory _deploy-test SERVICE=floci TEST=TestDeployAWS

test-deploy-azure: ## Deploy test for the Azure module only (floci-az)
	@$(MAKE) --no-print-directory _deploy-test SERVICE=floci-az TEST=TestDeployAzure

test-deploy-gcp: ## Deploy test for the GCP module only (floci-gcp)
	@$(MAKE) --no-print-directory _deploy-test SERVICE=floci-gcp TEST=TestDeployGCP

_deploy-test:
	@$(FLOCI_COMPOSE) up -d --wait $(SERVICE)
	@TF_PLUGIN_CACHE_DIR="$${TF_PLUGIN_CACHE_DIR:-$$HOME/.terraform.d/plugin-cache}" \
		$(GOTEST) -count=1 -tags integration -timeout 20m -v -run '^$(TEST)$$' ./terraform/test/

deploy-info: ## Show outputs from the deployed stack
	@$(LOAD_ENV) && \
		cd terraform/$${CLOUD_PROVIDER:-aws} && tofu output

deploy-destroy: ## Tear down the deployed stack
	@echo "$(YELLOW)Destroying deployment...$(NC)"
	@$(TOFU_ENV) && \
		cd terraform/$$PROVIDER && tofu destroy
	@echo "$(GREEN)✓ Destroyed$(NC)"

dns: ## Point name.com A records at the VM. Requires NAMECOM_USERNAME + NAMECOM_TOKEN in .env
	@$(LOAD_ENV) && \
		if [ -z "$$NAMECOM_USERNAME" ] || [ -z "$$NAMECOM_TOKEN" ]; then \
			echo "Set NAMECOM_USERNAME and NAMECOM_TOKEN in .env" >&2; \
			exit 1; \
		fi && \
		if [ -z "$$DOMAIN" ]; then \
			echo "Set DOMAIN in .env" >&2; \
			exit 1; \
		fi && \
		VM_IP="$${VM_IP:-$$(cd terraform/$${CLOUD_PROVIDER:-aws} 2>/dev/null && tofu output -raw ip 2>/dev/null | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$$')}" && \
		if [ -z "$$VM_IP" ]; then \
			echo "Set VM_IP=... in .env or deploy first" >&2; \
			exit 1; \
		fi && \
		scripts/dns-namecom.sh "$$NAMECOM_USERNAME" "$$NAMECOM_TOKEN" "$$DOMAIN" "$$VM_IP"
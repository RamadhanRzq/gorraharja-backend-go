GO ?= go
COMPOSE ?= docker compose

.DEFAULT_GOAL := help

.PHONY: help run build test test-race vet fmt tidy up down logs psql clean

help: ## Show this help.
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## Run the API locally.
	$(GO) run ./cmd/api

build: ## Build the API binary into bin/api.
	$(GO) build -trimpath -o bin/api ./cmd/api

test: ## Run the test suite.
	$(GO) test ./...

test-race: ## Run the test suite with the race detector.
	$(GO) test -race ./...

vet: ## Run go vet.
	$(GO) vet ./...

fmt: ## Format all Go source files.
	$(GO) fmt ./...

tidy: ## Tidy and verify go.mod/go.sum.
	$(GO) mod tidy

up: ## Start the full stack in the background.
	$(COMPOSE) up -d --build

down: ## Stop the stack and remove containers.
	$(COMPOSE) down

logs: ## Follow logs from all services.
	$(COMPOSE) logs -f

psql: ## Open a psql shell on the postgres service.
	$(COMPOSE) exec postgres psql -U postgres -d gorraharja

clean: ## Remove build artifacts.
	rm -rf bin coverage.out coverage.txt coverage.html

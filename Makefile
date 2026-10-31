SHELL := /bin/bash

BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO_LDFLAGS := -trimpath -ldflags "-s -w -X main.version=$(VERSION)"
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo ./.tools/golangci-lint)

.PHONY: help
help: ## Print this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the bot and migrate binaries into ./bin
	go build $(GO_LDFLAGS) -o $(BIN_DIR)/bot ./cmd/bot
	go build $(GO_LDFLAGS) -o $(BIN_DIR)/cron ./cmd/cron
	go build $(GO_LDFLAGS) -o $(BIN_DIR)/migrate ./cmd/migrate

.PHONY: run
run: build migrate-up ## Run migrations and start the bot locally
	./$(BIN_DIR)/bot

.PHONY: test
test: ## Run unit tests with the race detector
	go test -race ./...

.PHONY: test-integration
test-integration: ## Run repository/migration tests (requires TEST_POSTGRES_DSN)
	go test -tags=integration -race -count=1 ./internal/storage/postgres/...

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests (requires TEST_POSTGRES_* and TEST_REDIS_ADDR)
	go test -tags=e2e -race -count=1 ./internal/app/...

.PHONY: test-all
test-all: test test-integration test-e2e ## Run every test level

.PHONY: test-infra
test-infra: ## Run integration + e2e using variables from .env (start docker-up-dev first)
	./scripts/test-all.sh

.PHONY: cover
cover: ## Run tests and print the total coverage
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

.PHONY: lint
lint: ## Check formatting, vet and run golangci-lint
	test -z "$$(gofmt -l ./cmd ./internal ./migrations)"
	go vet ./...
	$(GOLANGCI_LINT) run --timeout=5m

.PHONY: fmt
fmt: ## Format the code
	gofmt -w .

.PHONY: run-cron
run-cron: build ## Run the reminder scheduler locally
	./$(BIN_DIR)/cron

.PHONY: remind-once
remind-once: build ## Send reminders once and exit (local check of the scheduler)
	./$(BIN_DIR)/cron -once

.PHONY: backup
backup: ## Dump the Docker Compose PostgreSQL database to ./backups
	./scripts/backup.sh

.PHONY: migrate-up
migrate-up: ## Apply pending database migrations
	go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## Roll back the last migration
	go run ./cmd/migrate down

.PHONY: migrate-status
migrate-status: ## Show migration status
	go run ./cmd/migrate status

.PHONY: docker-up-dev
docker-up-dev: ## Start PostgreSQL and Redis for local development
	docker compose -f docker/docker-compose.dev.yml --env-file .env up -d

.PHONY: docker-down-dev
docker-down-dev: ## Stop the development infrastructure
	docker compose -f docker/docker-compose.dev.yml --env-file .env down

.PHONY: docker-build
docker-build: ## Build the production image
	docker build -f docker/Dockerfile --build-arg VERSION=$(VERSION) -t mega-trainer:$(VERSION) .

.PHONY: docker-up
docker-up: ## Start the full stack (database, migrations, bot) in Docker
	docker compose -f docker/docker-compose.yml --env-file .env up -d --build

.PHONY: docker-down
docker-down: ## Stop the Docker stack
	docker compose -f docker/docker-compose.yml --env-file .env down

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) coverage.out

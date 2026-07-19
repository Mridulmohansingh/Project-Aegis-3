.PHONY: all build test lint fmt vet clean migrate-up migrate-down docker-build generate help

# Build variables
APP_NAME := aegis
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.commit=$(COMMIT) -s -w"

# Go variables
GO := go
GOTEST := $(GO) test
GOVET := $(GO) vet
GOBUILD := $(GO) build
GOFMT := gofmt

# Directories
CMD_DIR := ./cmd
BIN_DIR := ./bin
MIGRATION_DIR := ./migrations

# Services
SERVICES := question-bank paper-engine exam-delivery scoring audit crypto-service api-gateway

# Database
DB_DSN ?= postgres://aegis:aegis@localhost:5432/aegis?sslmode=disable

## help: Show this help message
help:
	@echo "Project AEGIS — National Digital Assessment Platform"
	@echo ""
	@echo "Usage:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'

## all: Build all services
all: fmt vet lint test build

## build: Build all service binaries
build:
	@echo "Building all services..."
	@mkdir -p $(BIN_DIR)
	@for service in $(SERVICES); do \
		echo "  Building $$service..."; \
		$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$$service $(CMD_DIR)/$$service/main.go; \
	done
	@echo "Build complete."

## build-%: Build a specific service (e.g., make build-question-bank)
build-%:
	@echo "Building $*..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$* $(CMD_DIR)/$*/main.go

## test: Run all tests with race detection
test:
	@echo "Running tests..."
	$(GOTEST) -race -cover -coverprofile=coverage.out -count=1 ./...
	@echo "Tests complete."

## test-coverage: Generate HTML coverage report
test-coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -race -tags=integration -count=1 ./tests/integration/...

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	golangci-lint run --timeout 5m ./...

## fmt: Format Go source files
fmt:
	@echo "Formatting..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running vet..."
	$(GOVET) ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) coverage.out coverage.html
	@echo "Clean complete."

## migrate-up: Run database migrations
migrate-up:
	@echo "Running migrations..."
	migrate -path $(MIGRATION_DIR) -database "$(DB_DSN)" up

## migrate-down: Rollback last migration
migrate-down:
	@echo "Rolling back last migration..."
	migrate -path $(MIGRATION_DIR) -database "$(DB_DSN)" down 1

## migrate-create: Create new migration (usage: make migrate-create NAME=create_items)
migrate-create:
	@echo "Creating migration $(NAME)..."
	migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)

## docker-build: Build Docker images for all services
docker-build:
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		echo "  Building image aegis/$$service..."; \
		docker build -t aegis/$$service:$(VERSION) -f deployments/docker/Dockerfile --build-arg SERVICE=$$service .; \
	done

## docker-compose-up: Start local development environment
docker-compose-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

## docker-compose-down: Stop local development environment
docker-compose-down:
	docker compose -f deployments/docker/docker-compose.yml down

## generate: Run code generation
generate:
	$(GO) generate ./...

## deps: Download and tidy dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

## security-scan: Run security scanning
security-scan:
	govulncheck ./...
	trivy fs --scanners vuln .

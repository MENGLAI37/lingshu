.PHONY: all build test lint dev-up dev-down migrate-up migrate-down clean help

APP_NAME := ops-ai
ALERTD_NAME := ops-ai-alertd
VERSION := v0.1.0
BUILD_DIR := bin

GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

GOLANGCI_LINT_VERSION := v1.59.0
MIGRATE_VERSION := v4.17.1

help:
	@echo "Available targets:"
	@echo "  make build        - Build all binaries"
	@echo "  make test         - Run unit tests"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make dev-up       - Start development environment (Docker Compose)"
	@echo "  make dev-down     - Stop development environment"
	@echo "  make migrate-up   - Run database migrations up"
	@echo "  make migrate-down - Run database migrations down"
	@echo "  make clean        - Clean build artifacts"

all: build

build:
	@echo "==> Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/ops-ai
	@echo "==> Building $(ALERTD_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(ALERTD_NAME) ./cmd/alertd
	@echo "Build complete. Binaries in $(BUILD_DIR)/"

test:
	@echo "==> Running tests..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out ./...
	@echo "Tests complete."

lint:
	@echo "==> Running golangci-lint..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found, installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run ./...
	@echo "Lint complete."

dev-up:
	@echo "==> Starting development environment..."
	docker-compose up -d
	@echo "Development environment started."
	@echo "PostgreSQL: localhost:5432"
	@echo "Redis: localhost:6379"
	@echo "MinIO: localhost:9000"
	@echo "ChromaDB: localhost:8000"

dev-down:
	@echo "==> Stopping development environment..."
	docker-compose down
	@echo "Development environment stopped."

migrate-up:
	@echo "==> Running database migrations up..."
	@if ! command -v migrate &> /dev/null; then \
		echo "migrate not found, installing..."; \
		$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION); \
	fi
	migrate -path migrations -database "postgres://opsai:opsai@localhost:5432/opsai?sslmode=disable" up

migrate-down:
	@echo "==> Running database migrations down..."
	@if ! command -v migrate &> /dev/null; then \
		echo "migrate not found, installing..."; \
		$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION); \
	fi
	migrate -path migrations -database "postgres://opsai:opsai@localhost:5432/opsai?sslmode=disable" down

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
	@echo "Clean complete."

.PHONY: all build test test-integration lint dev-up dev-down migrate-up migrate-down clean help
.PHONY: docker-build docker-push kind-create kind-delete helm-install helm-uninstall

APP_NAME := ops-ai
ALERTD_NAME := ops-ai-alertd
VERSION := v0.1.0
BUILD_DIR := bin

GO := go
GOFLAGS := -v
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

GOLANGCI_LINT_VERSION := v1.59.0
MIGRATE_VERSION := v4.17.1

# Docker
DOCKER_REGISTRY ?= ghcr.io
DOCKER_IMAGE := $(DOCKER_REGISTRY)/$(shell git config user.name 2>/dev/null || echo "lingshu")/ops-ai-agent

# Kind
KIND_CLUSTER := opsai-dev

# Helm
HELM_RELEASE := ops-ai

help:
	@echo "Available targets:"
	@echo "  build            - Build all binaries"
	@echo "  test             - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-short       - Run tests without integration tests"
	@echo "  lint             - Run golangci-lint"
	@echo "  lint-fix         - Run golangci-lint with auto-fix"
	@echo ""
	@echo "  dev-up           - Start development environment (Docker Compose)"
	@echo "  dev-down         - Stop development environment"
	@echo "  migrate-up       - Run database migrations up"
	@echo "  migrate-down     - Run database migrations down"
	@echo ""
	@echo "  docker-build     - Build Docker images"
	@echo "  docker-push      - Push Docker images to registry"
	@echo "  kind-create      - Create kind cluster"
	@echo "  kind-delete      - Delete kind cluster"
	@echo "  helm-install     - Install Helm chart"
	@echo "  helm-upgrade     - Upgrade Helm chart"
	@echo "  helm-uninstall   - Uninstall Helm chart"
	@echo "  helm-template    - Render Helm template"
	@echo ""
	@echo "  coverage         - Generate coverage report"
	@echo "  clean            - Clean build artifacts"

all: build

# ===========================================================================
# Build targets
# ===========================================================================

build:
	@echo "==> Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/ops-ai
	@echo "==> Building $(ALERTD_NAME)..."
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(ALERTD_NAME) ./cmd/alertd
	@echo "Build complete. Binaries in $(BUILD_DIR)/"

build-all:
	@echo "==> Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$${os}" = "windows" ] && [ "$${arch}" = "arm64" ]; then \
				continue; \
			fi; \
			if [ "$${os}" = "darwin" ] && [ "$${arch}" = "arm64" ]; then \
				continue; \
			fi; \
			ext=""; \
			if [ "$${os}" = "windows" ]; then ext=".exe"; fi; \
			echo "Building for $${os}/$${arch}..."; \
			CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$${os}_$${arch}/$(APP_NAME)$${ext} ./cmd/ops-ai; \
			CGO_ENABLED=0 GOOS=$${os} GOARCH=$${arch} $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$${os}_$${arch}/$(ALERTD_NAME)$${ext} ./cmd/alertd; \
		done; \
	done
	@echo "Build complete."

# ===========================================================================
# Test targets
# ===========================================================================

test:
	@echo "==> Running tests..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Tests complete."

test-short:
	@echo "==> Running short tests..."
	$(GO) test $(GOFLAGS) -short ./...
	@echo "Short tests complete."

test-integration:
	@echo "==> Running integration tests..."
	@echo "Note: Integration tests require a running kind cluster"
	@if ! command -v kind &> /dev/null; then \
		echo "kind not found. Install from: https://kind.sigs.k8s.io/docs/user/quick-start/"; \
		exit 1; \
	fi
	$(GO) test $(GOFLAGS) -v -tags=integration -integration ./tests/integration/...
	@echo "Integration tests complete."

test-coverage:
	@echo "==> Running tests with coverage..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -covermode=atomic -coverpkg=./... ./...
	@echo "Coverage report:"
	$(GO) tool cover -func=coverage.out | tail -n +2

# ===========================================================================
# Lint targets
# ===========================================================================

lint:
	@echo "==> Running golangci-lint..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found, installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run ./...
	@echo "Lint complete."

lint-fix:
	@echo "==> Running golangci-lint with auto-fix..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found, installing..."; \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run --fix ./...
	@echo "Lint fix complete."

lint-docker:
	@echo "==> Running golangci-lint in Docker..."
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:latest golangci-lint run ./...

# ===========================================================================
# Development environment
# ===========================================================================

dev-up:
	@echo "==> Starting development environment..."
	docker-compose up -d
	@echo "Development environment started."
	@echo "PostgreSQL: localhost:5432"
	@echo "Redis: localhost:6379"
	@echo "MinIO: localhost:9000"
	@echo "ChromaDB: localhost:8000"
	@echo ""
	@echo "Run 'make migrate-up' to initialize the database"

dev-down:
	@echo "==> Stopping development environment..."
	docker-compose down
	@echo "Development environment stopped."

dev-logs:
	docker-compose logs -f

dev-recreate:
	@echo "==> Recreating development environment..."
	docker-compose down -v
	docker-compose up -d
	@echo "Development environment recreated."

# ===========================================================================
# Database migrations
# ===========================================================================

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

migrate-create:
	@if [ -z "$(NAME)" ]; then \
		echo "Usage: make migrate-create NAME=init_users_table"; \
		exit 1; \
	fi
	@echo "==> Creating migration: $(NAME)"
	@if ! command -v migrate &> /dev/null; then \
		$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION); \
	fi
	migrate create -ext sql -dir migrations -seq $(NAME)

# ===========================================================================
# Docker targets
# ===========================================================================

docker-build:
	@echo "==> Building Docker images..."
	docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .
	@echo "Docker images built: $(DOCKER_IMAGE):$(VERSION), $(DOCKER_IMAGE):latest"

docker-buildx:
	@echo "==> Building Docker images with buildx..."
	docker buildx build --platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(VERSION) \
		-t $(DOCKER_IMAGE):latest \
		--push .

docker-push:
	@echo "==> Pushing Docker images..."
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest
	@echo "Docker images pushed."

docker-clean:
	@echo "==> Cleaning Docker images..."
	docker rmi $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest || true
	docker image prune -f

# ===========================================================================
# Kind cluster targets
# ===========================================================================

kind-create:
	@echo "==> Creating kind cluster: $(KIND_CLUSTER)..."
	@if ! command -v kind &> /dev/null; then \
		echo "kind not found. Install from: https://kind.sigs.k8s.io/docs/user/quick-start/"; \
		exit 1; \
	fi
	@if kind get clusters | grep -q "^$(KIND_CLUSTER)$$"; then \
		echo "Kind cluster '$(KIND_CLUSTER)' already exists"; \
	else \
		kind create cluster --name $(KIND_CLUSTER) --config kind-config.yaml; \
		kubectl apply -f deployments/k8s/test-apps.yaml; \
		echo "Kind cluster '$(KIND_CLUSTER)' created with test apps"; \
	fi

kind-delete:
	@echo "==> Deleting kind cluster: $(KIND_CLUSTER)..."
	@if command -v kind &> /dev/null; then \
		kind delete cluster --name $(KIND_CLUSTER); \
	fi
	@echo "Kind cluster deleted."

kind-load:
	@echo "==> Loading Docker image into kind cluster..."
	kind load docker-image $(DOCKER_IMAGE):$(VERSION) --name $(KIND_CLUSTER)

kind-logs:
	kubectl cluster-info --context kind-$(KIND_CLUSTER)
	kubectl get nodes -o wide
	kubectl get pods -A

# ===========================================================================
# Helm targets
# ===========================================================================

helm-install:
	@echo "==> Installing Helm chart: $(HELM_RELEASE)..."
	helm install $(HELM_RELEASE) ./charts/ops-ai \
		--namespace ops-ai \
		--create-namespace \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(VERSION)

helm-upgrade:
	@echo "==> Upgrading Helm chart: $(HELM_RELEASE)..."
	helm upgrade $(HELM_RELEASE) ./charts/ops-ai \
		--namespace ops-ai \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(VERSION) \
		--install

helm-uninstall:
	@echo "==> Uninstalling Helm chart: $(HELM_RELEASE)..."
	helm uninstall $(HELM_RELEASE) --namespace ops-ai || true

helm-template:
	@echo "==> Rendering Helm template..."
	helm template $(HELM_RELEASE) ./charts/ops-ai \
		--set image.repository=$(DOCKER_IMAGE) \
		--set image.tag=$(VERSION)

helm-repo-update:
	helm repo update

helm-lint:
	helm lint ./charts/ops-ai

# ===========================================================================
# Utility targets
# ===========================================================================

coverage:
	@echo "==> Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

clean:
	@echo "==> Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -rf .cache
	@echo "Clean complete."

# ===========================================================================
# CI targets
# ===========================================================================

ci-test:
	@echo "==> Running CI tests..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "CI tests complete."

ci-lint:
	@echo "==> Running CI lint..."
	golangci-lint run --timeout=5m ./...

# ===========================================================================
# Dependency management
# ===========================================================================

deps:
	@echo "==> Checking dependencies..."
	$(GO) mod download
	$(GO) mod verify

deps-update:
	@echo "==> Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy

tidy:
	@echo "==> Tidying modules..."
	$(GO) mod tidy
	$(GO) mod verify

.PHONY: all build build-all test test-short test-integration test-coverage
.PHONY: lint lint-fix lint-docker
.PHONY: dev-up dev-down dev-logs dev-recreate
.PHONY: migrate-up migrate-down migrate-create
.PHONY: docker-build docker-buildx docker-push docker-clean
.PHONY: kind-create kind-delete kind-load kind-logs
.PHONY: helm-install helm-upgrade helm-uninstall helm-template helm-repo-update helm-lint
.PHONY: coverage clean
.PHONY: ci-test ci-lint
.PHONY: deps deps-update tidy

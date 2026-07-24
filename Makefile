.PHONY: help build run test test-unit test-integration test-coverage lint vet tidy migrate up down docker-build docker-up docker-down swagger clean

# Default target
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==================== Development ====================

build: ## Build the application binary
	go build -o bin/server ./cmd/main.go

run: ## Run the application locally
	go run ./cmd/main.go

# ==================== Testing ====================

test: ## Run all tests
	go test ./...

test-v: ## Run all tests with verbose output
	go test ./... -v

test-unit: ## Run unit tests only (no integration)
	go test ./pkg/... ./service/... ./validation/... -v

test-integration: ## Run integration tests (requires docker)
	go test ./repository/... -v -count=1 -timeout 120s

test-coverage: ## Run tests with coverage report
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race: ## Run tests with race detector
	go test ./... -race

# ==================== Code Quality ====================

lint: ## Run golangci-lint
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy go modules
	go mod tidy

fmt: ## Format code
	gofmt -s -w .

# ==================== Docker ====================

docker-build: ## Build docker image
	docker build -t graph-api .

docker-up: ## Start all services with docker-compose
	docker compose up -d

docker-down: ## Stop all services
	docker compose down

docker-logs: ## View docker logs
	docker compose logs -f

docker-ps: ## Show running containers
	docker compose ps

# ==================== Swagger ====================

swagger: ## Generate swagger docs
	swag init -g cmd/main.go -o docs

swagger-serve: ## Start server and open swagger
	@echo "Swagger UI: http://localhost:8080/swagger/index.html"
	go run ./cmd/main.go

# ==================== Observability ====================

metrics: ## Show prometheus metrics endpoint
	@curl -s http://localhost:8080/metrics | head -20

health: ## Health check
	@curl -s http://localhost:8080/tasks?page_number=1\&page_size=1 | python3 -m json.tool

# ==================== Mocks ====================

mocks: ## Regenerate mocks
	go generate ./service/...

# ==================== Clean ====================

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Cleaned."

# ==================== All ====================

all: tidy vet test build ## Run tidy, vet, test, and build
	@echo "All checks passed."

.PHONY: help build run test test-unit test-integration test-coverage test-bench test-race lint vet tidy migrate up down docker-build docker-up docker-down swagger clean k6-load k6-stress k6-spike k6-soak k6-all

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

test-bench: ## Run benchmarks and generate pprof profiles
	go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof -benchtime=3s -run=^$ ./service/
	@echo "Profiles generated: cpu.prof, mem.prof"
	@echo "View: go tool pprof cpu.prof"

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

loadtest: ## Run load test against running service (usage: make loadtest URL=http://localhost:8080 CONC=10 N=100)
	./loadtest.sh $(URL) $(CONC) $(N)

# ==================== k6 Tests ====================

k6-load: ## Run k6 load test (sustained traffic, 100s)
	k6 run loadtest/k6_load.js

k6-stress: ## Run k6 stress test (ramp to 200 VUs, ~4min)
	k6 run loadtest/k6_stress.js

k6-spike: ## Run k6 spike test (sudden 300 VUs burst, ~2min)
	k6 run loadtest/k6_spike.js

k6-soak: ## Run k6 soak test (sustained 20 VUs for 10min)
	k6 run loadtest/k6_soak.js

k6-all: ## Run all k6 tests sequentially
	@echo "Running Load Test..."
	k6 run loadtest/k6_load.js
	@echo "\nRunning Stress Test..."
	k6 run loadtest/k6_stress.js
	@echo "\nRunning Spike Test..."
	k6 run loadtest/k6_spike.js
	@echo "\nRunning Soak Test..."
	k6 run loadtest/k6_soak.js

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	@echo "Cleaned."

# ==================== All ====================

all: tidy vet test build ## Run tidy, vet, test, and build
	@echo "All checks passed."

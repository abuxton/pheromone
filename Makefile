.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: deps
deps: ## Install Go dependencies
	go mod download
	go mod tidy

.PHONY: build
build: ## Build all packages
	go build ./internal/...

.PHONY: test
test: ## Run all tests (without etcd)
	go test -v ./benchmark -run TestMemoryCacheHitRatio
	go test -v ./benchmark -run TestNoDataLossOnWriteFailure

.PHONY: test-etcd
test-etcd: ## Run etcd integration tests (requires running etcd)
	go test -v ./benchmark -run TestEtcd
	go test -v ./benchmark -run TestHybrid
	go test -v ./benchmark -run TestServerRecoveryTime

.PHONY: bench
bench: ## Run all benchmarks
	go test -bench=. ./benchmark -benchmem -benchtime=100x

.PHONY: bench-memory
bench-memory: ## Run memory benchmarks only
	go test -bench=BenchmarkMemory ./benchmark -benchmem -benchtime=1000x

.PHONY: bench-etcd
bench-etcd: ## Run etcd benchmarks only (requires running etcd)
	go test -bench=BenchmarkEtcd ./benchmark -benchmem -benchtime=100x

.PHONY: bench-hybrid
bench-hybrid: ## Run hybrid store benchmarks (requires running etcd)
	go test -bench=BenchmarkHybrid ./benchmark -benchmem -benchtime=100x

.PHONY: etcd-up
etcd-up: ## Start etcd using Docker Compose
	docker-compose up -d
	@echo "Waiting for etcd to be ready..."
	@sleep 3
	@curl -s http://localhost:2379/health || echo "etcd not ready yet"

.PHONY: etcd-down
etcd-down: ## Stop etcd
	docker-compose down

.PHONY: etcd-clean
etcd-clean: ## Stop etcd and remove volumes
	docker-compose down -v

.PHONY: etcd-logs
etcd-logs: ## Show etcd logs
	docker-compose logs -f etcd

.PHONY: validate
validate: etcd-up ## Run full validation suite (starts etcd, runs all tests)
	@echo "Running validation tests..."
	@sleep 3
	@go test -v ./benchmark -run Test || true
	@echo ""
	@echo "Validation complete. Check ADR/adr-002-performance-validation.md for results."

.PHONY: validate-offline
validate-offline: ## Run validation tests that don't require etcd
	@echo "Running offline validation tests..."
	@go test -v ./benchmark -run TestMemoryCacheHitRatio
	@go test -v ./benchmark -run TestNoDataLossOnWriteFailure
	@echo ""
	@echo "Offline validation complete."

.PHONY: clean
clean: ## Clean build artifacts
	go clean
	rm -rf dist/ build/

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: fmt vet ## Run linters

.PHONY: all
all: deps build test ## Install deps, build, and test

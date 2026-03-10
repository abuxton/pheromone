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
build: ## Build all packages and CLI binaries
	go build ./internal/...
	go build -o dist/pheromone-server ./cmd/pheromone-server
	go build -o dist/pheromone-agent  ./cmd/pheromone-agent

.PHONY: test
test: ## Run all tests (without etcd)
	go test -v ./benchmark -run TestMemoryCacheHitRatio
	go test -v ./benchmark -run TestNoDataLossOnWriteFailure
	go test -v -race ./internal/config/...

.PHONY: test-grpc-spike
test-grpc-spike: ## Run ADR-003 gRPC spike validation tests (1000-agent load, latency, throughput)
	go test -v -timeout 180s ./benchmark -run '^TestGRPC'

.PHONY: test-etcd
test-etcd: ## Run etcd integration tests (requires running etcd)
	go test -v ./benchmark -run TestEtcd
	go test -v ./benchmark -run TestHybrid
	go test -v ./benchmark -run TestServerRecoveryTime

.PHONY: bench
bench: ## Run all benchmarks
	go test -bench=. -run='^$' ./benchmark -benchmem -benchtime=100x

.PHONY: bench-grpc
bench-grpc: ## Run ADR-003 gRPC spike benchmarks (Register throughput, TelemetryStream throughput)
	go test -bench=BenchmarkGRPC -run='^$' ./benchmark -benchmem -benchtime=200x

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

.PHONY: vagrant-up
vagrant-up: ## Start all Vagrant VMs (server + agent-ubuntu + agent-debian)
	cd vagrant && vagrant up

.PHONY: vagrant-up-server
vagrant-up-server: ## Start server VM only
	cd vagrant && vagrant up server

.PHONY: vagrant-up-agents
vagrant-up-agents: ## Start both agent VMs only
	cd vagrant && vagrant up agent-ubuntu agent-debian

.PHONY: vagrant-halt
vagrant-halt: ## Stop all Vagrant VMs
	cd vagrant && vagrant halt

.PHONY: vagrant-destroy
vagrant-destroy: ## Destroy all Vagrant VMs (WARNING: deletes VM disk state)
	cd vagrant && vagrant destroy -f

.PHONY: vagrant-status
vagrant-status: ## Show status of all Vagrant VMs
	cd vagrant && vagrant status

.PHONY: vagrant-validate
vagrant-validate: ## Validate the Vagrantfile syntax
	cd vagrant && vagrant validate

.PHONY: vagrant-provision
vagrant-provision: ## Re-run provisioning scripts on all VMs (no destroy)
	cd vagrant && vagrant provision

.PHONY: vagrant-reload
vagrant-reload: ## Restart all Vagrant VMs
	cd vagrant && vagrant reload

.PHONY: proto-lint
proto-lint: ## Lint proto files with buf
	buf lint

.PHONY: proto-generate
proto-generate: ## Generate Go code from proto files using protoc
	@mkdir -p internal/proto
	protoc \
	  --proto_path=.proto \
	  --go_out=internal/proto \
	  --go_opt=paths=source_relative \
	  --go-grpc_out=internal/proto \
	  --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
	  pheromone/v1/agent_registry.proto \
	  pheromone/v1/twin_control.proto \
	  pheromone/v1/telemetry.proto \
	  pheromone/v1/action_events.proto

.PHONY: clean
clean: ## Clean build artifacts
	go clean
	rm -rf dist/ build/

.PHONY: config-validate-server
config-validate-server: build ## Validate server configuration (uses PHEROMONE_SERVER_CONFIG_PATH or /etc/pheromone/server)
	./dist/pheromone-server config validate

.PHONY: config-validate-agent
config-validate-agent: build ## Validate agent configuration (uses PHEROMONE_AGENT_CONFIG_PATH or /etc/pheromone/agent)
	./dist/pheromone-agent config validate

.PHONY: config-generate-server
config-generate-server: build ## Generate default server configuration to ./dist/config/server/
	mkdir -p dist/config/server
	./dist/pheromone-server config generate --format yaml --component all --config-path dist/config/server

.PHONY: config-generate-agent
config-generate-agent: build ## Generate default agent configuration to ./dist/config/agent/
	mkdir -p dist/config/agent
	./dist/pheromone-agent config generate --format yaml --component all --config-path dist/config/agent

.PHONY: fmt
fmt: ## Format Go code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: fmt vet ## Run linters

.PHONY: security-scan
security-scan: govulncheck gosec ## Run all security scans (govulncheck + gosec)

.PHONY: govulncheck
govulncheck: ## Check Go dependencies for known vulnerabilities (requires govulncheck)
	@command -v govulncheck >/dev/null 2>&1 || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

.PHONY: gosec
gosec: ## Run SAST security linter (requires gosec)
	@command -v gosec >/dev/null 2>&1 || go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -fmt sarif -out gosec-results.sarif ./... || gosec ./...

.PHONY: all
all: deps build test ## Install deps, build, and test

# Feature Specification: AI Agent-Based Digital Twin Mesh Configuration Management Platform (Pheromone)

**Feature Branch**: `feature/ADR-001-digital-twin-platform`
**Created**: 2026-02-18
**Status**: Draft
**Derived From**: Constitution v2.0.0, ADR framework, Summary.md (1:Many model)
**Input**: "Build out the AI agent-based digital twin mesh configuration management tool 'Pheromone' including research for available open source solutions for agents, protocols, central server, message queue and process management"

---

## User Scenarios & Testing

### User Story 1 - Infrastructure Operator: Unified Twin View of System State (Priority: P1)

An infrastructure operator needs a single, real-time view of both OS-level (infrastructure) and workload-level (application) twins across a fleet of heterogeneous nodes to make informed resource allocation decisions.

**Why this priority**: Without unified visibility, operators cannot respond to system state changes in real-time. This is foundational to the entire platform's value proposition (1:Many model).

**Independent Test**: Can be fully tested by deploying agents to 3+ nodes, syncing their state to a management server, and displaying/querying the unified twin state via a CLI or API. Delivers immediate ops visibility.

**Acceptance Scenarios**:
1. **Given** an agent running on a node with OS and workload twins registered, **When** querying the management server for current system state, **Then** the server returns accurate OS metrics (CPU, memory, disk) and workload metrics (service status, request count)
2. **Given** multiple agents across different nodes, **When** requesting a topology view, **Then** the system displays hierarchical relationships between twins (OS → Workloads)
3. **Given** a twin state change on an agent, **When** checking the server's view within 1 second, **Then** the change is reflected

---

### User Story 2 - Configuration Manager: Push Configuration to Clusters via Twin Models (Priority: P1)

A configuration manager defines desired state in a central twin model and needs to push configuration changes atomically to all matching nodes (1:Many pattern), with rollback capability.

**Why this priority**: Configuration management at scale is the core use case. Without this, the platform is just monitoring.

**Independent Test**: Can be fully tested by defining an OS-level twin model (e.g., "production-web-servers"), pushing a configuration change (e.g., security patch version), and verifying all matching agents enforce it. Delivers immediate config governance.

**Acceptance Scenarios**:
1. **Given** a defined digital twin model with constraints (e.g., "all nginx servers with kernel ≥5.15"), **When** pushing a new configuration payload, **Then** only matching agents receive and apply it
2. **Given** a configuration push to 100 nodes, **When** 95 succeed and 5 timeout, **Then** the system rolls back all 95 and reports partial failure with detailed logs
3. **Given** a configuration applied, **When** an agent reports non-compliance, **Then** the management layer flags it and suggests remediation

---

### User Story 3 - Platform Architect: Design Twin Hierarchy with Observability Telemetry (Priority: P1)

A platform architect defines the structure of OS-level and workload-level twins once, and all agents autonomously report metrics following that structure with full traceability for debugging.

**Why this priority**: The layered architecture (Principle I of constitution) is foundational. Without this, scalability and observability collapse.

**Independent Test**: Can be fully tested by defining a twin schema (e.g., OS metrics: {cpu, memory, disk}; Workload metrics: {requests/sec, p99_latency}), deploying agents, and verifying structured logs contain all defined fields. Delivers architecture consistency.

**Acceptance Scenarios**:
1. **Given** a twin schema defined in the management layer, **When** an agent starts, **Then** it understands and reports only schema-conformant metrics
2. **Given** a metric reported by an agent, **When** tracing through to observability backend (logs/metrics), **Then** all traces include: agent-id, twin-id, metric-name, timestamp, value
3. **Given** a schema update (adding new metric field), **When** deployed, **Then** agents gracefully add field without breaking existing pipelines

---

### User Story 4 - Developer: Deploy Python/Go/Rust Agent Code with Minimal boilerplate (Priority: P2)

A developer writes lightweight custom agent code (Python/Go/Rust) and needs a standardized lifecycle hook interface (init, metrics-collect, enforce-config, shutdown) with auto-registration to management server.

**Why this priority**: Enables extensibility. While core agents are built-in, custom agents are competitive advantage.

**Independent Test**: Can be fully tested by writing a minimal custom agent (50 lines), deploying it, and verifying it registers to the server and reports metrics. Demonstrates framework maturity.

**Acceptance Scenarios**:
1. **Given** an agent template (scaffolding), **When** implementing lifecycle hooks, **Then** the agent auto-connects to the management server without manual config
2. **Given** a custom agent deployed, **When** checking observability, **Then** metrics are automatically tagged with agent-type and custom metadata
3. **Given** a malformed custom agent, **When** CI/CD validates it, **Then** smoke tests fail with actionable error messages

---

### User Story 5 - DevOps Team: Multi-Tenant Twin Namespacing with RBAC (Priority: P3)

A DevOps team manages multiple environments (dev, staging, prod) and needs twin models scoped to environments with role-based access control (only staging team can modify staging twins).

**Why this priority**: Critical for enterprise adoption; can be achieved post-MVP with proper schema design.

**Independent Test**: Can be fully tested by creating isolated namespaces, assigning roles, and verifying a user without prod access cannot query/modify prod twins.

**Acceptance Scenarios**:
1. **Given** a user with staging-operator role, **When** attempting to modify a prod twin, **Then** the system rejects the operation with RBAC error
2. **Given** a twin scoped to "staging" namespace, **When** querying from prod-operator, **Then** it is not visible/accessible

---

### Edge Cases

- What happens when an agent loses connection to the management server (network partition)? → Agent MUST buffer metrics locally and replay on reconnection.
- How does the system handle clock skew between agents and server? → Timestamps MUST be validated; out-of-range timestamps logged but not rejected (graceful degradation).
- What if a twin model is updated while agents are mid-enforcement? → In-flight enforcement uses previous model version; next sync uses new model (no atomicity guarantee across cluster for model changes).
- How does the system scale to 10,000+ agents? → Kafka message queue is MUST for telemetry streaming; gRPC for control plane; tested via load testing (see Success Criteria).

---

## Requirements

### Functional Requirements

**Architecture & Twin Model**:
- **FR-001**: System MUST support a 1:Many digital twin model where one twin definition can be enforced across multiple matching agents
- **FR-002**: System MUST implement two twin categories: **OS-Level Twins** (infrastructure metrics/configs) and **Workload-Level Twins** (application services/configs)
- **FR-003**: System MUST enable hierarchical twin composition (e.g., a "Database Server" twin contains OS twin + PostgreSQL workload twin)
- **FR-004**: System MUST support twin versioning (tracking history of config changes with timestamps and rollback capability)

**Agent Communication & Control Plane**:
- **FR-005**: All agents MUST communicate with management server via gRPC bidirectional streaming for commands and metrics
- **FR-006**: Agents MUST support both pull-based (server → agent: "collect metrics XYZ") and push-based (agent → server: "I have metrics ABC") communication patterns
- **FR-007**: Management server MUST expose RESTful or GraphQL API for CLI/UI queries (gRPC used internally between agents/server)
- **FR-008**: System MUST implement automatic agent discovery and registration (agents announce themselves; server tracks active fleet)

**Observability & Metrics**:
- **FR-009**: All components MUST emit structured logs (JSON format) with trace-id, component-id, log-level, message, timestamp
- **FR-010**: Agents MUST collect and report metrics in OpenMetrics format (Prometheus-compatible)
- **FR-011**: Management server MUST support export to stdout/files and integration with Prometheus/Datadog/similar (via Kafka streaming)
- **FR-012**: System MUST provide query interface for historical metrics and logs (e.g., "show metrics for host-123 from 2 hours ago")

**Configuration Enforcement**:
- **FR-013**: Management server MUST support defining desired-state twin models (YAML/JSON) with constraints (e.g., "all prod-web-servers have nginx ≥1.21")
- **FR-014**: System MUST support atomic pushing of configuration to all matching agents (1:Many enforcement) with rollback on partial failure
- **FR-015**: Agents MUST report enforcement status (success/partial/failed) with detailed error logs
- **FR-016**: System MUST track configuration drift (detected when agent reports non-compliance with expected state)

**Protocol & Framework Choices** (from ADR-001):
- **FR-017**: Primary inter-component protocol MUST be gRPC with Protocol Buffers schema (backward-compatible versioning required)
- **FR-018**: Telemetry aggregation MUST use Kafka or equivalent message queue for high-volume metric streaming
- **FR-019**: Optional pub/sub protocol (MQTT) MAY be supported for low-bandwidth agent scenarios (but NOT primary)
- **FR-020**: System MUST support both Go and Rust implementations; first implementation in Go for rapid prototyping

**Testing & Quality** (from Constitution Principle IV):
- **FR-021**: All agent/server code MUST include unit tests (Go: `go test`, Rust: `cargo test`) with ≥70% coverage
- **FR-022**: System MUST include integration tests validating gRPC contracts between agent and server
- **FR-023**: Pre-commit hooks MUST enforce linting (golangci-lint, clippy), formatting (gofmt, rustfmt), and basic type checking
- **FR-024**: CI/CD MUST run smoke test suite on every PR; failures are BLOCKING

**ADR & Decision Traceability** (from Constitution Principle II):
- **FR-025**: Every architectural decision MUST be documented in `./docs/adr/adr-NNN-*.md` files using Michael Nygard template
- **FR-026**: Feature branches MUST reference ADR number (e.g., `feature/ADR-001-platform`); PRs MUST link to ADR
- **FR-027**: System architecture diagrams MUST trace back to ADRs explaining design rationale

---

### Key Entities

**OS-Level Twin**: Represents operating system state and configuration
- Attributes: host_id, os_type (linux/windows/macos), kernel_version, cpu_count, memory_bytes, disk_partitions, installed_packages
- Relationships: "contains" workload-level twins; "reports-to" management layer

**Workload-Level Twin**: Represents application/service state and configuration
- Attributes: service_name, service_type (web/db/cache), version, status (running/stopped/degraded), port, dependencies (list of other workload twins)
- Relationships: "runs-on" OS twin; "reports-to" management layer

**Management Server**: Orchestrates fleet of agents and enforces twin models
- Attributes: server_id, active_agents (count), twin_model_definitions (list), configuration_history (log of changes)
- Relationships: "commands" agents; "syncs" with agents

**Configuration Model**: Desired state for a group of twins (1:Many pattern)
- Attributes: model_id, name, selectors (matching criteria), desired_config (YAML/JSON), version, created_at, last_deployed_at
- Relationships: "targets" matching twins; "has-versions" (history)

**Agent**: Autonomous process running on a node, manages local twins and reports to server
- Attributes: agent_id, agent_type (native/custom), os_twins (list), workload_twins (list), last_heartbeat_time, connection_status
- Relationships: "manages" twins; "connects-to" management server; "emits" metrics/logs

---

## Success Criteria

### Measurable Outcomes

**Functional Completeness** ✓T:
- **SC-001**: Platform MUST support defining and querying a twin model for 5+ distinct OS/workload types (Linux servers, containers, databases, web services, caches)
- **SC-002**: Management server MUST successfully push configuration to 100+ agents in <5 seconds (measured end-to-end from submission to agent ACK)
- **SC-003**: System MUST handle 10,000 concurrent agents reporting metrics without packet loss or >1% error rate

**Observability & Debuggability**:
- **SC-004**: 100% of metrics from agents MUST include trace-id enabling end-to-end tracing from agent collection through server → observability backend
- **SC-005**: All error scenarios MUST produce actionable error messages (not just "failed" but "failed because config schema mismatch: expected version=2, got version=1")
- **SC-006**: Platform MUST expose query interface allowing operators to answer "what is the current state of twin X across all agents?" in <2 seconds

**Scalability**:
- **SC-007**: Management server MUST handle 10,000+ agents with a single server instance (state management via etcd/consul for HA)
- **SC-008**: Kafka/message queue throughput MUST sustain 100,000 metrics/second without backlog growth

**Quality & Developer Experience**:
- **SC-009**: New custom agent implementation (Developer Story 4) MUST be deployable in <1 hour by a developer unfamiliar with codebase
- **SC-010**: Platform MUST provide CLI tool for querying twins, submitting configs, viewing agent status (feature parity with API)
- **SC-011**: Documentation MUST include runnable examples (5+ complete scenarios) for each use case

**Resilience & Operations**:
- **SC-012**: Agent connectivity loss MUST NOT result in data loss; buffered metrics MUST be replayed on reconnection
- **SC-013**: Management server failure MUST NOT prevent agents from continuing to enforce last-known configuration; agents MUST resume reporting once server recovers
- **SC-014**: Configuration rollback (revert to previous version) MUST be completable in <30 seconds operator time

---

## Open Source Solutions Research

### Agent Frameworks & Implementations

#### Candidate 1: Telegraf (InfluxData)
- **Purpose**: Lightweight metrics collector agent
- **Pros**: Widely adopted, 300+ plugins (OS metrics, services, custom), plugin ecosystem mature
- **Cons**: Primarily metrics-focused; lacks native config enforcement; distributed as daemon (not library)
- **Fit for Pheromone**: Could be adapted for OS-level metrics collection (FR-010, FR-012) but insufficient for full twin modeling and config enforcement (FR-013 to FR-016)
- **Recommendation**: **USE AS REFERENCE** for metrics schema and collection patterns; not as primary agent

#### Candidate 2: Fluent Bit (CNCF)
- **Purpose**: Edge log processor and forwarder
- **Pros**: Lightweight (C-based), multi-output support (S3, HTTP, Kafka), used in Kubernetes
- **Cons**: Log-focused; no config enforcement or twin state tracking
- **Fit for Pheromone**: Complementary for observability pipeline (FR-011) but not core agent
- **Recommendation**: **USE FOR** log aggregation to Kafka/observability backend; not as primary agent framework

#### Candidate 3: Custom Go Agent (Lightweight)
- **Purpose**: Purpose-built agent framework for Pheromone
- **Pros**: Full control over twin lifecycle, gRPC integration native, Go standard library + minimal deps, fast prototyping
- **Cons**: Requires building/maintaining own lifecycle, discovery, resilience
- **Fit for Pheromone**: **PRIMARY** implementation aligns with ADR-001 (Go preferred, gRPC native, supports 1:Many pattern)
- **Recommendation**: **BUILD CUSTOM** with following architecture:
  ```
  - Agent core (lifecycle: init → collect → enforce → sync → shutdown)
  - Twin model interface (OS/Workload twin abstraction)
  - gRPC client for server communication
  - Metrics exporter (OpenMetrics format)
  - Local config store (SQLite/JSON for resilience)
  ```

#### Candidate 4: Rust Agent Implementation (Future)
- **Purpose**: Performance-critical agents (memory safety, low latency)
- **Pros**: Memory-safe, excellent for systems-level metric collection (via /proc, /sys parsing)
- **Cons**: Longer development timeline
- **Fit for Pheromone**: **FUTURE** (Phase 2, Post-MVP)
- **Recommendation**: Establish Rust agent framework scaffolding in repos; implement Go agent first

---

### Protocols & Communication

#### Candidate 1: gRPC + Protocol Buffers
- **Purpose**: High-performance RPC framework with schema-driven communication
- **Pros**: Bidirectional streaming, HTTP/2 binary efficient, automatic code gen, backpressure handling
- **Cons**: Binary protocol (harder debug without tools); requires proto schema management
- **Fit for Pheromone**: **PRIMARY** for agent ↔ management server (FR-005, FR-006, FR-017)
- **Recommendation**: **MANDATORY** for control plane; use proto schema versioning (pheromone.v1.agent.proto, etc.)

#### Candidate 2: MQTT (Eclipse)
- **Purpose**: Lightweight pub/sub protocol for IoT/edge
- **Pros**: Ultra-low bandwidth, simple pub/sub model
- **Cons**: Publish-only from agent (no request/response RPC easily); single broker single-point-of-failure
- **Fit for Pheromone**: **OPTIONAL** (FR-019) for low-bandwidth scenarios (edge devices); not primary
- **Recommendation**: Research MQTT as **FALLBACK** for agents with tight bandwidth constraints; not MVP

#### Candidate 3: REST API (Supplementary)
- **Purpose**: Human-friendly queries and configuration submission
- **Pros**: Standard HTTP, easy CLI integration (curl), cacheable
- **Cons**: Binary payload inefficient; lacks streaming
- **Fit for Pheromone**: **SUPPLEMENTARY** (FR-007) for operator CLI/UI; not agent communication
- **Recommendation**: Wrap gRPC server with REST gateway (protoc-gen-grpc-gateway for automatic translation)

---

### Central Server & State Management

#### Candidate 1: etcd (CNCF)
- **Purpose**: Distributed key-value store for shared configuration
- **Pros**: Strong consistency, HA via Raft, built-in lease/watch mechanism, widely adopted (Kubernetes uses it)
- **Cons**: Requires external deployment; eventual consistency not suitable for all use cases
- **Fit for Pheromone**: **PRIMARY** for twin model definitions and configuration (FR-013, FR-016; supports versioning)
- **Recommendation**: **USE etcd** as primary store for DesiredState (twin models, config); watch etcd for changes to push/notify agents

#### Candidate 2: Consul (HashiCorp)
- **Purpose**: Service mesh + distributed configuration
- **Pros**: Built-in service discovery, health checking, HTTP API, multi-datacenter support
- **Cons**: Heavier than etcd; more complex
- **Fit for Pheromone**: **ALTERNATIVE** if multi-DC or service mesh required
- **Recommendation**: **EVALUATE** for Phase 2 (multi-DC support)

#### Candidate 3: PostgreSQL/SQLite + Custom State Engine
- **Purpose**: Relational database for state
- **Pros**: ACID transactions, mature tooling, SQL queryability
- **Cons**: Polling required for change notification (no watch mechanism); harder to scale
- **Fit for Pheromone**: **NOT RECOMMENDED** for primary state store (gRPC bidirectional push better)
- **Recommendation**: USE PostgreSQL as **SECONDARY** for audit logs and configuration history (immutable log)

#### Candidate 4: Custom In-Memory Server (Go)
- **Purpose**: Lightweight state engine for MVP
- **Pros**: Full control, minimal dependencies, fast
- **Cons**: No HA/persistence without external store; requires custom implementation
- **Fit for Pheromone**: **HYBRID APPROACH**: Build custom state engine in Go + persist to etcd for production
- **Recommendation**: **PROTOTYPE** with in-memory state (MVP); UPGRADE to etcd-backed for production

**Recommended Hybrid Stack for MVP**:
```
┌─────────────────────┐
│   Operator CLI      │
│   (gh CLI + REST)   │
└──────────┬──────────┘
           │
    ┌──────▼──────┐
    │ Management  │
    │   Server    │
    │   (Go)      │
    │ State:      │
    │ in-memory   │
    │ + etcd      │
    └──────┬──────┘
      ┌────┴────┬────────┬─────────┐
      │   gRPC  │  gRPC  │  gRPC   │
      ▼         ▼        ▼         ▼
   Agent-1   Agent-2  Agent-3   Agent-N
   [Linux]  [Linux]  [Linux]   [macOS]
```

---

### Message Queue & Telemetry Aggregation

#### Candidate 1: Apache Kafka
- **Purpose**: Distributed event streaming platform
- **Pros**: High throughput (1M+ msgs/sec), persistent (replay), consumer groups, widely adopted for observability
- **Cons**: Operational complexity (ZooKeeper deprecated, requires Kraft mode); bit overkill for MVP
- **Fit for Pheromone**: **RECOMMENDED** for production telemetry (FR-011, SC-008); higher scale
- **Recommendation**: **DEFER to Phase 2** (post-MVP scale-out); use simple fanout initially

#### Candidate 2: RabbitMQ
- **Purpose**: Message broker with routing
- **Pros**: Reliable delivery, flexible routing (topic/fanout/direct), easier than Kafka
- **Cons**: Less throughput optimization than Kafka
- **Fit for Pheromone**: **GOOD** for MVP telemetry aggregation (FR-011)
- **Recommendation**: **EVALUATE** for MVP; simpler than Kafka

#### Candidate 3: NATS (Synadia)
- **Purpose**: Cloud-native messaging
- **Pros**: Lightweight, single binary, pub/sub + load-balanced queues, low latency
- **Cons**: Fewer operational tools than Kafka/RabbitMQ
- **Fit for Pheromone**: **GOOD** for MVP (cloud-native, matches pheromone spirit)
- **Recommendation**: **USE NATS** for MVP telemetry; easy migration path to Kafka later

#### Candidate 4: Direct HTTP/Webhook Push
- **Purpose**: Simple metric export without intermediate broker
- **Pros**: One fewer component to operate; simple webhook integration
- **Cons**: No load shedding/queueing; backpressure issues
- **Fit for Pheromone**: **NOT RECOMMENDED** for production scale
- **Recommendation**: **USE FOR MVP** (server → Prometheus direct push); upgrade to NATS/Kafka for prod

**Recommendation for Telemetry Path**:
```
MVP:
Agent --gRPC--> Server --HTTP push--> Prometheus
                    └──> SQLite (audit log)

Phase 2:
Agent --gRPC--> Server --NATS/Kafka--> Prometheus/Datadog/ClickHouse
                    │
                    └──> PostgreSQL (audit + analytics)
```

---

### Process Management & Deployment

#### Candidate 1: systemd (Linux)
- **Purpose**: Init system for process lifecycle
- **Pros**: Standard on Linux; service restart/restart-on-failure
- **Cons**: Linux-only; agent running as system service (not ideal for custom agents)
- **Fit for Pheromone**: **USE for** native agent distribution (systemd unit file)
- **Recommendation**: **PROVIDE systemd unit** as standard deployment; support custom scripts

#### Candidate 2: Docker + Container Orchestration
- **Purpose**: Containerized agent deployment
- **Pros**: Portable, Cloud-native standard, auto-restart, resource limits
- **Cons**: Additional layer of abstraction
- **Fit for Pheromone**: **PRIMARY** deployment for containerized workloads; agent can run as sidecar
- **Recommendation**: **PROVIDE Dockerfile** for agent; support sidecar patterns (Kubernetes)

#### Candidate 3: Systemd + User Units
- **Purpose**: Run custom agents as unprivileged user processes
- **Pros**: Better isolation, easier testing
- **Cons**: More limited metrics (can't access privileged files)
- **Fit for Pheromone**: **OPTIONAL** for workload-level agents
- **Recommendation**: **SUPPORT** for custom workload agents

#### Candidate 4: Supervisor / Circus
- **Purpose**: Process monitoring and restart
- **Pros**: Cross-platform, lightweight
- **Cons**: Less modern than systemd
- **Fit for Pheromone**: **NOT RECOMMENDED** (prefer systemd/Docker)

**Recommendation**:
```
Primary Agent Deployment:
- Linux (systemd) → native service
- macOS (launchd) → plist file
- Container → Docker sidecar or daemonset

Custom Agent Deployment:
- Go: compile + run as systemd service
- Python: venv + systemd service
- Rust: compile + run, supports same systemd approach
```

---

## Assumptions & Decisions

### Design Assumptions

1. **Assumption**: Initial deployment targets Linux (primary) + macOS (developer testing); Windows deferred to Phase 2
   - Rationale: Linux dominates server market; Go/Rust excellent on both; Windows adds platform-specific complexity

2. **Assumption**: MVP focuses on ~100-1000 agents; >10K agents requires Kafka + horizontal scaling (Phase 2)
   - Rationale: Single Go server can handle 1K+ concurrent gRPC connections; etcd HA for persistence

3. **Assumption**: Configuration updates are NOT atomic across all agents (eventual consistency acceptable)
   - Rationale: Distributed systems principle; true atomicity requires coordination overhead; monitoring for drift suffices

4. **Assumption**: Twin models are defined by operators (YAML/JSON); system does NOT auto-generate from infrastructure
   - Rationale: Keeps initial scope focused; auto-discovery is Phase 2 feature

5. **Assumption**: Observability tooling (Prometheus, Datadog, ELK) is externally provided; Pheromone exports data only
   - Rationale: Pheromone focus is twin management, not full observability; leverage existing tools

### Technology Stack (Recommended)

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Agent | Go | Fast prototyping, minimal runtime, native gRPC |
| Management Server | Go + etcd | Matched language, distributed state, HA |
| Protocol | gRPC + Protocol Buffers | Bidirectional streaming, schema versioning, IDL-driven |
| State Store | etcd | Distributed configuration, watch mechanism for change notification |
| Telemetry | OpenMetrics → Prometheus/HTTP | Standard format, easy integration with observability tools |
| Messages | NATS (MVP) → Kafka (Phase 2) | Lightweight start, scalable upgrade path |
| Deployment | systemd (Linux), Docker (containers) | Standard, minimal dependencies |
| Testing | Go testing + integration tests | Native test framework, gRPC contract tests |
| CI/CD | GitHub Actions | GitHub-native, pre-commit hooks for lint/test |

---

## Next Steps (ADR Required Before Implementation)

Before feature implementation begins, the following ADRs MUST be created and accepted:

1. **ADR-002**: Choose between in-memory server + etcd persistence vs. pure etcd-backed server (Phase 1 architecture)
2. **ADR-003**: Define gRPC service contracts (protobuf schemas) for agent ↔ server communication
3. **ADR-004**: Define twin model schema (YAML format for OS/Workload twin definitions)
4. **ADR-005**: Choose message queue (NATS vs. Kafka) and metrics export format
5. **ADR-006**: Define agent lifecycle interface (hooks for custom agent implementations)

These ADRs will feed into the Phase 2 specification phase and implementation tasks.

---


# ADR 003: gRPC Service Contracts - Agent ↔ Server Communication Protocol

## Status
Proposed

## Context

Pheromone agents and management server must communicate via gRPC (per ADR-001, Constitution Principle III). The system requires:

1. **Control Operations**: Server → Agent (push config, collect metrics, enforce state)
2. **Telemetry Streaming**: Agent → Server (continuous metrics, logs, status updates)
3. **Discovery & Registration**: Agent announces itself; server learns fleet topology
4. **Request/Response Patterns**: Agent queries server for desired state; server queries agent for actual state

**Current Constraints**:
- gRPC v1.x stability required (Protocol Buffers v3)
- Bidirectional streaming for metrics (FR-006, FR-010)
- Request/response for configuration (FR-013, FR-014)
- Support multiple agent types (native Go, custom Python/Rust) with same wire contract (FR-020)
- Version management required (ADR-001 decision: backward-compatible unless explicit breaking change)

## Decision

**Define Three gRPC Services**:

### 1. **AgentRegistry Service** (Request/Response)
Agents register themselves; server discovers and subscribes to heartbeats.

```proto
service AgentRegistry {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
}

message RegisterRequest {
  string agent_id = 1;
  string agent_type = 2;  // "native-go", "native-python", "custom-rust"
  string hostname = 3;
  repeated string twin_ids = 4;  // OS twins and workload twins managed by this agent
  map<string, string> metadata = 5;  // custom key-value pairs (e.g., "version": "v1.0.0")
}

message RegisterResponse {
  string server_id = 1;
  bool accepted = 2;
  string config_version = 3;  // version of desired config server has for this agent
}
```

### 2. **TwinControl Service** (Bidirectional Streaming)
Server pushes twin configuration; agent streams desired/actual state and enforcement status.

```proto
service TwinControl {
  rpc SyncTwinState(stream TwinSyncRequest) returns (stream TwinSyncResponse);
}

message TwinSyncRequest {
  string agent_id = 1;
  repeated Twin actual_state = 2;  // agent reports current state
  repeated EnforcementStatus enforcement_status = 3;  // compliance with desired config
}

message TwinSyncResponse {
  string server_id = 1;
  repeated ConfigAction actions = 2;  // server command (enforce config, collect metrics, etc.)
  string desired_config_version = 3;  // version of desired state server is pushing
}
```

### 3. **TelemetryStream Service** (Bidirectional Streaming)
Agent streams metrics and logs; server acknowledges and batches for export.

```proto
service TelemetryStream {
  rpc StreamMetrics(stream MetricsRequest) returns (stream MetricsResponse);
}

message MetricsRequest {
  string agent_id = 1;
  string trace_id = 2;
  repeated Metric metrics = 3;  // OpenMetrics format serialized
  repeated LogEntry logs = 4;  // structured logs
}

message MetricsResponse {
  string server_id = 1;
  int32 acked_count = 2;  // number of metrics received
  bool ready_for_next = 3;  // backpressure signal
}
```

### Versioning Strategy

- **Package Versioning**: `pheromone.v1`, `pheromone.v2` (breaking changes increment major version)
- **Message Evolution**: New fields are optional and default-valued (backward compatible)
- **Service Expansion**: Additive-only (new service versions coexist, old retired after deprecation period)
- **Rollback**: Agents/servers can pin `pheromone.v1` during upgrades

## Consequences

### Positive
- Clear separation of concerns: control (AgentRegistry, TwinControl) vs. telemetry (TelemetryStream)
- Bidirectional streaming enables pub/push capability (FR-006)
- Protocol Buffers provide efficient serialization + code generation
- Version management straightforward (separate .proto files per version)
- Enables custom agent implementations (Python, Rust) with same wire contracts

### Negative
- Three services ≈ three long-lived connections per agent (memory overhead, but acceptable for <10K agents)
- Requires careful state synchronization on service reconnections (agent must replay missing state)
- gRPC binary protocol harder to debug than REST (requires gRPC CLI tools)
- Message schema changes require careful migration (no dynamic schema evolution)

### Testing Requirements
- **Integration Tests**: Agent ↔ Server bidirectional streaming with message loss/delay
- **Contract Tests**: Verify message serialization/deserialization for all agent types
- **Backward Compatibility**: Old agent version must work with new server version (and vice versa for 1 release cycle)

## Implementation Details

- **Tooling**:
  - `protoc-gen-go` for Go code generation
  - `protoc-gen-go-grpc` for gRPC service stubs
  - `buf` for proto linting and version management
  - gRPC Interceptors for logging, metrics, auth (Phase 2)

- **Error Handling**:
  - gRPC status codes: UNAVAILABLE (server down), INVALID_ARGUMENT (bad request), PERMISSION_DENIED (RBAC - Phase 3)
  - Detailed error messages in status.message (includes troubleshooting guidance)

- **Proto File Location**: `.proto/pheromone/v1/*.proto` (standard layout)

## Follow-Up ADRs

- **ADR-004**: Twin model schema (defines structure of Twin message used in TwinControl)
- **ADR-005**: Telemetry export (how metrics from TelemetryStream are exported to Prometheus/NATS)

## References

- gRPC Protocol Buffer Specification v3: https://developers.google.com/protocol-buffers/docs/proto3
- gRPC Best Practices: https://grpc.io/docs/guides/performance-best-practices/
- Spec-001, FR-005, FR-006, FR-007, SC-002
- Constitution Principle III (Protocol foundation)

---

**Decision Date**: 2026-02-18  
**Status Update**: Proposed (pending gRPC proto schema review)

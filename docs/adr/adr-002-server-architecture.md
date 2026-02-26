# ADR 002: Server Architecture - In-Memory with etcd Persistence vs. Pure etcd-Backed

## Status
Accepted

## Context

Pheromone's management server needs to store and manage digital twin models, agent state, and configuration history. Two primary architectural approaches have been identified:

1. **Hybrid Approach**: In-memory server with periodic/event-triggered etcd persistence
   - Fast reads/writes for twin state and configuration
   - etcd used as backing store for recovery and multi-instance HA
   - Simpler initial implementation (MVP-friendly)
   - Requires change notification mechanism (etcd watch)

2. **Pure etcd-Backed Approach**: All state directly in etcd
   - Strong consistency guarantees out-of-box
   - Simpler architecture (no dual state concerns)
   - Slower latency for frequent queries (etcd watch patterns)
   - Better for eventual distributed deployment

**Constraints**:
- MVP must support ~100-1000 agents with sub-5-second configuration push latency (FR-002, SC-002)
- Must support high-volume metric queries without blocking agent communication (FR-012, SC-006)
- Phase 1 single-server deployment; Phase 2 multi-server HA

## Decision

**Choose: Hybrid Approach (In-Memory + etcd Persistence)**

The management server will maintain twin and configuration state in-memory for performance, with etcd as the distributed backing store for durability and multi-instance synchronization.

### Rationale

1. **Performance**: In-memory queries enable sub-5-second config push and <2-second operator queries (SC-002, SC-006)
2. **Consistency Model**: Agent changes written to in-memory immediately; synced to etcd asynchronously (eventual consistency acceptable per Architecture Assumptions)
3. **Scalability**: Single server handles 1K+ agents; etcd watch mechanism notifies peers of changes (supports Phase 2 multi-server)
4. **Development Speed**: MVP iteration faster with direct in-memory mutations; etcd integration straightforward later
5. **State Recovery**: On server restart, load persisted state from etcd (no data loss)

### Implementation Details

- **State Layers**:
  - Layer 1 (Fast): In-memory map[string]Twin in Go
  - Layer 2 (Durable): etcd /pheromone/twins/* keyspace (CDC pattern)
  - Layer 3 (History): PostgreSQL `twin_audit_log` table (Phase 2)

- **Change Flow**:
  1. Agent connects → server updates in-memory twin state
  2. In-memory state mutation triggers etcd write (via transactional batch)
  3. etcd watch notifies other servers (Phase 2 multi-server sync)
  4. Audit log appended asynchronously (Phase 2)

- **Failure Handling**:
  - Server crash: Restart loads from etcd via range query
  - etcd unavailability: Server continues in-memory (degraded mode); syncs on recovery
  - Agent disconnect: In-memory twin marked "stale"; etcd reflects this

## Consequences

### Positive
- MVP delivers sub-2-second query latency (enables user story 1 and SC-006)
- Configuration push achieves <5 second target (enables user story 2 and SC-002)
- Single codebase for in-memory + persistent logic (simpler than pure etcd patterns)
- Natural upgrade path to multi-server via etcd state sharing (Phase 2)

### Negative
- Potential for in-memory ↔ etcd divergence if sync fails (requires monitoring)
- More complex error handling (etcd unavailability fallback paths)
- Not immediately suitable for >10K agents (Phase 2 requires Kafka for telemetry, load-balancing)
- Requires explicit cache invalidation logic if etcd updated externally

### Timeline
- Phase 1 (MVP): In-memory + etcd for single server
- Phase 2 (Scale): Multi-server with etcd as distributed cache + Kafka for metrics

## Follow-Up ADRs

- **ADR-003**: gRPC service contracts (impacts in-memory twin struct definitions)
- **ADR-004**: Twin model schema (format for storing in both in-memory + etcd)
- **ADR-005**: Message queue selection (telemetry path separate from control plane state)

## Update — 2026-02-20

The server architecture must additionally support **agentic AI agents** (ADR-007). The following capabilities MUST be added to the hybrid server design:

1. **Agent Capability Registry**: Extend in-memory twin state to include per-agent capability records (AI model, skill versions, `ai_reasoning_enabled` flag). Persisted to etcd under `/pheromone/agents/{agent_id}/capabilities`.
2. **Action Proposal Queue**: In-memory queue for `ProposeAction` RPC calls from agents; backed by etcd for durability. Supports human-in-the-loop approval workflows.
3. **AI Telemetry Path**: `TelemetryStream` ingestion extended to accept AI decision trace payloads alongside metrics/logs. Decision traces stored in PostgreSQL audit log (Phase 2) or exported via NATS.
4. **Skill-Aware Config Push**: Server checks agent's `skills` capability list before pushing a twin configuration; avoids pushing unsupported skill versions.

These additions do not change the hybrid in-memory + etcd state management choice; they extend the state schema and gRPC service contracts (ADR-003 updated accordingly).

## References

- Spec-001, FR-002, FR-013, SC-002, SC-006
- ADR-007 (Agentic AI Agent Model — server capability extensions)
- Constitution Principle I (Layered architecture), Principle III (Protocol foundation)
- Related: etcd API 3.5 (range, watch, transaction operations)

---

**Decision Date**: 2026-02-18
**Status Update**: Accepted (2026-02-26 — all performance criteria validated with live etcd; see `docs/adr/adr-002-performance-validation.md`)

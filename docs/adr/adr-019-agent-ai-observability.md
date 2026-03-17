# ADR-019: Agent & AI Observability — Reasoning Traces, Twin Diff, Action Audit

**Status**: Accepted
**Date**: 2026-03-17
**Author**: @copilot
**Refs**: ADR-007 (Agentic AI Model), ADR-008 (AI Model Selection), ADR-017 (UI/API Gateway), ADR-018 (Observability Stack)

---

## Context

The `RuleBasedReasoner` (Tier 0) and future `OllamaReasoner` (Tier 1) implementations execute but emit no structured observability about what decisions they make, what actions they take, or what the outcome was. Operators have no visibility into the AI reasoning loop at runtime.

**Identified gaps (issue #66):**

1. **No AI reasoning trace** — observations → plan → actions → outcome lifecycle is invisible.
2. **No twin diff view** — desired state vs actual state is not surfaced as a structured delta via the REST API.
3. **No action audit log** — no per-agent history of the last N decisions.
4. **ADR-007 defines the reasoning loop** but specifies no telemetry hook.
5. **ADR-018 adds Prometheus metrics** (`pheromone_reasoning_decisions_total`) but counters alone cannot reconstruct individual decisions.

### Constraints

| Constraint | Value |
|---|---|
| New dependencies | None — implementation uses existing Go stdlib and in-memory data structures |
| NATS integration | Trace emission interface designed for future NATS JetStream (`traces.{agent_id}`) publishing without API changes |
| Backwards compatibility | All existing `/api/v1/*` paths and middleware chain preserved; two new read-only endpoints added |
| Resource overhead | Ring buffer capped at 100 traces per agent; `TraceStore` adds ≤ O(N) memory where N is number of agents |

---

## Decision

### 1. `ReasonerTrace` struct (`internal/skill/trace.go`)

A new `ReasonerTrace` struct captures the complete observability record for a single reasoning cycle:

| Field | Type | Description |
|-------|------|-------------|
| `Timestamp` | `time.Time` | When `Plan()` was called (UTC) |
| `AgentID` | `string` | Agent that performed the reasoning cycle |
| `Reasoner` | `string` | Implementation type: `"rule-based"`, `"ollama"`, etc. |
| `Input` | `TraceInput` | Twin ID, drift flag, and list of drifted field names |
| `Actions` | `[]string` | Planned action types (empty when no drift) |
| `Outcome` | `string` | `"no-drift"`, `"actions-planned"`, or `"error"` |
| `DurationMs` | `int64` | `Plan()` wall-clock time in milliseconds |
| `FallbackUsed` | `bool` | True when a higher-tier reasoner failed and rule-based fallback substituted |

### 2. `TraceEmitter` interface

```go
type TraceEmitter interface {
    Emit(trace ReasonerTrace)
}
```

`TraceEmitter` is an injectable dependency. The server wires `TraceStore` into `RuleBasedReasoner`. Future implementations can wire a NATS publisher that forwards traces to the `traces.{agent_id}` JetStream subject without changing the reasoner.

### 3. `TraceStore` — in-memory ring buffer (`internal/skill/trace.go`)

`TraceStore` is a bounded per-agent ring buffer (default capacity: 100). It satisfies `TraceEmitter` and is safe for concurrent access. When the buffer is full the oldest entry is evicted. Capacity is configurable via `NewTraceStore(n)`.

### 4. `RuleBasedReasoner` updated to emit traces

`RuleBasedReasoner` gains two exported fields:

- `AgentID string` — included in every emitted trace.
- `Emitter TraceEmitter` — if non-nil, a `ReasonerTrace` is published after every `Plan()` call. If nil, trace emission is skipped and behaviour is identical to the pre-ADR-019 implementation (zero overhead).

Existing usages of `NewRuleBasedReasoner()` continue to work without modification.

### 5. REST API endpoints

Two new read-only endpoints are added to the management server:

#### `GET /api/v1/agents/{id}/traces`

Returns the most recent reasoning traces for a specific agent (default: 10, max: 100 via `?limit=N`).

**Response** (array of `AgentTrace`):

```json
[
  {
    "timestamp": "2026-03-17T20:00:00Z",
    "agent_id": "agent-os-02",
    "reasoner": "rule-based",
    "twin_id": "twin-os-02",
    "has_drift": true,
    "drifted_fields": ["kernel"],
    "actions": ["apply-config"],
    "outcome": "actions-planned",
    "duration_ms": 0,
    "fallback_used": false
  }
]
```

#### `GET /api/v1/twins/{id}/diff`

Returns the field-by-field delta between desired and actual state for a twin.

**Response** (`TwinDiff`):

```json
{
  "twin_id": "twin-os-02",
  "has_drift": true,
  "fields": [
    {"field": "kernel", "desired": "6.1.0-28-amd64", "actual": "6.1.0-27-amd64", "drifted": true},
    {"field": "os", "desired": "Debian 12", "actual": "Debian 12", "drifted": false}
  ]
}
```

Both endpoints require authentication (subject to the existing auth middleware).

### 6. UI — trace viewer panel

The agent detail page (`showAgentDetail`) fetches traces in parallel with the agent resource and renders a **Recent Reasoning Traces** card showing the last 10 decisions:

- Twin ID and drift indicator badge (`drift` / `ok`)
- Reasoner type
- Drifted field names (tag list)
- Planned action types
- Outcome badge (`actions-planned` / `no-drift` / `error`)
- Relative timestamp
- Duration (when non-zero)

The twin detail page already renders desired vs actual state side-by-side; the new `/diff` endpoint provides a structured machine-readable version of the same comparison.

### 7. NATS trace publishing (deferred to Phase 2)

The `TraceEmitter` interface is designed so that a NATS publisher can be wired without modifying `RuleBasedReasoner`. When ADR-005 (NATS JetStream) is fully activated, a `NATSTraceEmitter` implementing `Emit(ReasonerTrace)` can publish to subject `traces.{agent_id}` — the in-memory `TraceStore` can optionally remain wired alongside it for UI queries.

---

## Consequences

### Positive

- Every reasoning cycle now produces a structured trace consumable by operators, alerting systems, and future LLM explainability tools.
- `GET /api/v1/agents/{id}/traces` provides a queryable decision history with zero additional infrastructure (in-memory, no database required in Phase 1).
- `GET /api/v1/twins/{id}/diff` provides a machine-readable desired-vs-actual diff useful for GitOps reconciliation pipelines and dashboards.
- UI agent detail page now shows the last 10 reasoning decisions, fulfilling the acceptance criterion from issue #66.
- Ring buffer design bounds memory growth to O(capacity × agents); 100 traces × 4 agents × ~400 bytes/trace ≈ 160 KiB worst case for the demo set.
- `TraceEmitter` is nil-safe — zero overhead when no emitter is wired.

### Negative / Trade-offs

- Traces are ephemeral (in-memory only). A server restart loses all trace history. Durable storage deferred to Phase 2 (depends on ADR-016 PostgreSQL dataplane or ADR-005 NATS JetStream).
- The `/diff` endpoint recomputes the diff on every request from live twin state; no caching. Acceptable for Phase 1 traffic volumes.
- `RuleBasedReasoner` now takes slightly longer per `Plan()` call when an emitter is wired (one `Emit()` call with a small allocation). The resource-usage tech spike (ADR-007) remains well within targets.

---

## Alternatives Considered

| Option | Reason Rejected |
|--------|----------------|
| Structured logging only (no ring buffer) | Logs are not queryable via REST; operators need a machine-readable API |
| Always allocate `ReasonerTrace` regardless of emitter | Adds allocation overhead on resource-constrained edge devices; nil check is idiomatic Go |
| OpenTelemetry spans | Full OTel SDK dependency (~4 MB); deferred to Phase 2 with gRPC interceptor chain (ADR-018 §5) |
| Store traces in etcd | etcd is the control plane, not a time-series store; adds unnecessary write amplification |
| Separate `trace.go` package | `TraceEmitter` must be co-located with `AIReasoner` for the interface to be in scope without circular imports |

---

## Implementation Notes

- `internal/skill/trace.go` — `ReasonerTrace`, `TraceInput`, `TraceEmitter`, `TraceStore`.
- `internal/skill/reasoner.go` — `RuleBasedReasoner.AgentID`, `RuleBasedReasoner.Emitter`; trace emitted at end of `Plan()`.
- `internal/skill/trace_test.go` — unit tests for `TraceStore` ring buffer and `RuleBasedReasoner` trace emission.
- `internal/api/types.go` — `AgentTrace`, `TwinDiff`, `DiffField` wire types.
- `internal/api/handlers.go` — `handleAgentTraces`, `handleTwinDiff`, `traceToAPI` helper; `handleAgent` updated to dispatch `/traces` sub-resource.
- `internal/api/server.go` — `traceStore *skill.TraceStore` field on `Server`; `routeTwin` updated to dispatch `/diff`; demo traces seeded in `Seed()`.
- `ui/index.html` — `showAgentDetail` updated to fetch and render trace panel; `outcomeBadge` helper added.
- `docs/adr/INDEX.md` — ADR-019 entry added.

---

## References

- ADR-007 (Agentic AI Agent Model — reasoning loop definition)
- ADR-008 (AI Model Selection — Tier 0/1/2 reasoner taxonomy)
- ADR-017 (UI/API Gateway — REST endpoint conventions)
- ADR-018 (Observability Stack — `pheromone_reasoning_decisions_total` counter)
- Issue #66 (Architectural gap review — observability gaps)
- Issue #75 (Feature: ADR-019 — Agent & AI Observability)

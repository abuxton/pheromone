# Pheromone ADR Index & Decision Status

**Updated**: 2026-03-17 (ADR-019 accepted — Agent & AI Observability; reasoning traces, twin diff endpoint, action audit log)

## Decision Timeline & Status

| ADR | Title | Status | Phase | Focus | Links |
|-----|-------|--------|-------|-------|-------|
| **001** | Digital Twin Architecture Design | ✅ Accepted | Phase 0 (Foundation) | Initial architecture, lang/protocols/frameworks | Original ADR |
| **002** | Server Architecture | ✅ Accepted | Phase 1 (MVP) | In-memory + etcd hybrid | spec-001 FR-002,013,014 |
| **003** | gRPC Service Contracts | ✅ Accepted | Phase 1 (MVP) | Three services (Registry/Control/Telemetry) | spec-001 FR-005,006,007 |
| **004** | Twin Model Schema Format | ✅ Accepted | Phase 1 (MVP) | YAML + JSON Schema validation | spec-001 FR-001,003 |
| **005** | Message Queue Selection | ⏳ Proposed | Phase 1 (MVP) | NATS (MVP) → Kafka (Phase 2) | spec-001 FR-010,011,SC-008 |
| **006** | Agent Lifecycle Interface — Skill Deployment & Distribution Framework | ⏳ Proposed | Phase 1 (MVP) | Skills as unit of deployment; hierarchical twin-level access control; Go framework in `internal/skill/` | spec-001 FR-020,SC-009 |
| **007** | Agentic AI Agent Model | ✅ Accepted | Phase 0 (Foundation) | Digital twin as agent skill; AI reasoning loop | ADR-001,003,006 |
| **008** | AI Model Selection — Local vs. Remote Reasoning Engine | ✅ Accepted | Phase 2 (AI Reasoning) | Ollama (local), llamafile (edge), remote API; OS recommendations | ADR-007,006,003 |
| **009** | OpenClaw Evaluation — Central Server and Agent Role Assessment | ✅ Accepted | Phase 2 (AI Reasoning) | OpenClaw as server/agent candidate; extends ADR-008 | ADR-008,007,003 |
| **010** | Evaluate Signal Protocol (signalapp) for Server↔Agent Communication | ✅ Accepted | Phase 1 (Security Review) | Signal Protocol not suitable; gRPC+mTLS confirmed; AGPL/Go/throughput constraints | ADR-001,003,005 |
| **011** | Post-Action Hooks — Notifications, Webhooks, and Package Delivery | ✅ Accepted | Phase 1 (MVP) | Notification Skill (agent-side) + Post-Action Hook Service (server-side); direct end-user notify | ADR-003,005,006,007,010 |
| **012** | Vagrant Testing Environment | ✅ Accepted | Phase 1 (MVP) | Multi-machine Vagrant setup (Ubuntu 24.04 + Debian 12) for server/agent integration testing | ADR-001,002,008 |
| **013** | Swamp Evaluation — System Initiative AI Automation CLI | ⏳ Proposed | Phase 0 (Spike) | Swamp not adopted (AGPL v3, TypeScript/Deno, single-machine CLI); Definition/CEL model noted as ADR-004 reference | ADR-003,004,007,010 |
| **014** | Envoy Proxy Evaluation — Monitoring, Observability, and Service Mesh Integration | ⏳ Proposed | Phase 1 (Spike) | Server-side ingress recommended (Phase 1); per-agent sidecar and xDS control plane deferred to Phase 2 | ADR-002,003,005,010,012 |
| **014** | Security Architecture | ✅ Accepted | Phase 1 (Security) | TLS enforcement, gRPC auth interceptors (Phase 2), config permissions hardened, vulnerability scanning in CI | ADR-003,007,010,011 |
| **015** | User Access Control & Identity Provider Integration | ⏳ Proposed | Phase 1–3 (Auth/IAM) | Local auth (bcrypt/etcd), four-role RBAC, RS256 JWT, gRPC interceptor chain, IdP plugin adapter (LDAP/SAML/OIDC) | ADR-002,003,006,007,014; spec-002 |
| **016** | Stateless Server — Dataplane Selection | ⏳ Proposed | Phase 2 (Scalability) | PostgreSQL as primary dataplane; etcd retained for control plane; server becomes stateless | ADR-002,003,014 |
| **017** | UI/API Gateway Design — SSE, OpenAPI, k8s Probes | ✅ Accepted | Phase 1 (MVP) | SSE endpoint `/api/v1/events`; k8s health probes `/healthz`/`/readyz`; OpenAPI 3.1 spec; rate limiting & request size middleware | ADR-002,011,014 |
| **018** | Observability Stack — Prometheus, OpenTelemetry, Grafana | ✅ Accepted | Phase 1 (MVP) | `/metrics` endpoint; 6 key metrics; Prometheus + Grafana in docker-compose; isolated registry design | ADR-003,005,014,017 |
| **019** | Agent & AI Observability — Reasoning Traces, Twin Diff, Action Audit | ✅ Accepted | Phase 1 (MVP) | `ReasonerTrace` struct; `TraceStore` ring buffer; `/agents/{id}/traces` + `/twins/{id}/diff` REST endpoints; UI trace viewer panel | ADR-007,008,017,018 |

---

## ADR Decision Map

### Layer 1: Core Architecture (ADR-001, 007, 008 - Foundation)
- ✅ Choose Go + Rust as implementation languages
- ✅ Choose custom agentic AI agent framework (not Telegraf/Fluent Bit)
- ✅ Choose gRPC as primary protocol
- ✅ Choose etcd/Consul for state management
- ✅ Establish agentic AI agent model; digital twin as agent skill (ADR-007)
- ✅ Choose AI reasoning engine: Ollama (local), llamafile (edge), remote API (ADR-008)
- ✅ Evaluate OpenClaw as server/agent candidate — not adopted as core infra (ADR-009)
- ⏳ Evaluate Swamp (System Initiative) — not adopted; YAML/CEL model noted as ADR-004 reference (ADR-013)

### Layer 2: Control Plane (ADR-002, 003, 004, 010)
- ✅ ADR-002: How to structure server state (in-mem + etcd)
- ✅ ADR-003: How agents communicate with server (gRPC contracts)
- ✅ ADR-004: How operators define twin models (YAML schema)
- ✅ ADR-010: Signal Protocol (signalapp) evaluated; gRPC+mTLS confirmed as server↔agent security

### Layer 3: Telemetry & Extensibility (ADR-005, 006, 011)
- ⏳ ADR-005: How metrics flow to observability tools (NATS/Kafka)
- ⏳ ADR-006: How developers extend platform (agent scaffold)
- ⏳ ADR-011: Post-action hooks — Notification Skill + server-side hook service; webhooks, package delivery, direct end-user notification

### Layer 4: Security & Identity (ADR-014, ADR-015)
- ✅ ADR-014: Security posture — TLS enforcement, gRPC interceptors, config permissions, vulnerability scanning
- ⏳ ADR-015: User identity & access control — local auth (bcrypt/etcd), four-role RBAC, RS256 JWT, gRPC interceptor chain, IdP plugin adapter (LDAP/SAML/OIDC)
### Layer 4: Scalability & Data Plane (ADR-016)
- ⏳ ADR-016: Stateless server dataplane — PostgreSQL as primary durable store; etcd retained for control-plane coordination only

### Layer 5: UI/API Gateway (ADR-017)
- ✅ ADR-017: Real-time SSE stream at `/api/v1/events`; k8s liveness/readiness probes at `/healthz`/`/readyz`; OpenAPI 3.1 spec at `docs/api/openapi.yaml`; per-IP rate limiting and request body size limit middleware

---

## Key Decisions to Ratify

### Must Review (Architectural Impact)
- **ADR-002**: In-memory + etcd hybrid performance characteristics (affects latency targets; extended for AI capability registry)
- **ADR-003**: gRPC bidirectional streaming patterns (affects agent implementation complexity; extended for capability advertisement and ProposeAction)
- **ADR-007**: Agentic AI agent model (affects all agent implementations and server design)
- **ADR-008**: AI model selection (Ollama/llamafile/remote); OS recommendations (Ubuntu 24.04 LTS, Talos Linux)
- **ADR-009**: OpenClaw evaluation (confirms ADR-008 Ollama decision; no core architecture change)
- **ADR-010**: Signal Protocol (signalapp) evaluation — NOT adopted; gRPC+mTLS confirmed as server↔agent security layer ✅ **Accepted**
- **ADR-011**: Port assignment — 4426 (gRPC control), 4427 (gRPC telemetry), 443/80 (HTTP/HTTPS REST API)
- **ADR-013**: Swamp (System Initiative) evaluation — NOT adopted; AGPL v3 licence + TypeScript/Deno mismatch; YAML/CEL Definition model noted as ADR-004 reference
- **ADR-014**: Envoy Proxy evaluation — server-side ingress recommended (Phase 1); per-agent sidecar and xDS control plane deferred to Phase 2
- **ADR-015**: User Access Control — local auth + RBAC + JWT + gRPC interceptor chain; IdP adapter pattern (Phase 2)
- **ADR-016**: Stateless server dataplane — PostgreSQL selected as primary durable store; etcd retained for control-plane coordination; server becomes crash-recoverable


### Should Review (Implementation Strategy)
- **ADR-005**: NATS for MVP; Kafka upgrade path (affects telemetry architecture)
- **ADR-006**: Skill deployment framework — skills as unit of deployment with hierarchical twin-level access control (Go framework implemented; revision complete)
- **ADR-011**: Post-action hooks (Notification Skill + server hook service; affects agent scaffold and server API design)

---

## ADR Dependencies

```
ADR-001 (Foundation)
   ├─→ ADR-002 (Server Architecture)
   │    └─→ ADR-003 (gRPC Contracts)
   │         ├─→ ADR-004 (Twin Schema)
   │         └─→ ADR-006 (Agent Interface)
   │
   ├─→ ADR-005 (Message Queue)
   │    └─→ ADR-006 (Agent Lifecycle)
   │         └─→ ADR-011 (Post-Action Hooks — Notification Skill)
   │
   ├─→ ADR-004 (Twin Schema) [indirect]
   │
   └─→ ADR-007 (Agentic AI Agent Model)
        ├─→ ADR-006 (Agent Lifecycle — updated)
        ├─→ ADR-002 (Server — AI capability registry)
        ├─→ ADR-003 (gRPC — capability advertisement)
        │    └─→ ADR-011 (Post-Action Hooks — ActionEventService extends ADR-003)
        ├─→ ADR-008 (AI Model Selection — Ollama/llamafile/remote) [accepted]
        │    └─→ ADR-009 (OpenClaw Evaluation — confirms ADR-008) [accepted]
        ├─→ ADR-010 (Signal Protocol — mTLS confirmed) [proposed]
        │    └─→ ADR-011 (Post-Action Hooks — mTLS for webhook credential transport)
        └─→ ADR-011 (Post-Action Hooks — notification as post-reasoning-loop step)
ADR-014 (Envoy Proxy Evaluation — monitoring, observability, service mesh)
   ├─→ ADR-003 (gRPC Contracts — Envoy proxies existing services)
   ├─→ ADR-010 (mTLS confirmed — Envoy can own TLS termination via SDS)
   └─→ ADR-012 (Vagrant — Envoy added to server provisioning scripts)
ADR-015 (User Access Control & Identity Provider Integration)
   ├─→ ADR-002 (Server Architecture — etcd user record store)
   ├─→ ADR-003 (gRPC Contracts — AuthService extends existing service patterns)
   ├─→ ADR-006 (Agent Lifecycle — twin-level ACLs form fine-grained gate below RBAC)
   ├─→ ADR-007 (Agentic AI Agent Model — agents use mTLS; unaffected by human auth)
   └─→ ADR-014 (Security Architecture — interceptor chain extended; TLS prerequisite)
ADR-016 (Stateless Server Dataplane — PostgreSQL selection)
   ├─→ ADR-002 (Server Architecture — Layer 2 durable state moves from etcd to PostgreSQL)
   ├─→ ADR-003 (gRPC Contracts — agents access dataplane via server only)
   └─→ ADR-014 (Security — mTLS and encryption-at-rest requirements drive candidate scoring)
ADR-017 (UI/API Gateway Design — SSE, OpenAPI, k8s Probes)
   ├─→ ADR-002 (Server Architecture — extends existing HTTP listener and mux)
   ├─→ ADR-011 (Post-Action Hooks — events complement the notification hook system)
   └─→ ADR-014 (Security Architecture — rate limiting and size limits harden the API surface)
ADR-018 (Observability Stack — Prometheus, OpenTelemetry, Grafana)
   ├─→ ADR-003 (gRPC Contracts — RED metric labels align with service/method)
   ├─→ ADR-005 (Message Queue — NATS telemetry backbone feeds pheromone_nats_messages_total)
   ├─→ ADR-014 (Security Architecture — /metrics access control via network policy)
   └─→ ADR-017 (UI/API Gateway — /metrics follows same unauthenticated probe pattern as /healthz)
```

**Critical Path**: ADR-001 → ADR-007 → ADR-002 → ADR-003 → ADR-006 → ADR-014 → ADR-015 (auth layer sits atop all prior decisions)

---

## Tech Spike Validation Required

Before ADR Acceptance, run these validation spikes:

| ADR | Spike | Goal | Estimated Effort |
|-----|-------|------|------------------|
| ADR-002 | etcd sync latency + cache hit ratio | Prove <100ms sync, high hit rate | 8 hours |
| ADR-003 | gRPC bidirectional stream prototype | Test 1000 agents streaming simultaneously | 12 hours |
| ADR-005 | NATS load test 100K msgs/sec | Measure latency, memory overhead, JetStream persistence | 8 hours |
| ADR-006 | Go skill framework + 2 example custom skills | Time end-to-end: skill impl (<1 hour, ~50 lines); hierarchical access control validated | ✅ 6 hours (complete — `internal/skill/`) |
| ADR-007 | Agentic AI loop resource usage | Measure memory/CPU overhead of reasoning loop on typical instance | 6 hours |
| ADR-008 | Ollama + phi3.5:mini resource benchmark | Measure RAM/CPU on 8 GB instance; test fallback to rule-based reasoner | 6 hours |
| ADR-009 | OpenClaw HTTP API integration spike (optional) | Prototype operator-interface bridge (OpenClaw → Pheromone API); assess Slack/Discord channel feasibility for operator UX only | 4 hours |
| ADR-011 | HTTP webhook dispatch at scale | Server dispatching hooks to 100 endpoints with 1000 simultaneous agent events | 6 hours |
| ADR-011 | Agent direct notify resilience | Verify reasoning loop unblocked when notification destinations unreachable | 4 hours |
| ADR-011 | HMAC webhook verification prototype | Signature generation/verification across Go agent and Python receiver | 2 hours |
| ADR-014 | Envoy server-side ingress prototype | Add Envoy to docker-compose; validate gRPC traffic metrics and admin API | 4 hours |
| ADR-014 | Per-agent sidecar RAM overhead spike | Measure `envoy:distroless` memory footprint on target managed instances | 4 hours |
| ADR-014 | go-control-plane xDS server spike | Prototype Pheromone server as xDS control plane feeding agent registration into EDS | 12 hours |
| ADR-015 | bcrypt cost=12 latency on arm64 (Raspberry Pi 4) | Verify login latency < 500 ms on resource-constrained hardware; evaluate cost=10 tradeoff | 2 hours |
| ADR-015 | JWT RS256 interceptor hot-path benchmark | Measure per-RPC overhead of RSA public-key JWT validation under 1000 concurrent agents | 4 hours |
| ADR-015 | LDAP adapter integration spike | Bind + group search against containerised OpenLDAP; validate group→role mapping | 6 hours |

**Total Spike Effort**: ~94 hours (can run in parallel; +12 hours added for ADR-015 auth spikes)
| ADR-016 | PostgreSQL throughput at 1 000 agents | Measure pgxpool read/write p99 with 1 000 twin blobs (≤64 KB each) | 6–8 hours |
| ADR-016 | Server cold-start rehydration benchmark | Measure time to load 10 000 twin records from PostgreSQL into in-memory cache | 4 hours |

**Total Spike Effort**: ~92 hours (can run in parallel; +10 hours added for ADR-016 PostgreSQL spikes)

---

## Approval Requirements (Per Constitution)

- **Minimum Approvers**: 2 per ADR
- **Feedback Period**: 1 week minimum (community review)
- **Amendments**: Changes to Proposed ADRs update Status + add dated rationale (living document)
- **Supersession**: New ADR created if decision reversal needed (not modify existing)

---

## Traceability to Specification

### User Stories → ADRs
- **Story 1 (Unified Twin View)** → ADR-002, ADR-003, ADR-004
- **Story 2 (Push Configuration)** → ADR-002, ADR-004
- **Story 3 (Design Hierarchy)** → ADR-001, ADR-004, ADR-006
- **Story 4 (Custom Agents)** → ADR-006
- **Story 5 (RBAC)** → Future ADR (Phase 2)

### Functional Requirements → ADRs
- **FR-001 to FR-024** (27 total) traced in ADR "References" sections
- **SC-001 to SC-014** (14 success criteria) linked to ADR implementation consequences

---

## Next Steps

1. **Week 1**: Team review of spec-001 + ADR-001 through 006
2. **Week 2**: ADR feedback period; tech spikes execute in parallel
3. **Week 3**: ADR amendments based on spike results + feedback
4. **Week 4**: ADR ratification (Status: Proposed → Accepted)
5. **Week 5+**: Planning phase with Speckit `plan` agent

---

## Required GitHub Issues

All GitHub issues required to process ADR material and unblock development are tracked in:

> **[`docs/REQUIRED-ISSUES.md`](../REQUIRED-ISSUES.md)**

| Issue | Type | ADR | Effort |
|-------|------|-----|--------|
| Complete etcd validation with live etcd | Tech Spike | ADR-002 | 2h |
| gRPC Bidirectional Stream Prototype | Tech Spike | ADR-003 | 12h |
| Agentic AI Loop Resource Usage | Tech Spike | ADR-007 | 6h |
| Ollama + phi3.5:mini Benchmark | Tech Spike | ADR-008 | 6h |
| HTTP Webhook Dispatch at Scale | Tech Spike | ADR-011 | 6h |
| Agent Direct Notify Resilience | Tech Spike | ADR-011 | 4h |
| HMAC Webhook Verification Prototype | Tech Spike | ADR-011 | 2h |
| Review and Accept ADR-002 | ADR Review | ADR-002 | — |
| Review and Accept ADR-003 | ADR Review | ADR-003 | — |
| Review and Accept ADR-004 | ADR Review | ADR-004 | — |
| Review and Accept ADR-008/009 | ADR Review | ADR-008/009 | — |
| Review and Accept ADR-010 | ADR Review | ADR-010 | — |
| Review and Accept ADR-011 | ADR Review | ADR-011 | — |
| Review and Accept ADR-012 | ADR Review | ADR-012 | — |
| Review and Accept ADR-013 | ADR Review | ADR-013 | — |
| Review Swamp YAML/CEL Definition model as ADR-004 reference input | Research | ADR-013/004 | 2h |
| Review and Accept ADR-014 | ADR Review | ADR-014 | — |
| Add Envoy proxy to `docker-compose.yml` for server-side monitoring | Implementation | ADR-014 | 2h |
| Create static Envoy config for Pheromone gRPC ingress | Implementation | ADR-014 | 3h |
| Document Pheromone Prometheus/Envoy metric catalogue | Documentation | ADR-014 | 2h |
| Add Envoy to Vagrant server provisioning scripts | Implementation | ADR-014 | 2h |
| Spike: per-agent Envoy sidecar RAM overhead on target instances | Tech Spike | ADR-014 | 4h |
| Spike: go-control-plane xDS server in Pheromone server process | Tech Spike | ADR-014 | 12h |
| Spike: SDS certificate management via Envoy for agent mTLS | Tech Spike | ADR-014 | 8h |
| Implement Vagrant provisioning scripts | Implementation | ADR-012 | 4h |
| Implement gRPC proto definitions | Implementation | ADR-003 | 8h |
| Review and Accept ADR-016 | ADR Review | ADR-016 | — |
| Implement PostgreSQL dataplane adapter in Pheromone server | Implementation | ADR-016 | 16–24h |
| One-time migration utility: etcd Layer 2 keys → PostgreSQL | Implementation | ADR-016 | 8–12h |
| Configure mTLS for server ↔ PostgreSQL (sslmode=verify-full) | Implementation | ADR-016 | 4–8h |
| Deploy PostgreSQL to Vagrant environment and docker-compose | Implementation | ADR-016 | 4–6h |
| Implement LUKS encryption at rest for PostgreSQL data directory | Implementation | ADR-016 | 4–8h |
| Write backup and recovery runbook (pg_basebackup + WAL archiving) | Documentation | ADR-016 | 4h |
| Add Prometheus metrics for PostgreSQL health, connection pool, query latency | Implementation | ADR-016 | 4h |
| Spike: PostgreSQL throughput at 1 000 agents with pgxpool | Tech Spike | ADR-016 | 6–8h |

---

## References

- **Specification**: `.specify/memory/spec-001-digital-twin-platform.md`
- **Specification (ADR-016)**: `.specify/memory/spec-002-stateless-server-dataplane.md`
- **Status**: `.specify/memory/PHASE1-STATUS.md`
- **Quality Checklist**: `.specify/memory/checklists/spec-001-quality.md`
- **Constitution**: `.specify/memory/constitution.md` (v2.0.0)
- **ADR Framework**: `docs/adr/README.md`
- **Required Issues**: `docs/REQUIRED-ISSUES.md`

---

**Status**: ✅ **ADRs 001, 002, 003, 004, 007, 008, 009, 010, 011, 012, 014(Security), 017, 018 ACCEPTED — ADRs 005, 006, 013, 014(Envoy), 015, 016 PROPOSED - READY FOR TEAM REVIEW**
**Updated**: 2026-03-12 (ADR-018 accepted — Observability Stack: Prometheus `/metrics` endpoint, 6 key metrics, Grafana docker-compose services)


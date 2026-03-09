# Pheromone ADR Index & Decision Status

**Updated**: 2026-03-09 (ADR-010 accepted — Signal Protocol evaluation ratified; gRPC+mTLS confirmed)

## Decision Timeline & Status

| ADR | Title | Status | Phase | Focus | Links |
|-----|-------|--------|-------|-------|-------|
| **001** | Digital Twin Architecture Design | ✅ Accepted | Phase 0 (Foundation) | Initial architecture, lang/protocols/frameworks | Original ADR |
| **002** | Server Architecture | ✅ Accepted | Phase 1 (MVP) | In-memory + etcd hybrid | spec-001 FR-002,013,014 |
| **003** | gRPC Service Contracts | ✅ Accepted | Phase 1 (MVP) | Three services (Registry/Control/Telemetry) | spec-001 FR-005,006,007 |
| **004** | Twin Model Schema Format | ⏳ Proposed | Phase 1 (MVP) | YAML + JSON Schema validation | spec-001 FR-001,003 |
| **005** | Message Queue Selection | ⏳ Proposed | Phase 1 (MVP) | NATS (MVP) → Kafka (Phase 2) | spec-001 FR-010,011,SC-008 |
| **006** | Agent Lifecycle Interface — Skill Deployment & Distribution Framework | ⏳ Proposed | Phase 1 (MVP) | Skills as unit of deployment; hierarchical twin-level access control; Go framework in `internal/skill/` | spec-001 FR-020,SC-009 |
| **007** | Agentic AI Agent Model | ✅ Accepted | Phase 0 (Foundation) | Digital twin as agent skill; AI reasoning loop | ADR-001,003,006 |
| **008** | AI Model Selection — Local vs. Remote Reasoning Engine | ✅ Accepted | Phase 2 (AI Reasoning) | Ollama (local), llamafile (edge), remote API; OS recommendations | ADR-007,006,003 |
| **009** | OpenClaw Evaluation — Central Server and Agent Role Assessment | ✅ Accepted | Phase 2 (AI Reasoning) | OpenClaw as server/agent candidate; extends ADR-008 | ADR-008,007,003 |
| **010** | Evaluate Signal Protocol (signalapp) for Server↔Agent Communication | ✅ Accepted | Phase 1 (Security Review) | Signal Protocol not suitable; gRPC+mTLS confirmed; AGPL/Go/throughput constraints | ADR-001,003,005 |
| **011** | Post-Action Hooks — Notifications, Webhooks, and Package Delivery | ⏳ Proposed | Phase 1 (MVP) | Notification Skill (agent-side) + Post-Action Hook Service (server-side); direct end-user notify | ADR-003,005,006,007,010 |
| **012** | Vagrant Testing Environment | ✅ Accepted | Phase 1 (MVP) | Multi-machine Vagrant setup (Ubuntu 24.04 + Debian 12) for server/agent integration testing | ADR-001,002,008 |
| **013** | Swamp Evaluation — System Initiative AI Automation CLI | ⏳ Proposed | Phase 0 (Spike) | Swamp not adopted (AGPL v3, TypeScript/Deno, single-machine CLI); Definition/CEL model noted as ADR-004 reference | ADR-003,004,007,010 |

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
- ⏳ ADR-004: How operators define twin models (YAML schema)
- ✅ ADR-010: Signal Protocol (signalapp) evaluated; gRPC+mTLS confirmed as server↔agent security

### Layer 3: Telemetry & Extensibility (ADR-005, 006, 011)
- ⏳ ADR-005: How metrics flow to observability tools (NATS/Kafka)
- ⏳ ADR-006: How developers extend platform (agent scaffold)
- ⏳ ADR-011: Post-action hooks — Notification Skill + server-side hook service; webhooks, package delivery, direct end-user notification

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
```

**Critical Path**: ADR-001 → ADR-007 → ADR-002 → ADR-003 → ADR-006 (affects implementation order)

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

**Total Spike Effort**: ~62 hours (can run in parallel)

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
| Implement Vagrant provisioning scripts | Implementation | ADR-012 | 4h |
| Implement gRPC proto definitions | Implementation | ADR-003 | 8h |

---

## References

- **Specification**: `.specify/memory/spec-001-digital-twin-platform.md`
- **Status**: `.specify/memory/PHASE1-STATUS.md`
- **Quality Checklist**: `.specify/memory/checklists/spec-001-quality.md`
- **Constitution**: `.specify/memory/constitution.md` (v2.0.0)
- **ADR Framework**: `docs/adr/README.md`
- **Required Issues**: `docs/REQUIRED-ISSUES.md`

---

**Status**: ✅ **ADRs 001, 002, 003, 007, 008, 009, 010, 012 ACCEPTED — ADRs 004-006, 011, 013 PROPOSED - READY FOR TEAM REVIEW**
**Updated**: 2026-03-09 (ADR-010 accepted — Signal Protocol evaluation ratified; gRPC+mTLS confirmed as server↔agent security)


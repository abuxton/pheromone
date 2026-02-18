# Pheromone ADR Index & Decision Status

**Updated**: 2026-02-18

## Decision Timeline & Status

| ADR | Title | Status | Phase | Focus | Links |
|-----|-------|--------|-------|-------|-------|
| **001** | Digital Twin Architecture Design | ✅ Accepted | Phase 0 (Foundation) | Initial architecture, lang/protocols/frameworks | Original ADR |
| **002** | Server Architecture | ⏳ Proposed | Phase 1 (MVP) | In-memory + etcd hybrid | spec-001 FR-002,013,014 |
| **003** | gRPC Service Contracts | ⏳ Proposed | Phase 1 (MVP) | Three services (Registry/Control/Telemetry) | spec-001 FR-005,006,007 |
| **004** | Twin Model Schema Format | ⏳ Proposed | Phase 1 (MVP) | YAML + JSON Schema validation | spec-001 FR-001,003 |
| **005** | Message Queue Selection | ⏳ Proposed | Phase 1 (MVP) | NATS (MVP) → Kafka (Phase 2) | spec-001 FR-010,011,SC-008 |
| **006** | Agent Lifecycle Interface | ⏳ Proposed | Phase 1 (MVP) | Language-agnostic scaffold (Go/Python/Rust) | spec-001 FR-020,SC-009 |

---

## ADR Decision Map

### Layer 1: Core Architecture (ADR-001 - Foundation)
- ✅ Choose Go + Rust as implementation languages
- ✅ Choose custom agent framework (not Telegraf/Fluent Bit)
- ✅ Choose gRPC as primary protocol
- ✅ Choose etcd/Consul for state management

### Layer 2: Control Plane (ADR-002, 003, 004)
- ⏳ ADR-002: How to structure server state (in-mem + etcd)
- ⏳ ADR-003: How agents communicate with server (gRPC contracts)
- ⏳ ADR-004: How operators define twin models (YAML schema)

### Layer 3: Telemetry & Extensibility (ADR-005, 006)
- ⏳ ADR-005: How metrics flow to observability tools (NATS/Kafka)
- ⏳ ADR-006: How developers extend platform (agent scaffold)

---

## Key Decisions to Ratify

### Must Review (Architectural Impact)
- **ADR-002**: In-memory + etcd hybrid performance characteristics (affects latency targets)
- **ADR-003**: gRPC bidirectional streaming patterns (affects agent implementation complexity)
- **ADR-004**: YAML schema and multi-environment support (affects operator usability)

### Should Review (Implementation Strategy)  
- **ADR-005**: NATS for MVP; Kafka upgrade path (affects telemetry architecture)
- **ADR-006**: Scaffold-based agent development (affects time-to-first-custom-agent)

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
   │
   └─→ ADR-004 (Twin Schema) [indirect]
```

**Critical Path**: ADR-001 → ADR-002 → ADR-003 → ADR-006 (affects implementation order)

---

## Tech Spike Validation Required

Before ADR Acceptance, run these validation spikes:

| ADR | Spike | Goal | Estimated Effort |
|-----|-------|------|------------------|
| ADR-002 | etcd sync latency + cache hit ratio | Prove <100ms sync, high hit rate | 8 hours |
| ADR-003 | gRPC bidirectional stream prototype | Test 1000 agents streaming simultaneously | 12 hours |
| ADR-005 | NATS load test 100K msgs/sec | Measure latency, memory overhead, JetStream persistence | 8 hours |
| ADR-006 | Go agent scaffold + custom agent | Time end-to-end: scaffold → implement → test | 6 hours |

**Total Spike Effort**: ~34 hours (can run in parallel)

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

## References

- **Specification**: `.specify/memory/spec-001-digital-twin-platform.md`
- **Status**: `.specify/memory/PHASE1-STATUS.md`
- **Quality Checklist**: `.specify/memory/checklists/spec-001-quality.md`
- **Constitution**: `.specify/memory/constitution.md` (v2.0.0)
- **ADR Framework**: `ADR/README.md`

---

**Status**: ✅ **ADRs 002-006 PROPOSED - READY FOR TEAM REVIEW**


# Feature Specification: Stateless Server — Dataplane Offload Research and ADR

**Feature Branch**: `copilot/feature-evaluate-data-plane-implementation`
**Created**: 2026-06-18
**Status**: Draft
**Input**: "[Feature]: stateless server offload dataplane from server to suitable service or application research and ADR"

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Platform Architect: Structured Dataplane Options Research (Priority: P1)

A platform architect needs a rigorous, criteria-driven evaluation of all candidate dataplane technologies so that the team can make an informed, defensible architecture decision about where the Pheromone server stores its operational state.

**Why this priority**: Without completed research, no technology selection can proceed. This is the prerequisite for every downstream decision — server refactoring, deployment topology, and security controls all depend on knowing which dataplane is chosen.

**Independent Test**: Can be fully tested by confirming that a research document exists covering all six named candidate technologies plus any additional options discovered, each scored against a shared set of documented evaluation criteria. The document is independently valuable even before an ADR is written.

**Acceptance Scenarios**:

1. **Given** the list of candidate technologies (PostgreSQL, BoltDB, BadgerDB, etcd, Redis, flat file, and other options), **When** the research phase concludes, **Then** a structured comparison exists with each technology assessed against identical criteria: performance characteristics, operational complexity, security posture, backup and recovery capability, agent-accessibility model, and licensing.
2. **Given** the existing ADR-002 hybrid in-memory + etcd architecture, **When** each candidate is evaluated, **Then** the research explicitly describes how each option relates to, replaces, or complements the current in-memory store and the existing etcd control-plane usage documented in ADR-002.
3. **Given** a candidate that requires mutual TLS or encryption at rest, **When** assessing that candidate, **Then** the research documents whether those capabilities are built-in, third-party, or unsupported.

---

### User Story 2 — Platform Architect: ADR with Recommended Dataplane Selection (Priority: P1)

A platform architect publishes a formal Architecture Decision Record that captures the evaluation outcome, the selected dataplane technology, the rationale for selection, and the consequences of the decision — enabling the development team to implement against a stable, reviewed choice.

**Why this priority**: The ADR is the contractual output of this feature. Without it the server statelessness work has no authoritative direction; follow-on implementation issues cannot be meaningfully scoped.

**Independent Test**: Can be fully tested by reviewing the ADR document alone: it must contain a clearly stated status, context section referencing ADR-002, a documented decision with rationale, positive and negative consequences, and explicit pointers to follow-up issues.

**Acceptance Scenarios**:

1. **Given** the completed research, **When** the ADR is written, **Then** the ADR identifies a single recommended dataplane technology (or a ranked shortlist if a phased adoption is proposed) with written justification traceable to the evaluation criteria.
2. **Given** the ADR recommendation, **When** reviewing security requirements, **Then** the ADR documents how the selected dataplane satisfies encryption at rest, transport encryption, and mutual TLS requirements (or describes mitigations where a requirement cannot be fully met).
3. **Given** the ADR is accepted, **When** it is published, **Then** it includes a section listing concrete follow-up GitHub issues required for implementation, each with a brief description and suggested priority.
4. **Given** the ADR references ADR-002, **When** reviewing migration impact, **Then** the ADR describes the transition path from the current hybrid in-memory + etcd store to the selected dataplane, including what (if anything) the existing etcd deployment continues to own.

---

### User Story 3 — Security Engineer: Documented Data Security Requirements and Controls (Priority: P2)

A security engineer needs the dataplane selection process to surface and document data security requirements — covering data sovereignty, encryption at rest, transport security, and mutual TLS — so that the selected technology meets the platform's security posture before implementation begins.

**Why this priority**: Security requirements locked in at the architecture stage are far cheaper to address than retrofits. This story ensures security is a first-class evaluation criterion, not an afterthought.

**Independent Test**: Can be fully tested by reviewing the ADR and supporting research for an explicit security section that maps each security requirement to the candidate technologies' capabilities and documents whether the recommended technology meets each requirement.

**Acceptance Scenarios**:

1. **Given** the requirement for data sovereignty (data must not leave a defined boundary), **When** each candidate is assessed, **Then** the evaluation documents whether the technology supports deployment in isolated, on-premises, or air-gapped environments.
2. **Given** the requirement for encryption at rest, **When** evaluating each candidate, **Then** the research documents whether native encryption is supported, requires a third-party layer, or is unsupported.
3. **Given** the requirement for transport encryption and mutual TLS between the server and the dataplane, **When** assessing each candidate, **Then** the research confirms TLS/mTLS support and notes any configuration required to enable it.
4. **Given** the ADR recommendation, **When** the security section is reviewed, **Then** any security gaps in the recommended technology are explicitly documented with proposed mitigations rather than omitted.

---

### User Story 4 — Platform Operator: Documented Backup and Recovery Procedures (Priority: P2)

A platform operator needs to understand the backup and recovery model for the selected dataplane so that operational runbooks can be written, RTO/RPO targets set, and recovery tested before the stateless server goes to production.

**Why this priority**: A stateless server achieves nothing if the dataplane it depends on cannot be reliably backed up and restored. Documenting this in the ADR prevents it from becoming a gap discovered only during an incident.

**Independent Test**: Can be fully tested by reviewing the ADR's backup and recovery section: it must describe at minimum the backup mechanism (snapshot, WAL, replication), frequency assumptions, restore procedure, and the expected recovery time when the server restarts against a populated dataplane.

**Acceptance Scenarios**:

1. **Given** the selected dataplane technology, **When** backup capabilities are assessed, **Then** the ADR documents at least one supported backup mechanism (e.g., snapshot export, continuous replication, WAL archiving) and the granularity of recovery points it provides.
2. **Given** a scenario where the Pheromone server crashes and restarts, **When** it reconnects to the dataplane, **Then** the ADR describes the expected recovery behaviour — specifically that no committed twin or agent state is lost and the server becomes fully operational within the target recovery window.
3. **Given** the backup procedure documented in the ADR, **When** a test restore is performed in the evaluation environment, **Then** the restored dataplane contains all state that was committed prior to the backup point.

---

### User Story 5 — Development Team: Follow-Up Issues for Implementation (Priority: P3)

A development team member needs the research and ADR to produce a concrete, prioritised list of follow-up GitHub issues so that implementation work can be planned into sprints without further ambiguity about scope.

**Why this priority**: The ADR's value is only realised when the team acts on it. Embedding follow-up issues in the output bridges research and delivery without requiring a separate planning meeting.

**Independent Test**: Can be fully tested by reviewing the ADR's follow-up section: issues must be listed, each with a title, one-sentence description, and a suggested priority label (P1/P2/P3).

**Acceptance Scenarios**:

1. **Given** the accepted ADR, **When** the follow-up issues section is reviewed, **Then** at least three implementation issues are listed, each covering a distinct area (e.g., server integration, security hardening, backup tooling).
2. **Given** a follow-up issue in the ADR, **When** a developer reads it, **Then** the issue description is unambiguous enough to write an acceptance criterion without additional architecture discussion.

---

### Edge Cases

- What happens if the evaluation reveals that no single candidate meets all requirements? The ADR must propose a phased or composite solution, not defer the decision.
- What happens if the existing etcd deployment (ADR-002) introduces a conflict of concern when also considered as a dataplane candidate? The ADR must explicitly address the separation of control-plane versus data-plane roles.
- What happens if a recommended technology does not natively support mutual TLS? A compensating control (e.g., a TLS-terminating sidecar or service mesh) must be documented as a required follow-up issue.
- What happens if backup restore time exceeds the server's target recovery window? The ADR must either disqualify the candidate or document the gap and propose mitigation.
- What if a candidate imposes a licensing constraint incompatible with the project's open-source goals? The research must flag this as a disqualifying criterion with reasoning.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The research MUST evaluate each of the following candidate technologies against documented criteria: PostgreSQL, BoltDB, BadgerDB, etcd (separated from its ADR-002 control-plane role), Redis, and flat file store. Additional candidates discovered during research MAY be included.
- **FR-002**: The evaluation MUST apply a consistent, documented set of criteria to every candidate, covering at minimum: read/write performance characteristics, operational complexity, security posture (encryption at rest, transport encryption, mTLS support), backup and recovery capabilities, data sovereignty suitability, and licensing.
- **FR-003**: The research MUST explicitly assess how each candidate relates to the existing in-memory store and the etcd deployment described in ADR-002, distinguishing control-plane state ownership from dataplane state ownership.
- **FR-004**: The ADR MUST document a selected dataplane technology (or ranked shortlist with phased adoption rationale) with written justification traceable to the evaluation criteria defined in FR-002.
- **FR-005**: The ADR MUST include a dedicated security section covering: encryption at rest, transport encryption between the server and dataplane, mutual TLS configuration, and data sovereignty constraints. Any unmet security requirements MUST list a proposed mitigation.
- **FR-006**: The ADR MUST include a backup and recovery section describing: supported backup mechanism(s), recovery point granularity, restore procedure, and expected server recovery time after restart against a populated dataplane.
- **FR-007**: The ADR MUST describe the transition path from the current hybrid in-memory + etcd architecture (ADR-002) to the selected dataplane, including what state the existing etcd continues to own post-transition.
- **FR-008**: The ADR MUST list at least three concrete follow-up GitHub issues, each with a title, a one-sentence description, and a suggested priority, required to implement the selected dataplane integration.
- **FR-009**: The dataplane evaluation MUST confirm that agents communicate exclusively with the Pheromone server for all dataplane read and write operations — agents MUST NOT connect directly to the dataplane service.
- **FR-010**: The research document and ADR MUST be published to the repository's `docs/adr/` directory following the existing ADR numbering and formatting conventions.

### Key Entities

- **Dataplane**: The persistent storage service that the stateless Pheromone server reads from and writes to on behalf of agents. Distinct from the control-plane etcd store (ADR-002). Stores twin state, agent registration, configuration history, and AI decision traces.
- **Evaluation Criterion**: A named, documented dimension (e.g., "encryption at rest") applied consistently to every candidate technology during research. Each criterion has a weight or must-have/nice-to-have designation.
- **Candidate Technology**: Any storage technology assessed during research (PostgreSQL, BoltDB, BadgerDB, etcd, Redis, flat file, or other options discovered during evaluation).
- **ADR (Architecture Decision Record)**: A structured document following the project's ADR format, recording context, decision, rationale, and consequences. Published under `docs/adr/` and cross-referenced from the relevant specification.
- **Follow-up Issue**: A discrete GitHub issue derived from the ADR that describes a bounded unit of implementation work required to realise the decision. Each issue must stand alone as a developer task.
- **Control Plane State**: State currently managed by etcd under ADR-002 — twin models, agent capabilities, action proposal queues. Remains etcd-owned unless the ADR explicitly decides otherwise.
- **Data Plane State**: Operational state offloaded from the server's in-memory store — may include twin snapshots, audit logs, metric aggregates, and configuration history, depending on the ADR decision.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A research document covering all six named candidate technologies plus any additional options discovered is completed and committed to the repository before the ADR is drafted.
- **SC-002**: Every candidate technology is assessed against the same set of at least six documented evaluation criteria, with no criteria omitted for any candidate.
- **SC-003**: The published ADR identifies a recommended dataplane technology (or phased shortlist) with a rationale section that references the evaluation criteria — reviewers can map each rationale point back to a specific criterion score or assessment.
- **SC-004**: The ADR security section explicitly addresses all four security requirements: encryption at rest, transport encryption, mutual TLS, and data sovereignty. Gaps are documented with mitigations rather than omitted.
- **SC-005**: The ADR backup and recovery section documents a restore procedure that, when followed in the evaluation environment, returns the dataplane to a consistent state with no committed data loss.
- **SC-006**: The ADR lists a minimum of three follow-up implementation issues, each independently actionable by a developer without requiring further architecture clarification.
- **SC-007**: The transition path from the current ADR-002 hybrid in-memory + etcd architecture is documented with sufficient clarity that a developer can identify which existing code paths are affected by the dataplane change.
- **SC-008**: The accepted ADR is published under `docs/adr/` following the project's ADR numbering and format conventions, cross-referenced from this specification, and indexed in `docs/adr/INDEX.md`.

---

## Scope and Boundaries

### In Scope

- Research and structured comparison of all named and any additionally discovered dataplane candidate technologies.
- Definition and documentation of evaluation criteria before candidates are assessed.
- Publication of an ADR with a technology recommendation, security assessment, backup/recovery description, and follow-up issues.
- Explicit mapping of the transition from the ADR-002 hybrid in-memory + etcd architecture to the selected dataplane.
- Security requirements coverage: encryption at rest, transport encryption, mutual TLS, data sovereignty.

### Out of Scope

- Implementation of the selected dataplane integration (follow-up issues from the ADR cover this).
- Changes to the gRPC service contracts (ADR-003) — may be noted as a follow-up if required.
- Benchmark tooling or load testing of the selected technology at this stage.
- Changes to how agents communicate with the server — agent-to-server gRPC contracts are unchanged; only server-to-dataplane communication is in scope.
- Migration of production data (no production deployment exists at this stage).

---

## Assumptions

- The existing etcd deployment (ADR-002) continues to own control-plane state (twin models, agent capabilities, action proposal queues) unless the ADR explicitly decides to consolidate or migrate that ownership.
- "Stateless server" means the Pheromone server process holds no durable state itself — all state survives server restarts because it is stored in the dataplane service.
- The in-memory cache (Layer 1 of ADR-002) may be retained as a read-through cache for performance, but is not the source of truth — the dataplane is.
- Agents never connect directly to the dataplane; the server mediates all reads and writes on behalf of agents, consistent with the existing gRPC communication model.
- The evaluation environment is a single-machine or small-cluster setup sufficient for correctness assessment; full-scale performance benchmarking is deferred to a follow-up issue.
- Open-source licensing is a hard requirement; any candidate with a restrictive proprietary licence is noted as disqualified.
- The project's current Go-primary language stack is the assumed implementation context for any integration libraries assessed during research.

---

## Dependencies

- **ADR-002** (Server Architecture — In-Memory + etcd Hybrid): Provides the current state baseline. The new ADR must explicitly reference and build on or supersede the relevant portions of ADR-002.
- **ADR-003** (gRPC Service Contracts): Dataplane access is via server-mediated gRPC; changes to RPC contracts may be required and should be flagged as follow-up issues.
- **ADR-014** (Security Architecture): Establishes mTLS and encryption requirements that the dataplane candidate must satisfy or mitigate.
- **`docs/adr/INDEX.md`**: Must be updated when the new ADR is published.
- **`specs/constitution.md`**: Principles 1 (Modularity), 2 (Performance), and 3 (Technology Choices) inform evaluation criteria weightings.

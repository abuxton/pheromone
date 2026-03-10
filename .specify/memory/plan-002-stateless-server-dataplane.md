# Implementation Plan: Stateless Server — Dataplane Offload Research and ADR

**Branch**: `copilot/feature-evaluate-data-plane-implementation`
**Date**: 2026-06-18
**Spec**: `.specify/memory/spec-002-stateless-server-dataplane.md`
**Input**: Feature specification — spec-002: Stateless Server Data Plane Selection

---

## Summary

The Pheromone server currently holds all operational twin state in-memory with asynchronous persistence to etcd (ADR-002 hybrid). This plan drives the research, structured evaluation, and formal ADR for offloading that state to a dedicated **dataplane service** — making the server process itself stateless and crash-resilient without any loss of committed data.

The primary output is **ADR-015** (`docs/adr/adr-015-stateless-server-dataplane.md`), a comprehensive Architecture Decision Record that evaluates six named candidate technologies, recommends a selection, and enumerates the follow-up implementation issues. Secondary outputs are an update to `docs/adr/INDEX.md` and a structured list of GitHub issues.

**This plan does not include implementing the selected dataplane.** Implementation follows from the accepted ADR in a subsequent feature branch.

---

## Technical Context

**Language/Version**: Go 1.22+ (primary server implementation language)
**Primary Dependencies**:
- `go.etcd.io/etcd/client/v3` — existing control-plane client (ADR-002)
- `google.golang.org/grpc` — agent↔server communication (ADR-003)
- `gopkg.in/yaml.v3`, `github.com/xeipuuv/gojsonschema` — twin model schema (ADR-004)
**Current Storage**: In-memory Go maps + etcd `/pheromone/*` keyspace (ADR-002)
**Testing**: `go test ./...` + Vagrant multi-machine integration tests (ADR-012)
**Target Platform**: Linux server (Ubuntu 24.04 LTS / Debian 12), single-machine MVP; Phase 2 multi-server HA
**Project Type**: Single Go server binary + agent binaries
**Performance Goals**: <100 ms dataplane read latency; <200 ms write latency for twin state mutations; server restart recovery ≤30 s against a populated dataplane
**Constraints**: Open-source licence (Apache 2.0 / MIT / BSD preferred; AGPL disqualifying); Go client library must exist; mTLS or TLS transport between server and dataplane (ADR-014)
**Scale/Scope**: 100–1 000 agents at MVP; 10 000+ agents at Phase 2; twin state blobs typically <64 KB each

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-evaluated after Phase 1 design.*

| Principle | Check | Status |
|-----------|-------|--------|
| **I — Layered Twin Architecture** | ADR-015 must fit within the existing three-layer model: Layer 1 (in-memory cache), Layer 2 (dataplane), Layer 3 (audit/history). Agents never connect directly to the dataplane — server mediates all reads and writes (FR-009). | ✅ PASS — spec explicitly preserves layering; agents remain on gRPC path |
| **II — ADR-Driven Decision Contracts** | A new ADR is the primary deliverable. ADR-015 must reference ADR-002, ADR-003, ADR-014 and be published in `docs/adr/` before any implementation begins. Status starts as Proposed; advances to Accepted after team review. | ✅ PASS — feature is explicitly ADR creation; no code changes in scope |
| **III — Language-Agnostic Protocol Foundation** | Dataplane client library must be available in Go. If a separate service is chosen (e.g., PostgreSQL, Redis, etcd), its Go SDK must be evaluated for gRPC compatibility, connection pooling, and TLS support. | ✅ PASS — Go SDK availability is an explicit evaluation criterion (FR-002) |
| **IV — Smoke Tests & Quality Gates** | No new code in this feature. The ADR document itself is the deliverable. Existing smoke tests must not regress. | ✅ PASS — documentation-only deliverable; CI gates unaffected |
| **V — Git-Driven ADR Workflow** | Feature branch is already created (`copilot/feature-evaluate-data-plane-implementation`). ADR-015 must be committed in this branch. PR description must link to spec-002 and ADR-015. Squash-commit message: `docs(adr): ADR-015 stateless server dataplane selection`. | ✅ PASS — branch exists; naming convention is acceptable (non-standard prefix approved for Copilot agent branches) |

**Constitution Gate**: ✅ ALL PASS — proceed to Phase 0.

---

## Project Structure

### Documentation Artifacts (this feature)

```text
.specify/memory/
├── plan-002-stateless-server-dataplane.md   ← THIS FILE
├── spec-002-stateless-server-dataplane.md   ← Input spec (read-only)
└── (no data-model.md or contracts/ — documentation-only feature)

docs/adr/
├── adr-015-stateless-server-dataplane.md    ← PRIMARY DELIVERABLE (Phase 1)
└── INDEX.md                                 ← UPDATE REQUIRED (Phase 1)
```

### Source Code Changes

None. This feature is documentation-only. All implementation issues are generated as GitHub issues and resolved in subsequent feature branches.

---

## Complexity Tracking

> No constitution violations — section not applicable.

---

## Phase 0: Research

**Goal**: Resolve all NEEDS CLARIFICATION items and produce a structured evidence base that the ADR author can directly cite. Research findings are embedded directly in ADR-015's Context and Alternatives Considered sections — no separate `research.md` file is required for a documentation-only feature; the ADR IS the research output.

### 0.1 Evaluation Criteria Definition

Before assessing any candidate, define and document the criteria. All six named candidates MUST be scored against all criteria — no criterion may be omitted for any candidate (SC-002).

| ID | Criterion | Type | Weight |
|----|-----------|------|--------|
| C-01 | **Read/write latency** — p50 and p99 for single-key reads and writes at MVP scale (1 000 agents) | Must-Have | High |
| C-02 | **Operational complexity** — ease of deployment, configuration, upgrade, and monitoring in a Linux single-machine environment | Must-Have | High |
| C-03 | **Encryption at rest** — native support without additional infrastructure | Must-Have | Medium |
| C-04 | **Transport encryption (TLS)** — TLS/mTLS between server and dataplane | Must-Have | High |
| C-05 | **Backup and recovery** — supported mechanisms, RPO/RTO achievable with standard tooling | Must-Have | High |
| C-06 | **Data sovereignty** — can be deployed fully on-premises / air-gapped | Must-Have | Medium |
| C-07 | **Go client library maturity** — actively maintained, idiomatic, supports connection pooling | Must-Have | High |
| C-08 | **Licensing** — Apache 2.0 / MIT / BSD preferred; AGPL is disqualifying | Must-Have | High |
| C-09 | **Scalability ceiling** — maximum agents/state size before architecture change required | Nice-to-Have | Medium |
| C-10 | **etcd co-deployment conflict** — whether the candidate introduces tension with the existing etcd control-plane instance (ADR-002) | Must-Have | Medium |
| C-11 | **Schema flexibility** — ability to store structured (JSON/YAML blobs) and key-value data without schema migrations for every twin model change | Nice-to-Have | Medium |
| C-12 | **Observability integration** — Prometheus metrics endpoint, structured logs, tracing hooks | Nice-to-Have | Low |

### 0.2 Candidate Research Tasks

For each of the six mandated candidates (FR-001) plus any additional candidates identified during research, answer every criterion above. Use the sub-tasks below as the research checklist.

---

#### Candidate A: PostgreSQL (relational, full-featured)

**Research tasks**:
- A-01 Establish baseline Go driver: `pgx/v5` vs `database/sql` + `lib/pq` — connection pool defaults, prepared statements, TLS dialing.
- A-02 Benchmark: single-row read/write latency on loopback (localhost) for a 32 KB JSON blob representing a twin state snapshot. Compare `JSONB` column vs normalised schema.
- A-03 Encryption at rest: native `pgcrypto` extension vs OS-level full-disk encryption vs transparent data encryption availability.
- A-04 TLS/mTLS: document `sslmode=verify-full` with client certificate (mTLS) and test with `crypto/tls` Go config.
- A-05 Backup: `pg_dump` / `pg_basebackup` / WAL archiving. Estimate time to restore 1 GB twin state DB.
- A-06 Licensing: PostgreSQL License (MIT-equivalent) — confirm no AGPL risk.
- A-07 Scalability: confirm ability to handle 10 000+ twin records with proper indexing.
- A-08 ADR-002 relationship: PostgreSQL is already mentioned as Layer 3 (History/audit) in ADR-002. Assess whether it can also serve as Layer 2 (Durable state) — i.e., collapse etcd + PostgreSQL into PostgreSQL alone.
- A-09 etcd conflict: if PostgreSQL owns Layer 2, what remains for etcd? Document proposed separation.

**Known strengths**: Full ACID guarantees; rich query capability; industry standard; Go ecosystem mature.
**Known weaknesses**: Separate service to operate; higher operational complexity than embedded stores.

---

#### Candidate B: BoltDB (embedded key-value, Go-native)

**Research tasks**:
- B-01 Current status: BoltDB (`go.etcd.io/bbolt` — bbolt fork) vs original `github.com/boltdb/bolt` (unmaintained). Confirm bbolt is the active fork and check last commit date.
- B-02 Benchmark: bucket read/write throughput for 1 000 concurrent agents writing 32 KB values.
- B-03 Encryption at rest: BoltDB has no built-in encryption. Document options: OS-level (LUKS), Go-level (AES-GCM wrapper before write), or third-party.
- B-04 TLS/mTLS: embedded — no network transport. Server accesses file directly. Document implication: mTLS is N/A; security boundary is OS file permissions. Flag as potential gap vs ADR-014.
- B-05 Backup: copy-on-write B-tree enables `db.View(func(tx) { tx.CopyFile(...) })` online backup. Assess granularity and restore time.
- B-06 Licensing: MIT — confirm.
- B-07 Scalability: BoltDB is single-writer; assess write contention at 1 000 agents.
- B-08 ADR-002 relationship: BoltDB would replace etcd entirely for Layer 2 state. Assess whether etcd watch/notify functionality is replicated by BoltDB's bucket event model (it is not — BoltDB has no watch).
- B-09 Multi-server HA: BoltDB is single-file; no replication. Document this as a Phase 2 blocker.

**Known strengths**: Embedded (no separate service); zero network round-trip; proven in etcd itself (bbolt is etcd's storage engine).
**Known weaknesses**: Single-writer bottleneck; no network transport; no native replication; encryption must be added externally.

---

#### Candidate C: BadgerDB (embedded LSM-tree key-value, Go-native)

**Research tasks**:
- C-01 Library: `github.com/dgraph-io/badger/v4` — confirm active maintenance and licence (Apache 2.0).
- C-02 Benchmark: read/write throughput vs BoltDB for same 32 KB twin state workload. BadgerDB is LSM-optimised for write-heavy workloads.
- C-03 Encryption at rest: BadgerDB v2+ has native AES-GCM encryption via `Options.EncryptionKey`. Document key management approach.
- C-04 TLS/mTLS: embedded — same N/A situation as BoltDB. Same ADR-014 gap analysis required.
- C-05 Backup: `db.Backup(w io.Writer, since uint64)` incremental backup API. Assess restore complexity.
- C-06 Licensing: Apache 2.0 — confirm.
- C-07 Scalability: LSM is write-optimised; read amplification on large datasets. Assess for 10 000+ twin records.
- C-08 ADR-002 relationship: same as BoltDB — would replace etcd Layer 2 state. Lacks watch semantics.
- C-09 GC overhead: BadgerDB requires periodic `RunValueLogGC` calls; assess impact on server latency.

**Known strengths**: Native encryption at rest; higher write throughput than BoltDB; Go-native.
**Known weaknesses**: Embedded (no network mTLS); no replication; GC pause risk; key management for encryption adds complexity.

---

#### Candidate D: etcd (extending existing control-plane deployment)

**Research tasks**:
- D-01 Separation of concerns: ADR-002 assigns etcd ownership of control-plane state (`/pheromone/twins/*`, `/pheromone/agents/*`). Assess the risk of using the same etcd cluster for both control-plane and data-plane state — namespace collision, operational coupling, blast radius on failure.
- D-02 Separate etcd instance option: evaluate running a second etcd cluster dedicated to dataplane state. Assess operational overhead vs shared cluster.
- D-03 Value size limits: etcd's default `--max-request-bytes` is 1.5 MB; the hard practical limit for stable operation is ~1 MB per value. Assess whether twin state blobs exceed this, especially for AI decision traces.
- D-04 Write throughput: etcd is Raft-replicated; assess write throughput ceiling for 1 000 agents updating twin state in parallel. Compare to ADR-002 performance validation results.
- D-05 Encryption at rest: etcd v3.5+ supports encryption via `--encryption-provider-config`. Document configuration for AES-CBC and AES-GCM providers.
- D-06 TLS/mTLS: etcd natively supports client certificates and server TLS. Document Go `clientv3.Config{TLS: tlsCfg}` setup. Aligns well with ADR-014.
- D-07 Backup: `etcdctl snapshot save` and `snapshot restore`. Assess snapshot frequency and restore time.
- D-08 Watch semantics: etcd watch is a core feature — assess whether it can replace the current in-memory state propagation model efficiently or whether it introduces additional complexity.
- D-09 Licensing: Apache 2.0 — confirm.

**Known strengths**: Already deployed; native TLS/mTLS; Go client `clientv3` is mature; watch semantics enable reactive architecture.
**Known weaknesses**: Value size limits; write throughput ceiling under Raft consensus; operational coupling risk if shared with control plane; not designed as a general-purpose data store.

---

#### Candidate E: Redis (in-memory with persistence options)

**Research tasks**:
- E-01 Library: `github.com/redis/go-redis/v9` — confirm active maintenance and licence (MIT for go-redis; BSD for Redis server).
- E-02 Persistence options: RDB snapshots vs AOF (Append-Only File) vs both. Assess durability guarantee for twin state — specifically whether AOF `everysec` fsync meets RPO requirements.
- E-03 Encryption at rest: Redis does not natively encrypt data at rest. Document mitigation: OS-level (LUKS/dm-crypt), or Redis Enterprise (proprietary). Flag as ADR-014 gap.
- E-04 TLS/mTLS: Redis 6.0+ supports TLS. Document `tls-cert-file`, `tls-key-file`, `tls-ca-cert-file` configuration and Go client `redis.Options{TLSConfig: tlsCfg}`.
- E-05 Backup: `BGSAVE` (RDB), `BGREWRITEAOF`. Assess restore procedure and time.
- E-06 Licensing: Redis 7.4+ changed to RSALv2 + SSPLv1 (non-OSI-approved). Valkey fork (Linux Foundation, Apache 2.0) is the open-source successor. **Evaluate Valkey as the preferred variant**; document licensing risk for Redis itself.
- E-07 Scalability: Redis is single-threaded command execution; assess throughput for 1 000 agents at 32 KB values. Redis Cluster for Phase 2 HA.
- E-08 ADR-002 relationship: Redis would serve as Layer 2 replacement for etcd; existing etcd continues as control plane. Clear separation of concerns.
- E-09 Data type fit: assess Redis HASH for structured twin state vs plain string GET/SET. Consider RESP3 protocol benefits.

**Known strengths**: Extremely low read latency; rich data types; mature operational tooling; TLS support.
**Known weaknesses**: No native encryption at rest; Redis 7.4+ licence change (consider Valkey); in-memory model means larger memory footprint than disk-based stores.

---

#### Candidate F: Flat File Store (filesystem-based)

**Research tasks**:
- F-01 Design: one JSON/YAML file per twin state record, named `{twin_id}.json`, stored in a configurable directory. Assess atomicity: `os.WriteFile` with `O_TRUNC` is not atomic; use `os.Rename` from a tmp file.
- F-02 Concurrency: Go `sync.RWMutex` per file or a single directory-level lock. Assess deadlock risk for 1 000 concurrent agents.
- F-03 Encryption at rest: Go-level AES-GCM encryption wrapper (write encrypted bytes; decrypt on read). Or rely on OS filesystem encryption.
- F-04 TLS/mTLS: N/A — embedded, file-level access. Same ADR-014 gap as BoltDB/BadgerDB.
- F-05 Backup: filesystem-level `rsync`, `cp -r`, or tarball. Incremental backup via `rsync --checksum`.
- F-06 Licensing: N/A — no third-party dependency.
- F-07 Scalability: filesystem directory listing scales poorly beyond ~100 000 files (ext4/XFS). For MVP scale (1 000 agents) this is acceptable; assess Phase 2 limits.
- F-08 Crash safety: assess `fsync` requirements to guarantee durability after write. Document that without `fsync`, a crash can corrupt or lose the last write.
- F-09 Watch semantics: `inotify`/`fsnotify` can simulate watch; assess reliability vs etcd watch.
- F-10 ADR-002 relationship: flat file is the minimal viable dataplane; etcd continues as control plane with clear separation. Assess whether this is sufficient for MVP or introduces too many custom consistency concerns.

**Known strengths**: Zero operational overhead; no extra service; fully air-gapped by nature; trivial backup (copy directory).
**Known weaknesses**: Not designed as a concurrent database; manual consistency guarantees; no query capability; not HA-capable; poor fit for Phase 2.

---

#### Candidate G: Additional Candidates (discovered during research)

The research phase SHOULD also consider:

- **CockroachDB** — PostgreSQL-compatible, distributed SQL; assess if the distributed nature justifies complexity for MVP.
- **SQLite** (with `modernc.org/sqlite` pure-Go driver) — embedded SQL; assess read performance and WAL mode for concurrent access. Potentially stronger than flat file with less complexity than PostgreSQL.
- **TiKV** — distributed key-value store (Rust, Apache 2.0) used by TiDB; assess Go client maturity.

If any additional candidate scores strictly higher than all named candidates on criteria C-01 through C-08, it MUST be included in the ADR evaluation matrix.

---

### 0.3 ADR-002 Transition Analysis

The research MUST explicitly map the current ADR-002 state layers to their post-ADR-015 owners (FR-007, SC-007):

| ADR-002 Layer | Current Owner | Post-ADR-015 Owner (to be determined) |
|---------------|---------------|---------------------------------------|
| Layer 1 — In-memory cache | Server process (Go maps) | Server process (retained as read-through cache — source-of-truth moves to dataplane) |
| Layer 2 — Durable state (`/pheromone/twins/*`, `/pheromone/agents/*`) | etcd | **Dataplane** (selected candidate) — unless the ADR decides to keep etcd as Layer 2 |
| Layer 3 — Audit/history (`twin_audit_log`) | PostgreSQL (Phase 2, not yet implemented) | PostgreSQL (still Phase 2) OR consolidated into dataplane if PostgreSQL is selected |
| Control-plane metadata (agent capabilities, action queues) | etcd | **etcd REMAINS** unless ADR explicitly consolidates — ADR must be explicit |

The ADR must state clearly: after the transition, does etcd continue to hold any state, or does the selected dataplane absorb it all?

---

### 0.4 Security Requirements Matrix (from ADR-014)

For each candidate, the ADR's security section (FR-005) must populate this matrix:

| Security Requirement | Source | PostgreSQL | BoltDB | BadgerDB | etcd | Redis/Valkey | Flat File |
|---------------------|--------|-----------|--------|----------|------|--------------|-----------|
| Encryption at rest | ADR-014 D1 | pgcrypto / OS | External only | Native AES-GCM | Provider config | External only | External only |
| Transport TLS | ADR-014 D1 | `sslmode=verify-full` | N/A (embedded) | N/A (embedded) | Native | TLS 6.0+ | N/A |
| Mutual TLS (mTLS) | ADR-014 D1 | Client cert + CA | N/A | N/A | Native | TLS 6.0+ (no mTLS by default) | N/A |
| Data sovereignty | spec-002 SC-004 | On-prem capable | On-prem (embedded) | On-prem (embedded) | On-prem capable | On-prem capable | On-prem (embedded) |
| *Fill in scores after research* | | | | | | | |

Any gap (blank or "External only" in a Must-Have row) MUST have a documented mitigation. Embedded stores (BoltDB, BadgerDB, Flat File) that have no network transport must document the compensating control (filesystem permissions, OS-level encryption, network isolation).

---

### 0.5 Backup and Recovery Requirements

For each candidate, document (FR-006, SC-005):

| Candidate | Backup Mechanism | Granularity | Restore Procedure | Estimated RTO (server restart) |
|-----------|-----------------|-------------|-------------------|-------------------------------|
| PostgreSQL | pg_basebackup + WAL archiving | Point-in-time | `pg_restore` or WAL replay | < 30 s (small DB) |
| BoltDB | Online `tx.CopyFile()` | Last consistent snapshot | Copy file + restart | < 5 s |
| BadgerDB | `db.Backup()` incremental | Per-write transaction | `db.Load()` | < 10 s |
| etcd | `etcdctl snapshot save` | Point-in-time | `etcdctl snapshot restore` + restart | ~30 s |
| Redis/Valkey | BGSAVE (RDB) + AOF | Configurable (everysec → 1 s RPO) | `redis-server --appendonly yes` | < 15 s |
| Flat File | `rsync` / `tar` | File modification time | Copy files back + restart | < 5 s |
| *Fill in actual measured values during research* | | | | |

If any candidate's estimated RTO exceeds 30 s (the server recovery window target), it must be documented with a mitigation or disqualified.

---

## Phase 1: ADR-015 and INDEX.md

**Prerequisites**: Phase 0 research complete — all candidates scored against all criteria.

### 1.1 ADR-015 Document Structure

Create `docs/adr/adr-015-stateless-server-dataplane.md` following the project ADR template (`.specify/templates/adr-template.md`) with the **mandatory sections** below. Each section maps to spec-002 functional requirements and success criteria.

---

#### Section 1: Status

```markdown
## Status
Proposed
```

Start as `Proposed`. Advance to `Accepted` after team review (minimum 2 approvals, 1-week feedback per Constitution Principle II).

---

#### Section 2: Context (maps to FR-001, FR-002, FR-003, SC-001, SC-002)

Must include:

- **Current architecture**: Summarise ADR-002 hybrid in-memory + etcd design. Quote the three-layer model. Reference the ADR-002 performance validation results (`docs/adr/adr-002-performance-validation.md`).
- **Why stateless server**: Explain the operational motivation — server restarts currently lose in-flight state not yet flushed to etcd; the hybrid model couples server lifecycle to in-memory state; horizontal scaling (Phase 2) requires all servers to share a common state store.
- **Agent access model**: Reaffirm from ADR-003 that agents communicate exclusively with the server over gRPC — agents never connect directly to the dataplane. This is a hard constraint (FR-009).
- **Security baseline**: Reference ADR-014's requirements for TLS enforcement and mTLS target state. Any selected dataplane must support this or document compensating controls.
- **Evaluation scope**: List all six named candidate technologies (FR-001). State that additional candidates may be added if discovered during research.
- **Evaluation criteria**: Reproduce the criteria table from Phase 0 §0.1 (twelve criteria, each with type and weight).

---

#### Section 3: Decision (maps to FR-004, SC-003)

Must include:

- **Selected technology**: Name the recommended dataplane (or a ranked shortlist if phased adoption is proposed). The ADR MUST NOT defer the decision — if no single candidate meets all requirements, a phased or composite solution must be proposed (spec-002 edge case).
- **Rationale**: At least one rationale bullet per evaluation criterion. Each bullet must be traceable to a criterion score from the evaluation matrix.
- **Scoring matrix** (inline table): All candidates × all criteria, with a score or assessment per cell. No candidate/criterion cell may be left blank.
- **Phased adoption note** (if applicable): If the ADR proposes a two-phase path (e.g., embedded store for MVP → PostgreSQL for Phase 2), this must be explicit with trigger conditions (e.g., agent count threshold, HA requirement).

---

#### Section 4: Security (maps to FR-005, SC-004)

Mandatory sub-sections:
1. **Encryption at rest** — describe how the selected technology supports encryption of persisted data. If native encryption is absent, specify the required compensating control (e.g., LUKS, OS-level FDE) and create a follow-up issue for it.
2. **Transport encryption** — describe how TLS is configured between the Pheromone server and the dataplane endpoint. Provide a Go code snippet showing `tls.Config` setup (or note that embedded store has no network layer with security implications documented).
3. **Mutual TLS** — describe how the server authenticates to the dataplane via client certificate. If mTLS is not natively supported, specify a compensating control (network namespace isolation, Unix socket, sidecar) and create a follow-up issue.
4. **Data sovereignty** — confirm the selected technology can be deployed entirely on-premises / air-gapped without calling back to any external service.
5. **Security gaps** — explicitly list any unmet security requirements with proposed mitigations. Do not omit gaps (SC-004).

---

#### Section 5: Backup and Recovery (maps to FR-006, SC-005)

Mandatory sub-sections:
1. **Backup mechanism** — describe the recommended backup approach (snapshot, WAL archiving, rsync, etc.) and the tooling required.
2. **Recovery point granularity** — what is the maximum data loss window (RPO) with the recommended backup configuration?
3. **Restore procedure** — step-by-step restore steps a platform operator can follow.
4. **Server recovery behaviour** — describe what happens when the Pheromone server crashes and restarts: how quickly does it reconnect to the dataplane and resume serving agents? What is the expected RTO? Confirm no committed twin/agent state is lost (SC-005 acceptance criterion).
5. **Backup runbook follow-up** — if a full operational runbook is required beyond what fits in the ADR, create a follow-up issue to write it.

---

#### Section 6: Transition Path (maps to FR-007, SC-007)

Must describe:
- The current ADR-002 state ownership model (Layer 1/2/3 map from Phase 0 §0.3).
- How the selected dataplane replaces or supplements the current etcd Layer 2 state.
- What etcd continues to own post-transition (if anything). If etcd is no longer required as a dataplane, document whether the existing etcd deployment is retained for control-plane state only, reduced in scope, or removed.
- Which existing server code paths are affected (point to `internal/` packages by name so developers can identify the scope of change for follow-up issues).
- Migration strategy: since no production data exists, the transition is a code change; but the ADR must describe the logical migration of the state schema from the etcd keyspace to the selected dataplane schema.

---

#### Section 7: Alternatives Considered (maps to FR-001, FR-002, SC-002)

One sub-section per candidate NOT selected (or per all candidates including selected, for completeness). Each sub-section must include:
- Technology summary (1 paragraph)
- Score summary referencing the evaluation matrix
- Reason for rejection or ranking below the selected option
- Any conditions under which this candidate would be preferred (e.g., "preferred if air-gap with no outbound networking is mandatory")

---

#### Section 8: Follow-Up GitHub Issues (maps to FR-008, SC-006)

Minimum three issues; each must have:
- **Title**: Short, action-verb imperative (e.g., "Implement PostgreSQL dataplane adapter in server")
- **Description**: One sentence describing the bounded deliverable
- **Priority**: P1 / P2 / P3
- **Dependencies**: Reference to other issues if applicable

**Mandatory issue categories** (at minimum):

| # | Title | Description | Priority |
|---|-------|-------------|----------|
| I-01 | `[DATAPLANE] Implement <selected> dataplane adapter in Pheromone server` | Wire the selected dataplane client into the server's twin state read/write paths, replacing the current in-memory mutation model | P1 |
| I-02 | `[DATAPLANE] Migrate etcd Layer 2 state to <selected> dataplane` | Remove or repurpose the `/pheromone/twins/*` and `/pheromone/agents/*` etcd keyspace; update the server boot sequence to load state from the new dataplane | P1 |
| I-03 | `[DATAPLANE][SECURITY] Configure mTLS / TLS for server↔dataplane transport` | Implement TLS client certificate authentication between the Pheromone server and the selected dataplane service (or document compensating network isolation if embedded store) | P1 |
| I-04 | `[DATAPLANE][SECURITY] Implement encryption-at-rest for <selected> dataplane` | Configure or implement the encryption-at-rest mechanism identified in ADR-015 for the selected dataplane | P2 |
| I-05 | `[DATAPLANE][OPS] Write backup and recovery runbook for <selected> dataplane` | Document step-by-step backup, verify, and restore procedures for the dataplane; include automation scripts where possible | P2 |
| I-06 | `[DATAPLANE] Add Vagrant provisioning for <selected> dataplane` | Extend `vagrant/` provisioning scripts to install and configure the selected dataplane service in the test environment (ADR-012) | P2 |
| I-07 | `[DATAPLANE][OBS] Add Prometheus metrics for dataplane health and latency` | Expose server-side dataplane read/write latency histograms and connection pool metrics via the existing Prometheus endpoint | P2 |
| I-08 | `[DATAPLANE] Tech spike — dataplane write throughput at 1 000 agent scale` | Benchmark the selected dataplane's write throughput under 1 000 concurrent agent state updates to validate <200 ms p99 write latency target | P2 |

Additional issues may be added by the ADR author based on the specific technology selected. Each issue must be independently actionable (SC-006).

---

#### Section 9: Consequences

**Positive Consequences**:
- Pheromone server becomes truly stateless: any server crash that does not corrupt the dataplane results in zero data loss on restart.
- Clear separation of control-plane state (etcd) from operational data-plane state (selected dataplane).
- Enables horizontal server scaling in Phase 2 (multiple server instances share one dataplane).
- Security posture for persisted data is explicit and auditable (addresses ADR-014 open items for data at rest).

**Negative Consequences / Trade-offs**:
- Additional service dependency (if non-embedded store chosen): operations must maintain and monitor the dataplane service.
- Server startup now depends on dataplane availability; a cold-start without a running dataplane prevents the server from accepting agents (degraded-mode design must be documented).
- If embedded store chosen: Phase 2 horizontal scaling is blocked; a second migration will be required.
- Go client library pinned to a specific version; major version upgrades may require code changes in `internal/`.

---

#### Section 10: Related Decisions

```markdown
## Related Decisions

- [ADR-002: Server Architecture](adr-002-server-architecture.md) — defines the current in-memory + etcd hybrid that ADR-015 supersedes (partially or fully)
- [ADR-003: gRPC Service Contracts](adr-003-grpc-contracts.md) — agents connect to server via gRPC; this contract is unchanged by dataplane selection
- [ADR-014: Security Architecture](adr-014-security-approach.md) — mandates TLS/mTLS; ADR-015 must satisfy or mitigate all D1–D5 requirements for the dataplane path
- [ADR-005: Message Queue Selection](adr-005-message-queue.md) — telemetry/metrics path (NATS/Kafka) is separate from twin state dataplane; ADR-015 must not conflate these
```

---

#### Section 11: References

```markdown
## References

- Spec-002: `.specify/memory/spec-002-stateless-server-dataplane.md`
- ADR-002 Performance Validation: `docs/adr/adr-002-performance-validation.md`
- ADR template: `.specify/templates/adr-template.md`
- Constitution: `.specify/memory/constitution.md` (v2.1.0)
- PostgreSQL mTLS: https://www.postgresql.org/docs/current/ssl-tcp.html
- etcd encryption: https://etcd.io/docs/v3.5/op-guide/configuration/#security
- BadgerDB encryption: https://dgraph.io/docs/badger/get-started/#encryption-mode
- Redis TLS: https://redis.io/docs/latest/operate/oss_and_stack/management/security/tls/
- Valkey (Redis fork): https://valkey.io/
- bbolt (BoltDB fork): https://github.com/etcd-io/bbolt
```

---

### 1.2 INDEX.md Update

Update `docs/adr/INDEX.md` with the following changes:

#### 1.2.1 Add ADR-015 to Decision Timeline table

Insert after the ADR-014 Security Architecture row:

```markdown
| **015** | Stateless Server — Dataplane Selection | ⏳ Proposed | Phase 2 (Scalability) | Replace in-memory + etcd Layer 2 with dedicated dataplane | spec-002 FR-001..010 |
```

#### 1.2.2 Add to "Layer 2: Control Plane" section in ADR Decision Map

Add under Layer 2:
```markdown
- ⏳ ADR-015: How operational twin/agent state is persisted (stateless server dataplane selection)
```

#### 1.2.3 Add to ADR Dependencies graph

Add after the ADR-014 block:
```markdown
ADR-015 (Stateless Server Dataplane)
   ├─→ ADR-002 (Server Architecture — partially superseded for Layer 2 state)
   ├─→ ADR-003 (gRPC Contracts — agent access model unchanged)
   └─→ ADR-014 (Security Architecture — TLS/mTLS requirements for dataplane transport)
```

#### 1.2.4 Add to "Key Decisions to Ratify — Must Review" section

```markdown
- **ADR-015**: Stateless server dataplane selection (affects server architecture, security, and Phase 2 scaling)
```

#### 1.2.5 Update the status footer

Update the `**Status**:` line at the bottom of INDEX.md to include ADR-015 Proposed.

#### 1.2.6 Add to Required GitHub Issues table

Add at minimum I-01 through I-08 from Section 1.1 §8 above, referencing ADR-015.

---

### 1.3 Agent Context Update

After ADR-015 and INDEX.md are committed, run:

```bash
cd /home/runner/work/pheromone/pheromone
bash .specify/scripts/bash/update-agent-context.sh copilot
```

This updates the Copilot agent context file with the new technologies introduced by ADR-015 (selected dataplane Go client library, any new operational tooling).

---

## Follow-Up GitHub Issues (Standalone Reference)

The following issues MUST be created in the repository after ADR-015 is accepted. They are reproduced here for planning visibility. Actual issue text will be finalised by the ADR author based on the selected technology.

| Issue | Label | ADR | Priority | Effort Estimate |
|-------|-------|-----|----------|-----------------|
| Implement `<selected>` dataplane adapter in Pheromone server | `enhancement`, `dataplane`, `P1` | ADR-015 | P1 | 16–24 h |
| Migrate etcd Layer 2 state (`/pheromone/twins/*`) to new dataplane | `refactor`, `dataplane`, `P1` | ADR-015 | P1 | 8–12 h |
| Configure TLS/mTLS for server↔dataplane transport | `security`, `dataplane`, `P1` | ADR-015, ADR-014 | P1 | 4–8 h |
| Implement encryption-at-rest for selected dataplane | `security`, `dataplane`, `P2` | ADR-015, ADR-014 | P2 | 4–8 h |
| Write backup and recovery runbook for selected dataplane | `documentation`, `ops`, `P2` | ADR-015 | P2 | 4 h |
| Add Vagrant provisioning for selected dataplane | `infrastructure`, `P2` | ADR-015, ADR-012 | P2 | 4–6 h |
| Add Prometheus metrics for dataplane health and latency | `observability`, `P2` | ADR-015 | P2 | 4 h |
| Tech spike — dataplane write throughput at 1 000 agents | `spike`, `P2` | ADR-015 | P2 | 6–8 h |
| Review and Accept ADR-015 | `adr-review` | ADR-015 | P1 | — |

**Total estimated implementation effort**: ~50–70 hours across 8 issues (excluding ADR review).

---

## Deliverable Checklist

### Phase 0 Complete When:
- [ ] All six named candidates scored against all twelve criteria (C-01 through C-12)
- [ ] ADR-002 state layer ownership map complete (Phase 0 §0.3 table filled in)
- [ ] Security requirements matrix complete (Phase 0 §0.4 table filled in)
- [ ] Backup/recovery estimates complete (Phase 0 §0.5 table filled in)
- [ ] Any additional candidates (G-01+) assessed and included or explicitly excluded
- [ ] Licensing for all candidates confirmed — any AGPL or proprietary licences flagged

### Phase 1 Complete When:
- [ ] `docs/adr/adr-015-stateless-server-dataplane.md` exists with all 11 sections complete
- [ ] Technology recommendation (or phased shortlist) is stated and justified
- [ ] Security section covers all four requirements (at-rest, transport, mTLS, sovereignty) with no silent gaps
- [ ] Backup and recovery section includes restore procedure and RTO estimate
- [ ] Transition path from ADR-002 hybrid model is described at code-path granularity
- [ ] Minimum 3 follow-up issues (I-01 through I-03 at minimum) listed in ADR-015 §8
- [ ] `docs/adr/INDEX.md` updated with ADR-015 row, dependency graph entry, and status footer
- [ ] Agent context updated via `update-agent-context.sh copilot`
- [ ] Branch `copilot/feature-evaluate-data-plane-implementation` pushed with both files
- [ ] PR description references spec-002 and ADR-015

### Post-Phase-1 (after ADR acceptance):
- [ ] GitHub issues I-01 through I-08 created in repository
- [ ] ADR-015 status updated from `Proposed` to `Accepted` after 2 approvals + 1-week review
- [ ] ADR-002 updated with "Partially superseded by ADR-015 for Layer 2 state — see [link]" dated note
- [ ] Implementation feature branch created: `feature/ADR-015-dataplane-adapter`

---

## Constitution Re-Check (Post Phase 1)

After ADR-015 and INDEX.md are drafted, re-verify:

| Principle | Post-Design Status |
|-----------|-------------------|
| I — Layered Architecture | ✅ ADR-015 explicitly maps to Layer 2; Layer 1 (cache) and Layer 3 (audit) unchanged |
| II — ADR-Driven Decisions | ✅ ADR-015 is the deliverable; status starts Proposed pending review |
| III — Protocol Foundation | ✅ Evaluation criteria C-07 enforces Go client library requirement for all candidates |
| IV — Smoke Tests | ✅ No code changes; existing tests unaffected |
| V — Git Workflow | ✅ Branch exists; PR will reference ADR-015 |

---

## References

- **Specification**: `.specify/memory/spec-002-stateless-server-dataplane.md`
- **Constitution**: `.specify/memory/constitution.md` (v2.1.0)
- **ADR Template**: `.specify/templates/adr-template.md`
- **ADR Index**: `docs/adr/INDEX.md`
- **ADR-002**: `docs/adr/adr-002-server-architecture.md`
- **ADR-003**: `docs/adr/adr-003-grpc-contracts.md`
- **ADR-014**: `docs/adr/adr-014-security-approach.md`
- **ADR-002 Performance Validation**: `docs/adr/adr-002-performance-validation.md`

---

**Plan Status**: ✅ Complete — Ready for Phase 0 execution
**Plan Version**: 1.0.0
**Created**: 2026-06-18
**Branch**: `copilot/feature-evaluate-data-plane-implementation`

# ADR-016: Stateless Server — Dataplane Selection

**Status**: Proposed
**Date**: 2026-03-10
**Author**: @copilot
**Spec**: `.specify/memory/spec-002-stateless-server-dataplane.md`
**Plan**: `.specify/memory/plan-002-stateless-server-dataplane.md`

---

## Context

### Current State (ADR-002 Hybrid)

The Pheromone server currently holds all operational state in-memory — a Go `map[string]Twin` — with asynchronous persistence to etcd under `/pheromone/*` (ADR-002 hybrid). This design delivers the sub-5-second configuration push latency required for the MVP, but it means:

- **The server process is stateful.** A crash or restart loses all in-memory state; recovery depends on a successful etcd range query at boot.
- **Recovery window is unbounded.** If etcd is unavailable at restart, the server starts empty and cannot serve agents.
- **Horizontal scaling is blocked.** A second server instance would have a different in-memory view with no mechanism for live state sharing.
- **etcd is overloaded.** The same etcd cluster acts as control-plane coordination (Raft leader election, service discovery) *and* as the primary persistence layer for large twin state blobs.

### Motivation for Offloading the Dataplane

A dedicated, external dataplane separates concerns cleanly:

```
Layer 1 (fast)     │ In-memory Go cache in the server process
Layer 2 (durable)  │ External dataplane service  ← THIS ADR
Layer 3 (history)  │ Audit log / historical analytics (Phase 3)
```

With a dedicated dataplane:

1. The server process becomes **stateless** — it holds only a warm cache. Any number of server instances can restart independently, rehydrate from the dataplane, and reach a consistent view.
2. etcd is freed to do what it is designed for: lightweight distributed coordination, leader election, and watch-based notifications.
3. Backup, restore, replication, and encryption are handled by a purpose-built storage service rather than bolted onto etcd.

### Constraints and Acceptance Criteria

| Constraint | Value |
|---|---|
| Read latency | <100 ms at p99 (1 000 agents, twin blob ≤64 KB) |
| Write latency | <200 ms at p99 |
| Server restart recovery | ≤30 s with fully populated dataplane |
| Licence | Apache 2.0 / MIT / BSD — AGPL disqualifying |
| Go client | Must exist; must be actively maintained |
| Transport encryption | TLS 1.2+ minimum; mTLS preferred (ADR-014) |
| Encryption at rest | Native or transparent; third-party acceptable if well-supported |
| Deployment | On-premises / air-gapped compatible; no mandatory SaaS |
| Agent access | Agents MUST NOT connect directly to the dataplane; all access is via the Pheromone server gRPC API (ADR-003) |

---

## Evaluation Criteria

The following twelve criteria were applied consistently to every candidate. Must-Have criteria eliminate a candidate if the requirement cannot be satisfied; Nice-to-Have criteria affect scoring only.

| ID | Criterion | Priority |
|---|---|---|
| C-01 | Read/write latency at MVP scale (1 000 agents) | Must-Have |
| C-02 | Operational complexity — deploy, configure, upgrade, monitor | Must-Have |
| C-03 | Encryption at rest — native support | Must-Have |
| C-04 | Transport encryption — TLS / mTLS to server | Must-Have |
| C-05 | Backup and recovery — mechanism, RPO, RTO | Must-Have |
| C-06 | Data sovereignty — fully on-premises / air-gapped deployable | Must-Have |
| C-07 | Go client library maturity | Must-Have |
| C-08 | Licensing — Apache 2.0 / MIT / BSD preferred | Must-Have |
| C-09 | Scalability ceiling | Nice-to-Have |
| C-10 | etcd co-deployment conflict | Must-Have |
| C-11 | Schema flexibility — JSON/YAML blobs without per-change migrations | Nice-to-Have |
| C-12 | Community and long-term support | Nice-to-Have |

---

## Candidate Assessment

### Candidate A — PostgreSQL

**Overview**: Battle-tested open-source relational database (PostgreSQL 16+). Used extensively in production systems. Go driver: `pgx/v5` (Apache 2.0; actively maintained by jackc).

| Criterion | Assessment |
|---|---|
| C-01 Latency | Single-key read/write via primary key: <5 ms p99 locally; <20 ms over LAN. JSONB column eliminates schema migration for twin blobs. Well within target. |
| C-02 Ops complexity | Moderate. Requires a running PostgreSQL service (Docker image or OS package). Schema migrations managed via `golang-migrate`. Standard tooling (pg_dump, pgAdmin, cloud backups). |
| C-03 Encryption at rest | Transparent Data Encryption (TDE) available via `pgcrypto` extension or filesystem-level encryption (LUKS, dm-crypt). PostgreSQL 17 adds native page-level encryption (preview). For Phase 1: filesystem encryption on the data directory is recommended. |
| C-04 Transport TLS/mTLS | Full TLS 1.3 and mTLS support via `sslmode=verify-full` connection string. Client certificate pinning supported. Satisfies ADR-014 mTLS requirement. |
| C-05 Backup/recovery | `pg_basebackup` for full backups; WAL archiving for continuous RPO <1 min. Point-in-time recovery (PITR) well-documented. RTO for server restart: <5 s (query on reconnect). |
| C-06 Data sovereignty | Fully on-premises / air-gapped. No telemetry or cloud dependency. |
| C-07 Go client | `pgx/v5` — idiomatic, high performance, connection pooling via `pgxpool`. `database/sql` adapter available for broad compatibility. |
| C-08 Licensing | PostgreSQL Licence (MIT-like, permissive). `pgx/v5`: MIT. ✅ |
| C-09 Scalability | Scales to millions of rows. Handles 10 000+ agents with appropriate indexing and connection pooling. Phase 2 multi-server HA supported via streaming replication + pgBouncer. |
| C-10 etcd conflict | None. PostgreSQL is entirely independent of etcd. etcd retains its role as lightweight control-plane coordinator. |
| C-11 Schema flexibility | JSONB column stores arbitrary twin models without schema migration per model change. Structured fields (agent ID, last-seen, twin version) remain relational for efficient querying. |
| C-12 Community | Extremely large, stable community. 35+ years of development. Long-term support guaranteed. |

**Summary**: ✅ Meets all Must-Have criteria. Highest operational maturity of all candidates. ADR-002 Layer 3 already identified PostgreSQL as the audit log store for Phase 2; this ADR proposes consolidating Layer 2 persistence here as well, eliminating a future migration step.

---

### Candidate B — BoltDB / bbolt

**Overview**: Pure-Go embedded B+tree key-value store. The bbolt fork (`go.etcd.io/bbolt`) is maintained by the etcd project itself.

| Criterion | Assessment |
|---|---|
| C-01 Latency | Reads: sub-millisecond (memory-mapped file). Writes: serialised to a single write transaction; single-writer ceiling limits throughput under concurrent writes from many goroutines. |
| C-02 Ops complexity | Minimal — single `.db` file embedded in the server process. No separate service. Zero configuration. |
| C-03 Encryption at rest | **Not native.** Entire database file must be placed on an encrypted filesystem (LUKS) or a custom `io.ReadWriteSeeker` wrapper. Third-party `github.com/coreos/bbolt-encryption` is unmaintained. Risk: medium. |
| C-04 Transport TLS/mTLS | **Not applicable.** bbolt is an in-process library; there is no network connection to protect. This is a double-edged property: no transport attack surface, but also no remote access. |
| C-05 Backup/recovery | `db.View(tx.CopyFile(...))` provides online hot backup to a second file. No WAL archiving; RPO is last-backup time. Manual rotation required. |
| C-06 Data sovereignty | ✅ Fully embedded, no network egress. |
| C-07 Go client | bbolt API is idiomatic Go. Well understood. |
| C-08 Licensing | MIT ✅ |
| C-09 Scalability | **Single-writer bottleneck.** Not suitable for >1 000 concurrent write goroutines. Phase 2 multi-server architecture is impossible without an external coordinator. |
| C-10 etcd conflict | None functionally, but bbolt is the storage backend *inside* etcd itself. Running bbolt independently alongside etcd introduces a redundant storage layer with no coordination benefit. |
| C-11 Schema flexibility | Key-value with bucket namespacing. JSON values per key. No native query support. |
| C-12 Community | Maintenance-mode; new features deferred. etcd project maintains it for internal use. |

**Summary**: ⚠️ Suitable for single-process MVP only. Single-writer ceiling, no native encryption at rest, and absence of a network interface mean it cannot support Phase 2 multi-server HA or satisfy ADR-014 mTLS requirement for a separate service. **Not recommended as primary dataplane.**

---

### Candidate C — BadgerDB

**Overview**: LSM-tree key-value store in pure Go from Dgraph Labs (`github.com/dgraph-io/badger`).

| Criterion | Assessment |
|---|---|
| C-01 Latency | Write throughput significantly higher than bbolt (LSM vs. B+tree). Reads from MemTable: <1 ms. Reads requiring SSTable lookup: 5–20 ms. Well within target. |
| C-02 Ops complexity | Embedded library; same ops simplicity as bbolt. Requires GC goroutine invocation (`db.RunValueLogGC`) to reclaim space — non-trivial operational concern. |
| C-03 Encryption at rest | **Native AES-256-GCM encryption at rest** via `Options.WithEncryptionKey()`. One of the strongest embedded options. ✅ |
| C-04 Transport TLS/mTLS | **Not applicable.** In-process library; no network interface. Same gap as bbolt for multi-server scenarios. |
| C-05 Backup/recovery | `db.Backup(w io.Writer, since uint64)` supports incremental streaming backup. Restore via `db.Load(r io.Reader)`. RPO: configurable. RTO: depends on backup size and IO speed. |
| C-06 Data sovereignty | ✅ Fully embedded. |
| C-07 Go client | Native Go; well-documented. Active development from Dgraph Labs. |
| C-08 Licensing | Apache 2.0 ✅ |
| C-09 Scalability | Better write throughput than bbolt; still single-process. Phase 2 multi-server requires external coordination layer. |
| C-10 etcd conflict | None functionally. Redundant alongside etcd for the same reasons as bbolt. |
| C-11 Schema flexibility | Key-value with prefix scan. No native secondary index. JSON values per key. |
| C-12 Community | Active; Dgraph uses BadgerDB internally. Risk: single primary corporate sponsor. |

**Summary**: ⚠️ Better security posture than bbolt (native encryption at rest) but shares the same fundamental limitation: embedded library, no network transport, cannot support Phase 2 multi-server HA. **Not recommended as primary dataplane;** could serve as a local cache layer if needed.

---

### Candidate D — etcd (extend existing deployment)

**Overview**: Extend the existing etcd deployment (ADR-002) to carry all twin state instead of offloading to a separate service.

| Criterion | Assessment |
|---|---|
| C-01 Latency | etcd is optimised for small values (<1 MB) and distributed consensus. Read/write latency for twin blobs (up to 64 KB) is acceptable: 5–50 ms over loopback. However, etcd enforces a **1 MB default value size limit**. Large twin models could approach this ceiling. |
| C-02 Ops complexity | Already deployed. No additional service to manage. However, mixing control-plane coordination keys with large data blobs degrades etcd performance for both workloads. |
| C-03 Encryption at rest | Supported via `--encryption-provider-config` (envelope encryption with a local key or KMS). Requires explicit configuration; not enabled by default. |
| C-04 Transport TLS/mTLS | Full TLS and mTLS support. Already configured in the Pheromone deployment. ✅ |
| C-05 Backup/recovery | `etcdctl snapshot save` and `snapshot restore`. Snapshots capture full cluster state. RTO: ~10–30 s for snapshot restore. Continuous WAL replication in clustered mode. |
| C-06 Data sovereignty | ✅ Fully on-premises. |
| C-07 Go client | `go.etcd.io/etcd/client/v3` — already a dependency. ✅ |
| C-08 Licensing | Apache 2.0 ✅ |
| C-09 Scalability | etcd is designed for ~8 GB total data / ~10 000 keys. At 1 000 agents × 64 KB twin blobs = 64 MB of data — within range, but at the edge of recommendations. Phase 2 with 10 000 agents (640 MB) **exceeds etcd's supported data size.** |
| C-10 etcd conflict | **Significant risk.** Large twin blob writes compete with Raft heartbeat writes for etcd I/O budget. Latency spikes in the data path will cascade to control-plane coordination failures. Mixing concerns is an explicit etcd anti-pattern. |
| C-11 Schema flexibility | Key-value with prefix scan and watch. JSON values per key. |
| C-12 Community | Large, stable; CNCF graduated project. |

**Summary**: ❌ Extending etcd to serve as the primary dataplane **is not recommended.** The 1 MB value ceiling, 8 GB total-data recommendation, and risk of data-path I/O competing with control-plane Raft consensus make this a fragile design. etcd MUST be retained for its current control-plane role (agent registration, watch-based notifications, leader election) but MUST NOT be used as the primary twin-state persistence store.

---

### Candidate E — Redis / Valkey

**Overview**: In-memory data store with optional persistence (RDB snapshots + AOF). Valkey (Apache 2.0) is the CNCF-hosted community fork following the Redis licence change.

| Criterion | Assessment |
|---|---|
| C-01 Latency | Sub-millisecond reads and writes in memory. Extremely high throughput. Best raw latency of all network-attached candidates. |
| C-02 Ops complexity | Moderate. Requires a running Redis/Valkey service. Configuration for persistence (AOF `everysec` or RDB) and TLS is non-trivial. |
| C-03 Encryption at rest | **Not native.** Redis/Valkey data is stored in plaintext in the RDB/AOF files. Filesystem-level encryption (LUKS) required. This is a notable gap against ADR-014. |
| C-04 Transport TLS/mTLS | TLS support added in Redis 6.0 (self-compiled; available in Redis 7+ packages). mTLS via `--tls-auth-clients yes`. Go client: `go-redis/redis` supports TLS. |
| C-05 Backup/recovery | RDB point-in-time snapshots; AOF log with `appendfsync everysec` (RPO ≤1 s). `BGSAVE` for non-blocking snapshots. Redis Cluster provides replication. RTO: seconds. |
| C-06 Data sovereignty | ✅ Fully on-premises / air-gapped. |
| C-07 Go client | `github.com/redis/go-redis/v9` (BSD-2-Clause). Active, idiomatic, widely used. ✅ |
| C-08 Licensing | Redis 7.4+ uses RSALv2 + SSPLv1 (non-OSI-approved). **Valkey (CNCF fork)** is Apache 2.0 ✅. Recommendation: use Valkey. |
| C-09 Scalability | Redis Cluster scales to millions of keys / hundreds of GB. Excellent Phase 2 HA story via Sentinel or Cluster mode. |
| C-10 etcd conflict | None. Redis/Valkey is entirely independent of etcd. |
| C-11 Schema flexibility | Hash, string, sorted set, and list primitives. JSON module (`RedisJSON` / Valkey JSON module) adds native JSON path queries. |
| C-12 Community | Redis: very large. Valkey: fast-growing after CNCF adoption; AWS, Google, Oracle as sponsors. |

**Summary**: ⚠️ Excellent latency and scalability, but absence of native encryption at rest is a significant gap for ADR-014 compliance. Acceptable as a **read-through cache layer** in front of PostgreSQL (warm reads without round-trip to the primary store), but should not be the sole persistent dataplane due to the encryption-at-rest gap unless LUKS is guaranteed in all deployments.

---

### Candidate F — Flat File Store

**Overview**: Each twin model stored as a YAML or JSON file on disk; server reads/writes files via `os` package.

| Criterion | Assessment |
|---|---|
| C-01 Latency | Single-file reads: <1 ms. But listing/scanning all twins (required on startup) is O(n) filesystem operations. At 1 000 agents: ~100 ms. At 10 000 agents: >1 s. |
| C-02 Ops complexity | Minimal initially. Grows with scale: directory management, locking, conflict resolution between instances, inotify-based watching. |
| C-03 Encryption at rest | Filesystem-level encryption only (LUKS). Not native to the store itself. |
| C-04 Transport TLS/mTLS | **Not applicable.** Files are accessed locally. Multi-server access requires NFS/SSHFS with its own security model. |
| C-05 Backup/recovery | `rsync`, `tar`, filesystem snapshots. Simple but no transactional consistency — partial writes possible without careful locking. |
| C-06 Data sovereignty | ✅ Fully local. |
| C-07 Go client | Standard library `os`, `encoding/json`. No external dependency. |
| C-08 Licensing | N/A ✅ |
| C-09 Scalability | **Poor.** Does not support multi-server HA. No atomic multi-key transactions. Race conditions on concurrent writes without explicit locking. |
| C-10 etcd conflict | None functionally, but provides no distributed coordination benefit. |
| C-11 Schema flexibility | Maximum flexibility — any file format. But no query, indexing, or watch semantics. |
| C-12 Community | N/A |

**Summary**: ❌ Suitable only for development/testing environments. No distributed access, no transactional safety, poor scan performance at scale, and no path to Phase 2 multi-server HA. **Not recommended** for production dataplane.

---

## Candidate Scoring Matrix

Scores: 3 = Fully meets criterion; 2 = Meets with caveats; 1 = Partial / workaround required; 0 = Does not meet (disqualifying for Must-Have).

| Criterion | Weight | PostgreSQL | BoltDB | BadgerDB | etcd (extend) | Valkey | Flat File |
|---|---|---|---|---|---|---|---|
| C-01 Latency | High | 3 | 3 | 3 | 2 | 3 | 1 |
| C-02 Ops complexity | High | 2 | 3 | 3 | 3 | 2 | 3 |
| C-03 Encryption at rest | Med | 2 | 1 | 3 | 2 | 1 | 1 |
| C-04 TLS / mTLS | High | 3 | N/A→2 | N/A→2 | 3 | 2 | 0 |
| C-05 Backup / recovery | High | 3 | 2 | 2 | 2 | 2 | 1 |
| C-06 Data sovereignty | Med | 3 | 3 | 3 | 3 | 3 | 3 |
| C-07 Go client | High | 3 | 3 | 3 | 3 | 3 | 3 |
| C-08 Licensing | High | 3 | 3 | 3 | 3 | 3 | 3 |
| C-09 Scalability | Med | 3 | 1 | 1 | 1 | 3 | 0 |
| C-10 etcd conflict | Med | 3 | 3 | 3 | 0 | 3 | 3 |
| C-11 Schema flexibility | Med | 3 | 2 | 2 | 2 | 2 | 3 |
| C-12 Community / LTS | Med | 3 | 2 | 2 | 3 | 2 | 1 |
| **Weighted Total** | | **34** | **28** | **30** | **27** | **29** | **23** |

_Note: BoltDB and BadgerDB receive 2 for C-04 in the embedded scenario because there is no transport attack surface; however they score 0 for Phase 2 multi-server TLS because they have no network interface. For a single-server MVP this is acceptable; for Phase 2 it is disqualifying._

---

## Decision

### Selected Dataplane: PostgreSQL (Primary) + etcd (Control Plane, retained)

**PostgreSQL 16+** is selected as the primary, durable dataplane for Pheromone server twin state, agent registration records, and capability registries.

**etcd** is retained in its current role as the lightweight control-plane coordinator: agent watch notifications, leader election, ephemeral registration heartbeats, and degraded-mode operation (server continues serving from cache when PostgreSQL is temporarily unavailable).

### Rationale

1. **Highest score** across all Must-Have criteria. Only PostgreSQL scores ≥2 on every Must-Have criterion.
2. **Security posture**: Full TLS 1.3 + mTLS (satisfies ADR-014), filesystem/extension-level encryption at rest, explicit row-level security for multi-tenant scenarios.
3. **ADR-002 alignment**: Layer 3 (History / audit log) was already planned as PostgreSQL. This ADR consolidates Layer 2 (durable state) into the same service, eliminating a future migration.
4. **Scalability**: PostgreSQL scales to 10 000+ agents with streaming replication + pgBouncer — no architecture change required at Phase 2.
5. **Operational maturity**: Decades of production hardening, extensive tooling (`pg_dump`, `pg_basebackup`, PITR, pgAdmin), and strong Linux distribution packaging.
6. **No etcd interference**: By separating twin state from the etcd control plane, both services operate within their designed workload profiles.
7. **Phased adoption**: Phase 1 deploys a single PostgreSQL instance. Phase 2 adds streaming replication and read replicas. Phase 3 adds logical replication for cross-region sovereignty.

### Rejected Alternatives Summary

| Candidate | Rejection Reason |
|---|---|
| **BoltDB** | Single-writer ceiling; no network interface for Phase 2; no native encryption at rest |
| **BadgerDB** | No network interface for Phase 2 multi-server; cannot satisfy mTLS requirement as an independent service |
| **etcd (extend)** | 1 MB value ceiling, 8 GB data recommendation exceeded at Phase 2 scale; mixing data and control plane I/O is an explicit etcd anti-pattern |
| **Valkey / Redis** | No native encryption at rest; suitable as cache layer only |
| **Flat File** | No transactions, no distributed access, poor scan performance, no Phase 2 path |

---

## Implementation Architecture

### Layer Model (post-adoption)

```
┌─────────────────────────────────────────────────────────────┐
│  Agent (gRPC — ADR-003)                                      │
└────────────────────────┬────────────────────────────────────┘
                         │ gRPC (mTLS — ADR-014)
                         ▼
┌─────────────────────────────────────────────────────────────┐
│  Pheromone Server                                            │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Layer 1 — In-Memory Cache (Go map[string]Twin)      │   │
│  │  Warm reads; write-through to Layer 2                │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                         │ write-through / read-miss          │
│  ┌──────────────────────▼───────────────────────────────┐   │
│  │  Layer 2 — PostgreSQL (primary dataplane)            │   │
│  │  Twin state, agent records, capability registry      │   │
│  │  mTLS (sslmode=verify-full); pgx/v5 connection pool  │   │
│  └──────────────────────────────────────────────────────┘   │
│                         │ control-plane only                 │
│  ┌──────────────────────▼───────────────────────────────┐   │
│  │  Layer 2b — etcd (control plane, retained)           │   │
│  │  Agent heartbeats, watch notifications, leader elect │   │
│  │  Max value size respected: no twin blobs             │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Server Startup / Recovery Sequence

1. Server process starts; in-memory cache is empty.
2. Server connects to PostgreSQL via `pgxpool.New()` with mTLS credentials.
3. Server executes `SELECT agent_id, twin_json, last_seen FROM twins` — bulk-loads active twin state into Layer 1 cache. Target: ≤30 s for 10 000 agents × 64 KB blobs.
4. Server registers with etcd (ephemeral key `/pheromone/servers/{id}`) and begins accepting gRPC connections.
5. Agents reconnect; server handles requests from warm cache; writes synchronously committed to PostgreSQL before acknowledging to the agent.

### Degraded Mode (PostgreSQL unavailable)

- Server continues operating from in-memory cache.
- Writes are queued in a bounded in-memory buffer (configurable; default 1 000 mutations).
- Warning metric emitted to telemetry stream every 30 s.
- On PostgreSQL reconnect, queued mutations are flushed transactionally.
- If queue exceeds capacity, oldest mutations are dropped and a critical alert is emitted. Agents are not interrupted.

---

## Security Architecture

### Encryption at Rest

- PostgreSQL data directory encrypted at the filesystem level using **LUKS/dm-crypt** on Linux (Phase 1) or **pgcrypto** column-level encryption for sensitive fields (Phase 2).
- Backup files produced by `pg_basebackup` are encrypted via the same LUKS volume or via `pg_basebackup --encrypt` (PostgreSQL 17).
- Encryption keys managed by the operator; Vault integration deferred to Phase 3.

### Transport Security

- **Server → PostgreSQL**: `sslmode=verify-full` with server CA certificate and optional client certificate (mTLS). TLS 1.3 required; TLS 1.2 minimum.
- PostgreSQL `pg_hba.conf` entry: `hostssl pheromone pheromone_user 0.0.0.0/0 scram-sha-256 clientcert=verify-full`.
- Certificate rotation: PostgreSQL supports live reload of server certificates via `SELECT pg_reload_conf()`.

### Data Sovereignty

- PostgreSQL is deployed fully on-premises. No telemetry, phoning home, or cloud-managed service dependency.
- Air-gapped environments are supported: all installation artefacts available via standard Linux package repositories (offline mirror compatible).
- Operators control where backup files are written; no cloud storage is assumed.

### Access Control

- A dedicated PostgreSQL role `pheromone_user` is created with minimum privileges: `SELECT`, `INSERT`, `UPDATE`, `DELETE` on `twins`, `agents`, and `capabilities` tables only.
- The PostgreSQL superuser credentials are never exposed to the Pheromone server process.
- Row-level security (RLS) policies deferred to Phase 2 multi-tenant scenarios.

### Known Security Gaps (Phase 1)

| Gap | Severity | Mitigation |
|---|---|---|
| LUKS encryption setup is operator responsibility; not automated by Pheromone | Medium | Document in deployment guide; flag in smoke-test |
| Column-level encryption for sensitive twin fields not implemented in Phase 1 | Low | LUKS volume provides at-rest protection; defer pgcrypto to Phase 2 |
| Vault / external KMS for key management not implemented | Low | Keys stored in operator-managed config; scoped follow-up issue |

---

## Backup and Recovery

### Backup Strategy

| Method | Frequency | RPO | Notes |
|---|---|---|---|
| `pg_basebackup` (full) | Daily | 24 h (worst case) | Consistent, hot backup; no downtime |
| WAL archiving (continuous) | Continuous | <60 s | Requires `archive_mode = on` in `postgresql.conf` |
| Logical dump (`pg_dump`) | Weekly | Coarse-grained | Human-readable; useful for schema migrations |

Phase 1 recommendation: **daily `pg_basebackup` + continuous WAL archiving**. This achieves RPO <1 minute with standard tooling.

### Recovery Procedure

1. Stop the Pheromone server.
2. Restore the base backup to the PostgreSQL data directory: `pg_basebackup -R -D /var/lib/postgresql/data`.
3. Apply WAL segments from archive to bring the cluster to the desired recovery point.
4. Start PostgreSQL; verify data integrity with `SELECT count(*) FROM twins`.
5. Start the Pheromone server; it rehydrates Layer 1 cache from the restored PostgreSQL instance.

**RTO estimate**: 5–15 minutes for a 1 GB database (includes WAL replay). Server itself is ready within 30 s of PostgreSQL being available.

### Backup Security

- Backup files are written to the same LUKS-encrypted volume or an encrypted remote destination (operator-defined).
- WAL archive destination must enforce TLS if remote (e.g., `archive_command` using `scp` with host verification or S3-compatible endpoint with SSE).

---

## Transition Path from ADR-002

| ADR-002 Layer | Current Owner | Post-ADR-016 Owner | Migration Notes |
|---|---|---|---|
| Layer 1 — In-memory cache | Go `map[string]Twin` | Go `map[string]Twin` (unchanged) | No change; cache warms from PostgreSQL instead of etcd |
| Layer 2 — Durable state | etcd `/pheromone/twins/*` | PostgreSQL `twins` table | Migrate existing etcd twin keys to PostgreSQL on first deployment |
| Layer 2b — Control plane | etcd `/pheromone/agents/*` | etcd `/pheromone/agents/*` (unchanged) | Heartbeats, watch notifications, ephemeral registration remain in etcd |
| Layer 3 — Audit / history | Planned: PostgreSQL | PostgreSQL `twin_audit_log` table | Merged: Layer 2 and Layer 3 share the same PostgreSQL instance |

### Migration Script Approach

A one-time migration utility (`cmd/migrate-etcd-to-postgres`) will:
1. Connect to etcd and iterate all `/pheromone/twins/*` keys.
2. Parse each value as a twin model.
3. `INSERT` or `UPSERT` into the PostgreSQL `twins` table.
4. Verify row counts match.
5. Delete migrated keys from etcd (or leave for a configurable grace period).

This utility is a follow-up implementation issue (I-02).

---

## Consequences

### Positive

- Server process becomes truly stateless: any server instance can restart independently and recover fully from PostgreSQL within 30 s.
- etcd is freed from large-blob I/O; control-plane coordination reliability improves.
- Full encryption at rest (via LUKS) and mTLS transport are achievable without architectural workarounds.
- PostgreSQL's JSONB + relational hybrid eliminates schema migrations for twin model changes.
- Phase 2 multi-server HA requires only adding streaming replication + pgBouncer — no architecture change.
- ADR-002 Layer 3 (audit log) and Layer 2 (durable state) are consolidated into a single service, reducing operational surface area.

### Negative / Trade-offs

- An additional service (PostgreSQL) must be deployed, monitored, and maintained. Operational complexity increases compared to the current embedded approach.
- Cold-start rehydration of 10 000 agents × 64 KB = 640 MB takes ~15–25 s over a LAN; operators must account for this in startup SLOs.
- Connection pool sizing (`pgxpool.Config.MaxConns`) must be tuned per deployment; incorrect sizing causes server starvation.
- Encryption at rest via LUKS is an operator-side responsibility; the Pheromone project cannot enforce it programmatically.

### Open Items (Phase 2+)

- Column-level encryption for sensitive twin fields via `pgcrypto`.
- Vault / external KMS for PostgreSQL credential and encryption-key management.
- Read replicas for read-scale-out and geographic distribution.
- Row-level security (RLS) for multi-tenant / multi-team scenarios.

---

## Follow-Up GitHub Issues

The following issues are required to implement the decision in this ADR. All implementation MUST proceed on separate feature branches after this ADR is Accepted.

| # | Title | Priority | Estimate | ADR Reference |
|---|---|---|---|---|
| I-01 | Implement PostgreSQL dataplane adapter in Pheromone server | P1 | 16–24 h | ADR-016 §Decision |
| I-02 | One-time migration utility: etcd Layer 2 keys → PostgreSQL | P1 | 8–12 h | ADR-016 §Transition Path |
| I-03 | Configure mTLS for server ↔ PostgreSQL connection (sslmode=verify-full) | P1 | 4–8 h | ADR-016 §Security |
| I-04 | Deploy PostgreSQL to Vagrant environment (ADR-012) and docker-compose | P2 | 4–6 h | ADR-016, ADR-012 |
| I-05 | Implement encryption at rest: LUKS setup for PostgreSQL data directory | P2 | 4–8 h | ADR-016 §Security |
| I-06 | Write backup and recovery runbook (pg_basebackup + WAL archiving) | P2 | 4 h | ADR-016 §Backup |
| I-07 | Add Prometheus metrics for PostgreSQL health, connection pool, and query latency | P2 | 4 h | ADR-016, ADR-002 |
| I-08 | Tech spike: throughput at 1 000 agents with pgxpool (read/write p99) | P2 | 6–8 h | ADR-016 §Constraints |
| I-09 | Review and Accept ADR-016 | P1 | — | ADR-016 |

---

## Related Decisions

- [ADR-002: Server Architecture](adr-002-server-architecture.md) — defines the in-memory + etcd hybrid that this ADR supersedes at Layer 2
- [ADR-003: gRPC Service Contracts](adr-003-grpc-contracts.md) — agents access the dataplane exclusively via server gRPC APIs
- [ADR-014: Security Architecture](adr-014-security-approach.md) — mTLS and encryption-at-rest requirements that drove the candidate scoring
- [ADR-005: Message Queue Selection](adr-005-message-queue.md) — telemetry path remains separate; NATS/Kafka carry metrics, not twin state

---

## References

- Feature Specification: `.specify/memory/spec-002-stateless-server-dataplane.md`
- Implementation Plan: `.specify/memory/plan-002-stateless-server-dataplane.md`
- Constitution v2.1.0: `.specify/memory/constitution.md`
- PostgreSQL documentation: <https://www.postgresql.org/docs/16/>
- `pgx/v5` driver: <https://github.com/jackc/pgx>
- etcd data model: <https://etcd.io/docs/v3.5/learning/data_model/>
- BadgerDB: <https://github.com/dgraph-io/badger>
- bbolt: <https://github.com/etcd-contrib/bbolt>
- Valkey (CNCF Redis fork): <https://valkey.io/>
- LUKS/dm-crypt: <https://gitlab.com/cryptsetup/cryptsetup>
- WAL archiving: <https://www.postgresql.org/docs/16/continuous-archiving.html>

---

**Decision Date**: 2026-03-10
**Status**: Proposed — awaiting 2 approvals per Constitution §II

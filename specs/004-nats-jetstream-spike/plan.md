# Implementation Plan: NATS JetStream Load-Test Spike

**Branch**: `feature/ADR-005-nats-jetstream-spike`  
**Date**: 2025-01-27  
**Spec**: [`specs/004-nats-jetstream-spike/spec.md`](./spec.md)  
**ADR**: [`docs/adr/adr-005-message-queue.md`](../../docs/adr/adr-005-message-queue.md)  
**GitHub Issue**: [#4 — Tech Spike: Validate NATS JetStream at 100K msgs/sec](https://github.com/abuxton/pheromone/issues/4)

---

## Summary

Validate that NATS with JetStream persistence can sustain **100,000 messages per second** with
p95 delivery latency < 10 ms, p99 < 50 ms, and server RSS < 512 MB under 1 M buffered messages —
the empirical evidence required to move ADR-005 from **Proposed → Accepted** (or **Amended**).

The spike adds three concrete artefacts to the repository:

1. A `pheromone-nats` service in `docker-compose.yml` (JetStream enabled, port 4222).
2. `benchmark/nats_jetstream_spike_test.go` — a Go benchmark/test file that mirrors
   `benchmark/grpc_spike_test.go` exactly in structure: `package benchmark`, named threshold
   constants, `testing.T` validation tests, `testing.B` harness, percentile helper, and a
   printed summary table.
3. An updated `docs/adr/adr-005-message-queue.md` with measured spike results and a final
   status of **Accepted** or **Amended**.

No production integration code is in scope. Existing tests MUST NOT be broken.

---

## Technical Context

| Dimension | Value |
|-----------|-------|
| **Language / Version** | Go 1.24.13 (from `go.mod`) |
| **New runtime dependency** | `nats.io/nats.go` (latest — currently absent from `go.mod`) |
| **Existing dependencies** | `go.etcd.io/etcd/client/v3 v3.5.16`, `google.golang.org/grpc v1.79.1` |
| **NATS Docker image** | `nats:latest` (≥ 2.10; JetStream-capable) |
| **Container name** | `pheromone-nats` |
| **NATS port** | `4222` (no conflict; etcd owns 2379/2380, gRPC owns 4426/4427) |
| **JetStream flag** | `-js` server flag in Docker command |
| **Connection URL** | `nats://localhost:4222` (no TLS, no auth for spike) |
| **Stream name** | `TELEMETRY` |
| **Subject** | `metrics.test` |
| **Payload size** | 256 bytes (fixed; nanosecond timestamp embedded at offset 0) |
| **Storage** | File-backed JetStream (24 h retention, 10 M message cap) |
| **Testing framework** | `go test` / `testing.B` — same harness as `grpc_spike_test.go` |
| **Benchmark invocation** | `go test -bench=. ./benchmark/...` with Docker running |
| **CI skip guard** | `t.Skip` / `b.Skip` when `dial(nats://localhost:4222)` returns connection refused |
| **Target platform** | Linux / macOS developer host; GitHub Actions (≥ 2 cores, ≥ 4 GB RAM) |
| **Performance goals** | ≥ 100 K msgs/sec sustained; p95 < 10 ms; p99 < 50 ms; RSS < 512 MB |
| **Constraints** | ≤ 20% JetStream overhead vs core NATS; zero message loss at peak |

---

## Constitution Check

*GATE: must pass before Phase 0. Re-checked after Phase 1 design.*

### Principle I — Layered Digital Twin Architecture
- ✅ **Compliant.** This is a pure benchmarking spike; no digital-twin layer code is modified.
  NATS is a future transport for the Management Layer but is isolated here as an infrastructure
  component under test.

### Principle II — ADR-Driven Decision Contracts & Observability
- ✅ **Compliant.** ADR-005 exists and is in Proposed state. This spike is explicitly the
  empirical validation step before ADR-005 can be Accepted. The plan correctly precedes
  implementation. The benchmark emits structured stdout results (observability gate).
- ⚠️ **Post-spike gate**: ADR-005 MUST be updated (FR-007) before the branch is merged.
  Merging with ADR-005 still in "Proposed" state is a constitution violation.

### Principle III — Language-Agnostic Protocol Foundation
- ✅ **Compliant.** Go is the preferred language for rapid prototyping and server work. `nats.io/nats.go`
  is the canonical Go client. No Protobuf schema changes; benchmark payloads are raw bytes.

### Principle IV — Smoke Tests & Quality Gates
- ✅ **Compliant.** New file is in `package benchmark` and is runnable via `go test ./benchmark/...`.
  Skip guards ensure CI does not fail when Docker is absent. The PR must not reduce coverage
  on any package outside `benchmark/`.
- ⚠️ **Gate**: `go test ./...` (excluding integration paths) MUST pass before PR submission.
  The NATS test file must compile cleanly with `go vet ./benchmark/...`.

### Principle V — Git-Driven ADR Workflow
- ✅ **Compliant.** Branch name `feature/ADR-005-nats-jetstream-spike` matches the
  `feature/ADR-NNN-descriptive-name` convention. PR description must link ADR-005 and
  close Issue #4.

**No violations require a Complexity Tracking entry.**

---

## Project Structure

### Documentation (this feature)

```text
specs/004-nats-jetstream-spike/
├── spec.md              # Feature specification (existing)
├── plan.md              # This file
├── research.md          # Phase 0 output — NATS client API, JetStream stream creation,
│                        #   percentile helper, memory measurement, Docker stats approach
├── data-model.md        # Phase 1 output — benchmark entities, stream config schema,
│                        #   result record structure
├── contracts/
│   └── nats-stream-config.md   # JetStream stream/consumer configuration contract
└── quickstart.md        # Phase 1 output — how to run the benchmark end-to-end
```

### Source Code Changes

```text
docker-compose.yml                          # ADD pheromone-nats service
go.mod / go.sum                             # ADD nats.io/nats.go dependency
benchmark/
└── nats_jetstream_spike_test.go            # NEW benchmark file
docs/adr/
└── adr-005-message-queue.md               # UPDATE: Spike Results section + status
```

No new packages or directories are created inside `internal/` — this spike is
deliberately non-invasive to production code.

---

## Phase 0: Research

**Goal**: Resolve all NEEDS CLARIFICATION items and produce `research.md`.

### Research Questions

| ID | Question | Resolution Approach |
|----|----------|---------------------|
| R-01 | What is the exact `nats.io/nats.go` API for creating a JetStream stream programmatically and publishing with sync ack? | Read `nats.go` JetStream API docs; confirm `js.AddStream()` + `js.Publish()` signatures |
| R-02 | How do we embed a nanosecond publish timestamp in a 256-byte payload for latency measurement without a Protobuf dependency? | Use `binary.BigEndian.PutUint64` at payload offset 0 (8 bytes) — same approach viable in pure `[]byte` |
| R-03 | How is JetStream overhead (core NATS vs JetStream) measured in the same benchmark run? | Run `nc.Publish()` (core) vs `js.Publish()` (JetStream) back-to-back with the same payload; compare throughput and p95 |
| R-04 | What is the portable approach to reading NATS server RSS from Go test code? | `exec.Command("docker", "stats", "--no-stream", "--format", "{{.MemUsage}}", "pheromone-nats")` — parse MB value; fall back to `runtime.ReadMemStats` for in-process heap |
| R-05 | Does `nats:latest` (≥ 2.10) support `-js` flag on the container `command` field? | ADR-005 Deployment section confirms this; validate with `docker run --rm nats:latest -js --help` |
| R-06 | Does adding `nats.io/nats.go` introduce any indirect dependency conflicts with current `go.mod`? | Run `go get nats.io/nats.go@latest` in a dry-run and inspect `go.mod` diff |
| R-07 | What is the exact percentile helper signature already used in `grpc_spike_test.go`? | `grpcPercentile(sorted []time.Duration, p float64) time.Duration` — replicate as `natsPercentile` in the new file |
| R-08 | What is the safest `t.Skip` trigger when NATS is unavailable? | Attempt `nats.Connect("nats://localhost:4222", nats.Timeout(500*time.Millisecond))` in a helper; if `err != nil && strings.Contains(err.Error(), "connection refused")` → skip |

**Output**: `specs/004-nats-jetstream-spike/research.md` — one entry per question with
Decision / Rationale / Alternatives.

---

## Phase 1: Design & Contracts

**Prerequisite**: `research.md` complete with all R-0x items resolved.

### 1a. Data Model

**Output**: `specs/004-nats-jetstream-spike/data-model.md`

#### Entity: BenchmarkMessage (wire payload)

| Field | Offset | Size | Type | Description |
|-------|--------|------|------|-------------|
| `publish_ns` | 0 | 8 B | `uint64` big-endian | Unix nanosecond publish timestamp |
| `seq` | 8 | 8 B | `uint64` big-endian | Producer sequence number (loss detection) |
| `producer_id` | 16 | 4 B | `uint32` big-endian | Goroutine/producer index |
| `padding` | 20 | 236 B | zeros | Fill to 256 bytes total |

Total: **256 bytes** (matches spec assumption and ADR-005 OpenMetrics approximation).

#### Entity: BenchmarkResult (in-memory, printed to stdout)

| Field | Go Type | Description |
|-------|---------|-------------|
| `Scenario` | `string` | `"core-nats"` or `"jetstream"` |
| `Stage` | `string` | `"1K"` / `"10K"` / `"50K"` / `"100K"` |
| `TargetRate` | `int` | msgs/sec target for this stage |
| `MeasuredRate` | `float64` | actual msgs/sec achieved |
| `P50Ms` | `float64` | p50 latency (ms) |
| `P95Ms` | `float64` | p95 latency (ms) |
| `P99Ms` | `float64` | p99 latency (ms) |
| `LostMsgs` | `int64` | sequence-gap detected losses |
| `PeakMemMB` | `float64` | NATS server RSS at peak (MB) |
| `PassFail` | `string` | `"PASS"` / `"FAIL"` per ADR-005 SC |

#### Entity: JetStream Stream Configuration

| Parameter | Value |
|-----------|-------|
| Name | `TELEMETRY` |
| Subjects | `["metrics.test"]` |
| Storage | `nats.FileStorage` |
| Retention | `nats.LimitsPolicy` |
| MaxAge | `24 * time.Hour` |
| MaxMsgs | `10_000_000` |
| MaxBytes | `-1` (unlimited) |
| Replicas | `1` |

The stream is created in a `TestMain` setup (or per-test helper) and deleted in cleanup.

### 1b. API Contracts

**Output**: `specs/004-nats-jetstream-spike/contracts/nats-stream-config.md`

```
Stream: TELEMETRY
  Subjects:   metrics.test
  Storage:    file
  Retention:  limits
  MaxMsgs:    10000000
  MaxAge:     86400s
  Replicas:   1

Consumer (push, ephemeral, for latency tests):
  Subject:    metrics.test
  DeliverSubject: _INBOX.<random>
  AckPolicy:  explicit
  MaxDeliver: 1
```

> Note: the consumer is created programmatically via `js.Subscribe()` in each test function;
> no durable consumer is required for the spike.

### 1c. Benchmark File Architecture

`benchmark/nats_jetstream_spike_test.go` mirrors `grpc_spike_test.go` precisely:

```
package benchmark
│
├── File header comment       — ADR-005 spike reference, SC list
├── import block              — nats.go, standard library only (no grpc)
├── Threshold constants       — natsTargetRate, natsP95Ms, natsP99Ms, natsMemMB, natsOverhead
├── natsConnect() helper      — connects or t.Skip; called at top of every test/benchmark
├── natsCreateStream() helper — idempotent stream creation + t.Cleanup drain/delete
├── natsPercentile() helper   — identical signature to grpcPercentile()
├── natsMemMB() helper        — docker stats query → float64 MB
│
├── TestNATSThroughputRamp    — FR-003 / SC-001
│   Ramp 1K→10K→50K→100K msgs/sec (5 s per stage)
│   4 producer goroutines (default); configurable via NATS_PRODUCERS env var
│   Reports per-stage throughput + aggregate; t.Error on < 100K or any loss
│
├── TestNATSConsumerLatency   — FR-004 / SC-002 / SC-003
│   Producer + push consumer running concurrently at 100K msgs/sec target
│   Records per-message latency (subscribe timestamp − publish_ns from payload)
│   Reports p50/p95/p99; t.Error on p95 ≥ 10 ms or p99 ≥ 50 ms
│
├── TestNATSJetStreamOverhead — FR-005 / SC-005
│   Runs identical 30-second burst first with nc.Publish (core), then js.Publish
│   Reports throughput and p95 for each; calculates overhead %; t.Error on > 20%
│
├── TestNATSMemoryProfile     — FR-006 / SC-004
│   Pauses consumer goroutine; publishes 1 M messages; polls docker stats
│   Reports peak RSS; t.Error on ≥ 512 MB
│   Resumes consumer; verifies RSS returns to within 10% of baseline
│
├── BenchmarkNATSPublishCore  — testing.B harness, core NATS publish
└── BenchmarkNATSPublishJetStream — testing.B harness, JetStream publish
```

#### Key Implementation Patterns (from `grpc_spike_test.go` reference)

```go
// ── threshold constants ───────────────────────────────────────────────────────
const (
    natsTargetRateMsgsPerSec = 100_000          // SC-001
    natsP95ThresholdMs       = 10.0             // SC-002
    natsP99ThresholdMs       = 50.0             // SC-003
    natsMemThresholdMB       = 512.0            // SC-004
    natsJSOverheadPct        = 20.0             // SC-005
    natsURL                  = "nats://localhost:4222"
    natsStream               = "TELEMETRY"
    natsSubject              = "metrics.test"
    natsPayloadSize          = 256
    natsRampHoldSec          = 5
    natsProducerDefault      = 4
)

// ── availability guard ────────────────────────────────────────────────────────
func natsConnect(t testing.TB) *nats.Conn {
    t.Helper()
    nc, err := nats.Connect(natsURL, nats.Timeout(500*time.Millisecond))
    if err != nil {
        t.Skipf("NATS unavailable at %s (%v) — skipping (run docker compose up pheromone-nats)", natsURL, err)
    }
    t.Cleanup(func() { nc.Drain() })
    return nc
}

// ── percentile helper (same pattern as grpcPercentile) ───────────────────────
func natsPercentile(sorted []time.Duration, p float64) time.Duration {
    if len(sorted) == 0 {
        return 0
    }
    idx := int(float64(len(sorted)) * p / 100.0)
    if idx >= len(sorted) {
        idx = len(sorted) - 1
    }
    return sorted[idx]
}

// ── memory helper ─────────────────────────────────────────────────────────────
func natsServerMemMB(t testing.TB) float64 {
    t.Helper()
    out, err := exec.Command("docker", "stats", "--no-stream",
        "--format", "{{.MemUsage}}", "pheromone-nats").Output()
    if err != nil {
        t.Logf("docker stats unavailable: %v — using in-process heap as proxy", err)
        var ms runtime.MemStats
        runtime.ReadMemStats(&ms)
        return float64(ms.Alloc) / (1024 * 1024)
    }
    // parse "123.4MiB / 7.8GiB" → 123.4
    // ... (string parsing logic)
}
```

### 1d. `docker-compose.yml` Addition

```yaml
  pheromone-nats:
    image: nats:latest
    container_name: pheromone-nats
    ports:
      - "4222:4222"
    command: ["-js"]
    restart: unless-stopped
```

This is appended under `services:` in `docker-compose.yml`. No existing service is modified.
Port 4222 is confirmed free (etcd: 2379/2380; gRPC: 4426/4427).

### 1e. `go.mod` Addition

```bash
go get nats.io/nats.go@latest
```

Expected new direct dependency:

```
require (
    ...
    nats.io/nats.go v1.x.x
)
```

Indirect dependencies (e.g., `github.com/nats-io/nkeys`, `github.com/nats-io/nuid`) will be
added to `go.sum` automatically. No existing dependency versions are expected to change.

### 1f. ADR-005 Update Structure

`docs/adr/adr-005-message-queue.md` must receive:

1. **Status line change**: `Proposed` → `Accepted` (or `Amended`).
2. **Decision Date** updated to spike completion date.
3. **New section** appended after `## Testing & Validation`:

```markdown
## Spike Results (ADR-005 Empirical Validation)

**Run Date**: [DATE]  
**Branch**: feature/ADR-005-nats-jetstream-spike  
**Host**: [hardware summary]  
**NATS Version**: [docker inspect pheromone-nats | grep Image]

### Throughput (SC-001)
| Stage | Target (msgs/sec) | Measured (msgs/sec) | Lost | Result |
|-------|-------------------|---------------------|------|--------|
| 1K    | 1,000             | _____               | 0    | PASS   |
| 10K   | 10,000            | _____               | 0    | PASS   |
| 50K   | 50,000            | _____               | 0    | PASS   |
| 100K  | 100,000           | _____               | 0    | ____   |

### Latency at 100K msgs/sec (SC-002, SC-003)
| Percentile | Threshold | Measured | Result |
|------------|-----------|----------|--------|
| p50        | —         | ___ ms   | —      |
| p95        | < 10 ms   | ___ ms   | ____   |
| p99        | < 50 ms   | ___ ms   | ____   |

### JetStream vs Core NATS Overhead (SC-005)
| Metric      | Core NATS | JetStream | Overhead | Threshold | Result |
|-------------|-----------|-----------|----------|-----------|--------|
| Throughput  | ___ msg/s | ___ msg/s | ___%     | ≤ 20%    | ____   |
| p95 Latency | ___ ms    | ___ ms    | ___%     | ≤ 20%    | ____   |

### Memory Under 1M Buffered Messages (SC-004)
| Measurement | Value | Threshold | Result |
|-------------|-------|-----------|--------|
| Baseline RSS (pre-publish) | ___ MB | — | — |
| Peak RSS (1M msgs buffered) | ___ MB | < 512 MB | ____ |
| Post-drain RSS | ___ MB | ≤ baseline + 10% | ____ |

### Overall ADR-005 Decision
[ACCEPTED / AMENDED — rationale if AMENDED]
```

> **AMENDED example**: If sustained throughput falls below 100K msgs/sec on the target hardware,
> the ADR status becomes "Amended" with a note reducing the SC-001 target or accelerating
> the Phase 2 Kafka timeline.

---

## Quickstart

**Output**: `specs/004-nats-jetstream-spike/quickstart.md`

```bash
# 1. Start NATS with JetStream
docker compose up -d pheromone-nats

# 2. Verify JetStream is active
docker exec pheromone-nats nats-server --version
# Expected: nats-server: v2.10.x

# 3. Add NATS Go client dependency (first time only)
go get nats.io/nats.go@latest

# 4. Run all benchmark tests
go test -v -bench=. -timeout 120s ./benchmark/...

# 5. Run only validation tests (no benchmarks)
go test -v -run TestNATS ./benchmark/...

# 6. Run only throughput benchmark
go test -bench=BenchmarkNATSPublish -benchtime=10s ./benchmark/...

# 7. Teardown
docker compose down pheromone-nats
```

**Expected output structure** (mirrors `grpc_spike_test.go` stdout):

```
=== RUN   TestNATSThroughputRamp
    nats_jetstream_spike_test.go:NNN: Stage 1K:  1,000 target → 1,023 msgs/sec  lost=0
    nats_jetstream_spike_test.go:NNN: Stage 10K: 10,000 target → 10,412 msgs/sec  lost=0
    nats_jetstream_spike_test.go:NNN: Stage 50K: 50,000 target → 51,887 msgs/sec  lost=0
    nats_jetstream_spike_test.go:NNN: Stage 100K: 100,000 target → 103,241 msgs/sec  lost=0  [SC-001 PASS]
--- PASS: TestNATSThroughputRamp (21.34s)

=== RUN   TestNATSConsumerLatency
    nats_jetstream_spike_test.go:NNN: Latency @ 100K msgs/sec — P50: 1.2ms  P95: 4.7ms  P99: 9.3ms
    nats_jetstream_spike_test.go:NNN: [SC-002 PASS: p95 4.7ms < 10ms]  [SC-003 PASS: p99 9.3ms < 50ms]
--- PASS: TestNATSConsumerLatency (32.10s)

=== RUN   TestNATSJetStreamOverhead
    nats_jetstream_spike_test.go:NNN: Core NATS:  118,440 msgs/sec  p95=3.1ms
    nats_jetstream_spike_test.go:NNN: JetStream:  103,241 msgs/sec  p95=4.7ms
    nats_jetstream_spike_test.go:NNN: Overhead: throughput=12.8%  p95=51.6%  [SC-005 PASS / FAIL]
--- PASS/FAIL: TestNATSJetStreamOverhead

=== RUN   TestNATSMemoryProfile
    nats_jetstream_spike_test.go:NNN: Baseline RSS: 18.3 MB
    nats_jetstream_spike_test.go:NNN: Peak RSS (1M msgs): 187.4 MB  [SC-004 PASS: < 512 MB]
    nats_jetstream_spike_test.go:NNN: Post-drain RSS: 19.1 MB  [leak check PASS]
--- PASS: TestNATSMemoryProfile (45.22s)
```

---

## Implementation Sequence & Dependencies

```
Step 1: docker-compose.yml
    └─→ adds pheromone-nats service
           (no Go code dependency; can be done immediately)

Step 2: go get nats.io/nats.go@latest
    └─→ updates go.mod + go.sum
           (prerequisite for Step 3)

Step 3: benchmark/nats_jetstream_spike_test.go
    ├─→ depends on Step 2 (nats package available)
    ├─→ compile-check: go vet ./benchmark/...
    └─→ skip-guard: all tests skip gracefully without Docker

Step 4: Run benchmark (Docker required)
    └─→ docker compose up pheromone-nats
        go test -v -bench=. -timeout 120s ./benchmark/...

Step 5: docs/adr/adr-005-message-queue.md
    └─→ depends on Step 4 (fill in measured values)
           Status: Proposed → Accepted or Amended

Step 6: PR + close Issue #4
    └─→ squash commit message: "feat(benchmark): ADR-005 NATS JetStream spike results"
```

---

## Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `nats:latest` tag moves to 3.x before spike runs; `-js` flag deprecated | Low | Medium | Pin to `nats:2.10` if `latest` breaks; update ADR-005 Deployment snippet |
| Indirect dependency conflict (e.g., `nkeys` version) with existing `go.sum` | Low | Low | `go mod tidy` after `go get`; no shared deps with etcd/grpc expected |
| CI runners (2 cores) cannot sustain 100K msgs/sec → SC-001 fails in CI | Medium | Medium | SC-006 explicitly notes CI hardware may underperform developer machines; annotate results, do not auto-fail CI on throughput gate (use `t.Logf` not `t.Error` for CI throughput shortfalls) |
| JetStream p95 overhead > 20% on shared hardware (SC-005 fail) | Medium | High | If overhead > 20%: mark SC-005 as AMENDED in ADR-005; document why file-backed storage on developer NVMe meets production target but exceeds it on rotational CI disk |
| `docker stats` unavailable (rootless Docker, CI restrictions) | Medium | Low | `natsServerMemMB` falls back to in-process heap via `runtime.ReadMemStats`; note in test output that value is a proxy |
| `docker compose` vs `docker-compose` CLI difference across environments | Low | Low | `docker-compose.yml` uses `version: '3.8'` (already established); both CLIs support it |

---

## Acceptance Checklist

Before the PR can be merged, ALL of the following must be true:

- [ ] `go test ./...` (excluding benchmark integration paths) passes without error
- [ ] `go vet ./benchmark/...` produces no warnings on `nats_jetstream_spike_test.go`
- [ ] `go test -bench=. ./benchmark/...` **skips gracefully** (not fails) when `pheromone-nats` is not running
- [ ] `go test -v -run TestNATS -timeout 120s ./benchmark/...` **passes** with `pheromone-nats` running
- [ ] `docker compose up pheromone-nats` starts without error (port 4222 available)
- [ ] ADR-005 status is updated from "Proposed" to "Accepted" or "Amended" with measured spike results
- [ ] ADR-005 "Spike Results" section is filled with actual numbers (not placeholders)
- [ ] GitHub Issue #4 is referenced in the PR description (`Closes #4`)
- [ ] PR description links ADR-005 and this plan
- [ ] Squash commit message follows convention: `feat(benchmark): ADR-005 NATS JetStream spike results`
- [ ] No files under `internal/`, `cmd/`, or `ui/` are modified
- [ ] `go.mod` and `go.sum` are committed alongside the benchmark file

---

## Phase 2 Handoff (Out of Scope for This Spike)

Once ADR-005 is Accepted, a follow-up feature branch will implement the production
gRPC-to-NATS bridge. That work should reference:

- This spike's `research.md` for validated connection patterns
- The `TELEMETRY` stream configuration from `contracts/nats-stream-config.md`
- The `quickstart.md` for local development setup
- ADR-003 (`grpc-services`) for the gRPC side of the bridge

---

*Plan generated by `speckit.plan` — feature/ADR-005-nats-jetstream-spike — 2025-01-27*

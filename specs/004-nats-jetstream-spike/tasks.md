# Tasks: NATS JetStream Load-Test Spike (ADR-005)

**Feature Branch**: `feature/ADR-005-nats-jetstream-spike`  
**Spec**: `specs/004-nats-jetstream-spike/spec.md`  
**Plan**: `specs/004-nats-jetstream-spike/plan.md`  
**ADR**: `docs/adr/adr-005-message-queue.md`  
**Reference**: `benchmark/grpc_spike_test.go`  
**GitHub Issue**: [#4 — Tech Spike: Validate NATS JetStream at 100K msgs/sec](https://github.com/abuxton/pheromone/issues/4)

---

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Can run in parallel with other [P] tasks in the same phase (different files, no shared state)
- **[US#]**: Which user story this task belongs to
- File paths are relative to repository root
- Every test function calls `natsConnect(t/b)` as its first statement — the skip guard is automatic

---

## Phase 1: Setup

**Purpose**: Add NATS infrastructure and the Go client dependency. These two tasks target
different files and have no mutual dependency — run them in parallel.

- [ ] T001 [P] Add `pheromone-nats` JetStream service block to `docker-compose.yml` under `services:`
- [ ] T002 [P] Add `nats.io/nats.go` Go client to `go.mod` and `go.sum` by running `go get nats.io/nats.go@latest && go mod tidy`

---

## Phase 2: Foundational — Benchmark File Skeleton

**Purpose**: Create `benchmark/nats_jetstream_spike_test.go` with the shared scaffolding that
every test function depends on. The file must compile cleanly and skip gracefully before any
test body is written.

**⚠️ CRITICAL**: T002 must be complete before T003 (package must be importable). T003 and T004
must be complete before any user-story phase begins.

- [ ] T003 Create `benchmark/nats_jetstream_spike_test.go` skeleton — file header comment (ADR-005 spike reference, SC list), `package benchmark` declaration, import block (`nats.io/nats.go`, `encoding/binary`, `fmt`, `math/rand`, `os/exec`, `runtime`, `sort`, `strconv`, `strings`, `sync`, `sync/atomic`, `testing`, `time`), threshold constants block, and four shared helpers
- [ ] T004 Verify the skeleton compiles clean and the skip guard works without Docker running

**Checkpoint**: `go vet ./benchmark/...` exits 0. `go test -count=1 -run TestNATS ./benchmark/...`
(without NATS running) exits 0 with every test reporting `--- SKIP`.

---

## Phase 3: User Story 1 — Producer Throughput Validation (Priority: P1) 🎯 MVP

**Goal**: Prove a single NATS server with JetStream enabled sustains ≥ 100,000 msgs/sec across
a 4-stage ramp (1K → 10K → 50K → 100K, 5 s hold per stage) with zero message loss.

**Independent Test**:
```bash
docker compose up -d pheromone-nats
go test -v -run TestNATSThroughputRamp -timeout 60s ./benchmark/...
```
Pass condition: final stage log line contains `≥ 100,000 msgs/sec` and `lost=0`; test exits `PASS`.

- [ ] T005 [US1] Implement `TestNATSThroughputRamp` in `benchmark/nats_jetstream_spike_test.go`

**Checkpoint**: `go test -v -run TestNATSThroughputRamp ./benchmark/...` with NATS running
produces per-stage throughput lines and exits PASS (or a documented FAIL that drives an ADR
amendment).

---

## Phase 4: User Story 2 — Consumer Delivery Latency Measurement (Priority: P1)

**Goal**: Measure p50/p95/p99 end-to-end latency under sustained 100K msgs/sec load and confirm
p95 < 10 ms and p99 < 50 ms as required by ADR-005.

**Independent Test**:
```bash
go test -v -run TestNATSConsumerLatency -timeout 60s ./benchmark/...
```
Pass condition: output contains `[SC-002 PASS: p95 Xms < 10ms]` and `[SC-003 PASS: p99 Xms < 50ms]`.

- [ ] T006 [US2] Implement `TestNATSConsumerLatency` in `benchmark/nats_jetstream_spike_test.go`

**Checkpoint**: `go test -v -run TestNATSConsumerLatency ./benchmark/...` prints a p50/p95/p99
histogram with millisecond values and exits PASS.

---

## Phase 5: User Story 3 — JetStream Persistence Overhead Assessment (Priority: P2)

**Goal**: Quantify the throughput and p95-latency cost of enabling JetStream persistence vs
core NATS (no persistence); confirm both overhead figures are ≤ 20%.

**Independent Test**:
```bash
go test -v -run TestNATSJetStreamOverhead -timeout 90s ./benchmark/...
```
Pass condition: comparison table printed; `t.Error` fires only if an overhead column exceeds 20%.

- [ ] T007 [US3] Implement `TestNATSJetStreamOverhead` in `benchmark/nats_jetstream_spike_test.go`

**Checkpoint**: `go test -v -run TestNATSJetStreamOverhead ./benchmark/...` prints a side-by-side
Core / JetStream table with overhead percentages and exits PASS (or FAIL with documented values
for ADR amendment).

---

## Phase 6: User Story 4 — Memory Profile Under Buffered Load (Priority: P2)

**Goal**: Confirm that buffering 1,000,000 unacknowledged JetStream messages keeps the NATS
server RSS below 512 MB, and that memory returns to baseline (within 10%) after consumer drains.

**Independent Test**:
```bash
go test -v -run TestNATSMemoryProfile -timeout 120s ./benchmark/...
```
Pass condition: output contains `[SC-004 PASS: < 512 MB]` and `[leak check PASS]`.

- [ ] T008 [US4] Implement `TestNATSMemoryProfile` in `benchmark/nats_jetstream_spike_test.go`

**Checkpoint**: `go test -v -run TestNATSMemoryProfile ./benchmark/...` prints baseline, peak,
and post-drain RSS in MB; exits PASS when peak < 512 MB and post-drain ≤ baseline × 1.10.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Add the `testing.B` harnesses, run the full suite to capture benchmark numbers,
and produce the output tables needed for the ADR update.

- [ ] T009 Add `BenchmarkNATSPublishCore` and `BenchmarkNATSPublishJetStream` harnesses to `benchmark/nats_jetstream_spike_test.go`
- [ ] T010 [P] Run final compile and skip-guard regression: `go vet ./benchmark/...` exits 0; `go test -count=1 -run TestNATS ./benchmark/...` without Docker exits 0 with all four tests SKIP — confirms T004 acceptance criteria hold for the completed file
- [ ] T011 Run the full benchmark suite with `pheromone-nats` running and capture stdout for ADR tables: `docker compose up -d pheromone-nats && go test -v -bench=. -timeout 120s ./benchmark/...`

**Checkpoint**: T011 output contains passing results for all four `TestNATS*` functions and
non-zero `ns/op` values for both `Benchmark*` functions. Capture this output — it is the
required input for T012.

---

## Phase 8: User Story 5 — ADR-005 Update with Empirical Results (Priority: P3)

**Goal**: Finalise ADR-005 with the measured benchmark numbers, change status from `Proposed`
to `Accepted` (or `Amended` with rationale), and close GitHub Issue #4.

**Independent Test**: Open `docs/adr/adr-005-message-queue.md` and verify:
1. `## Status` line reads `Accepted` or `Amended` (not `Proposed`)
2. `## Spike Results` section is present after `## Testing & Validation`
3. All four result tables contain non-blank measured values
4. `**Decision Date**` and `**Status Update**` lines are updated

- [ ] T012 [US5] Add `## Spike Results (ADR-005 Empirical Validation)` section to `docs/adr/adr-005-message-queue.md` immediately after `## Testing & Validation`
- [ ] T013 [US5] Update status and metadata fields in `docs/adr/adr-005-message-queue.md`: change `## Status` from `Proposed` to `Accepted` or `Amended`, update `**Decision Date**` and `**Status Update**` lines at the bottom of the file

**Checkpoint**: All four result tables in the new section contain actual measured values. ADR
status is no longer `Proposed`. Issue #4 is referenced as closed in the PR description.

---

## Detailed Task Specifications

### T001 — Add NATS service to `docker-compose.yml`

| Field | Detail |
|-------|--------|
| **File** | `docker-compose.yml` |
| **Change** | Append the following block under `services:` (after the `etcd` service block, before `volumes:`) |
| **Port** | `4222:4222` (confirmed free: etcd owns 2379/2380, gRPC owns 4426/4427) |
| **Acceptance** | `docker compose config` parses without error; `docker compose up -d pheromone-nats` starts the container; `docker exec pheromone-nats nats-server --version` prints `v2.10.x` or later |
| **Depends on** | None |

```yaml
  pheromone-nats:
    image: nats:latest
    container_name: pheromone-nats
    ports:
      - "4222:4222"
    command: ["-js"]
    restart: unless-stopped
```

---

### T002 — Add `nats.io/nats.go` dependency

| Field | Detail |
|-------|--------|
| **Files** | `go.mod`, `go.sum` |
| **Change** | Run `go get nats.io/nats.go@latest && go mod tidy` from the repository root |
| **Expected go.mod addition** | `nats.io/nats.go v1.x.x` under `require` (direct); indirect deps `github.com/nats-io/nkeys` and `github.com/nats-io/nuid` added to `go.sum` automatically |
| **Acceptance** | `go build ./...` exits 0; `grep 'nats.io/nats.go' go.mod` returns a version line; no existing dependency versions are changed |
| **Depends on** | None |

---

### T003 — Create `benchmark/nats_jetstream_spike_test.go` skeleton

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (new file) |
| **Acceptance** | `go vet ./benchmark/...` exits 0; `go test -list '.*' ./benchmark/...` lists the four `TestNATS*` names (even though bodies are stubs); no functions from `grpc_spike_test.go` are duplicated |
| **Depends on** | T002 |

The skeleton must contain exactly these sections in order:

**1. File header comment**
```go
// Tech Spike: ADR-005 — NATS JetStream Benchmark
//
// Validates NATS JetStream against ADR-005 success criteria:
//
//   SC-001: Sustained throughput ≥ 100,000 msgs/sec (zero loss)
//   SC-002: p95 end-to-end latency < 10 ms at 100K msgs/sec
//   SC-003: p99 end-to-end latency < 50 ms at 100K msgs/sec
//   SC-004: NATS server RSS < 512 MB buffering 1M messages
//   SC-005: JetStream persistence overhead ≤ 20% vs core NATS
```

**2. Package declaration**
```go
package benchmark
```

**3. Import block** — include `nats "nats.io/nats.go"` plus stdlib packages:
`encoding/binary`, `fmt`, `math/rand`, `os/exec`, `runtime`, `sort`,
`strconv`, `strings`, `sync`, `sync/atomic`, `testing`, `time`

**4. Threshold constants**
```go
const (
    natsTargetRateMsgsPerSec = 100_000   // SC-001
    natsP95ThresholdMs       = 10.0      // SC-002
    natsP99ThresholdMs       = 50.0      // SC-003
    natsMemThresholdMB       = 512.0     // SC-004
    natsJSOverheadPct        = 20.0      // SC-005
    natsURL                  = "nats://localhost:4222"
    natsStream               = "TELEMETRY"
    natsSubject              = "metrics.test"
    natsPayloadSize          = 256
    natsRampHoldSec          = 5
    natsProducerDefault      = 4
)
```

**5. `natsConnect(t testing.TB) *nats.Conn`** — availability guard
```go
func natsConnect(t testing.TB) *nats.Conn {
    t.Helper()
    nc, err := nats.Connect(natsURL, nats.Timeout(500*time.Millisecond))
    if err != nil {
        t.Skipf("NATS unavailable at %s (%v) — skipping"+
            " (run: docker compose up -d pheromone-nats)", natsURL, err)
    }
    t.Cleanup(func() { nc.Drain() })
    return nc
}
```

**6. `natsCreateStream(t testing.TB, js nats.JetStreamContext)`** — idempotent
JetStream stream creation using `js.AddStream` with `natsStream`/`natsSubject`,
`nats.FileStorage`, `nats.LimitsPolicy`, `MaxAge: 24*time.Hour`, `MaxMsgs: 10_000_000`,
`Replicas: 1`. Register `t.Cleanup` to call `js.DeleteStream(natsStream)`.

**7. `natsPercentile(sorted []time.Duration, p float64) time.Duration`** — identical
signature and body to `grpcPercentile` in `benchmark/grpc_spike_test.go`:
```go
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
```

**8. `natsServerMemMB(t testing.TB) float64`** — queries `docker stats --no-stream
--format {{.MemUsage}} pheromone-nats`; parses the leading numeric value in MiB/GiB;
falls back to `runtime.ReadMemStats` heap alloc if docker is unavailable, logging
`"docker stats unavailable — using in-process heap as proxy"`.

---

### T004 — Verify skeleton compiles and skip guard works

| Field | Detail |
|-------|--------|
| **Files** | Read-only verification step (no file changes) |
| **Commands** | `go vet ./benchmark/...` then `go test -count=1 -run TestNATS ./benchmark/...` (NATS must NOT be running) |
| **Acceptance** | `go vet` exits 0 with no output. Test run exits 0. Each of the four `TestNATS*` functions prints `--- SKIP: TestNATSxxx (0.00s)`. No test prints `--- FAIL`. |
| **Depends on** | T001 (skip message references the docker compose service name), T003 |

---

### T005 — Implement `TestNATSThroughputRamp`

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (append function) |
| **Acceptance** | `go test -v -run TestNATSThroughputRamp -timeout 60s ./benchmark/...` (NATS running) exits with PASS or FAIL; never panics or hangs; every stage logs `target → measured msgs/sec  lost=N` |
| **Depends on** | T003 |

Implementation requirements:
- Call `natsConnect(t)` first (skip guard)
- Acquire JetStream context via `nc.JetStream()`; call `natsCreateStream(t, js)`
- Read producer count from `NATS_PRODUCERS` env var; default to `natsProducerDefault` (4)
- Define stages: `[]struct{rate int; label string}` = `{1000,"1K"}, {10000,"10K"}, {50000,"50K"}, {100000,"100K"}`
- Per stage: compute inter-message sleep as `time.Second / rate / producers`; run `producers` goroutines each calling `js.Publish(natsSubject, payload)` in a tight loop for `natsRampHoldSec` seconds; collect actual publish count via `sync/atomic`
- Build payload as `[natsPayloadSize]byte` with `binary.BigEndian.PutUint64(payload[0:8], uint64(time.Now().UnixNano()))` and `binary.BigEndian.PutUint64(payload[8:16], seqNum)` per message
- After each stage: measure `lostMsgs` by comparing expected vs acknowledged sequence gaps
- Log per stage: `t.Logf("Stage %s: %d target → %d msgs/sec  lost=%d", label, target, measured, lost)`
- After the 100K stage: `t.Errorf` if `measuredRate < natsTargetRateMsgsPerSec || lost > 0`; otherwise log `[SC-001 PASS]`

---

### T006 — Implement `TestNATSConsumerLatency`

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (append function) |
| **Acceptance** | `go test -v -run TestNATSConsumerLatency -timeout 60s ./benchmark/...` (NATS running) prints a p50/p95/p99 histogram in ms; `t.Error` fires if p95 ≥ 10 ms or p99 ≥ 50 ms |
| **Depends on** | T003, T005 (same file — add after `TestNATSThroughputRamp`) |

Implementation requirements:
- Call `natsConnect(t)` first; acquire JetStream context; call `natsCreateStream(t, js)`
- Subscribe with push consumer: `nc.Subscribe(natsSubject, handler)` where handler reads `publish_ns` from `msg.Data[0:8]` via `binary.BigEndian.Uint64`, computes delta = `time.Now().UnixNano() - publish_ns`, appends to `latencies []time.Duration` via mutex
- Producer goroutine: publish at `natsTargetRateMsgsPerSec` msgs/sec for 30 seconds embedding `time.Now().UnixNano()` at payload offset 0
- After producer completes: wait for consumer to drain (100 ms timeout per batch)
- Sort `latencies` and call `natsPercentile` for p50 (50), p95 (95), p99 (99)
- Log: `t.Logf("Latency @ 100K msgs/sec — P50: %.2fms  P95: %.2fms  P99: %.2fms", p50ms, p95ms, p99ms)`
- `t.Errorf` if `p95ms >= natsP95ThresholdMs` or `p99ms >= natsP99ThresholdMs`; otherwise log `[SC-002 PASS]` and `[SC-003 PASS]`

---

### T007 — Implement `TestNATSJetStreamOverhead`

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (append function) |
| **Acceptance** | `go test -v -run TestNATSJetStreamOverhead -timeout 90s ./benchmark/...` (NATS running) prints a comparison table; `t.Error` fires only when an overhead column exceeds 20% |
| **Depends on** | T003, T006 (same file) |

Implementation requirements:
- **Core NATS run** (no JetStream): use `nc.Publish(natsSubject, payload)` (plain subject, no stream needed); run for 30 seconds; record `coreRate float64` and `coreP95Ms float64`
- **JetStream run**: call `natsCreateStream(t, js)`; use `js.Publish(natsSubject, payload)` for 30 seconds; record `jsRate float64` and `jsP95Ms float64`
- Calculate: `tpOverhead = (coreRate-jsRate)/coreRate*100`; `latOverhead = (jsP95Ms-coreP95Ms)/coreP95Ms*100`
- Log comparison table:
  ```
  t.Logf("Core NATS:  %.0f msgs/sec  p95=%.2fms", coreRate, coreP95Ms)
  t.Logf("JetStream:  %.0f msgs/sec  p95=%.2fms", jsRate, jsP95Ms)
  t.Logf("Overhead:  throughput=%.1f%%  p95=%.1f%%", tpOverhead, latOverhead)
  ```
- `t.Errorf` if `tpOverhead > natsJSOverheadPct || latOverhead > natsJSOverheadPct`; otherwise log `[SC-005 PASS]`

---

### T008 — Implement `TestNATSMemoryProfile`

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (append function) |
| **Acceptance** | `go test -v -run TestNATSMemoryProfile -timeout 120s ./benchmark/...` (NATS running) prints baseline, peak, and post-drain RSS; `t.Error` fires if peak ≥ 512 MB or post-drain > baseline × 1.10 |
| **Depends on** | T003, T007 (same file) |

Implementation requirements:
- Call `natsConnect(t)`; acquire JS context; call `natsCreateStream(t, js)`
- Record `baselineMB := natsServerMemMB(t)`
- Create a drain gate channel `gate := make(chan struct{})` to pause the consumer
- Subscribe with handler that blocks on `gate` before acking each message (simulates paused consumer)
- Publish 1,000,000 × 256-byte messages to `natsSubject`; poll `natsServerMemMB` every 5 seconds to find `peakMB`
- Log `t.Logf("Baseline RSS: %.1f MB", baselineMB)` and `t.Logf("Peak RSS (1M msgs): %.1f MB  [SC-004 %s]", peakMB, passFailStr)`
- `t.Errorf` if `peakMB >= natsMemThresholdMB`
- Close `gate` to resume consumer; wait for stream to drain (sequence count via `js.StreamInfo`)
- Record `postDrainMB := natsServerMemMB(t)`; `t.Errorf` if `postDrainMB > baselineMB*1.10`; log `[leak check PASS/FAIL]`

---

### T009 — Add `BenchmarkNATSPublishCore` and `BenchmarkNATSPublishJetStream`

| Field | Detail |
|-------|--------|
| **File** | `benchmark/nats_jetstream_spike_test.go` (append two functions) |
| **Acceptance** | `go test -bench=BenchmarkNATS -benchtime=5s ./benchmark/...` (NATS running) reports non-zero `ns/op` and `MB/s` for both harnesses; matches structural pattern of `BenchmarkGRPCTelemetryStream` in `grpc_spike_test.go` |
| **Depends on** | T008 (file complete) |

**`BenchmarkNATSPublishCore`**: `natsConnect(b)` → `b.SetBytes(natsPayloadSize)` → `b.ResetTimer()` → loop `b.N` iterations calling `nc.Publish(natsSubject, payload)`.

**`BenchmarkNATSPublishJetStream`**: `natsConnect(b)` → JS context → `natsCreateStream(b, js)` → `b.SetBytes(natsPayloadSize)` → `b.ResetTimer()` → loop `b.N` iterations calling `js.Publish(natsSubject, payload)`.

---

### T010 — Final compile and skip-guard regression

| Field | Detail |
|-------|--------|
| **Files** | Read-only verification (no file changes) |
| **Commands** | `go vet ./benchmark/...` then `go test -count=1 -run TestNATS ./benchmark/...` without NATS |
| **Acceptance** | Identical to T004: `go vet` clean; all four `TestNATS*` report SKIP; no FAIL |
| **Depends on** | T009 |

---

### T011 — Run full benchmark suite and capture output

| Field | Detail |
|-------|--------|
| **Files** | No source changes — execution only |
| **Command** | `docker compose up -d pheromone-nats && sleep 2 && go test -v -bench=. -timeout 120s ./benchmark/... 2>&1 \| tee /tmp/nats_spike_results.txt` |
| **Acceptance** | Exit code 0 (or documented non-zero with explanation). All four `TestNATS*` report PASS or FAIL (never SKIP or panic). Both `Benchmark*` report `ns/op`. `/tmp/nats_spike_results.txt` contains the full output for copy-paste into T012 tables. |
| **Depends on** | T001 (NATS service defined), T010 (file fully verified) |

---

### T012 — Add `## Spike Results` section to ADR-005

| Field | Detail |
|-------|--------|
| **File** | `docs/adr/adr-005-message-queue.md` |
| **Change** | Insert the block below immediately after the closing line of `## Testing & Validation` |
| **Acceptance** | File parses as valid Markdown. All four result tables present. No table cell contains `_____` (blank placeholder) — every cell must have a real measured value from T011 output. |
| **Depends on** | T011 (requires actual measured numbers) |

Insert this section (fill values from T011 output):

```markdown
## Spike Results (ADR-005 Empirical Validation)

**Run Date**: [DATE from T011 run]
**Branch**: feature/ADR-005-nats-jetstream-spike
**Host**: [CPU model, RAM, storage type]
**NATS Version**: [output of `docker exec pheromone-nats nats-server --version`]

### Throughput (SC-001)

| Stage | Target (msgs/sec) | Measured (msgs/sec) | Lost | Result |
|-------|-------------------|---------------------|------|--------|
| 1K    | 1,000             | [measured]          | 0    | PASS   |
| 10K   | 10,000            | [measured]          | 0    | PASS   |
| 50K   | 50,000            | [measured]          | 0    | PASS   |
| 100K  | 100,000           | [measured]          | 0    | [PASS/FAIL] |

### Latency at 100K msgs/sec (SC-002, SC-003)

| Percentile | Threshold | Measured | Result      |
|------------|-----------|----------|-------------|
| p50        | —         | [X] ms   | —           |
| p95        | < 10 ms   | [X] ms   | [PASS/FAIL] |
| p99        | < 50 ms   | [X] ms   | [PASS/FAIL] |

### JetStream vs Core NATS Overhead (SC-005)

| Metric      | Core NATS   | JetStream   | Overhead | Threshold | Result      |
|-------------|-------------|-------------|----------|-----------|-------------|
| Throughput  | [X] msgs/s  | [X] msgs/s  | [X]%     | ≤ 20%    | [PASS/FAIL] |
| p95 Latency | [X] ms      | [X] ms      | [X]%     | ≤ 20%    | [PASS/FAIL] |

### Memory Under 1M Buffered Messages (SC-004)

| Measurement                | Value   | Threshold        | Result      |
|----------------------------|---------|------------------|-------------|
| Baseline RSS (pre-publish) | [X] MB  | —                | —           |
| Peak RSS (1M msgs buffered)| [X] MB  | < 512 MB         | [PASS/FAIL] |
| Post-drain RSS             | [X] MB  | ≤ baseline + 10% | [PASS/FAIL] |

### Overall ADR-005 Verdict

[ACCEPTED — all SC-001 through SC-005 criteria met]
OR
[AMENDED — <rationale: which SC failed, revised target or Phase 2 acceleration>]
```

---

### T013 — Update ADR-005 status and metadata

| Field | Detail |
|-------|--------|
| **File** | `docs/adr/adr-005-message-queue.md` |
| **Changes** | Three targeted edits (do not rewrite the whole file) |
| **Acceptance** | `grep 'Proposed' docs/adr/adr-005-message-queue.md` returns no matches. `grep -E 'Accepted\|Amended' docs/adr/adr-005-message-queue.md` returns at least two matches (status line + verdict). Decision Date is updated. |
| **Depends on** | T012 |

1. **`## Status` line** (top of file, line 4): change `Proposed` → `Accepted` or `Amended`
2. **`**Decision Date**` line** (bottom of file): update from `2026-02-18` to the actual run date
3. **`**Status Update**` line** (bottom of file): change from `Proposed (pending load test spike with Go NATS client)` to `Accepted (spike completed on feature/ADR-005-nats-jetstream-spike — closes #4)` or `Amended (spike completed — [brief rationale])`

---

## Dependencies & Execution Order

```
T001 [P] docker-compose.yml          ←── no deps; start immediately
T002 [P] go get nats.io/nats.go      ←── no deps; start immediately
          │
          ▼
T003     Create skeleton file         ←── depends on T002
          │
          ▼
T004     Verify vet + skip guard      ←── depends on T001 + T003
          │
    ┌─────▼──────────────────────────────────────────┐
    │   (same file — sequential additions)            │
    │   T005 → T006 → T007 → T008                    │
    │   [US1]   [US2]   [US3]   [US4]                │
    └─────────────────────────────────────────────────┘
          │
          ▼
T009     Add Benchmark* harnesses     ←── depends on T008 (file complete)
          │
          ▼
T010     Final vet + skip regression  ←── depends on T009
          │
          ▼
T011     Run suite + capture output   ←── depends on T001 + T010
          │
          ▼
T012     ADR Spike Results section    ←── depends on T011 (needs measured values)
          │
          ▼
T013     ADR status + metadata        ←── depends on T012
```

### User Story Dependencies

| Story | Priority | File Dependency | Independently Runnable |
|-------|----------|-----------------|----------------------|
| US1 — Throughput Ramp | P1 | T003 complete | `go test -run TestNATSThroughputRamp` |
| US2 — Latency | P1 | T005 complete (same file) | `go test -run TestNATSConsumerLatency` |
| US3 — JetStream Overhead | P2 | T006 complete (same file) | `go test -run TestNATSJetStreamOverhead` |
| US4 — Memory Profile | P2 | T007 complete (same file) | `go test -run TestNATSMemoryProfile` |
| US5 — ADR Update | P3 | T011 output captured | Review `docs/adr/adr-005-message-queue.md` |

### Parallel Opportunities

- **T001 ∥ T002** — different files; always run together
- **T010 ∥ T011** — if NATS is already up, vet and benchmark run can overlap in two terminals
- **T005–T008** — same file; must be sequential, but each story's _test run_ is independently invocable once the function exists

---

## Parallel Example: Phase 1

```bash
# Both setup tasks target different files — run simultaneously:

# Terminal 1 — add NATS service to docker-compose.yml (T001):
# Edit docker-compose.yml: append pheromone-nats block under services:
docker compose config   # verify parse

# Terminal 2 — add Go client dependency (T002):
go get nats.io/nats.go@latest
go mod tidy
git diff go.mod   # confirm nats.io/nats.go line added
```

## Parallel Example: Phase 7 Validation

```bash
# T010 and T011 can overlap if NATS is already running:

# Terminal 1 — compile gate (T010):
go vet ./benchmark/...
go test -count=1 -run TestNATS ./benchmark/...   # must all SKIP (Docker down)

# Terminal 2 — full run (T011), requires T001 service running:
docker compose up -d pheromone-nats
go test -v -bench=. -timeout 120s ./benchmark/... 2>&1 | tee /tmp/nats_spike_results.txt
```

---

## Implementation Strategy

### MVP: SC-001 Throughput Gate Only (US1)

1. ✅ T001 + T002 — infrastructure ready
2. ✅ T003 + T004 — skeleton compiles; skip guard verified
3. ✅ T005 — `TestNATSThroughputRamp` implemented
4. **STOP and VALIDATE**: `docker compose up -d pheromone-nats && go test -v -run TestNATSThroughputRamp ./benchmark/...`
5. SC-001 PASS → proceed to US2 (T006). SC-001 FAIL → record result, go straight to T012 + T013 with `Amended` status.

### Full Spike (All 5 Stories)

| Phase | Tasks | Duration (est.) |
|-------|-------|-----------------|
| Setup | T001, T002 | < 5 min |
| Skeleton | T003, T004 | 20 min |
| US1 Throughput | T005 | 30 min + 21 s run |
| US2 Latency | T006 | 20 min + 32 s run |
| US3 Overhead | T007 | 20 min + 60 s run |
| US4 Memory | T008 | 25 min + 45 s run |
| Polish | T009, T010, T011 | 15 min + 120 s run |
| ADR Update | T012, T013 | 20 min |
| **Total** | | **~3 h including benchmark runs** |

### Incremental Delivery Checkpoints

| Checkpoint | Tasks Done | Observable State |
|-----------|-----------|-----------------|
| Infrastructure ready | T001–T002 | `docker compose up -d pheromone-nats` succeeds; `nats.io/nats.go` importable |
| Skeleton green | T003–T004 | `go vet` clean; 4× SKIP without Docker |
| SC-001 validated | T005 | Throughput ramp test produces a measurable result |
| SC-002/003 validated | T006 | Latency histogram produced with p50/p95/p99 |
| SC-005 validated | T007 | Overhead % measured and compared |
| SC-004 validated | T008 | Memory ceiling checked; leak detection run |
| Suite complete | T009–T011 | Full `go test -bench=.` produces all result tables in stdout |
| **ADR accepted** | T012–T013 | ADR-005 status ≠ Proposed; all table cells filled; Issue #4 closeable |

---

## Notes

- File name is `benchmark/nats_jetstream_spike_test.go` per FR-002 and plan.md §1c — not `nats_spike_test.go`
- `go test ./...` on packages outside `benchmark/` must remain unaffected — spike is non-invasive to `internal/`
- `natsPercentile` must not be named `grpcPercentile` — two separate functions in the same package; same body is fine
- Commit message convention: `feat(benchmark): ADR-005 NATS JetStream spike — <phase summary>`
- PR description must include `Closes #4` and link `docs/adr/adr-005-message-queue.md`
- SC-006 (CI hardware note): if throughput falls below 100K on CI runners (2 cores), use `t.Logf` not `t.Error` for the throughput gate on CI — annotate result but do not auto-fail; developer-machine results are authoritative for the ADR decision
- [P] = different files, no dependency on concurrent task output
- [US#] label maps each implementation task to its user story for traceability

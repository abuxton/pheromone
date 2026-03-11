# Feature Specification: NATS JetStream Load Test Spike

**Feature Branch**: `feature/ADR-005-nats-jetstream-spike`
**Spec Directory**: `specs/004-nats-jetstream-spike/`
**Created**: 2025-01-27
**Status**: Draft
**GitHub Issue**: [#4 — Tech Spike: Validate NATS JetStream at 100K msgs/sec](https://github.com/abuxton/pheromone/issues/4)
**ADR Reference**: `docs/adr/adr-005-message-queue.md` (ADR-005)

---

## Clarifications

### Session 2026-03-11

- Q: What is the canonical JetStream stream name and subject? → A: Stream name `TELEMETRY`, subject `metrics.test`
- Q: What test duration and ramp pattern should the producer benchmark use? → A: Ramp 1K→10K→50K→100K msgs/sec, 5-second hold at each stage (20 seconds total)
- Q: What is the exact Go package name and CI invocation for the benchmark file? → A: `package benchmark`; runnable via `go test -bench=. ./benchmark/...` with Docker running
- Q: How is JetStream enabled in the Docker Compose NATS service? → A: Via the `-js` server flag on the `nats:latest` image (port 4222)
- Q: Is a gRPC-to-NATS bridge required as part of this spike? → A: No — this is a spike/benchmark only; no production gRPC bridge is in scope

---

## Overview

This spike validates that NATS with JetStream persistence can sustain 100,000 messages per second with acceptable latency and memory characteristics, as required by ADR-005 (Phase 1 message queue decision) and SC-008. The outcome determines whether NATS is fit-for-purpose for Pheromone's telemetry aggregation pipeline before any production integration begins.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Producer Throughput Validation (Priority: P1)

As a **platform engineer evaluating NATS for Phase 1**, I need to confirm that a single NATS server with JetStream enabled can accept and acknowledge 100,000 messages per second from concurrent Go producers, so that I have evidence to confirm or revise the ADR-005 decision.

**Why this priority**: This is the core gate for the entire spike. If sustained throughput cannot reach 100K msgs/sec, the ADR must be updated before any NATS integration work begins.

**Independent Test**: Run the benchmark producer harness in isolation against a local NATS Docker container. The test passes when `msgs_per_sec ≥ 100,000` is reported during the 5-second hold at the 100K msgs/sec stage with zero message loss.

**Acceptance Scenarios**:

1. **Given** a NATS server with JetStream enabled is running via Docker Compose, **When** the Go producer benchmark ramps publish rate from 1K → 10K → 50K → 100K msgs/sec (5-second hold at each stage), **Then** sustained throughput of ≥ 100,000 messages per second is measured and reported during the 100K msgs/sec stage.
2. **Given** the producer benchmark completes, **When** the benchmark report is read, **Then** zero messages are reported lost or unacknowledged during the sustained window.
3. **Given** producer count is scaled from 1 to 10 concurrent goroutines, **When** throughput is measured at each concurrency level, **Then** aggregate throughput increases sub-linearly (confirming NATS is not the bottleneck at the wire level).

---

### User Story 2 — Consumer Delivery Latency Measurement (Priority: P1)

As a **platform engineer**, I need to measure end-to-end message delivery latency at p50, p95, and p99 percentiles under the 100K msgs/sec load, so that I can confirm NATS meets the latency bounds specified in ADR-005 (p95 < 10 ms, p99 < 50 ms).

**Why this priority**: Throughput without latency data is insufficient — Pheromone's telemetry pipeline must deliver metrics with low enough latency for real-time observability dashboards.

**Independent Test**: Run producer and consumer benchmark together, instrument timestamp deltas, and report a percentile histogram. Test passes when the p95 column reads < 10 ms and p99 column reads < 50 ms.

**Acceptance Scenarios**:

1. **Given** both producer and consumer goroutines are running at full 100K msgs/sec load, **When** end-to-end timestamps are recorded per message, **Then** the p50 latency is reported.
2. **Given** the latency histogram is generated, **When** the p95 percentile is read, **Then** it is less than 10 milliseconds.
3. **Given** the latency histogram is generated, **When** the p99 percentile is read, **Then** it is less than 50 milliseconds.
4. **Given** the NATS server is placed under sustained maximum load, **When** latency is measured at each rate stage during the ramp (1K → 10K → 50K → 100K msgs/sec, 5 seconds per stage), **Then** p95 latency does not exceed 10 ms at the 100K msgs/sec stage (no latency cliff at peak load).

---

### User Story 3 — JetStream Persistence Overhead Assessment (Priority: P2)

As a **platform engineer**, I need to compare throughput and latency when JetStream persistence is enabled versus when only core NATS (no persistence) is used, so that I can quantify the persistence overhead and confirm it stays within ADR-005's 20% budget.

**Why this priority**: ADR-005 requires JetStream for at-least-once delivery guarantees. The overhead must be understood and bounded before committing to it in production.

**Independent Test**: Run the same benchmark twice — once with a plain NATS subject (no JetStream stream), once with a JetStream stream configured. Compare the throughput and p95 latency. Overhead budget is ≤ 20%.

**Acceptance Scenarios**:

1. **Given** the benchmark runs with JetStream disabled (core NATS only), **When** throughput and p95 latency are measured, **Then** a baseline result set is recorded.
2. **Given** the benchmark runs with JetStream enabled (stream with file-backed persistence), **When** throughput and p95 latency are measured, **Then** a JetStream result set is recorded.
3. **Given** both result sets are available, **When** overhead is calculated as `(jetstream_result - baseline) / baseline * 100`, **Then** throughput overhead is ≤ 20% and p95 latency overhead is ≤ 20%.

---

### User Story 4 — Memory Profile Under Buffered Load (Priority: P2)

As a **platform engineer**, I need to measure the NATS server's memory consumption while buffering 1 million unacknowledged messages, so that I can confirm it stays within ADR-005's 512 MB ceiling and does not grow unboundedly under backpressure.

**Why this priority**: Memory safety is critical for the single-server Phase 1 deployment where NATS shares host resources with the gRPC control plane and etcd.

**Independent Test**: Publish 1 million messages with JetStream persistence enabled but pause the consumer. Measure server RSS before, during, and after accumulation. Test passes when peak RSS < 512 MB.

**Acceptance Scenarios**:

1. **Given** a JetStream stream is configured and the consumer is paused, **When** 1,000,000 messages are published and held in the stream, **Then** the NATS server's resident memory usage is recorded.
2. **Given** 1 million messages are buffered, **When** peak memory is compared to the ADR-005 budget, **Then** peak RSS is less than 512 MB.
3. **Given** the consumer resumes and drains the stream, **When** memory is measured after drain completes, **Then** memory returns to within 10% of the pre-publish baseline (no permanent leak).

---

### User Story 5 — ADR-005 Update with Empirical Results (Priority: P3)

As a **platform architect**, I need the benchmark results documented in ADR-005 so that the decision record reflects validated evidence, and so that future engineers can understand the validated performance envelope of the Phase 1 NATS deployment.

**Why this priority**: Without updating ADR-005, the decision remains "proposed" with no empirical backing. This story has no code deliverable but finalises the spike.

**Independent Test**: Open `docs/adr/adr-005-message-queue.md` and confirm the "Testing & Validation" section contains actual measured numbers from the benchmark run, and that the status has been changed from "Proposed" to "Accepted" or "Amended" as appropriate.

**Acceptance Scenarios**:

1. **Given** all benchmark runs have completed, **When** ADR-005 is reviewed, **Then** it contains a "Spike Results" section with throughput, p95/p99 latency, memory, and JetStream overhead figures from the actual benchmark.
2. **Given** all ADR-005 success criteria are met by benchmark data, **When** the ADR status is updated, **Then** the status field reads "Accepted" and a decision date is recorded.
3. **Given** any ADR-005 success criterion is NOT met, **When** the ADR is updated, **Then** the status reads "Amended" with a documented rationale for the revised decision (e.g., reduced throughput target or Kafka Phase 2 acceleration).

---

### Edge Cases

- What happens when the NATS server runs out of disk space during JetStream persistence (file-backed stream)?
- How does throughput degrade when the consumer falls behind and the JetStream stream reaches its configured maximum size?
- What is the behaviour when the NATS server is restarted mid-benchmark — are JetStream-persisted messages recovered?
- How does the benchmark behave on a CI runner with constrained CPU (2 cores) versus a developer laptop (8+ cores)?
- What happens when message payload size is varied (e.g., 64 bytes vs. 1 KB vs. 16 KB)?

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST include a NATS service definition in `docker-compose.yml` with JetStream enabled via the `-js` server flag, runnable via `docker compose up nats`, using the official `nats:latest` image (≥ 2.10) on port 4222 with container name `pheromone-nats`.

- **FR-002**: The project MUST include a Go benchmark file at `benchmark/nats_jetstream_spike_test.go` in `package benchmark` — matching `benchmark/grpc_spike_test.go` exactly in structure: same package declaration, named constants for ADR-005 success thresholds, `testing.B` harness for benchmark functions, `testing.T` harness for validation tests, and a printed summary table to stdout on completion. The full suite MUST be invocable via `go test -bench=. ./benchmark/...` with Docker running, without additional manual setup.

- **FR-003**: The benchmark MUST include a producer test that ramps publish rate through four stages — 1,000, 10,000, 50,000, and 100,000 msgs/sec — with a 5-second hold at each rate (20 seconds total), using concurrent producer goroutines (configurable, default 4). Per-stage throughput and aggregate results are reported to stdout. The 100K msgs/sec stage constitutes the primary pass/fail window for SC-001.

- **FR-004**: The benchmark MUST include a consumer test that instruments per-message timestamps to calculate and report a latency percentile histogram including p50, p95, and p99 values in milliseconds.

- **FR-005**: The benchmark MUST include a comparative test that runs both the core NATS (persistence-off) and JetStream (persistence-on) scenarios back-to-back and reports the overhead percentage for throughput and p95 latency.

- **FR-006**: The benchmark MUST include a memory profiling test that publishes 1,000,000 messages with consumer paused, records peak NATS server RSS (via Docker stats or `/proc`-equivalent), and prints a pass/fail result against the 512 MB threshold.

- **FR-007**: The `docs/adr/adr-005-message-queue.md` file MUST be updated with a "Spike Results" section containing the empirical benchmark figures and the ADR status MUST be changed from "Proposed" to either "Accepted" or "Amended" with rationale.

### Key Entities

- **NATS Stream** (`TELEMETRY`): A JetStream stream named `TELEMETRY` configured with subject `metrics.test`, file-backed storage, 24-hour retention, and a maximum of 10 million messages. Represents the durable telemetry buffer used exclusively for spike benchmarking.
- **Benchmark Message**: A fixed-size byte payload (default 256 bytes) with an embedded nanosecond-precision publish timestamp, used as the unit of throughput and latency measurement.
- **Benchmark Result**: A structured record containing: run timestamp, scenario name (core/jetstream), concurrency level, measured throughput (msgs/sec), p50/p95/p99 latency (ms), peak memory (MB), and pass/fail verdict per ADR-005 criterion.
- **ADR-005 Spike Evidence**: The set of Benchmark Results appended to the ADR's "Testing & Validation" section, constituting the empirical basis for the ADR status decision.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001** *(maps to ADR-005 SC-008)*: Sustained throughput of ≥ 100,000 messages per second is demonstrated during the 5-second hold at the 100K msgs/sec ramp stage (following 1K → 10K → 50K warm-up stages of 5 seconds each) with zero message loss.

- **SC-002** *(maps to ADR-005 p95 target)*: End-to-end message delivery latency at the p95 percentile is less than 10 milliseconds under 100K msgs/sec load.

- **SC-003** *(maps to ADR-005 p99 target)*: End-to-end message delivery latency at the p99 percentile is less than 50 milliseconds under 100K msgs/sec load.

- **SC-004** *(maps to ADR-005 memory budget)*: NATS server peak resident memory usage does not exceed 512 MB while buffering 1,000,000 messages with JetStream persistence enabled.

- **SC-005** *(maps to ADR-005 persistence overhead)*: JetStream persistence overhead versus core NATS is ≤ 20% for both throughput and p95 latency, measured on identical hardware in the same benchmark run.

- **SC-006**: The benchmark completes successfully in a Docker-based CI environment (GitHub Actions or equivalent) when invoked as `go test -bench=. ./benchmark/...` with Docker running, without manual intervention, producing a machine-readable result summary.

- **SC-007**: ADR-005 status is updated from "Proposed" to "Accepted" or "Amended" with empirical evidence, closing GitHub Issue #4.

---

## Assumptions

- The spike runs on a single host; no NATS cluster configuration is required for Phase 1 validation.
- Default message payload size is 256 bytes, matching a representative OpenMetrics-format telemetry record for Pheromone agents. Payload size sensitivity is noted as an edge case but not benchmarked exhaustively in this spike.
- NATS version `latest` (≥ 2.10) is used; JetStream is enabled via the `-js` command flag as shown in ADR-005. The stream `TELEMETRY` with subject `metrics.test` is created programmatically at benchmark startup.
- CI environment provides at minimum 2 CPU cores and 4 GB RAM for the benchmark container; results on constrained CI runners are annotated but do not override developer-machine results for pass/fail determination.
- Memory measurement uses Docker stats (`docker stats --no-stream`) as a portable approximation; OS-level RSS is the source of truth on developer machines.
- The NATS Go client (`nats.io/nats.go`) will be added to `go.mod` as part of this spike; no other new runtime dependencies are introduced.
- Benchmark follows the exact structural pattern of `benchmark/grpc_spike_test.go` for consistency: `package benchmark`, named constants for success thresholds, `testing.B` harness, and a printed summary table.
- Port 4222 is reserved for NATS in `docker-compose.yml` and does not conflict with existing services (etcd uses 2379/2380, gRPC uses 4426/4427).
- The benchmark connect URL is `nats://localhost:4222`; no TLS or authentication is configured for the spike environment.
- Total benchmark wall-clock time for the ramp test is approximately 20 seconds (4 stages × 5 seconds each) plus NATS startup; the suite is designed to complete within GitHub Actions default job timeout limits.

---

## Dependencies

- **ADR-005** (`docs/adr/adr-005-message-queue.md`): This spike directly validates and updates ADR-005. ADR-005 must not be merged as "Accepted" before this spike completes.
- **GitHub Issue #4**: Closing Issue #4 is a deliverable of this spike (SC-007).
- **`benchmark/grpc_spike_test.go`**: Reference implementation; the NATS benchmark MUST follow its structural conventions.
- **`docker-compose.yml`**: Must be extended (not replaced) with the NATS service definition (FR-001).
- **`go.mod`**: Must be updated to add `nats.io/nats.go` dependency before the benchmark can be compiled.
- **ADR-003** (`docs/adr/adr-003-grpc-services.md`): The gRPC TelemetryStream spike (whose benchmark is in `grpc_spike_test.go`) validated SC-008 at the gRPC layer; this spike validates the same SC-008 target at the NATS layer.

---

## Out of Scope

- Configuring NATS cluster mode or NATS JetStream clustering (Phase 2 concern).
- Implementing the production gRPC-to-NATS bridge (this spike validates NATS in isolation as a pure benchmark; no gRPC bridge, integration layer, or production wiring is in scope — governed by a separate feature after this spike is accepted).
- Kafka evaluation or migration planning (ADR-005 Phase 2; a separate ADR-TBD).
- Multi-consumer fan-out testing (multiple exporters subscribing to the same stream) — noted as follow-up.
- TLS/authentication configuration for the NATS server.
- Message schema or Protobuf encoding for benchmark payloads (raw bytes sufficient for throughput/latency measurement).

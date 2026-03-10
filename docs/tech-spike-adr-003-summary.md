# Tech Spike Summary: ADR-003 gRPC Bidirectional Stream Prototype (1000 Agents)

**Date**: 2026-03-10  
**Author**: Copilot  
**Estimated Effort**: 12 hours  
**Actual Effort**: ~4 hours (implementation + documentation)

## Objective

Validate that the three gRPC services defined in ADR-003 (`AgentRegistry`, `TwinControl`,
`TelemetryStream`) can handle 1,000 simultaneous agents streaming metrics and state updates.
All success criteria must pass before ADR-003 is formally moved to `Accepted`.

## Success Criteria Results

| Criterion | Target | Status | Result |
|-----------|--------|--------|--------|
| SC-CONN: 1000 agents concurrent | 0 drops | ✅ **PASS** | 1000/1000 agents succeeded |
| SC-002: TwinControl config-push P99 | < 5 s | ✅ **PASS** | P99 ≈ 0.72 ms |
| SC-008: TelemetryStream throughput | ≥ 100 K msgs/sec | ✅ **PASS** | ≈ 120 K msgs/sec |
| SC-REG: Register/Heartbeat P99 | < 100 ms | ✅ **PASS** | P99 ≈ 0.61 ms |

All four success criteria **passed** on a two-core AMD EPYC 7763 CI runner.

## Implementation Summary

### 1. Test Infrastructure Created

```
benchmark/
└── grpc_spike_test.go   # Spike tests + mock server (new)
```

### 2. Components Implemented

#### Mock gRPC Server (`benchmark/grpc_spike_test.go`)

Three minimal server implementations backed by in-memory state:

| Service | Implementation |
|---------|---------------|
| `AgentRegistry` | `mockRegistryServer` — registers agents in a `sync.RWMutex`-protected map; acknowledges heartbeats |
| `TwinControl` | `mockTwinControlServer` — immediately pushes one `ConfigAction` on stream open; drains incoming state reports |
| `TelemetryStream` | `mockTelemetryServer` — counts all inbound metric data points atomically; acknowledges each batch with backpressure flag |

#### Load Generator (embedded in test goroutines)

Each simulated agent:
1. Dials the in-process server (`grpc.NewClient` with insecure credentials)
2. Calls `AgentRegistry.Register`
3. Opens `TwinControl.SyncTwinState` bidirectional stream, sends actual state, waits for config push
4. Opens `TelemetryStream.StreamMetrics` bidirectional stream, sends a metric batch, waits for ack
5. Calls `AgentRegistry.Heartbeat`

1,000 goroutines execute steps 1–5 fully concurrently.

### 3. Validation Tests

| Test | What it validates |
|------|-------------------|
| `TestGRPCRegistrationLatency` | P99 round-trip for `Register` + `Heartbeat` across 400 sequential samples |
| `TestGRPCTwinControlPushLatency` | Time from stream open to first `ConfigAction` received (50 samples) |
| `TestGRPCTelemetryThroughput` | Aggregate msgs/sec with 100 concurrent agents × 100 batches × 5 metrics |
| `TestGRPC1000AgentsConcurrent` | 1,000 goroutines all completing the full agent lifecycle without error |

### 4. Benchmark Results

Run via `make bench-grpc` (`-benchtime=200x` on a 2-core AMD EPYC 7763):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| `BenchmarkGRPCRegister` | 165,754 | 9,812 | 156 |
| `BenchmarkGRPCTelemetryStream` | 68,558 | 1,558 | 44 |

**Implied throughputs**:
- Register: ≈ 6,000 RPCs/sec/core
- TelemetryStream round-trip: ≈ 14,600 batches/sec/core

### 5. Memory Usage Under Load

During the 1,000-agent concurrent test (all agents simultaneously connected):

- **Heap before test**: ≈ 1.6 MB
- **Heap at peak (1,000 connections open)**: ≈ 200 MB
- **Per-connection estimate**: ≈ 200 KB

This is consistent with gRPC's documented per-stream overhead (~100–200 KB for HTTP/2 headers,
buffers, and goroutine stacks) and is acceptable for the target fleet size (< 10 K agents per
server per ADR-003 consequences).

## Running the Spike

```bash
# Run all four validation tests
make test-grpc-spike

# Run gRPC benchmarks
make bench-grpc

# Individual tests
go test -v -timeout 180s ./benchmark -run '^TestGRPC'
```

## Key Findings

### Latency — Well Within Budget

The in-process prototype showed P99 latencies far below the success-criteria thresholds:

| Operation | P50 | P99 | Threshold | Margin |
|-----------|-----|-----|-----------|--------|
| Register | ≈ 87 µs | ≈ 0.6 ms | 100 ms | 167× headroom |
| Heartbeat | ≈ 87 µs | ≈ 0.6 ms | 100 ms | 167× headroom |
| TwinControl config push | ≈ 130 µs | ≈ 0.7 ms | 5 s | 7,000× headroom |

Even adding realistic network RTT (≤ 1 ms LAN, ≤ 10 ms WAN) and serialisation overhead,
all latency targets are easily achievable.

### Throughput — SC-008 Met

100 concurrent agents, each sending 100 batches of 5 metrics, achieved ≈ 120 K msgs/sec
aggregate throughput with **zero drops**.  The serial send-ack per batch is the dominant
limiting factor; pipelining (sending without waiting for each ack) would easily push this
above 1 M msgs/sec if required.

### Concurrency — 1,000 Agents Confirmed

All 1,000 agents successfully completed registration, bidirectional twin-state sync, and
telemetry streaming without a single connection drop or error.  gRPC's HTTP/2 multiplexing
over a single TCP connection per agent handled the concurrent streams without issue.

### Memory Footprint

~200 KB per connected agent is the baseline at the prototype stage (unoptimised in-process
server, no connection pooling).  A production server with proper tuning (buffer pool, HTTP/2
window management) should reduce this to < 50 KB per agent, supporting 10 K agents in < 500 MB.

## Architecture Validation

The prototype confirms the ADR-003 service design is sound:

1. **AgentRegistry** (request/response) is adequate for registration and heartbeats — no
   streaming needed; P99 < 1 ms.
2. **TwinControl** (bidirectional stream) correctly delivers config push to the agent as the
   first message after stream open; push latency is effectively RTT, well within the 5-second
   SC-002 budget.
3. **TelemetryStream** (bidirectional stream) provides natural backpressure via the
   `ready_for_next` field in `MetricsResponse`; the server can throttle fast agents without
   dropping messages.

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| In-process latency < production | Test underestimates real-world numbers | Add ≤ 10 ms network RTT budget; all targets still met with > 100× margin |
| Memory growth per agent | Could limit fleet size | Connection pooling + HTTP/2 window tuning in Phase 1; target < 50 KB/agent |
| mTLS overhead (ADR-010) | Adds TLS handshake latency on reconnect | TLS 1.3 0-RTT session resumption; one-time cost amortised over long-lived streams |
| Stream reconnect storms | Server overload after network partition | Exponential backoff with jitter in agent reconnect logic (Phase 1 requirement) |

## Recommendations

### ✅ Proceed with ADR-003 as Accepted

All success criteria passed.  The three-service gRPC design (`AgentRegistry`, `TwinControl`,
`TelemetryStream`) is **recommended** for Phase 1 implementation.

### Next Steps

1. **Enable mTLS** (ADR-010 requirement): add `crypto/tls` credentials to server and agent
   before Phase 1 merge; validate with `TestGRPC1000AgentsConcurrent` under mTLS.
2. **Add reconnect logic**: implement exponential backoff in agent stream reconnect paths.
3. **Memory profiling**: run `go test -memprofile` at 10 K agent scale to establish production
   sizing targets.
4. **Integration test suite**: extend `TestGRPC1000AgentsConcurrent` to run on the Vagrant
   multi-machine environment (ADR-012) for realistic network conditions.
5. **Update CI**: add `make test-grpc-spike` to the GitHub Actions workflow.

## Deliverables

- [x] Proto files defined (`.proto/pheromone/v1/`) — already existed
- [x] Go code generated (`internal/proto/pheromone/v1/`) — already existed
- [x] Minimal mock server (all three services) — `benchmark/grpc_spike_test.go`
- [x] Load generator (1,000-goroutine concurrent agent simulator) — `benchmark/grpc_spike_test.go`
- [x] Latency measurements (P50/P99 for Register, Heartbeat, TwinControl push)
- [x] Throughput measurement (msgs/sec for TelemetryStream)
- [x] Memory measurement (heap growth under 1,000 concurrent agents)
- [x] Makefile targets (`test-grpc-spike`, `bench-grpc`)
- [x] Findings documented in this summary and in `docs/adr/adr-003-grpc-contracts.md`

## References

- [ADR-003: gRPC Service Contracts](adr/adr-003-grpc-contracts.md)
- [ADR-007: Agentic AI Agent Model](adr/adr-007-agentic-ai-agent-model.md)
- [ADR-010: Signal Protocol Evaluation](adr/adr-010-signal-protocol-evaluation.md)
- [ADR-011: Port Assignment](adr/adr-011-port-assignment.md)
- [Tech Spike ADR-002 Summary](tech-spike-adr-002-summary.md)
- [Benchmark Documentation](../benchmark/README.md)

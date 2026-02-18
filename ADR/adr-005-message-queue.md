# ADR 005: Message Queue Selection - Telemetry Aggregation & Export

## Status
Proposed

## Context

Pheromone agents emit high-volume structured metrics and logs (FR-010, FR-011, SC-008: 100K metrics/sec target). The system requires a message queue to:

1. **Handle Backpressure**: Server cannot always consume metrics immediately; queue buffers bursts
2. **Decouple Collection from Export**: Agent → Queue → Prometheus/Datadog (independent scaling)
3. **Enable Multi-Consumer**: Multiple telemetry backends (Prometheus, Datadog, ELK) consume same stream
4. **Provide Ordering**: Metrics for single agent must maintain causal order (trace-id correlation)

**Not Required** (per ADR-001/Constitution):
- Configuration distribution (gRPC TwinControl Service handles this)
- Agent discovery (gRPC AgentRegistry Service handles this)
- State synchronization (ADR-002 etcd handles this)

**Options Evaluated**:
- **Apache Kafka**: High-throughput, persistent, consumer groups, operational complexity
- **RabbitMQ**: Mature, reliable, routing features, less throughput than Kafka
- **NATS**: Lightweight, cloud-native, simple deployment, lower persistence guarantees
- **Redis Streams**: In-memory with AOF, simpler than Kafka, limited retention
- **AWS SQS/Azure Queue**: Managed, but vendor lock-in; skip for open-source MVP

**Constraints**:
- MVP single-server (simple deployment critical)
- Phase 2 multi-consumer support (horizontal scaling for telemetry export)
- Must preserve agent metric ordering (per-agent-partition or similar)
- 100K metrics/sec target (SC-008)

## Decision

**Phase 1 MVP: NATS (lightweight, single binary)**  
**Phase 2 Scale-Out: Kafka (high-throughput, distributed)**

### Rationale for Phase 1: NATS

1. **Minimal Operational Overhead**: Single binary, zero external dependencies (vs. Kafka's 3+ processes)
2. **Cloud-Native**: Kubernetes-friendly, small footprint (~20MB)
3. **Adequate Throughput**: 100K msgs/sec achievable on mid-range hardware
4. **Reliable Delivery**: At-least-once semantics (with JetStream persistence)
5. **Simple to Test**: Can run single NATS server in Docker for development/testing

### Telemetry Flow (Phase 1)

```
Agent 1 ──┐
Agent 2 ──┼──→ gRPC TelemetryStream (ADR-003)
Agent 3 ──┘                                    ↓
                                    NATS Subject: "metrics.{agent_id}"
                                               ↓
                                    [NATS Server + JetStream]
                                               ↓
     ┌─────────────────────────────────────────┼─────────────────┐
     ↓                                          ↓                 ↓
 Prometheus                           Datadog Exporter      Custom Consumer
 (direct push)                        (HTTP API client)     (analytics, archive)
```

### Phase 1 Implementation

- **NATS Server**: Single instance with JetStream enabled
  - Subject: `metrics.{agent_id}.{metric_type}` (e.g., `metrics.agent-123.cpu`)
  - Stream: `metricsStream` with 24-hour retention (tunable)
  - Ordering: Subjects preserve per-agent-id causality

- **Agent Publishing** (via gRPC server):
  ```go
  // Server receives metrics from agent
  func (s *Server) StreamMetrics(stream TelemetryStream_StreamMetricsServer) {
      for msg := range stream {
          for metric := range msg.Metrics {
              // Publish to NATS
              natsConn.Publish(
                  fmt.Sprintf("metrics.%s.%s", agentID, metric.Name),
                  metric.Bytes(),  // OpenMetrics format
              )
          }
      }
  }
  ```

- **Prometheus Export** (via exporter process):
  ```
  Prometheus HTTP Scrape
         ↓
  Go Exporter Process
         ↓
  NATS Consumer (built-in to exporter)
         ↓
  Format metrics as Prometheus exposition format
         ↓
  Return via /metrics endpoint
  ```

### Migration to Phase 2 (Kafka)

When scaling beyond 1 server (Phase 2 ADR-TBD):
1. Replace NATS with Kafka cluster (3+ brokers)
2. Topics: `metrics-{env}` (e.g., `metrics-prod`, `metrics-staging`)
3. Partitions: One per agent-id (preserves ordering)
4. Consumer groups: `prometheus`, `datadog`, `archive`
5. Go client: Switch from `nats.io/nats.go` to `github.com/segmentio/kafka-go`

**Migration Effort**: ~40 hours (API differences are minimal; same publish/subscribe patterns)

## Consequences

### Phase 1 (NATS) Advantages
- Single binary deployment (MVP speed)
- <1 second setup (no cluster coordination needed)
- JetStream provides durability (metrics not lost on crash)
- Horizontally scalable clients (100+ exporters possible with single NATS)
- Excellent for <50K msgs/sec (ample headroom for Phase 1)

### Phase 1 (NATS) Limitations
- Single point of failure (no built-in clustering; Phase 2 uses NATS cluster mode)
- Less mature for large-scale telemetry (Kafka better proven at >1M msgs/sec)
- No multi-datacenter routing (Phase 2 requirement)

### Phase 2 Upgrade Path
- Clear upgrade story (API compatible publish/subscribe)
- No application code changes needed (just new NATS-to-Kafka adapter)
- Testing strategy: Run both NATS and Kafka in staging (compare outputs)

## Testing & Validation

- **MVP Tests** (FRs 023, SC-008):
  - Load test: 100K msgs/sec sustained without backlog growth
  - Persistence: Server restart without message loss
  - Multi-consumer: Multiple exporters consume same stream
  - Ordering: Per-agent-id ordering verified

- **Integration Tests**: Agent → NATS → Prometheus → Grafana dashboard shows correct metrics

## Deployment

- **Docker Compose** (dev/testing):
  ```yaml
  nats:
    image: nats:latest
    ports:
      - "4222:4222"
    command: "-js"  # Enable JetStream
  ```

- **Kubernetes** (Phase 2):
  ```yaml
  apiVersion: apps/v1
  kind: StatefulSet
  metadata:
    name: nats
  spec:
    replicas: 3
    template:
      spec:
        containers:
          - name: nats
            image: nats:latest
            args: ["-c", "/config/nats.conf"]
  ```

## Follow-Up ADRs

- **ADR-TBD (Phase 2)**: Kafka migration strategy for multi-datacenter telemetry

## References

- NATS Documentation: https://docs.nats.io/
- JetStream Guide: https://docs.nats.io/nats-concepts/jetstream
- Apache Kafka vs NATS comparison: https://kafka.apache.org/ (architecture pages)
- Spec-001, FR-011, SC-008
- Constitution Principle III (Protocol foundation), Principle IV (Smoke tests)

---

**Decision Date**: 2026-02-18  
**Status Update**: Proposed (pending load test spike with Go NATS client)

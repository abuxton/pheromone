# Implementation Plan: Agent Health Monitoring

**Feature**: 001-health-monitoring  
**Plan Version**: 1.0  
**Date**: 2026-02-18

## Executive Summary

Implement agent health monitoring using gRPC streaming for heartbeats, time-series database for metrics storage, and a simple REST API for status queries. The solution leverages existing gRPC infrastructure (per ADR-001) and adds minimal overhead to agents.

## Architecture

### Components

```
┌─────────────────┐         ┌──────────────────┐         ┌─────────────────┐
│                 │  gRPC   │                  │  Store  │                 │
│  Agents         ├────────>│  Health Monitor  ├────────>│  Time-Series DB │
│                 │ Stream  │  Service         │         │  (Prometheus)   │
└─────────────────┘         └──────────────────┘         └─────────────────┘
                                     │
                                     │ REST
                                     ▼
                            ┌─────────────────┐
                            │                 │
                            │  Dashboard API  │
                            │                 │
                            └─────────────────┘
```

### Technology Stack

Per ADR-001 and project constitution:

- **Language**: Go (aligns with existing architecture)
- **Protocol**: gRPC streaming for heartbeats (existing infrastructure)
- **Metrics Storage**: Prometheus (de facto standard for Go)
- **API**: REST API using standard library `net/http`
- **Alerting**: Prometheus Alertmanager (integrates with Prometheus)

**Rationale**: This stack minimizes new dependencies and leverages existing infrastructure decisions documented in ADR-001.

## Detailed Design

### 1. Agent Heartbeat

**Implementation**:
```go
// Agent sends heartbeat every 30 seconds via gRPC stream
type Heartbeat struct {
    AgentID   string
    Timestamp time.Time
    Metrics   *AgentMetrics
}

type AgentMetrics struct {
    CPUUsage    float64
    MemoryUsage float64
    DiskUsage   float64
}
```

**Considerations**:
- Use gRPC bidirectional streaming (already supported per ADR-001)
- Include basic metrics in heartbeat to minimize round trips
- Implement exponential backoff on connection failures

### 2. Health Monitor Service

**Core Logic**:
```
1. Accept heartbeat streams from agents
2. Update last-seen timestamp in memory
3. Export metrics to Prometheus
4. Background goroutine checks for stale agents:
   - Every 30 seconds
   - Mark unhealthy if no heartbeat in 90 seconds (3 intervals)
   - Mark critical if no heartbeat in 150 seconds (5 intervals)
```

**State Management**:
- In-memory map: AgentID -> LastHeartbeat
- Metrics exported to Prometheus for persistence
- No database writes on heartbeat path (performance)

### 3. Storage Layer

**Prometheus Metrics**:
```
agent_heartbeat_timestamp{agent_id="..."}
agent_cpu_usage{agent_id="..."}
agent_memory_usage{agent_id="..."}
agent_disk_usage{agent_id="..."}
agent_health_status{agent_id="...", status="healthy|unhealthy|critical"}
```

**Retention**: Configure Prometheus for 90-day retention (NFR-005)

### 4. Status API

**Endpoints**:
```
GET  /api/v1/agents              - List all agents with status
GET  /api/v1/agents/{id}         - Get single agent status
GET  /api/v1/agents/{id}/history - Get health history (proxy to Prometheus)
POST /api/v1/alerts/ack          - Acknowledge alert
```

**Response Format** (JSON):
```json
{
  "agent_id": "agent-001",
  "status": "healthy",
  "last_heartbeat": "2026-02-18T19:34:00Z",
  "metrics": {
    "cpu_usage": 25.5,
    "memory_usage": 45.2,
    "disk_usage": 60.1
  }
}
```

### 5. Alerting

**Prometheus Alertmanager Rules**:
```yaml
groups:
  - name: agent_health
    rules:
      - alert: AgentUnhealthy
        expr: agent_health_status{status="unhealthy"} == 1
        for: 2m
        annotations:
          summary: "Agent {{ $labels.agent_id }} is unhealthy"
      
      - alert: AgentCritical
        expr: agent_health_status{status="critical"} == 1
        for: 1m
        annotations:
          summary: "Agent {{ $labels.agent_id }} is critical"
```

**Notification Channels**:
- Email (primary)
- Webhook (for future integration)

## Performance Considerations

### Scalability
- **Target**: 10,000 agents (NFR-003)
- **Heartbeat Rate**: 10,000 agents × 1/30s = ~333 heartbeats/second
- **Memory**: ~200 bytes per agent × 10,000 = ~2 MB
- **CPU**: Minimal (map updates + metrics export)

### Optimization Strategies
1. Use connection pooling for gRPC
2. Batch Prometheus metric updates
3. Implement agent sharding if >10K agents needed

## Security

- TLS for all gRPC connections (per ADR-001)
- API authentication using JWT tokens
- Rate limiting on API endpoints
- No sensitive data in heartbeats

## Testing Strategy

### Unit Tests
- Heartbeat processing logic
- Health status calculation
- Alert threshold detection

### Integration Tests
- Agent → Health Monitor → Prometheus flow
- API endpoint functionality
- Alerting triggers

### Load Tests
- 10,000 concurrent agent connections
- Sustained heartbeat rate
- API query performance under load

## Migration Plan

**Phase 1**: Core Implementation (Week 1-2)
- Health monitor service
- Agent heartbeat integration
- Basic metrics

**Phase 2**: Storage & API (Week 3)
- Prometheus integration
- REST API endpoints

**Phase 3**: Alerting (Week 4)
- Alertmanager setup
- Alert rules
- Notification channels

**Phase 4**: Dashboard (Future)
- Web UI (out of current scope)
- Visualization

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Prometheus single point of failure | High | Run Prometheus in HA mode with redundancy |
| Network partitions cause false alerts | Medium | Implement jitter and grace periods |
| gRPC stream overhead at scale | Medium | Load test early, implement sharding if needed |
| Clock skew between agents | Low | Require NTP, document in deployment guide |

## Open Questions

1. ~~Should we use gRPC or HTTP for heartbeats?~~
   - **Resolved**: gRPC per ADR-001
   
2. What alert severity levels?
   - **Decision**: Two levels: Unhealthy (warning), Critical (urgent)

3. Self-healing or manual remediation?
   - **Decision**: Manual remediation (auto-healing is future scope)

## Related Documents

- [ADR-001: Digital Twin Architecture](../../adrs/adr-001-digital-twin-architecture.md) - Protocol choice
- [Feature Specification](spec.md) - Requirements
- [Project Constitution](../constitution.md) - Technology principles

---

**Next Steps**:
1. Review this plan with team
2. Use `/speckit.tasks` to break down into implementation tasks
3. Consider creating ADR-002 for metrics storage choice if deemed significant

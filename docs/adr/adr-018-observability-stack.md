# ADR-018: Observability Stack — Prometheus, OpenTelemetry, Grafana

**Status**: Accepted
**Date**: 2026-03-12
**Author**: @copilot
**Refs**: ADR-005 (Message Queue / NATS), ADR-003 (gRPC Contracts), ADR-014 (Security Architecture), ADR-017 (UI/API Gateway)

---

## Context

The NATS JetStream telemetry pipeline has been validated (ADR-005 Accepted), but there is no runtime metrics export, no Prometheus scrape endpoint, and no Grafana dashboard. Operators cannot observe server or agent health in production.

Identified gaps (issue #66):

1. **No `/metrics` Prometheus scrape endpoint** on the management server.
2. **No per-agent metric aggregation** visible in any UI or backend.
3. **No OpenTelemetry trace export** through the gRPC interceptor chain.
4. **NATS telemetry consumers not wired** to any observability backend.
5. **No alerting rules or SLO definitions**.

### Constraints

| Constraint | Value |
|---|---|
| New dependencies | Minimise; prefer `prometheus/client_golang` already transitively present via etcd client |
| Existing auth middleware | `/metrics` is exempt from JWT auth; access control via network policy |
| Backwards compatibility | All existing `/api/v1/*` paths and `/healthz`, `/readyz` semantics preserved |
| Container stack | Extend existing `docker-compose.yml`; add Prometheus + Grafana as opt-in services |

---

## Decision

### 1. Prometheus `/metrics` endpoint

Add a `/metrics` HTTP endpoint to the existing management server (`internal/api/server.go`) using `github.com/prometheus/client_golang/prometheus/promhttp`. The endpoint is served on the same HTTP listener (port configured via `UIConfig`) without authentication, consistent with the k8s health probes (`/healthz`, `/readyz`).

A **custom non-global registry** (`metrics.Registry`) is used instead of the default global `prometheus.DefaultRegisterer`. This prevents conflicts with indirect dependencies (etcd client, gRPC) that may self-register against the default registry, and makes test isolation straightforward.

### 2. Metrics package (`internal/metrics`)

A new `internal/metrics` package owns the metric registry and all six required metric descriptors:

| Metric | Type | Labels |
|--------|------|--------|
| `pheromone_agents_total` | Gauge | `status` |
| `pheromone_twins_total` | Gauge | — |
| `pheromone_grpc_requests_total` | Counter | `service`, `method`, `code` |
| `pheromone_grpc_duration_seconds` | Histogram | `service`, `method` |
| `pheromone_nats_messages_total` | Counter | `subject` |
| `pheromone_reasoning_decisions_total` | Counter | `reasoner`, `outcome` |

The package exposes:
- `metrics.Registry` — package-level registry (used by the server handler and production code).
- `metrics.NewRegistry()` — factory that returns a fresh `(*prometheus.Registry, *Metrics)` pair for test isolation, avoiding duplicate-registration panics.

### 3. Seed-time gauge initialisation

`Server.Seed()` calls `syncMetrics()` after loading demo data, so `pheromone_agents_total` and `pheromone_twins_total` are immediately non-zero when Prometheus first scrapes.

### 4. Docker Compose observability stack

Two new services added to `docker-compose.yml`:

- **`prometheus`** (`prom/prometheus:v3.4.0`) scrapes the Pheromone server at `http://pheromone-server:8080/metrics` every 15 s, with 15-day TSDB retention.
- **`grafana`** (`grafana/grafana:12.0.1`) is provisioned via `deploy/grafana/provisioning/` with the Prometheus datasource and a pre-built **Pheromone Overview** dashboard (`deploy/grafana/dashboards/pheromone-overview.json`).

Both services join the existing `pheromone-network` bridge network.

Prometheus config: `deploy/prometheus/prometheus.yml`

### 5. OpenTelemetry trace propagation (deferred)

Full OTel trace propagation through gRPC interceptors is deferred to Phase 2, blocked on ADR-015 (gRPC auth interceptor chain). The `pheromone_grpc_requests_total` counter and `pheromone_grpc_duration_seconds` histogram provide equivalent RED signal without requiring a full OTel SDK dependency in Phase 1.

---

## Consequences

### Positive

- Operators can point Prometheus at the management server and immediately receive 6 key metrics.
- `docker compose up` now starts Prometheus and Grafana with zero additional configuration.
- Pre-built dashboard covers agents, twins, gRPC RED, NATS throughput, and reasoning decisions.
- Isolated registry design prevents test pollution and duplicate-registration panics.
- No breaking changes to existing API surface or middleware chain.

### Negative / Trade-offs

- `/metrics` is unauthenticated. Operators must enforce access via network policy, firewall rules, or a reverse proxy (Envoy, as evaluated in ADR-014) in production.
- The package-level `metrics.Registry` is a process-wide singleton; tests that exercise both the metrics package and the server package in the same process share gauge state. The `NewRegistry()` factory mitigates this for unit tests.
- OTel trace context propagation is deferred; distributed tracing across server↔agent boundaries requires a future ADR.

---

## Alternatives Considered

| Option | Reason Rejected |
|--------|----------------|
| Use `prometheus.DefaultRegisterer` (global) | etcd and gRPC transitively self-register; leads to duplicate-registration panics in tests |
| OpenTelemetry Prometheus exporter | Adds OTEL SDK dependency (~4 MB); deferred to Phase 2 with full interceptor chain |
| Envoy as Prometheus scrape target | Envoy sidecar evaluation ongoing (ADR-014 Proposed); would duplicate metric set without custom Pheromone labels |
| Grafana Agent / Alloy | Additional ops complexity; standard Prometheus scrape is sufficient for Phase 1 |

---

## Implementation Notes

- `internal/metrics/metrics.go` — registry, metric descriptors, `NewRegistry()` factory.
- `internal/metrics/metrics_test.go` — unit tests: no-panic on multiple `NewRegistry()` calls, all 6 metrics present, correct Prometheus types.
- `internal/api/server.go` — imports `internal/metrics` and `promhttp`; `/metrics` route added in `registerRoutes`; `syncMetrics()` called from `Seed()`.
- `deploy/prometheus/prometheus.yml` — Prometheus scrape config.
- `deploy/grafana/provisioning/` — Grafana datasource and dashboard provisioning.
- `deploy/grafana/dashboards/pheromone-overview.json` — pre-built overview dashboard.
- `docker-compose.yml` — Prometheus (port 9090) and Grafana (port 3000) services added.
- `docs/adr/INDEX.md` — ADR-018 entry added.

---

## References

- `github.com/prometheus/client_golang` v1.23.2
- `github.com/prometheus/client_model` v0.6.2
- ADR-005 (NATS Message Queue — telemetry backbone)
- ADR-003 (gRPC Contracts — RED metrics labels align with service/method)
- ADR-014 (Security Architecture — `/metrics` access control via network policy)
- ADR-017 (UI/API Gateway — `/metrics` follows same unauthenticated probe pattern as `/healthz`)
- Issue #66 (Architectural gap review — observability stack gaps)

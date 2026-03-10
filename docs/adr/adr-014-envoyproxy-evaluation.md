# ADR 014: Envoy Proxy Evaluation — Monitoring, Observability, and Service Mesh Integration

## Status

Proposed

## Context

This ADR documents a formal spike review of **Envoy Proxy** (<https://github.com/envoyproxy/envoy>)
for potential integration with the Pheromone platform, with a particular focus on monitoring,
troubleshooting, and service mesh capabilities.

The review was requested as a spike (see linked issue) with the following acceptance criteria:

1. Summary of Envoy Proxy — architecture, capabilities, and approach
2. ADR entry created and raised for review
3. Suggestions for integration development within Pheromone
4. Additional resources documented
5. Follow-on development issues raised if the integration is accepted

Pheromone currently uses gRPC with mutual TLS (ADR-003, ADR-010) as its primary
server↔agent communication protocol, and requires rich, autonomous metric reporting from
every agent (Constitution Principle II). Envoy's deep gRPC support and native observability
capabilities make this evaluation timely.

---

## What Is Envoy Proxy?

Envoy is an open-source, cloud-native **L3/L4/L7 proxy and communication bus** designed for
large-scale service-oriented architectures. Originally built at Lyft, it graduated as a CNCF
(Cloud Native Computing Foundation) top-level project in 2018 and is used as the data-plane
proxy in major service meshes (Istio, AWS App Mesh, Consul Connect).

### At a Glance

| Attribute | Detail |
|---|---|
| **Name** | Envoy Proxy |
| **Origin** | Lyft → CNCF (<https://github.com/envoyproxy/envoy>) |
| **Language** | C++17 (core); Go/Python/Rust extension SDKs via ext_proc and Wasm |
| **Deployment** | Sidecar proxy, edge/ingress proxy, or dedicated network proxy |
| **State management** | xDS API (dynamic, pull/push via gRPC); static YAML/JSON config |
| **Licence** | Apache 2.0 |
| **Maturity** | Production-grade; used at scale by Lyft, Google, AWS, Shopify, Stripe, and others |
| **Protocol support** | HTTP/1.1, HTTP/2, HTTP/3 (QUIC), gRPC, WebSocket, TCP, UDP, Thrift, Dubbo |
| **Observability** | StatsD, Prometheus (`/metrics`), distributed tracing (Jaeger, Zipkin, Datadog, OTLP), structured access logs |
| **Governance** | CNCF Graduated; vendor-neutral; active community with clear versioning policy |

### Core Architecture

Envoy's architecture is composed of a small set of first-class abstractions:

| Concept | Description |
|---|---|
| **Listener** | Accepts incoming connections on a port; applies filter chains |
| **Filter Chain** | Ordered stack of network and HTTP filters (e.g., TLS inspector, HTTP connection manager, gRPC-JSON transcoder) |
| **Cluster** | Named group of upstream endpoints; supports multiple load-balancing algorithms and health checks |
| **Endpoint** | Individual upstream host, with load-balancing metadata (weight, zone, health status) |
| **Route** | HTTP/gRPC routing rules — match requests and forward to clusters |
| **xDS API** | gRPC-based control-plane API (LDS/RDS/CDS/EDS/SDS) for dynamic, zero-downtime configuration updates |

```
Incoming traffic
    │
    ▼
┌────────────────────────────────┐
│           LISTENER              │  ← accepts connections (port 15001, 443, etc.)
│  ┌──────────────────────────┐  │
│  │     Filter Chain          │  │  ← TLS termination, HTTP connection manager
│  │  ┌────────────────────┐  │  │
│  │  │  HTTP Filters       │  │  │  ← Router, gRPC-JSON, rate limiting, auth
│  │  └────────────────────┘  │  │
│  └──────────────────────────┘  │
└────────────┬───────────────────┘
             │ routes
             ▼
┌────────────────────────────────┐
│           CLUSTER               │  ← named upstream group (e.g., pheromone-server)
│  ┌──────────────────────────┐  │
│  │  Load Balancer            │  │  ← round-robin, least-request, ring-hash, etc.
│  └──────────────────────────┘  │
│  ┌──────────────────────────┐  │
│  │  Health Checker           │  │  ← active (HTTP/gRPC) + passive (outlier detection)
│  └──────────────────────────┘  │
└────────────┬───────────────────┘
             │
             ▼
      Upstream endpoint(s)
```

### xDS Control Plane API

Envoy's configuration is fully dynamic via the **xDS** (discovery service) API family:

| xDS Service | Manages |
|---|---|
| **LDS** (Listener Discovery Service) | Listener configuration |
| **RDS** (Route Discovery Service) | HTTP/gRPC routing rules |
| **CDS** (Cluster Discovery Service) | Upstream cluster definitions |
| **EDS** (Endpoint Discovery Service) | Individual upstream endpoints |
| **SDS** (Secret Discovery Service) | TLS certificates and private keys |
| **ADS** (Aggregated Discovery Service) | Single gRPC stream for all of the above |

All xDS APIs use **gRPC bidirectional streaming** — the same transport Pheromone uses for
`TwinControl` and `TelemetryStream` (ADR-003). This architectural alignment is significant.

### Built-in Observability

Envoy's observability output is comprehensive and requires no code changes to the proxied services:

| Signal Type | Mechanism | Detail |
|---|---|---|
| **Metrics** | Prometheus `/stats/prometheus`; StatsD push | Thousands of built-in counters/gauges/histograms per listener, cluster, and filter |
| **Access Logs** | Structured JSON or text to stdout, file, or gRPC access log service | Per-request with full header, timing, and upstream metadata |
| **Distributed Tracing** | Jaeger, Zipkin, AWS X-Ray, Datadog, OpenTelemetry (OTLP) | Automatic span propagation via `x-b3-traceid` / `traceparent` headers |
| **Admin API** | `GET /stats`, `GET /clusters`, `GET /listeners`, `POST /logging` on port 9901 | Real-time operational introspection |
| **Health Check Logging** | Structured health event log | Upstream health state transitions logged on every change |

---

## Evaluation

### Question 1 — Suitability as a Monitoring and Troubleshooting Component

The issue specifically requests a review of Envoy for **monitoring and troubleshooting** the
Pheromone platform. This is the strongest use case for Envoy in the Pheromone context.

#### gRPC Traffic Observability (Strongest Match)

Pheromone's three gRPC services (`AgentRegistry`, `TwinControl`, `TelemetryStream`) are fully
visible to Envoy without instrumentation:

| Observable Signal | How Envoy Provides It | Pheromone Benefit |
|---|---|---|
| Per-RPC latency histograms | `grpc.method` metrics, response timing in access log | Detect slow `SyncTwinState` calls between server and agent |
| gRPC status code counters | `grpc_response_status` in Envoy stats | Identify `UNAVAILABLE`, `DEADLINE_EXCEEDED` errors across the fleet |
| Per-agent traffic volume | `upstream_rq_*` metrics per cluster endpoint | Identify noisy or failing agents by traffic pattern |
| Request rate (QPS) | `downstream_rq_total` per listener | Fleet-wide RPC rate visibility |
| TLS handshake failures | `ssl.handshake_error`, `ssl.connection_error` | Diagnose mTLS misconfiguration between agents and server |
| Distributed traces | Span for each RPC with agent/server identifiers | End-to-end latency breakdown across the reasoning loop |

A **single Envoy sidecar on the Pheromone server** would give operators full visibility into all
fleet gRPC traffic without modifying the Go server or any agent binary.

#### Troubleshooting Capabilities

Envoy's admin API provides real-time operational introspection:

```bash
# Live cluster health — see which agents are reachable
curl http://localhost:9901/clusters

# Current connection counts per upstream agent endpoint
curl http://localhost:9901/stats?filter=cluster.pheromone_agents

# Dynamic log level adjustment (no restart required)
curl -XPOST http://localhost:9901/logging?level=debug

# Drain listeners for graceful restart
curl -XPOST http://localhost:9901/drain_listeners
```

This is directly useful for operators troubleshooting fleet connectivity, agent registration
failures, or telemetry backpressure issues — all common in a 1:Many management system.

---

### Question 2 — Suitability as a Service Mesh / Sidecar Proxy

In a service mesh topology, each Pheromone agent would run an Envoy sidecar that intercepts
all gRPC traffic to/from the agent process.

| Requirement | Envoy capability | Assessment |
|---|---|---|
| Intercept outbound gRPC (agent → server) | ✅ Transparent proxy via iptables redirect | No agent code changes required |
| Intercept inbound gRPC (server → agent) | ✅ Inbound listener with filter chain | Full request/response visibility |
| mTLS between agent and server (ADR-010) | ✅ SDS-based certificate management; automatic rotation | Offloads mTLS from Go agent code |
| Dynamic routing and load balancing | ✅ CDS/EDS via xDS control plane | Enables server-side agent pooling |
| Circuit breaking | ✅ Outlier detection + connection pool limits | Protects server from misbehaving agents |
| Health checking (active + passive) | ✅ HTTP/gRPC health checks; outlier detection | Replaces manual agent heartbeat timeout logic |
| Rate limiting | ✅ Local + global rate limiting filters | Prevents a single agent from flooding the server |
| Go agent footprint overhead | ⚠️ Envoy binary: ~70–120 MB RAM | Significant overhead on small managed instances |

**Sidecar topology for Pheromone:**

```
Managed Instance
├── Pheromone Agent (Go binary, ~15–30 MB)
│    └── connects to localhost:15001 (Envoy inbound)
└── Envoy Sidecar (C++, ~70–120 MB RAM)
     ├── Inbound listener  :15001 → forwards to agent
     ├── Outbound listener :15006 → forwards to pheromone-server
     ├── Admin API :9901
     └── Prometheus metrics :9902

Pheromone Server
├── Pheromone Server (Go binary)
│    └── connects to localhost:15001
└── Envoy Sidecar
     ├── Inbound from agents → server
     ├── xDS control plane connection
     └── Prometheus scrape endpoint
```

**Resource overhead concern**: Envoy's C++ binary requires ~70–120 MB of RAM per instance,
which is significant on constrained managed instances (e.g., 512 MB–1 GB VMs or containers
representing the low end of Pheromone's target deployment range).
This is the primary drawback of a full sidecar mesh model for Pheromone.

---

### Question 3 — Suitability as an Edge/Ingress Proxy for the Server

Deploying Envoy at the Pheromone server boundary (not as a per-agent sidecar) avoids the
per-instance resource overhead and still provides centralised observability and security:

| Deployment pattern | Description | Resource cost |
|---|---|---|
| **Per-agent sidecar** | Envoy runs on every managed instance | High — ~70–120 MB × N agents |
| **Server-side ingress** | Single Envoy in front of Pheromone server | Low — one Envoy instance |
| **Server-side ingress + selective sidecars** | Envoy at server; sidecars only on critical agents | Medium — proportional to coverage |

A **server-side Envoy ingress** gives Pheromone:
- Full visibility into all incoming agent connections (registration, sync, telemetry)
- Centralised mTLS termination and certificate rotation (SDS)
- gRPC traffic metrics and access logs without touching agent code
- Fault injection for integration testing (inject delays, errors into gRPC calls)
- gRPC-JSON transcoding — exposes gRPC services as REST for operator tooling

This is the **recommended integration pattern** (see Decision section).

---

### Question 4 — Integration with Pheromone's Observability Architecture

Pheromone's Constitution (Principle II) mandates autonomous metric reporting. Envoy's
Prometheus endpoint provides a complementary infrastructure-layer metrics source:

| Metric Layer | Source | Detail |
|---|---|---|
| **Application metrics** | Pheromone agent / server (Go) | Twin state, AI reasoning latency, skill execution counts |
| **Transport metrics** | Envoy Proxy | gRPC latency, error rates, connection counts, TLS health |
| **System metrics** | Pheromone agent (OS metrics skill) | CPU, memory, disk, network (per ADR-006/007) |

Envoy metrics scrape easily into Prometheus and Grafana:

```yaml
# Prometheus scrape config for Envoy sidecar metrics
scrape_configs:
  - job_name: 'envoy'
    static_configs:
      - targets: ['pheromone-server:9902']
    metrics_path: /stats/prometheus
```

This provides a **three-layer observability stack** for Pheromone without duplication:
Envoy handles transport-layer signals; the Go application handles application-layer signals.

---

### Question 5 — Licence and Ecosystem Compatibility

| Consideration | Detail |
|---|---|
| **Licence** | Apache 2.0 — fully compatible with Pheromone's dependency policy |
| **No AGPL concerns** | Unlike ADR-010 (Signal) and ADR-013 (Swamp), no licence incompatibility |
| **CNCF Graduated** | Vendor-neutral governance; enterprise production-grade stability |
| **Go client (go-control-plane)** | Google maintains an official Go xDS control-plane SDK (`github.com/envoyproxy/go-control-plane`) |
| **Kubernetes integration** | Native Envoy support in k8s (Istio, Contour, Emissary); relevant for Phase 2 |
| **Active maintenance** | Regular releases; CNCF security disclosure process; large contributor community |

The **`go-control-plane`** library is particularly relevant: it allows the Pheromone server to
act as an **xDS control plane**, dynamically managing Envoy routing rules across the agent fleet
via the same gRPC streaming model Pheromone already uses for `TwinControl`.

---

### Evaluation Summary

| Role | Verdict | Key Reason |
|---|---|---|
| Monitoring and troubleshooting (server-side ingress) | ✅ **Strongly recommended** | Zero-instrumentation gRPC visibility; Prometheus metrics; admin API for ops |
| Server-side edge/ingress proxy | ✅ **Recommended for Phase 1** | Centralised mTLS, observability, gRPC-JSON transcoding; low resource cost |
| Per-agent sidecar (service mesh) | ⚠️ **Deferred to Phase 2** | High per-instance RAM overhead (~70–120 MB); value increases at scale |
| xDS control plane (Pheromone server → Envoy) | ⚠️ **Deferred to Phase 2** | Powerful but complex; `go-control-plane` enables this when needed |
| Replacement for gRPC (ADR-003) | ❌ **Not applicable** | Envoy proxies gRPC; it does not replace the gRPC service contracts |
| Replacement for mTLS logic (ADR-010) | ✅ **Simplification** | Envoy can own mTLS termination via SDS; reduces agent TLS implementation burden |

---

## Decision

**Envoy Proxy is accepted as a recommended infrastructure component for the Pheromone platform,
with a phased adoption approach:**

### Phase 1 (MVP — Server-Side Ingress)

Deploy a single Envoy instance as an **ingress proxy in front of the Pheromone server**.

This provides immediate value with minimal operational overhead:
- Full gRPC traffic observability (latency, errors, request rates) via Prometheus scrape
- Centralised mTLS termination for agent connections (offloads certificate management from Go code)
- gRPC-JSON transcoding for operator REST tooling (without separate REST endpoints)
- Fault injection for integration and chaos testing of the agent fleet
- Access logs for audit and troubleshooting

No agent changes are required. The Envoy sidecar operates transparently at the server boundary.

### Phase 2 (Scale — Per-Agent Sidecar and xDS Control Plane)

When the managed agent fleet grows beyond small deployments:
- Evaluate per-agent Envoy sidecars for full mesh observability and circuit breaking
- Implement the Pheromone server as an **xDS control plane** using `go-control-plane`
- Use EDS (Endpoint Discovery Service) to feed agent registration state into Envoy's
  load-balancing layer — aligning `AgentRegistry` (ADR-003) with Envoy's cluster topology

### What Envoy Does Not Replace

- **gRPC service contracts** (ADR-003): Envoy proxies the existing services; it does not
  replace `AgentRegistry`, `TwinControl`, or `TelemetryStream`.
- **NATS/Kafka message queue** (ADR-005): Envoy is not a message broker.
- **Agent AI reasoning loop** (ADR-007): Envoy is infrastructure, not an AI component.
- **Digital twin state** (ADR-002): Envoy has no state management role in twin sync.

---

## Suggestions for Integration Development

### Immediate (Phase 1 — 4–8 hours)

1. **Add Envoy to `docker-compose.yml`** as a sidecar to the Pheromone server container.
   Provide a static Envoy config (`envoy.yaml`) with:
   - Listener on `:4426` → cluster `pheromone_grpc` (forwards to server `:4426` — gRPC control port per ADR-011)
   - Listener on `:4427` → cluster `pheromone_telemetry` (forwards to server `:4427` — gRPC telemetry port per ADR-011)
   - Admin API on `:9901`
   - Prometheus stats on `:9902`

2. **Update Vagrant testing environment** (ADR-012) to provision Envoy alongside the server.
   Add an Envoy configuration role to the server provisioning scripts.

3. **Document baseline metrics**: Capture the Prometheus metric names that are most useful for
   monitoring Pheromone fleet health:
   - `envoy_cluster_upstream_rq_total{cluster="pheromone_agents"}` — total RPC count
   - `envoy_cluster_upstream_rq_time_bucket` — gRPC latency histogram
   - `envoy_cluster_upstream_cx_active` — active connections (fleet size proxy)
   - `envoy_listener_ssl_handshake` / `envoy_listener_ssl_handshake_error` — mTLS health

4. **Fault injection testing**: Use Envoy's fault filter to inject gRPC errors and test
   agent resilience during integration testing. This is a low-cost replacement for mock servers.

### Phase 2 (Service Mesh — 12–16 hours)

5. **Implement `go-control-plane` xDS server** within the Pheromone server process. Feed
   agent registration events (from `AgentRegistry`) into Envoy's EDS to dynamically update
   the mesh topology as agents join or leave the fleet.

6. **Per-agent sidecar evaluation spike**: Measure actual RAM overhead of `envoy:distroless`
   on a representative managed instance. If overhead is acceptable (<10% of available RAM),
   promote sidecar deployment to the standard agent installation.

7. **SDS-based certificate management**: Use Envoy's Secret Discovery Service to push
   per-agent mTLS certificates from the Pheromone server, replacing manual certificate
   distribution. Evaluate integration with a PKI (e.g., HashiCorp Vault, cert-manager).

---

## Consequences

### Positive

- **Zero-instrumentation observability**: Envoy provides transport-layer gRPC metrics without
  any changes to the Go agent or server code. This immediately satisfies Constitution Principle
  II (autonomous metric reporting) at the infrastructure layer.
- **mTLS simplification**: Offloading TLS termination to Envoy reduces the TLS implementation
  surface in the Go codebase, lowering the risk of misconfiguration.
- **Apache 2.0 licence**: No licence friction. Can be adopted as a dependency without legal review.
- **CNCF alignment**: Envoy is the de facto data-plane for cloud-native service meshes; adopting
  it prepares Pheromone for Kubernetes and cloud deployments in later phases.
- **gRPC-JSON transcoding**: Operators can interact with the Pheromone gRPC API using standard
  HTTP REST tools (curl, Postman) via Envoy's transcoding filter — no separate REST server needed.
- **Fault injection**: Envoy's fault filter supports chaos engineering for agent resilience
  testing without modifying test code.

### Negative

- **Per-agent sidecar resource overhead**: ~70–120 MB RAM per Envoy instance is significant
  on small managed instances. This limits full sidecar mesh adoption until managed instances
  have sufficient resources.
- **Operational complexity**: Adding Envoy introduces a new infrastructure component that must
  be configured, monitored, and upgraded. Operators unfamiliar with Envoy have a learning curve.
- **xDS control-plane implementation effort**: Using `go-control-plane` to implement Pheromone
  as an xDS control plane requires non-trivial engineering effort (estimated 12–16 hours for
  a working prototype; more for production hardening).
- **C++ build and debug complexity**: Envoy's C++ core means that any required extensions
  (custom filters) require C++ expertise. The Wasm and ext_proc extension SDKs mitigate this
  but add deployment complexity.

### Follow-on Issues

The following GitHub issues should be raised as a result of this evaluation:

| Issue | Type | Phase | Effort |
|---|---|---|---|
| Add Envoy proxy to `docker-compose.yml` for server-side monitoring | Implementation | Phase 1 | 2h |
| Create static Envoy config for Pheromone gRPC ingress | Implementation | Phase 1 | 3h |
| Document Pheromone Prometheus/Envoy metric catalogue | Documentation | Phase 1 | 2h |
| Add Envoy to Vagrant server provisioning scripts (ADR-012) | Implementation | Phase 1 | 2h |
| Spike: per-agent Envoy sidecar RAM overhead on target instances | Tech Spike | Phase 2 | 4h |
| Spike: `go-control-plane` xDS server in Pheromone server process | Tech Spike | Phase 2 | 12h |
| Spike: SDS certificate management via Envoy for agent mTLS | Tech Spike | Phase 2 | 8h |

---

## Additional Resources

### Official Documentation

- **Envoy documentation**: <https://www.envoyproxy.io/docs/envoy/latest/>
- **Envoy architecture overview**: <https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/arch_overview>
- **gRPC support in Envoy**: <https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/other_protocols/grpc>
- **xDS API reference**: <https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol>
- **Observability / stats**: <https://www.envoyproxy.io/docs/envoy/latest/operations/stats_overview>

### Go Integration

- **go-control-plane** (official Go xDS control-plane SDK): <https://github.com/envoyproxy/go-control-plane>
- **go-control-plane examples**: <https://github.com/envoyproxy/go-control-plane/tree/main/internal/example>

### Service Mesh Context

- **Envoy as Istio data plane**: <https://istio.io/latest/docs/ops/deployment/architecture/>
- **Envoy sidecar pattern**: <https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/front_proxy>

### Monitoring and Observability

- **Envoy stats reference** (full metric catalogue): <https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cluster_stats>
- **gRPC bridge filter** (method-level gRPC metrics): <https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_stats_filter>
- **OpenTelemetry tracing integration**: <https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/opentelemetry>

### Tutorials and Guides

- **Envoy getting started**: <https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/>
- **Envoy Docker sandbox examples**: <https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/>
- **gRPC service proxy walkthrough**: <https://www.envoyproxy.io/docs/envoy/latest/start/sandboxes/grpc_bridge>

---

## References

- Envoy Proxy repository: <https://github.com/envoyproxy/envoy>
- Envoy Proxy website: <https://www.envoyproxy.io/>
- go-control-plane repository: <https://github.com/envoyproxy/go-control-plane>
- ADR-001 (Digital Twin Architecture — establishes Go + gRPC as core technology)
- ADR-002 (Server Architecture — in-memory + etcd; Envoy adds transport-layer observability)
- ADR-003 (gRPC Service Contracts — defines `AgentRegistry`, `TwinControl`, `TelemetryStream` that Envoy proxies)
- ADR-005 (Message Queue — NATS/Kafka; Envoy does not replace messaging)
- ADR-006 (Agent Lifecycle — Go agent scaffold; Envoy is infrastructure, not agent code)
- ADR-007 (Agentic AI Agent Model — AI reasoning loop; Envoy is infrastructure, not AI)
- ADR-010 (Signal Protocol evaluation — established gRPC+mTLS as confirmed security layer; Envoy can own mTLS termination)
- ADR-012 (Vagrant Testing Environment — Envoy should be added to server provisioning scripts)
- Constitution v2.1.0 — Principle II (Observability instrumentation is NOT optional),
  Principle III (Language-Agnostic Protocol Foundation: gRPC + Protocol Buffers)

---

**Decision Date**: 2026-03-10
**Status**: Proposed — ready for team review

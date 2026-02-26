# ADR 011: Port Assignment for Pheromone Communication

## Status
Accepted

## Context

Pheromone requires well-defined, IANA-registered (or reserved) port numbers for all inter-component
communication. Without explicit port assignments, each deployment may choose arbitrary ports leading to:

- Conflicts with other services on the same host
- Firewall rule ambiguity across environments
- Inconsistent documentation for operators and integrators
- Difficulty distinguishing Pheromone traffic from other gRPC services

**Communication channels requiring port assignment**:

1. **gRPC Control Plane** — Agent ↔ Server: AgentRegistry + TwinControl services (ADR-003)
2. **gRPC Telemetry Plane** — Agent → Server: TelemetryStream service (ADR-003)
3. **HTTP/HTTPS REST API** — CLI / UI → Server: Operator query and configuration endpoint (FR-007)
4. **Internal Cluster Ports** — etcd (2379/2380): unchanged, already defined by the etcd project

**Evaluated port candidates**:

| Port  | IANA Status | Notes |
|-------|-------------|-------|
| 4426  | Registered (SMARTS/Sitara Networks — legacy, inactive product) | Available in practice; no known active deployments conflict |
| 4427  | Unregistered / available | No IANA assignment as of 2026-02-21 |
| 50051 | Conventional gRPC default (unregistered) | Widely used by gRPC examples; high collision risk in polyglot environments |
| 9090  | Registered (Openfire XMPP) | Used by Prometheus and OpenTelemetry; high collision risk |
| 8080  | Registered (HTTP Alternate) | Very high collision risk; often taken by local dev servers |
| 443   | HTTPS standard — reserved | Required for HTTPS REST API |
| 80    | HTTP standard — reserved | Required for HTTP REST API (redirect to 443 in production) |

## Decision

**Assign the following ports for all Pheromone communication**:

| Port | Service | Transport | Direction |
|------|---------|-----------|-----------|
| **4426** | gRPC Control Plane (AgentRegistry + TwinControl) | gRPC / HTTP2 | Agent ↔ Server |
| **4427** | gRPC Telemetry Plane (TelemetryStream) | gRPC / HTTP2 | Agent → Server |
| **443**  | HTTPS REST API (operator UI/CLI, Phase 2) | HTTPS | Client → Server |
| **80**   | HTTP REST API (redirect to 443 in production) | HTTP | Client → Server |

**Rationale for 4426 / 4427**:

1. **No active conflicts**: Port 4426's IANA registrant (Sitara Networks) is a defunct product. Port 4427 is
   unassigned. Neither port is occupied by any commonly deployed open-source software in the Pheromone
   target environment (Linux servers running etcd, Prometheus, Kafka, NATS).
2. **Separation of planes**: Using distinct ports for control (4426) and telemetry (4427) allows independent
   firewall rules, rate limiting, and traffic shaping without SNI or application-layer multiplexing.
3. **Memorable adjacency**: The two ports are sequential, making them easy to document, firewall, and
   remember as a pair.
4. **TLS upgrade path**: Both ports will use mTLS in production (Phase 2, per ADR-010). Separate ports
   simplify certificate scoping per service.
5. **Not 443/80**: HTTP and HTTPS standard ports are reserved for the REST API (FR-007) to avoid
   conflating gRPC binary protocol with HTTP/1.1 traffic.

## Consequences

### Positive
- Operators have a single, stable port reference for firewall configuration
- Traffic can be classified by port alone (no deep packet inspection needed for basic ACLs)
- etcd ports (2379/2380) remain unchanged — no impact on existing infrastructure
- Clear separation enables independent TLS certificate management per plane

### Negative
- Port 4426 technically has an IANA registration (Sitara Networks SMARTS); teams must verify no
  conflict exists in their environment before deployment (extremely unlikely in practice)
- Port 4427 is unregistered; a future IANA assignment could conflict (low probability)
- Two separate ports means two firewall rules and two connection pools per agent

### Mitigation
- Provide environment-variable overrides (`PHEROMONE_GRPC_CONTROL_PORT`, `PHEROMONE_GRPC_TELEMETRY_PORT`)
  so operators can remap ports without recompilation if a conflict arises
- Document the two ports prominently in README and operator guides

## Implementation Notes

- `docker-compose.yml`: expose `4426:4426` and `4427:4427` for the Pheromone server service
- Go server: default `grpcControlAddr = ":4426"` and `grpcTelemetryAddr = ":4427"` constants
- Proto package `pheromone.v1`: no proto change required; ports are transport-layer configuration
- Monitoring: scrape gRPC metrics on port 4426 via Prometheus gRPC interceptor (Phase 2)

## References

- ADR-003 (gRPC Service Contracts — defines the three services sharing these ports)
- ADR-010 (Signal Protocol evaluation — confirms gRPC + mTLS; no port changes from that ADR)
- ADR-001 (Protocol foundation — mandates gRPC for agent↔server communication)
- FR-007 (REST API for CLI/UI queries — drives the 443/80 assignment)
- IANA Service Name and Transport Protocol Port Number Registry: https://www.iana.org/assignments/service-names-port-numbers/

---

**Decision Date**: 2026-02-21
**Status**: Accepted

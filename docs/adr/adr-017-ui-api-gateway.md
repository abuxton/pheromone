# ADR-017: UI/API Gateway Design — WebSocket/SSE, OpenAPI, k8s Probes

**Status**: Accepted
**Date**: 2026-03-11
**Author**: @copilot
**Refs**: ADR-002 (Server Architecture), ADR-011 (Post-Action Hooks), ADR-014 (Security Architecture)

---

## Context

The current server (ADR-002) serves the management UI and REST API on a single HTTP listener, which is correct. However, four production-readiness gaps have been identified:

1. **No real-time push capability** — Clients must poll `/api/v1/agents` and `/api/v1/twins` to detect state changes. The UI must refresh manually, creating poor operator experience and unnecessary request volume.
2. **No Kubernetes-compatible health probes** — The existing `/api/v1/health` endpoint is not at a standard k8s path. Liveness and readiness probes (`/healthz`, `/readyz`) are absent, preventing correct Pod scheduling and rolling restart behaviour.
3. **No API documentation** — The `/api/v1/*` surface has no machine-readable spec, making integration by external tooling difficult.
4. **No request guardrails** — There is no rate limiting or request body size enforcement, exposing the server to trivial resource exhaustion.

### Constraints

| Constraint | Value |
|---|---|
| New dependencies | Minimise; prefer Go standard library and `golang.org/x/` packages already in the module graph |
| Protocol | HTTP/1.1; keep existing `net/http` + `http.ServeMux` stack (no new router) |
| Real-time direction | Server → client push only (agent status, twin state changes) |
| Backwards compatibility | All existing `/api/v1/*` paths and semantics must be preserved |
| Security | New endpoints must integrate with the existing auth/CORS/logging middleware chain |

---

## Decision

### 1. Server-Sent Events (SSE) at `/api/v1/events`

**Chosen over WebSocket** because:

- SSE is unidirectional (server→client), which is exactly the data flow required for agent/twin status updates from the server to the UI.
- SSE is a standard HTTP response; it works through every HTTP proxy and load balancer that forwards long-lived responses without any protocol-upgrade negotiation.
- SSE requires no additional dependency: the entire implementation uses `net/http` from the standard library.
- WebSocket (bidirectional) introduces upgrade complexity for no benefit when the client never sends data over the stream.

**Design**:
- Endpoint: `GET /api/v1/events`
- Authentication: Bearer token required via the `Authorization` header (same auth middleware as other `/api/v1` endpoints).
- Content-Type: `text/event-stream`
- Each **data event** is a JSON object serialised as an SSE `data:` line followed by a blank line.
- Event schema: `{ "type": "<event-type>", "payload": <object> }` — JSON event types include `agent.status_changed`, `twin.state_changed`, `connection.status_changed`.
- The server broadcasts an SSE **comment** line (`: heartbeat`) every 30 seconds to keep idle connections alive through proxies; this heartbeat is not a JSON `data:` event and does not use the `type`/`payload` schema.
- Clients that disconnect are cleaned up immediately; no goroutine leak.

### 2. Kubernetes-compatible Health Probes

Two probe endpoints are added at the **root path** (not under `/api/v1/`) to match standard k8s probe conventions and avoid authentication middleware:

| Path | Probe type | Semantics |
|---|---|---|
| `/healthz` | Liveness | Returns `200 OK` if the server process is alive and the HTTP listener is responding. Always succeeds once the process is running. |
| `/readyz` | Readiness | Returns `200 OK` if the server is ready to accept traffic (started and seeded). Returns `503 Service Unavailable` if the server is in a startup or shutdown phase. |

Both endpoints return `application/json` with `{ "status": "ok" | "starting" }`.

The existing `/api/v1/health` endpoint is retained unchanged for backwards compatibility.

### 3. OpenAPI Specification

A hand-maintained `docs/api/openapi.yaml` provides an OpenAPI 3.1 description of all `/api/v1/*` routes. It is committed to the repository and may be embedded into the UI build or served by the API process, but no specific public HTTP path is mandated by this ADR. The spec is the authoritative API contract for external consumers.

Annotation-driven generation (e.g. `swaggo/swag`) is deferred to a later phase when the API surface stabilises; it would require a build-time code-generation step and a new dependency.

### 4. Request Guardrails Middleware

Two lightweight middleware functions are added to `internal/api/middleware.go`:

| Middleware | Mechanism | Default limit |
|---|---|---|
| `requestSizeLimitMiddleware` | Wraps `w`/`r.Body` with `http.MaxBytesReader`; downstream handlers treat `*http.MaxBytesError` as `413 Request Entity Too Large` when the body exceeds the limit | 1 MiB |
| `rateLimitMiddleware` | Per-IP token-bucket rate limiter; returns `429 Too Many Requests` when a client exceeds the burst | 60 req/min, burst 20 |

Both are implemented using only Go standard library primitives (`net/http`, `sync`, `time`, `io`). No new module dependencies are introduced.

The rate limiter uses a lazy-initialised per-IP bucket stored in a `sync.Map`. Stale entries are evicted by a background goroutine that runs every 5 minutes, removing buckets not accessed in the last 10 minutes.

**Middleware chain** (outermost first):

```
corsMiddleware
  └─ rateLimitMiddleware         ← NEW: reject abusive clients early
       └─ requestSizeLimitMiddleware  ← NEW: reject oversized bodies early
            └─ loggingMiddleware
                 └─ authMiddleware
                      └─ mux
```

---

## Consequences

### Positive

- Operators see real-time agent/twin status in the management UI without polling.
- Kubernetes deployments can use standard liveness and readiness probes.
- External tools and integrations have a machine-readable API contract.
- The server is protected against naive resource-exhaustion attacks.

### Negative / Trade-offs

- SSE requires long-lived HTTP connections; each connected UI client holds one goroutine and one channel. At MVP scale (tens of operators) this is negligible; revisit if operators scale to thousands of concurrent sessions.
- The hand-maintained `openapi.yaml` can drift from the implementation. A CI linting step (e.g. `vacuum` or `spectral`) should be added as a follow-up to catch drift.
- The per-IP rate limiter is approximate; it does not account for clients behind NAT sharing a single IP. A session-scoped limiter (keyed by token sub) would be more accurate but requires the auth middleware to run first. This is a known acceptable limitation for the MVP.

### Follow-up Actions

| Action | ADR | Priority |
|---|---|---|
| Add CI lint step for `openapi.yaml` (spectral/vacuum) | ADR-017 | Medium |
| Token-scoped rate limiter (Phase 2) | ADR-015, ADR-017 | Low |
| gRPC-gateway generated OpenAPI (Phase 2) | ADR-003, ADR-017 | Low |

---

## References

- ADR-002: Server Architecture (in-memory + etcd; HTTP listener)
- ADR-011: Post-Action Hooks (existing event propagation)
- ADR-014: Security Architecture (TLS, auth middleware)
- [W3C Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html) — WHATWG HTML Living Standard, section on SSE
- [Kubernetes Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [OpenAPI 3.1 Specification](https://spec.openapis.org/oas/v3.1.0)

# ADR 011: Post-Action Hooks — Notifications, Webhooks, and Package Delivery

## Status

Accepted

<!-- 2026-03-10: ADR accepted after all three tech spikes passed.
  Spike #26 (HTTP webhook dispatch at scale): two-tier FIFO dispatcher
  (50 event workers + 1000 HTTP workers) delivered 100,000 webhooks
  (1000 events × 100 endpoints) in ~4.4 s under -race; P99 per-endpoint
  dispatch latency = 62 ms (SLA: < 2 s). Implementation in
  internal/hooks/hooks.go.
  Spike #27 (Agent direct notify resilience): NotifyDirect returns in < 10 ms
  regardless of destination reachability. Implementation in
  internal/notification/skill.go.
  Spike #28 (HMAC webhook verification): ComputeHMAC / VerifyHMAC with
  HMAC-SHA256 and constant-time comparison, compatible with Python
  hmac.compare_digest. Implementation in internal/notification/hmac.go.
  All spike tests pass: go test ./internal/hooks/... ./internal/notification/...
  -race -timeout 120s. -->

## Context

As Pheromone agents execute skills within their AI reasoning loop, downstream parties need to be informed of
outcomes: operators monitoring infrastructure, CI/CD pipelines awaiting package delivery confirmation, end
users expecting status updates, or external systems that integrate with Pheromone via webhooks. Currently no
standardised mechanism exists for agents or the server to publish action results beyond the internal
`TelemetryStream`.

This ADR addresses three closely related use cases:

1. **Operator / End-User Notifications** — an agent completes a config enforcement action and needs to notify
   an operator via Slack, email, or a custom HTTP endpoint that the action succeeded or failed.
2. **Webhook Integrations** — external systems (CI/CD, ticketing, incident management) register webhooks that
   must fire when specific action types complete on the server.
3. **Package / Artifact Delivery Notifications** — an agent installs, upgrades, or removes a software package
   and the platform must emit a structured delivery event (package name, version, host, outcome) to a
   downstream consumer (package registry, CMDB, auditing system).

The problem is further segmented by *where* the notification originates:

- **Agent-originated** — the agent has direct knowledge of the action outcome (e.g., package install result)
  and can publish directly to an external channel without a server round-trip, reducing latency and
  single-point-of-failure risk.
- **Server-originated** — the server aggregates outcomes from multiple agents and fires hooks based on fleet-
  wide policies (e.g., "notify when all agents in group X have converged").

**Key requirements derived from the problem statement:**

| Requirement | Source |
|---|---|
| R-01 | Agent MUST be able to send a gRPC message to the server reporting post-action status |
| R-02 | Agent MUST be able to notify end users directly (without mandatory server relay) |
| R-03 | Server MUST support registerable webhook hooks triggered by action events |
| R-04 | Notification destinations MUST include at minimum: HTTP webhook, gRPC relay, structured log |
| R-05 | Package delivery events MUST carry structured metadata (name, version, host, outcome, timestamp) |
| R-06 | Hook configuration MUST be part of the twin model or agent capability advertisement |
| R-07 | Post-action hooks MUST NOT block the agent's reasoning loop (fire-and-forget with retry) |
| R-08 | All hook invocations MUST be observable (logged, metriced, traceable) |

**Options Evaluated:**

| Option | Pros | Cons |
|---|---|---|
| Agent-only direct HTTP webhooks | Simple, low latency, no server dependency | Config fragmented across agents; no fleet-wide policy |
| Server-only relay | Centralised config and audit | Server bottleneck; agents can't report without server |
| Hybrid: Agent skill + Server hook service | Both latency and centralisation benefits; complements existing gRPC architecture | More protocol surface area |
| NATS subject per hook type (reuse ADR-005) | Reuses existing message queue | Adds NATS dependency to notification path; harder to gate delivery |

**Decision**: Hybrid model (Option 3). Agents gain a **Notification Skill** for direct delivery;
the server gains a **Post-Action Hook Service** for centralised webhook management and package delivery events.
Both paths emit observations through the existing `TelemetryStream` for full observability.

## Decision

**Implement post-action hooks as a composable Notification Skill on agents and a Post-Action Hook Service
on the server, communicating via an extended gRPC `ActionEventService`.**

### Architecture Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           Pheromone Agent                                    │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  AI Reasoning Loop (ADR-007)                                         │   │
│  │  … executes skill action …                                           │   │
│  │                    │ action outcome                                   │   │
│  │                    ▼                                                  │   │
│  │  ┌─────────────────────────────────────────────────────────────┐    │   │
│  │  │  Notification Skill (NEW)                                   │    │   │
│  │  │  - NotifyServer(event ActionEvent) → gRPC                   │    │   │
│  │  │  - NotifyDirect(event, []Destination) → HTTP/email/Slack    │    │   │
│  │  │  - EmitDeliveryEvent(pkg PackageEvent) → gRPC + direct      │    │   │
│  │  └────────────────────┬────────────────────────────────────────┘    │   │
│  └───────────────────────┼────────────────────────────────────────────┘    │
│                           │                                                  │
│        ┌──────────────────┼──────────────────────────┐                      │
│        │  gRPC: ActionEventService                    │                      │
│        │  (extends ADR-003 TwinControl/Telemetry)     │                      │
│        │  rpc ReportActionEvent(ActionEvent)          │                      │
│        │      returns (ActionEventAck)                │                      │
│        └──────────────────┼──────────────────────────┘                      │
│                           │  direct notify (fire-and-forget, async)          │
│        ┌──────────────────▼──────────────────────────┐                      │
│        │  Direct Notification Destinations            │                      │
│        │  - HTTP Webhook (POST)                       │                      │
│        │  - Structured Log (JSON, trace-id tagged)    │                      │
│        │  - Email (SMTP, via configurable gateway)    │                      │
│        │  - Slack / Teams (via incoming webhook URL)  │                      │
│        └─────────────────────────────────────────────┘                      │
└──────────────────────────────────────────────────────────────────────────────┘
                           │ gRPC ReportActionEvent
                           ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                    Pheromone Server                                           │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  Post-Action Hook Service (NEW)                                      │   │
│  │  - RegisterHook(HookConfig) — operator registers hooks via API       │   │
│  │  - EvaluateHooks(event ActionEvent) — match event → hooks            │   │
│  │  - DispatchHook(hook, event) — fire HTTP webhook, relay event        │   │
│  │  - RecordDeliveryEvent(pkg PackageEvent) — persist delivery record   │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  Hook Registry (persisted via etcd/ADR-002)                          │   │
│  │  - Hooks scoped to: agent_id, twin_id, skill_name, event_type        │   │
│  │  - Supports fleet-wide hooks (wildcard agent_id="*")                 │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │  Delivery Event Log (persisted)                                      │   │
│  │  - Immutable append-only log of all package/config delivery events   │   │
│  │  - Queryable by: agent_id, twin_id, package, date range, outcome     │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 1. Notification Skill (Agent-Side)

The `NotificationSkill` is a new agent skill that the AI reasoning loop invokes after executing any other
skill. It operates asynchronously (fire-and-forget with bounded retry) so it never blocks the reasoning cycle.

#### Skill Interface

```go
// skills/notification.go

// NotificationSkill sends post-action events to the server and/or directly to
// configured destinations. All methods are non-blocking (async dispatch).
type NotificationSkill interface {
    // NotifyServer sends a structured ActionEvent to the Pheromone server via gRPC.
    // The server will evaluate registered hooks and dispatch as appropriate.
    NotifyServer(ctx context.Context, event ActionEvent) error

    // NotifyDirect fires notifications to a list of destinations without going
    // through the server. Useful when the server is unreachable or latency is
    // critical (e.g., package delivery alerting to on-call channel).
    NotifyDirect(ctx context.Context, event ActionEvent, dests []NotificationDestination) error

    // EmitDeliveryEvent reports a package/artifact delivery outcome to the server
    // AND fires any configured direct delivery hooks.
    EmitDeliveryEvent(ctx context.Context, event PackageDeliveryEvent) error
}
```

#### Proto Extension (additive to `pheromone.v1`)

```proto
// action_events.proto — new file in pheromone.v1 package

service ActionEventService {
  // Agent reports the outcome of a skill execution to the server.
  rpc ReportActionEvent(ActionEvent) returns (ActionEventAck);

  // Agent reports a package/artifact delivery event.
  rpc ReportDeliveryEvent(PackageDeliveryEvent) returns (DeliveryEventAck);

  // Server streams registered hook configurations back to agent on connect
  // so the agent can fire direct hooks without a server round-trip.
  rpc StreamHookConfig(HookConfigRequest) returns (stream HookConfig);
}

message ActionEvent {
  string  agent_id       = 1;
  string  twin_id        = 2;
  string  skill_name     = 3;   // e.g., "config-enforce", "digital-twin"
  string  action_type    = 4;   // e.g., "package-install", "config-apply", "rollback"
  string  outcome        = 5;   // "success" | "failure" | "partial"
  string  detail         = 6;   // human-readable outcome summary
  int64   timestamp_unix = 7;
  string  trace_id       = 8;   // correlates with TelemetryStream trace
  map<string, string> labels = 9;  // arbitrary key-value context
}

message ActionEventAck {
  bool   received       = 1;
  int32  hooks_triggered = 2;  // how many server-side hooks were fired
}

message PackageDeliveryEvent {
  string agent_id      = 1;
  string twin_id       = 2;
  string package_name  = 3;
  string package_version = 4;
  string action        = 5;   // "install" | "upgrade" | "remove"
  string outcome       = 6;   // "success" | "failure"
  string error_detail  = 7;   // populated on failure
  int64  timestamp_unix = 8;
  string trace_id      = 9;
}

message DeliveryEventAck {
  bool received = 1;
}

message HookConfigRequest {
  string agent_id = 1;
}

message HookConfig {
  string hook_id        = 1;
  string scope_agent_id = 2;   // "*" = fleet-wide
  string scope_twin_id  = 3;   // "*" = any twin
  string scope_skill    = 4;   // "*" = any skill
  string event_type     = 5;   // "action" | "delivery" | "*"
  string outcome_filter = 6;   // "success" | "failure" | "*"
  NotificationDestination destination = 7;
}

message NotificationDestination {
  string type = 1;  // "http-webhook" | "slack" | "email" | "log"
  string url  = 2;  // webhook URL or email address
  map<string, string> headers = 3;  // custom HTTP headers (e.g., auth tokens)
  int32  timeout_seconds = 4;       // default: 10
  int32  max_retries     = 5;       // default: 3
}
```

### 2. Post-Action Hook Service (Server-Side)

The server receives `ActionEvent` and `PackageDeliveryEvent` messages and evaluates all registered `HookConfig`
entries. Matching hooks are dispatched asynchronously.

#### Hook Evaluation Logic

```
For each incoming ActionEvent / PackageDeliveryEvent:
  1. Load all HookConfigs from hook registry (cached in memory, persisted in etcd)
  2. For each HookConfig:
       if scope_agent_id matches AND scope_twin_id matches AND
          scope_skill matches AND event_type matches AND outcome_filter matches:
         → enqueue hook dispatch (async worker pool)
  3. Each dispatch worker:
       a. Serialise event to JSON
       b. Send HTTP POST to destination URL (with retry + exponential backoff)
       c. Record dispatch outcome in delivery event log
       d. Emit pheromone_hook_dispatch_total{outcome} metric
```

#### Hook Registration API (REST, Phase 1)

```
POST /api/v1/hooks
GET  /api/v1/hooks
GET  /api/v1/hooks/{hook_id}
DELETE /api/v1/hooks/{hook_id}
GET  /api/v1/events/delivery?agent_id=&package=&since=
```

Hook registration is also available via gRPC management service (Phase 2, pending ADR-002 server API design).

### 3. Direct End-User Notification from Agent (R-02)

Agents receive their applicable `HookConfig` entries from the server at connection time via
`StreamHookConfig`. On successful connection, the server streams all hooks scoped to `agent_id` (and
wildcard hooks). The agent caches these locally.

When the `NotificationSkill.NotifyDirect()` is invoked, the agent fires the matching hooks directly without a
server round-trip:

```go
// framework/notification_skill.go (agent-side implementation sketch)

func (s *NotificationSkillImpl) NotifyDirect(
    ctx context.Context,
    event ActionEvent,
    dests []NotificationDestination,
) error {
    for _, dest := range dests {
        go func(d NotificationDestination) {
            payload, _ := json.Marshal(event)
            s.dispatch(ctx, d, payload)  // HTTP POST with retry
        }(dest)
    }
    return nil  // fire-and-forget; errors logged, not propagated
}
```

This ensures agents can report status to end users even when the Pheromone server is temporarily unavailable.

### 4. Package Delivery Event Schema

Package delivery events carry structured metadata to enable downstream consumers (CMDB, audit, compliance):

```json
{
  "event_type": "package_delivery",
  "agent_id": "agent-host-1",
  "twin_id": "os-twin-host-1",
  "package_name": "nginx",
  "package_version": "1.24.0-1ubuntu1",
  "action": "install",
  "outcome": "success",
  "error_detail": "",
  "timestamp_unix": 1740081600,
  "trace_id": "abc-123-xyz"
}
```

Consumers receiving this via webhook can integrate with:
- **CMDB** systems (ServiceNow, Lansweeper) for automatic CMDB updates
- **Package registries** for deployment audit trails
- **Compliance tooling** for CIS/STIG enforcement verification
- **Incident management** (PagerDuty, Opsgenie) for delivery failures

### 5. Observability Requirements (Constitution Principle II)

All hook invocations MUST emit:

```
# Agent-side metrics
pheromone_notification_dispatched_total{agent_id, dest_type, outcome}
pheromone_notification_retry_total{agent_id, dest_type}
pheromone_notification_latency_seconds{agent_id, dest_type}  # histogram

# Server-side metrics
pheromone_hook_dispatch_total{hook_id, event_type, outcome}
pheromone_hook_dispatch_latency_seconds{hook_id}             # histogram
pheromone_delivery_events_total{agent_id, package_name, action, outcome}
```

All hook dispatches are logged with `trace_id` matching the originating `ActionEvent` for full end-to-end
traceability across `TelemetryStream` → `ActionEventService` → hook dispatch.

### 6. Agent Capability Advertisement (ADR-007 Extension)

The `AgentCapabilities` message (ADR-007, extended in ADR-003) MUST be updated to declare notification skill
support:

```proto
message AgentCapabilities {
  bool   ai_reasoning_enabled     = 1;
  string ai_model_id              = 2;
  repeated string skills          = 3;  // add "notification" to skill list
  string skill_contract_version   = 4;
  bool   direct_notify_enabled    = 5;  // NEW: agent supports direct end-user notification
  repeated string notify_dest_types = 6; // NEW: e.g., ["http-webhook", "slack"]
}
```

### 7. Integration with Reasoning Loop (ADR-006 Extension)

The agent framework's `Run()` loop MUST be extended to invoke the `NotificationSkill` after each skill
execution:

```go
// framework.go — addition to executeAction()
func (f *AgentFramework) executeAction(agent Agent, action Action) {
    result := f.invokeSkill(agent, action)
    f.telemetry.EmitTrace(action, result)         // existing: AI decision trace

    // NEW: post-action notification
    event := ActionEvent{
        AgentID:   f.agentID,
        TwinID:    action.TwinID,
        SkillName: action.Skill,
        ActionType: action.Type,
        Outcome:   result.Outcome,
        Detail:    result.Summary,
        TraceID:   action.TraceID,
    }
    f.notifySkill.NotifyServer(ctx, event)         // async, non-blocking
    f.notifySkill.NotifyDirect(ctx, event, f.cachedHooks.MatchingDests(event))  // async
}
```

## Consequences

### Positive

- **Operator Visibility**: Operators receive real-time notifications of agent actions without polling the
  server or parsing raw telemetry streams.
- **Integration Ecosystem**: Webhook support unlocks integration with any HTTP-capable system (ITSM, CI/CD,
  chat platforms, CMDB) without platform-specific connectors.
- **Resilience**: Direct agent-to-user notification path survives server outages; server path provides
  fleet-wide policy enforcement.
- **Audit Trail**: Package delivery events create an immutable, queryable record of all software changes
  across the fleet.
- **Extensibility**: `NotificationSkill` follows the same skill contract as `DigitalTwinSkill`; new
  destination types added without changing the agent framework core.
- **Low Coupling**: Agents cache hook configs locally; the reasoning loop is not blocked by notification
  dispatch.

### Negative

- **Additional Protocol Surface**: New gRPC service `ActionEventService` adds to the protocol contract
  surface area that must be versioned and maintained.
- **Hook Fan-Out Risk**: Fleet-wide wildcard hooks (`agent_id="*"`) on large fleets (>1K agents) can
  generate significant webhook volume; operators must set rate limits.
- **Credential Management**: Webhook destinations with auth tokens require secure storage; agents must not
  log or trace destination credentials.
- **Retry Complexity**: Exponential backoff with bounded retries adds complexity to the notification skill
  implementation; failed deliveries must be surfaced (dead-letter queue or alert).
- **Direct Notify Trust**: Agents firing webhooks directly bypass server-side audit; operators must decide
  whether to mandate server-relay-only mode for compliance.

### Security Considerations

- Webhook destination URLs and auth tokens MUST be stored encrypted in etcd and transmitted via gRPC with
  mTLS (ADR-010 confirmation).
- Agents MUST NOT include destination credentials in `TelemetryStream` traces or AI decision traces.
- The server MUST validate hook registration requests with RBAC (Phase 2 ADR-TBD); in Phase 1, all
  registered hooks are trusted (internal network assumed).
- Direct notify destinations received from server MUST be validated (URL allowlist configurable).
- HTTP webhook payloads MUST include an HMAC-SHA256 signature header (`X-Pheromone-Signature`) so receivers
  can verify authenticity.

### Migration / Compatibility

- **Phase 1 (MVP)**: `ActionEventService` deployed alongside existing services; `NotificationSkill` added to
  Go agent scaffold (ADR-006); server implements basic HTTP webhook dispatch. Slack/email destinations are
  Phase 1 stretch goals via generic HTTP webhook (Slack incoming webhook, SMTP gateway).
- **Phase 2**: Dedicated notification destinations (native Slack, email gateway, PagerDuty); dead-letter
  queue for failed dispatches (NATS JetStream, ADR-005).
- **Backward Compatibility**: Old agents without `NotificationSkill` continue to operate; server does not
  require `ReportActionEvent` from agents.

## Testing Strategy

```go
// Unit: NotificationSkill fires without blocking
func TestNotificationSkillNonBlocking(t *testing.T) {
    skill := NewNotificationSkill(mockGRPCClient, mockHTTPClient)
    start := time.Now()
    err := skill.NotifyDirect(ctx, ActionEvent{Outcome: "success"}, []NotificationDestination{
        {Type: "http-webhook", URL: "http://slow-server/hook"},
    })
    assert.NoError(t, err)
    assert.Less(t, time.Since(start), 10*time.Millisecond) // non-blocking
}

// Unit: HMAC signature generated correctly
func TestWebhookHMACSignature(t *testing.T) {
    payload := []byte(`{"outcome":"success"}`)
    sig := computeHMAC(payload, []byte("secret"))
    assert.True(t, verifyHMAC(payload, sig, []byte("secret")))
}

// Integration: ActionEvent round-trip (agent → server → webhook)
func TestActionEventRoundTrip(t *testing.T) {
    hookReceived := make(chan []byte, 1)
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        hookReceived <- body
        w.WriteHeader(200)
    }))
    // Register hook, fire ActionEvent, assert webhook received
    ...
    select {
    case body := <-hookReceived:
        var event ActionEvent
        json.Unmarshal(body, &event)
        assert.Equal(t, "success", event.Outcome)
    case <-time.After(5 * time.Second):
        t.Fatal("webhook not received")
    }
}

// Integration: PackageDeliveryEvent persisted and queryable
func TestDeliveryEventPersistence(t *testing.T) {
    server := newTestServer(t)
    server.ReportDeliveryEvent(ctx, &PackageDeliveryEvent{
        AgentID: "a1", PackageName: "nginx", PackageVersion: "1.24",
        Action: "install", Outcome: "success",
    })
    events := server.QueryDeliveryEvents(ctx, &DeliveryQuery{AgentID: "a1"})
    assert.Len(t, events, 1)
    assert.Equal(t, "nginx", events[0].PackageName)
}
```

## Tech Spike Required

All three spikes completed and validated (2026-03-10):

| Spike | Goal | Result |
|---|---|---|
| HTTP webhook dispatch at scale (issue #26) | Measure latency/throughput of server dispatching hooks to 100 registered endpoints when 1000 agents fire events simultaneously | ✅ 100,000 deliveries in ~4.4s; P99 per-endpoint dispatch latency = 62ms (SLA: < 2s) |
| Agent direct notify resilience (issue #27) | Verify agent continues reasoning loop when notification destinations are unreachable | ✅ NotifyDirect returns in < 10ms regardless of destination reachability |
| HMAC webhook verification (issue #28) | Prototype signature generation and verification across Go agent and Python receiver | ✅ HMAC-SHA256 with constant-time comparison; Python-compatible |

Implementation: `internal/hooks/`, `internal/notification/`

## Follow-Up ADRs

- **ADR-012** (Proposed): Dead-letter queue design for failed hook dispatches (depends on ADR-005 NATS
  JetStream)
- **ADR-TBD**: RBAC for hook registration and delivery event access (Phase 2 security)

## References

- ADR-003 (gRPC contracts — `ActionEventService` extends existing services)
- ADR-005 (NATS — Phase 2 dead-letter queue for failed dispatches)
- ADR-006 (Agent lifecycle — `NotificationSkill` added to scaffold)
- ADR-007 (Agentic AI Agent Model — notification is a post-action step in the reasoning loop)
- ADR-010 (mTLS — webhook destination credentials secured via same mTLS infrastructure)
- Spec-001, FR-011, FR-020, SC-008
- Constitution Principle II (Observability — all hook dispatches are metriced and traced)
- Constitution Principle III (Protocol — gRPC with Protocol Buffers for agent→server notification)
- HMAC-SHA256: https://datatracker.ietf.org/doc/html/rfc2104
- Webhook best practices: https://webhooks.fyi/

---

**Decision Date**: 2026-02-21
**Status**: Proposed — requires tech spike validation and team review before Accepted

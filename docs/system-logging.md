# System Logging Reference

This document describes every structured log event emitted by the Pheromone
server and agent processes, the log format and field breakdown, log-level
semantics, and examples for consuming and filtering system logs.

For audit-specific events (user logins, user management, API key operations),
see [Audit Logging](audit-logging.md).

---

## Overview

Pheromone uses Go's standard [`log/slog`](https://pkg.go.dev/log/slog) package
for all structured logging. Key properties:

- **Structured JSON** output (server processes, `pheromone-server`) or
  **structured text** output (agent bootstrap, `pheromone-agent`) depending on
  the component.
- **Runtime log-level control**: The management server exposes
  `POST /api/v1/admin/log-level` so operators can change verbosity without
  restarting the process.
- **NATS fan-out**: Agent processes can attach a `NATSHandler` that publishes
  every log record to `logs.{agent_id}` for centralised collection.
- **Request correlation**: Every HTTP handler receives a `request_id` that is
  propagated through context and appears in request log lines.

---

## Log Record Format

### JSON (server default)

The management server writes JSON to **stderr**. Each log line is a single
JSON object:

```json
{
  "time":       "2026-03-17T14:05:32.001234567Z",
  "level":      "INFO",
  "msg":        "http request",
  "method":     "GET",
  "path":       "/api/v1/agents",
  "status":     200,
  "duration_ms": 3,
  "remote":     "10.0.0.5:41234",
  "request_id": "a1b2c3d4",
  "component":  "api"
}
```

### Text (agent / bootstrap)

Agent startup and bootstrap use a text handler that writes to **stdout**:

```
time=2026-03-17T14:05:32.001Z level=INFO msg="agent started" agent_id=agent-01 twin_count=4 skill_count=2
```

### NATS-published records

When a `NATSHandler` is configured, each record is published as JSON to the
NATS subject `logs.{agent_id}`:

```json
{
  "time":       "2026-03-17T14:05:32.001234567Z",
  "level":      "INFO",
  "msg":        "reasoning cycle complete",
  "agent_id":   "agent-01",
  "request_id": "req-abc123",
  "skill":      "nginx-monitor",
  "reasoner":   "rule-based"
}
```

---

## Field Reference

| Field         | Type    | Always Present | Description |
|---------------|---------|----------------|-------------|
| `time`        | string (RFC3339Nano UTC) | yes | Timestamp when the record was created. |
| `level`       | string  | yes | Log level: `DEBUG`, `INFO`, `WARN`, or `ERROR`. |
| `msg`         | string  | yes | Human-readable event description. |
| `component`   | string  | no  | Subsystem that emitted the record (`api`, `reasoning-loop`, etc.). |
| `agent_id`    | string  | no  | ID of the agent emitting the record (set by `WithAgentAttrs`). |
| `skill`       | string  | no  | Active skill name when the record is emitted from within a skill execution. |
| `reasoner`    | string  | no  | Reasoner type (`rule-based`, `ollama`, etc.) when the record is emitted from the reasoning loop. |
| `request_id`  | string  | no  | Correlation ID for an HTTP request, propagated through `context.Context`. |
| `error`       | string  | no  | Error message string; present on `WARN` and `ERROR` records. |

Additional context-specific fields are documented per event below.

---

## Log Levels

| Level   | slog constant      | Usage |
|---------|--------------------|-------|
| `DEBUG` | `slog.LevelDebug`  | Verbose diagnostic information useful during development. Not emitted at the default `INFO` level. |
| `INFO`  | `slog.LevelInfo`   | Normal operational events (requests served, agents started, actions planned). Default level. |
| `WARN`  | `slog.LevelWarn`   | Recoverable conditions that warrant attention (failed hook dispatch, dropped events, nil twin state). |
| `ERROR` | `slog.LevelError`  | Unrecoverable or unexpected failures that require operator intervention. |

The default log level is `INFO`. To change at runtime:

```bash
curl -s -X POST http://localhost:8080/api/v1/admin/log-level \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'
```

Valid values: `debug`, `info`, `warn`, `error`.

The `log_level` key in `pheromone-server.yml` sets the initial level:

```yaml
log_level: info   # debug | info | warn | error
```

---

## Event Catalogue

### HTTP Layer — `component: api`

#### `http request` — Inbound HTTP request (INFO)

Emitted by the logging middleware for **every** HTTP request after it completes.

```json
{
  "time":        "2026-03-17T14:05:32.001Z",
  "level":       "INFO",
  "msg":         "http request",
  "method":      "GET",
  "path":        "/api/v1/agents",
  "status":      200,
  "duration_ms": 3,
  "remote":      "10.0.0.5:41234",
  "request_id":  "a1b2c3d4",
  "component":   "api"
}
```

| Field | Description |
|-------|-------------|
| `method` | HTTP verb (`GET`, `POST`, `PUT`, `DELETE`). |
| `path` | Request path (without query string). |
| `status` | HTTP response status code. |
| `duration_ms` | Request processing time in milliseconds. |
| `remote` | Client IP and port. |
| `request_id` | Per-request correlation ID. |

#### `audit` — Privileged operation (INFO)

A shadow copy of each audit entry emitted alongside the in-memory audit log
(see [Audit Logging](audit-logging.md) for the full event list).

```json
{
  "time":     "2026-03-17T14:05:32.001Z",
  "level":    "INFO",
  "msg":      "audit",
  "user":     "alice",
  "role":     "admin",
  "op":       "user.create",
  "resource": "bob",
  "result":   "ok"
}
```

#### `broadcast: marshal event failed` — SSE broadcast failure (WARN)

Emitted when the server cannot serialise a real-time event for the SSE stream.
The SSE stream is best-effort; this event does not affect API correctness.

```json
{
  "time":  "2026-03-17T14:10:00.000Z",
  "level": "WARN",
  "msg":   "broadcast: marshal event failed",
  "error": "json: unsupported type: chan struct {}"
}
```

#### `pheromone UI server listening` — Server ready (INFO)

Emitted once the HTTP listener is bound and ready to accept connections.

```json
{
  "time":   "2026-03-17T14:00:01.000Z",
  "level":  "INFO",
  "msg":    "pheromone UI server listening",
  "addr":   ":8080",
  "scheme": "http"
}
```

#### `log level updated` — Dynamic log-level change (INFO)

Emitted when an admin changes the server log level at runtime.

```json
{
  "time":  "2026-03-17T15:00:00.000Z",
  "level": "INFO",
  "msg":   "log level updated",
  "level": "debug"
}
```

---

### Agent / Skill Framework

#### `agent started` — Agent framework initialised (INFO)

Emitted once at agent startup after all twins and skills are registered.

```json
{
  "time":        "2026-03-17T14:00:00.500Z",
  "level":       "INFO",
  "msg":         "agent started",
  "agent_id":    "agent-01",
  "twin_count":  4,
  "skill_count": 2
}
```

#### `agent stopped` — Agent framework shut down cleanly (INFO)

Emitted after the agent's reasoning loop and all skills are stopped.

```json
{
  "time":     "2026-03-17T18:00:00.000Z",
  "level":    "INFO",
  "msg":      "agent stopped",
  "agent_id": "agent-01"
}
```

#### `reasoning cycle complete` — End of one reasoning tick (INFO)

Emitted at the conclusion of every reasoning loop iteration.

```json
{
  "time":                "2026-03-17T14:00:05.000Z",
  "level":               "INFO",
  "msg":                 "reasoning cycle complete",
  "agent_id":            "agent-01",
  "component":           "reasoning-loop",
  "observations_count":  8,
  "drift_detected":      true,
  "actions_planned":     2,
  "actions_executed":    2,
  "duration_ms":         47,
  "twin_updates":        "[{\"id\":\"twin-nginx\",\"state\":\"active\"}]"
}
```

| Field | Description |
|-------|-------------|
| `observations_count` | Total number of metric + twin observations collected in this tick. |
| `drift_detected` | `true` when the reasoner identified divergence between desired and current state. |
| `actions_planned` | Number of corrective actions emitted by the reasoner. |
| `actions_executed` | Number of actions successfully dispatched to skills. |
| `duration_ms` | Wall-clock time of the entire reasoning tick in milliseconds. |
| `twin_updates` | JSON-encoded list of twin state changes applied during this tick. |

#### `reasoning tick error` — Unrecoverable tick error (ERROR)

Emitted when an unexpected error propagates out of the reasoning loop. The
agent attempts to continue on the next tick.

```json
{
  "time":     "2026-03-17T14:00:06.000Z",
  "level":    "ERROR",
  "msg":      "reasoning tick error",
  "agent_id": "agent-01",
  "error":    "twin store unavailable: context deadline exceeded"
}
```

#### `shutdown error` — Error during graceful shutdown (ERROR)

Emitted when a skill's `Stop()` method returns an error during agent shutdown.

```json
{
  "time":  "2026-03-17T18:00:01.000Z",
  "level": "ERROR",
  "msg":   "shutdown error",
  "error": "nginx skill: connection reset by peer"
}
```

#### `action failed` — Skill action execution error (WARN)

Emitted when a skill's `Execute()` call returns an error for an individual
action. The remaining actions in the cycle are still attempted.

```json
{
  "time":        "2026-03-17T14:00:05.300Z",
  "level":       "WARN",
  "msg":         "action failed",
  "skill":       "nginx-monitor",
  "action_type": "restart-service",
  "error":       "exec: signal: killed"
}
```

#### `could not retrieve current twin state` — Twin read error (WARN)

Emitted when the agent cannot fetch the current (observed) state of a twin
during the observation phase. The twin is skipped for this tick.

```json
{
  "time":    "2026-03-17T14:00:04.100Z",
  "level":   "WARN",
  "msg":     "could not retrieve current twin state",
  "twin_id": "twin-nginx-01",
  "error":   "store: key not found"
}
```

#### `could not retrieve desired twin state` — Twin desired-state read error (WARN)

Emitted when the agent cannot fetch the desired (target) state of a twin.
The twin is skipped for this tick.

```json
{
  "time":    "2026-03-17T14:00:04.200Z",
  "level":   "WARN",
  "msg":     "could not retrieve desired twin state",
  "twin_id": "twin-nginx-01",
  "error":   "store: key not found"
}
```

---

### Skill Loader

#### `skill loader complete` — Skill loading summary (INFO)

Emitted once when the skill loader finishes registering all skills from specs.

```json
{
  "time":          "2026-03-17T14:00:00.200Z",
  "level":         "INFO",
  "msg":           "skill loader complete",
  "mode":          "file",
  "specs_found":   5,
  "skills_loaded": 4
}
```

| Field | Description |
|-------|-------------|
| `mode` | Source mode: `file` (YAML specs from disk) or `builtin`. |
| `specs_found` | Total number of skill spec definitions found in the source. |
| `skills_loaded` | Number of skills successfully registered. |

#### `skill factory could not instantiate spec` — Spec instantiation error (WARN)

Emitted when the factory function for a skill spec returns an error.

```json
{
  "time":    "2026-03-17T14:00:00.180Z",
  "level":   "WARN",
  "msg":     "skill factory could not instantiate spec",
  "skill":   "http-probe",
  "version": "1.0.0",
  "error":   "missing required field: endpoint"
}
```

#### `skill factory returned nil for spec; skipping` — Nil skill from factory (DEBUG)

Emitted at DEBUG level when a factory returns `nil` without an error. The spec
is skipped silently in production.

```json
{
  "time":    "2026-03-17T14:00:00.175Z",
  "level":   "DEBUG",
  "msg":     "skill factory returned nil for spec; skipping",
  "skill":   "experimental-skill",
  "version": "0.0.1"
}
```

#### `skill already registered; skipping` — Duplicate skill registration (WARN)

Emitted when two specs share the same name and version.

```json
{
  "time":    "2026-03-17T14:00:00.185Z",
  "level":   "WARN",
  "msg":     "skill already registered; skipping",
  "skill":   "nginx-monitor",
  "error":   "skill already registered: nginx-monitor"
}
```

---

### Hooks

#### `hooks: dispatch failed` — Hook HTTP delivery failure (WARN)

Emitted when the hooks dispatcher cannot deliver an event to a configured
webhook URL after all retry attempts.

```json
{
  "time":     "2026-03-17T14:00:10.500Z",
  "level":    "WARN",
  "msg":      "hooks: dispatch failed",
  "hook_id":  "hook-abc123",
  "url":      "https://example.com/webhook",
  "trace_id": "trace-xyz789",
  "error":    "Post \"https://example.com/webhook\": dial tcp: connection refused"
}
```

#### `hooks: marshal event failed` — Hook serialisation error (ERROR)

Emitted when the hooks dispatcher cannot serialise the event payload.

```json
{
  "time":     "2026-03-17T14:00:10.100Z",
  "level":    "ERROR",
  "msg":      "hooks: marshal event failed",
  "trace_id": "trace-xyz789",
  "error":    "json: unsupported value: +Inf"
}
```

#### `hooks: task queue full, dropping hook` — Hook queue overflow (WARN)

Emitted when the per-hook task buffer is full. The event is dropped for this
hook; other hooks are unaffected.

```json
{
  "time":     "2026-03-17T14:00:10.200Z",
  "level":    "WARN",
  "msg":      "hooks: task queue full, dropping hook",
  "hook_id":  "hook-abc123",
  "trace_id": "trace-xyz789"
}
```

#### `hooks: queue full, dropping event` — Hook event queue overflow (WARN)

Emitted when the global hook event queue is full. The event is dropped
entirely.

```json
{
  "time":     "2026-03-17T14:00:10.300Z",
  "level":    "WARN",
  "msg":      "hooks: queue full, dropping event",
  "agent_id": "agent-01",
  "trace_id": "trace-xyz789"
}
```

---

### Notifications

#### `notification: failed to marshal event` — Notification serialisation error (ERROR)

Emitted when the notification skill cannot serialise an outbound event.

```json
{
  "time":  "2026-03-17T14:00:20.000Z",
  "level": "ERROR",
  "msg":   "notification: failed to marshal event",
  "error": "json: unsupported type: func()"
}
```

#### `notification: failed to build request` — Request construction error (ERROR)

Emitted when an HTTP request to a notification destination cannot be
constructed (e.g. invalid URL).

```json
{
  "time":  "2026-03-17T14:00:20.100Z",
  "level": "ERROR",
  "msg":   "notification: failed to build request",
  "url":   "http://[::1]:namedport/callback",
  "error": "invalid port"
}
```

#### `notification: dispatch failed` — Notification HTTP delivery failure (ERROR)

Emitted when a notification HTTP request fails (network error, timeout, etc.).

```json
{
  "time":  "2026-03-17T14:00:20.200Z",
  "level": "ERROR",
  "msg":   "notification: dispatch failed",
  "url":   "https://ops-webhook.example.com/notify",
  "error": "context deadline exceeded"
}
```

#### `notification: non-2xx response` — Non-successful notification response (WARN)

Emitted when a notification destination returns a non-2xx HTTP status. The
notification is considered delivered (no retry) but the unusual response is
flagged.

```json
{
  "time":   "2026-03-17T14:00:20.300Z",
  "level":  "WARN",
  "msg":    "notification: non-2xx response",
  "url":    "https://ops-webhook.example.com/notify",
  "status": 429
}
```

---

## Consumption Examples

### Stream JSON Logs in Real Time

```bash
# Server — JSON to stderr (redirect to stdout for piping)
./pheromone-server 2>&1 | jq -c '.'

# Filter only WARN and ERROR
./pheromone-server 2>&1 | jq -c 'select(.level == "WARN" or .level == "ERROR")'

# Watch reasoning cycles
./pheromone-server 2>&1 | jq -c 'select(.msg == "reasoning cycle complete")'

# Show all slow requests (> 100 ms)
./pheromone-server 2>&1 | jq -c 'select(.msg == "http request" and .duration_ms > 100)'
```

### Identify Errors and Warnings

```bash
# All errors
./pheromone-server 2>&1 | jq -c 'select(.level == "ERROR")'

# Hook delivery failures only
./pheromone-server 2>&1 | jq -c 'select(.msg | startswith("hooks:"))'

# Notification failures
./pheromone-server 2>&1 | jq -c 'select(.msg | startswith("notification:"))'
```

### Collecting Agent Logs from NATS

```bash
# Subscribe to all log subjects (requires nats CLI)
nats sub 'logs.>'

# Subscribe to a specific agent
nats sub 'logs.agent-01'

# Filter for errors only via jq
nats sub --raw 'logs.>' | jq -c 'select(.level == "ERROR")'
```

### Log Rotation with systemd

The server writes to **stderr** without managing its own log files.
Use `systemd` to capture and rotate output:

```ini
# /etc/systemd/system/pheromone-server.service
[Service]
ExecStart=/usr/local/bin/pheromone-server --config /etc/pheromone/server.yml
StandardOutput=journal
StandardError=journal
SyslogIdentifier=pheromone-server
```

Then query with `journalctl`:
```bash
journalctl -u pheromone-server -f --output=json \
  | jq -c 'select(.MESSAGE | test("\"level\":\"ERROR\""))'
```

### Forwarding to Loki / Grafana

Example Promtail configuration to scrape Pheromone server logs:

```yaml
# promtail-config.yml
scrape_configs:
  - job_name: pheromone
    static_configs:
      - targets:
          - localhost
        labels:
          job: pheromone-server
          __path__: /var/log/pheromone/server.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            msg: msg
            component: component
      - labels:
          level:
          component:
```

### Forwarding to Elasticsearch / OpenSearch

Example Filebeat configuration:

```yaml
# filebeat.yml
filebeat.inputs:
  - type: log
    paths:
      - /var/log/pheromone/server.log
    json.message_key: msg
    json.keys_under_root: true
    json.add_error_key: true

output.elasticsearch:
  hosts: ["https://elasticsearch:9200"]
  index: "pheromone-logs-%{+yyyy.MM.dd}"

setup.template.settings:
  index.number_of_shards: 1
```

---

## Useful Log Query Patterns

| Goal | Command |
|------|---------|
| Watch all errors live | `./pheromone-server 2>&1 \| jq -c 'select(.level=="ERROR")'` |
| Watch all audit events | `./pheromone-server 2>&1 \| jq -c 'select(.msg=="audit")'` |
| Watch HTTP 5xx responses | `./pheromone-server 2>&1 \| jq -c 'select(.msg=="http request" and .status>=500)'` |
| Watch slow requests | `./pheromone-server 2>&1 \| jq -c 'select(.duration_ms > 100)'` |
| Watch drift detections | `./pheromone-server 2>&1 \| jq -c 'select(.drift_detected==true)'` |
| Find all hook failures | `./pheromone-server 2>&1 \| jq -c 'select(.msg\|startswith("hooks:") and .level=="WARN")'` |
| Count events by message | `./pheromone-server 2>&1 \| jq -s 'group_by(.msg) \| map({msg: .[0].msg, count: length}) \| sort_by(-.count)'` |

---

## Context Enrichment

The `internal/logging` package provides `WithAgentAttrs` to inject standard
agent context into any `slog.Logger`:

```go
log = logging.WithAgentAttrs(log, agentID, skillName, reasonerName)
```

This adds `agent_id`, `skill`, and `reasoner` fields to every subsequent log
record emitted by that logger, matching the fields described in the Field
Reference above.

Request IDs are propagated through `context.Context`:

```go
ctx = logging.WithRequestID(ctx, requestID)
// Later:
id := logging.RequestIDFromContext(ctx) // retrieves the correlation ID
```

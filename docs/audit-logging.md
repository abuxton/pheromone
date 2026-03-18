# Audit Logging Reference

This document covers every audit event emitted by the Pheromone management
server, the JSON record format, field-by-field breakdown, and example commands
for consuming and querying the audit log.

---

## Overview

The Pheromone server records an immutable, append-only audit trail for every
privileged operation. Each entry captures who performed an action, what
resource was affected, and whether the action succeeded or was rejected.

Audit entries are:

- Stored **in-memory** as a thread-safe slice that survives for the lifetime of
  the server process.
- Accessible via the **REST API** at `GET /api/v1/audit` (admin role required).
- Simultaneously emitted to the **structured system log** at `INFO` level (see
  [System Logging](system-logging.md)) so that external log shippers can ingest
  them alongside regular log lines.

---

## Audit Record Format

Every audit entry is serialised as a JSON object with the following fields.

```json
{
  "id":          "3f9a1b2c",
  "timestamp":   "2026-03-17T14:05:32.001Z",
  "user_id":     "alice",
  "role":        "admin",
  "operation":   "user.create",
  "resource_id": "bob",
  "remote_addr": "10.0.0.5:41234",
  "result":      "ok",
  "message":     ""
}
```

### Field Reference

| Field         | Type            | Always Present | Description |
|---------------|-----------------|----------------|-------------|
| `id`          | string (hex-8)  | yes | Unique identifier for this audit entry (8 random hex characters). |
| `timestamp`   | string (RFC3339 UTC) | yes | UTC timestamp of the event with millisecond precision. |
| `user_id`     | string          | yes | The username (or attempted username) that triggered the operation. |
| `role`        | string          | yes | The role of the acting user (`admin`, `viewer`, or empty on auth failure). |
| `operation`   | string          | yes | Dot-separated event code identifying the action type (see [Event Catalogue](#event-catalogue)). |
| `resource_id` | string          | no  | Identifier of the resource that was created, modified, or deleted. Omitted for session operations. |
| `remote_addr` | string          | no  | IP address and port of the request source (`ip:port`). Omitted when not available. |
| `result`      | string          | yes | Outcome of the operation: `ok` on success, `fail` on rejection or error. |
| `message`     | string          | no  | Human-readable detail; present on failures (e.g. `"invalid credentials"`) and on named operations (e.g. the API key display name). Omitted when empty. |

---

## Event Catalogue

### Authentication Events

#### `auth.login` — Login attempt

Emitted for every POST `/api/v1/auth/login` request, regardless of outcome.

| Scenario | `result` | `message` |
|----------|----------|-----------|
| Credentials valid, account active | `ok` | _(empty)_ |
| Unknown username or wrong password | `fail` | `invalid credentials` |
| Account exists but is disabled | `fail` | `account disabled` |

**Example — successful login:**
```json
{
  "id": "a1b2c3d4",
  "timestamp": "2026-03-17T14:05:32.001Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "auth.login",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": ""
}
```

**Example — failed login (bad credentials):**
```json
{
  "id": "e5f6a7b8",
  "timestamp": "2026-03-17T14:06:01.120Z",
  "user_id": "badactor",
  "role": "",
  "operation": "auth.login",
  "remote_addr": "192.168.1.99:55312",
  "result": "fail",
  "message": "invalid credentials"
}
```

**Example — disabled account:**
```json
{
  "id": "c9d0e1f2",
  "timestamp": "2026-03-17T14:06:45.877Z",
  "user_id": "bob",
  "role": "viewer",
  "operation": "auth.login",
  "remote_addr": "10.0.0.8:39010",
  "result": "fail",
  "message": "account disabled"
}
```

---

### User Management Events

#### `user.create` — Create user

Emitted by POST `/api/v1/admin/users` when an admin creates a new user account.

| Field | Value |
|-------|-------|
| `resource_id` | Username of the newly created user |
| `result` | `ok` |

**Example:**
```json
{
  "id": "11223344",
  "timestamp": "2026-03-17T15:00:00.000Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "user.create",
  "resource_id": "charlie",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": ""
}
```

#### `user.update` — Update user

Emitted by PUT `/api/v1/admin/users/{username}` when an admin modifies an
existing user account (e.g. role change, password reset, enable/disable).

| Field | Value |
|-------|-------|
| `resource_id` | Username of the modified user |
| `result` | `ok` |

**Example:**
```json
{
  "id": "55667788",
  "timestamp": "2026-03-17T15:10:00.000Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "user.update",
  "resource_id": "charlie",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": ""
}
```

#### `user.delete` — Delete user

Emitted by DELETE `/api/v1/admin/users/{username}` when an admin removes a
user account.

| Field | Value |
|-------|-------|
| `resource_id` | Username of the deleted user |
| `result` | `ok` |

**Example:**
```json
{
  "id": "99aabbcc",
  "timestamp": "2026-03-17T15:20:00.000Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "user.delete",
  "resource_id": "charlie",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": ""
}
```

---

### API Key Events

#### `apikey.create` — Create API key

Emitted by POST `/api/v1/admin/apikeys` when an admin generates a new API key.

| Field | Value |
|-------|-------|
| `resource_id` | Generated key ID (not the secret itself; never logged) |
| `message` | Human-readable display name supplied by the caller |
| `result` | `ok` |

**Example:**
```json
{
  "id": "ddeeff00",
  "timestamp": "2026-03-17T16:00:00.000Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "apikey.create",
  "resource_id": "key-a1b2c3",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": "ci-pipeline-key"
}
```

#### `apikey.revoke` — Revoke API key

Emitted by DELETE `/api/v1/admin/apikeys/{id}` when an admin invalidates an
existing API key.

| Field | Value |
|-------|-------|
| `resource_id` | ID of the revoked key |
| `result` | `ok` |

**Example:**
```json
{
  "id": "11223355",
  "timestamp": "2026-03-17T16:30:00.000Z",
  "user_id": "alice",
  "role": "admin",
  "operation": "apikey.revoke",
  "resource_id": "key-a1b2c3",
  "remote_addr": "10.0.0.5:41234",
  "result": "ok",
  "message": ""
}
```

---

## Retrieving Audit Logs via the REST API

The audit log is served at `GET /api/v1/audit`. This endpoint requires an
`admin` role JWT (or admin API key) in the `Authorization: Bearer <token>`
header.

### Query Parameters

| Parameter   | Type    | Default | Description |
|-------------|---------|---------|-------------|
| `user`      | string  | _(all)_ | Filter entries to a specific `user_id`. |
| `operation` | string  | _(all)_ | Filter entries to a specific operation code (e.g. `auth.login`). |
| `limit`     | integer | `100`   | Maximum number of entries to return (most-recent first). |

### Retrieve the Last 100 Entries

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}' \
  | jq -r .token)

curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/audit | jq .
```

### Filter Failed Login Attempts

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?operation=auth.login&limit=200" \
  | jq '[.[] | select(.result == "fail")]'
```

### Filter Activity by a Specific User

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?user=alice&limit=50" | jq .
```

### Filter All User Management Events

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?operation=user.create" | jq .
```

---

## Consuming the Embedded System-Log Audit Line

Every call to `addAuditEntry` also emits a structured `INFO` line to the
server's system log (see [System Logging](system-logging.md)). The message is
`"audit"` and carries the same key fields:

```json
{
  "time":     "2026-03-17T14:05:32.001234567Z",
  "level":    "INFO",
  "msg":      "audit",
  "user":     "alice",
  "role":     "admin",
  "op":       "auth.login",
  "resource": "",
  "result":   "ok"
}
```

This dual-emission means the audit trail is available both through the REST API
and through any log-shipping pipeline attached to the server's stderr/stdout.

---

## Consumption Examples

### Command-Line (jq)

```bash
# Pretty-print all audit entries
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/audit | jq '.'

# Count events by operation type
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/audit?limit=1000 \
  | jq 'group_by(.operation) | map({op: .[0].operation, count: length})'

# List all unique source IPs that attempted to log in
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?operation=auth.login" \
  | jq '[.[].remote_addr | split(":")[0]] | unique'

# Get all failed events in the last 500 entries
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?limit=500" \
  | jq '[.[] | select(.result == "fail")]'

# Extract all API-key creations with their display names
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?operation=apikey.create" \
  | jq '[.[] | {id: .resource_id, name: .message, by: .user_id, at: .timestamp}]'
```

### Exporting to a File

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit?limit=10000" > audit-$(date +%F).json
```

### Parsing the System Log Stream with jq

When the server writes JSON logs to stderr, pipe the stream and filter for
audit entries in real time:

```bash
./pheromone-server 2>&1 | jq -c 'select(.msg == "audit")'
```

### Forwarding to a SIEM or Log Aggregator

The server writes JSON-structured log lines to **stderr** by default. These
can be forwarded to a SIEM (e.g. Splunk, Elasticsearch, Loki) using any
standard log shipper:

**Filebeat example** (`filebeat.yml` snippet):
```yaml
filebeat.inputs:
  - type: container
    paths:
      - /var/log/pheromone/*.log
    json.message_key: msg
    json.keys_under_root: true
    processors:
      - drop_event:
          when.not.equals:
            msg: audit

output.elasticsearch:
  hosts: ["https://elasticsearch:9200"]
  index: "pheromone-audit-%{+yyyy.MM.dd}"
```

**Vector example** (`vector.toml` snippet):
```toml
[sources.pheromone_stderr]
type = "file"
include = ["/var/log/pheromone/server.log"]

[transforms.audit_only]
type = "filter"
inputs = ["pheromone_stderr"]
condition = '.msg == "audit"'

[sinks.loki]
type = "loki"
inputs = ["audit_only"]
endpoint = "http://loki:3100"
labels.job = "pheromone-audit"
```

---

## Security Notes

- **Passwords are never recorded.** The `message` field contains only the
  display-name label for API keys; secret key material is never logged.
- **The audit log is append-only** within the process lifetime. It cannot be
  modified or deleted via the API.
- **Audit entries are in-memory only.** For persistent, tamper-resistant audit
  storage, attach an external log shipper to the server's JSON log stream and
  persist the `"msg": "audit"` lines to durable storage.
- The `GET /api/v1/audit` endpoint is restricted to the `admin` role. Access
  attempts by lower-privileged users return HTTP 403.

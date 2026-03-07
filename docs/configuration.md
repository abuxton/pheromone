# Pheromone Configuration Guide

Pheromone server and agent processes are configured through files placed in a
**configuration directory**. Multiple files are supported per directory; they are
loaded in lexicographic order and merged, allowing layered configuration patterns.

## Supported Formats

| Extension | Format |
|-----------|--------|
| `.json`   | JSON   |
| `.yaml`   | YAML   |
| `.yml`    | YAML   |

Any other file extension in the configuration directory is silently ignored.

## Default Configuration Paths

| Component | Default path                    | Override environment variable             |
|-----------|---------------------------------|-------------------------------------------|
| Server    | `/etc/pheromone/server`         | `PHEROMONE_SERVER_CONFIG_PATH`            |
| Agent     | `/etc/pheromone/agent`          | `PHEROMONE_AGENT_CONFIG_PATH`             |

The `--config-path` flag on any CLI command takes precedence over the environment variable.

## File Merging Semantics

All files matching `*.json`, `*.yaml`, or `*.yml` in the configuration directory are
loaded in **lexicographic (alphabetical) order** and merged into a single configuration:

- **Scalar fields** (strings, numbers, booleans) from a later file override earlier ones
  when the later file's value is non-zero.
- **Slice fields** (`twins`, `listeners`, `skills`) are **appended**: each file
  contributes its own entries to the combined list.

This enables layered patterns such as:

```
/etc/pheromone/server/
  00-base.yaml          # Core server settings
  10-twins.yaml         # Twin definitions
  20-listeners.yaml     # Listener endpoints
  99-local-override.yaml  # Machine-specific overrides
```

## Generating Default Configuration

Use the `config generate` subcommand to create well-formed starter files:

```bash
# Generate all server component configs as YAML
pheromone-server config generate --format yaml --component all

# Generate a single agent config as JSON to a custom path
pheromone-agent config generate --format json --component agent \
  --output /etc/pheromone/agent/agent.json

# Generate a specific component only
pheromone-server config generate --format yaml --component twin \
  --config-path /etc/pheromone/server
```

Available components:

| Binary               | Components                          |
|----------------------|-------------------------------------|
| `pheromone-server`   | `server`, `twin`, `listener`, `all` |
| `pheromone-agent`    | `agent`, `skill`, `all`             |

## Validating Configuration

Use the `config validate` subcommand to check all files in a directory:

```bash
pheromone-server config validate --config-path /etc/pheromone/server
pheromone-agent  config validate --config-path /etc/pheromone/agent
```

Validation checks include:

- Required fields are present
- Port numbers are in the valid range (1–65535)
- Log level is one of `debug`, `info`, `warn`, `error`
- TLS certificate and key are provided together (not individually)
- Twin IDs are unique within the merged configuration
- Twin `type` is `os` or `workload`
- Listener names are unique
- Listener `type` is `webhook`, `nats`, or `kafka`
- Agent `server_addr` is set
- Skill distribution mode is `server`, `remote`, or `local`
- `remote` distribution mode requires `remote_url`
- `local` distribution mode requires `local_dir`
- Skill names are unique

---

## Server Configuration Reference

### Full example (`server.yaml`)

```yaml
server:
  address: "0.0.0.0"        # Network interface to bind (default: 0.0.0.0)
  port: 50051                # gRPC listen port (default: 50051)
  config_path: /etc/pheromone/server  # Config directory (informational)
  log_level: info            # debug | info | warn | error (default: info)
  tls_cert_file: ""          # Path to TLS certificate (optional)
  tls_key_file:  ""          # Path to TLS private key (optional)
  heartbeat_timeout: 30s     # Agent heartbeat timeout (default: 30s)

twins:
  - id: web-server-01
    name: Web Server 01
    type: os                 # os | workload
    metadata:
      environment: production
      region: us-east-1
    agents_file: /etc/pheromone/twins/web-server-01/AGENTS.md  # Optional per-twin AI instructions

listeners:
  - name: alerts-webhook
    type: webhook            # webhook | nats | kafka
    address: ":8080"
    options:
      path: /events
      secret: ""             # Webhook signing secret
```

### `server` settings

| Field               | Type     | Default                     | Description                                       |
|---------------------|----------|-----------------------------|---------------------------------------------------|
| `address`           | string   | `"0.0.0.0"`                 | Network interface for the gRPC server             |
| `port`              | integer  | `50051`                     | TCP port for the gRPC server                      |
| `config_path`       | string   | `/etc/pheromone/server`     | Config directory path (informational)             |
| `log_level`         | string   | `"info"`                    | `debug`, `info`, `warn`, or `error`               |
| `tls_cert_file`     | string   | `""`                        | TLS certificate file (requires `tls_key_file`)    |
| `tls_key_file`      | string   | `""`                        | TLS private-key file (requires `tls_cert_file`)   |
| `heartbeat_timeout` | duration | `30s`                       | Time before a silent agent is marked stale        |

### `twins[]` entries

| Field         | Type              | Required | Description                                              |
|---------------|-------------------|----------|----------------------------------------------------------|
| `id`          | string            | ✅       | Unique identifier for this twin                          |
| `name`        | string            |          | Human-readable label                                     |
| `type`        | string            | ✅       | `os` or `workload`                                       |
| `metadata`    | map[string]string |          | Arbitrary key-value tags                                 |
| `agents_file` | string            |          | Path to an `AGENTS.md` file for per-twin AI instructions |

### `listeners[]` entries

| Field     | Type              | Required | Description                                      |
|-----------|-------------------|----------|--------------------------------------------------|
| `name`    | string            | ✅       | Unique label for this listener                   |
| `type`    | string            | ✅       | `webhook`, `nats`, or `kafka`                    |
| `address` | string            | ✅       | Bind address or broker URL                       |
| `options` | map[string]string |          | Listener-specific settings (topic, credentials…) |

---

## Agent Configuration Reference

### Full example (`agent.yaml`)

```yaml
agent:
  id: agent-01               # Unique agent identifier (auto-generated if empty)
  name: my-agent             # Human-readable label
  server_addr: "localhost:50051"  # Pheromone server gRPC address (required)
  config_path: /etc/pheromone/agent  # Config directory (informational)
  log_level: info            # debug | info | warn | error (default: info)
  tick_interval: 5s          # Reasoning loop cadence (default: 5s)
  tls_ca_cert_file: ""       # CA cert for mTLS verification (optional)

  distribution:
    mode: server             # server | remote | local (default: server)
    remote_url: ""           # Required when mode == remote
    local_dir:  ""           # Required when mode == local
    tls_verify: true         # Verify TLS certs for remote calls (default: true)
    timeout_seconds: 10      # HTTP timeout for remote skill fetches (default: 10)

skills:
  - name: digital-twin
    version: "1.0.0"
    options: {}
```

### `agent` settings

| Field             | Type     | Default                   | Description                                      |
|-------------------|----------|---------------------------|--------------------------------------------------|
| `id`              | string   | (auto-generated)          | Unique agent identifier                          |
| `name`            | string   | `""`                      | Human-readable agent label                       |
| `server_addr`     | string   | —                         | **Required.** gRPC address of the server         |
| `config_path`     | string   | `/etc/pheromone/agent`    | Config directory (informational)                 |
| `log_level`       | string   | `"info"`                  | `debug`, `info`, `warn`, or `error`              |
| `tick_interval`   | duration | `5s`                      | Reasoning loop cadence                           |
| `tls_ca_cert_file`| string   | `""`                      | CA certificate for server TLS verification       |

### `agent.distribution` settings

| Field            | Type    | Default    | Description                                         |
|------------------|---------|------------|-----------------------------------------------------|
| `mode`           | string  | `"server"` | `server`, `remote`, or `local`                      |
| `remote_url`     | string  | `""`       | Required when `mode == remote`; skill registry URL  |
| `local_dir`      | string  | `""`       | Required when `mode == local`; local skill spec dir |
| `tls_verify`     | boolean | `true`     | Verify TLS certs for remote HTTP calls              |
| `timeout_seconds`| integer | `10`       | HTTP timeout for remote skill registry requests     |

### `skills[]` entries

| Field     | Type              | Required | Description                                  |
|-----------|-------------------|----------|----------------------------------------------|
| `name`    | string            | ✅       | Skill identifier, e.g. `"digital-twin"`      |
| `version` | string            |          | Skill version; latest if omitted             |
| `options` | map[string]string |          | Skill-specific key-value initialisation opts |

---

## Per-Twin AGENTS.md Files

Each `TwinConfig` entry supports an optional `agents_file` field that points to an
[AGENTS.md](https://agents.md/) file. When set, the agent runtime loads this Markdown
file and uses it as a **contextual prompt** for the AI reasoning loop, enabling
per-twin behaviour customisation without code changes (see ADR-007).

Example per-twin AGENTS.md at `/etc/pheromone/twins/web-server-01/AGENTS.md`:

```markdown
# Twin: web-server-01

## Purpose
This twin manages the Nginx web server on the primary load-balancer node.

## Constraints
- Do not restart the Nginx service between 09:00 and 17:00 UTC on weekdays.
- Always validate config with `nginx -t` before applying changes.
- Alert on memory usage > 80%.

## Expected Skills
- digital-twin (read/write OS twin state)
- config-enforce (apply and validate Nginx configuration)
- metrics (report CPU, memory, and connection counts)
```

Place one AGENTS.md per twin directory and reference it from the server config:

```yaml
twins:
  - id: web-server-01
    type: os
    agents_file: /etc/pheromone/twins/web-server-01/AGENTS.md
```

---

## Makefile Targets

| Target                    | Description                                                       |
|---------------------------|-------------------------------------------------------------------|
| `make build`              | Build all packages and the `pheromone-server`/`pheromone-agent` binaries |
| `make config-generate-server` | Generate default server config files to `dist/config/server/`  |
| `make config-generate-agent`  | Generate default agent config files to `dist/config/agent/`    |
| `make config-validate-server` | Validate server config from the default or env-specified path  |
| `make config-validate-agent`  | Validate agent config from the default or env-specified path   |

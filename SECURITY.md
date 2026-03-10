# Security Policy

This document describes the security posture of Pheromone, covering the approach to
secure communications, access control, configuration hardening, and vulnerability
management.

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please **do not** open a
public GitHub issue. Instead:

1. Email the maintainers directly (see repository contacts).
2. Include a description of the vulnerability, reproduction steps, and potential
   impact.
3. Allow a reasonable time for the issue to be assessed and patched before public
   disclosure (coordinated disclosure).

We will acknowledge your report within 48 hours and provide a timeline for remediation.

---

## Security Approach

### 1. Secure Communications

All inter-component communication in Pheromone uses **gRPC over TLS** (see [ADR-003]
and [ADR-014]).

#### Transport Layer Security (TLS)

| Component | Protocol | Config Field |
|-----------|----------|--------------|
| Server gRPC listener | TLS 1.2+ | `server.tls_cert_file`, `server.tls_key_file` |
| Agent → Server | TLS client auth | `agent.tls_ca_cert_file` |
| HTTP skill registry | HTTPS | `agent.distribution.tls_verify` (default: `true`) |

**Current state**: TLS fields are present in configuration but gRPC server/client
TLS wiring is a Phase 2 implementation task (see ADR-014). Until that work is
complete, deployments **must** be run in a network-isolated environment (e.g.,
private VPC, VPN, or loopback-only interface).

**Planned**: Mutual TLS (mTLS) for agent-to-server authentication so that both
sides verify each other's identity using X.509 certificates.

#### Webhook Signatures

Outgoing webhook notifications are signed with **HMAC-SHA256** using a per-listener
shared secret configured under `listeners[*].options.secret`. Signature is attached
as the `X-Pheromone-Signature` HTTP header (format: `sha256=<hex>`).

Constant-time comparison (`hmac.Equal`) is used when verifying signatures to
prevent timing attacks. See `internal/notification/hmac.go`.

#### etcd

The embedded etcd datastore accepts connections on port `2379`. In production
deployments:

- Restrict access to `127.0.0.1` or a private network interface.
- Enable etcd peer and client TLS via the standard etcd configuration.
- Use etcd RBAC to restrict pheromone's service account to the minimum required
  key prefix.

---

### 2. Access Control (ACLs)

Pheromone implements a **twin-level hierarchical access control model** for skills.
See `internal/skill/access.go` and [ADR-007].

#### Twin Levels

| Level | Description |
|-------|-------------|
| `OS` | Manages operating-system-level resources (files, processes, kernel settings) |
| `Workload` | Manages application-level resources (services, containers, config) |

#### Policy Rules

An `AccessPolicy` grants or denies a skill execution based on the agent's twin
ownership:

1. **Explicit grant** — an `agentID` may be explicitly whitelisted in the policy.
2. **OS-level grant** — an agent that manages an OS-level twin is granted access to
   all skills within that twin's scope.
3. **Workload-open grant** — if a skill is marked `WorkloadOpen`, any agent managing
   a workload twin in the same hierarchy is granted access.
4. **Default deny** — all other access attempts are rejected with a descriptive error.

#### Network-level Authentication

**Current gap**: gRPC services (AgentRegistry, TwinControl, TelemetryStream,
ActionEventService) do not yet implement network-level authentication interceptors.
This is tracked and planned in [ADR-014].

**Mitigation**: Until gRPC auth interceptors are implemented, restrict server
reachability at the network layer (firewall rules, security groups) so that only
authorised agents can reach the control-plane port (`4426`) and telemetry port
(`4427`).

---

### 3. Configuration Security

#### File Permissions

Generated configuration files are written with **mode 0600** (owner read/write only)
and their parent directories with **mode 0750**. This protects secrets stored in
configuration (webhook HMAC secrets, TLS paths, API tokens) from being read by
other local users.

Verify permissions after deployment:

```bash
ls -la /etc/pheromone/
# drwxr-x--- root pheromone  -> 0750
# -rw------- root pheromone server.yaml -> 0600
```

#### Sensitive Fields

The following configuration fields contain or reference secrets and must be protected:

| Field | Description |
|-------|-------------|
| `listeners[*].options.secret` | HMAC secret for webhook signature verification |
| `server.tls_cert_file` / `server.tls_key_file` | Paths to TLS certificate and key |
| `agent.tls_ca_cert_file` | Path to CA certificate bundle for agent TLS |

**Recommendation**: Store secrets via environment variable injection or a secrets
manager (e.g., HashiCorp Vault) rather than embedding them directly in config files.

#### Config Validation

`ValidateServerConfig` and `ValidateAgentConfig` enforce:

- Valid port ranges (1–65535)
- TLS cert and key must both be provided together or both empty
- Recognised log levels, twin types, listener types, and distribution modes
- No duplicate twin IDs, listener names, or skill names

---

### 4. Skill Execution Security

Skills execute inside the agent process and inherit its privileges.

**Current model (Phase 1)**: Only **trusted, built-in skills** (`digital_twin`,
`metrics`, `config_enforce`) are loaded. Third-party skills from the remote registry
require operator explicit opt-in.

**Planned (Phase 2)**: Sandboxed skill execution using Linux namespaces, cgroups,
and seccomp profiles to limit resource access and syscall surface.

**Operator guidance**:

- Do not load skills from untrusted registries without reviewing their source.
- Set `agent.distribution.tls_verify: true` (default) to verify the HTTPS identity
  of the remote skill registry.
- Restrict agent process privileges (run as a dedicated unprivileged user, use
  `CAP_NET_ADMIN` only if required by skills).

---

### 5. Logging and Observability

- Logs are structured JSON emitted to stdout (compatible with log aggregators).
- **Do not log secrets**: webhook secrets, TLS private key contents, and bearer
  tokens in notification destination headers must never appear in logs.
- The AI reasoning traces (`internal/skill/reasoner.go`) may contain system
  configuration details; treat logs as sensitive data.
- Audit-log all configuration changes at `warn` level or above.

---

## Vulnerability Scanning

### Automated Scanning (CI)

Every push and pull request runs the **Security Scan** GitHub Actions workflow
(`.github/workflows/security-scan.yml`), which performs:

| Tool | Purpose |
|------|---------|
| `govulncheck` | Detects Go dependencies with known CVEs (golang.org/x/vuln) |
| `gosec` | Go SAST: finds insecure code patterns (hardcoded secrets, weak crypto, etc.) |
| `actions/dependency-review-action` | Blocks PRs that introduce high-severity dependency vulnerabilities |

SARIF results from `gosec` are uploaded to GitHub Advanced Security for triage in
the **Security** tab.

A **scheduled weekly run** (Monday 08:00 UTC) catches newly published CVEs in
existing dependencies.

### Manual Scanning

Run security scans locally:

```bash
# Check Go dependencies for known CVEs
make govulncheck

# Run SAST (gosec)
make gosec

# Run both
make security-scan
```

### Dependency Verification

```bash
# Verify module checksums have not been tampered with
go mod verify
```

---

## Known Gaps and Mitigations

| Gap | Severity | Mitigation | Tracking |
|-----|----------|------------|---------|
| gRPC services lack authentication interceptors | Critical | Network isolation (firewall, VPN) | ADR-014 |
| TLS not enforced on gRPC server by default | Critical | Network isolation; set TLS fields in config | ADR-014 |
| Skill execution not sandboxed (Phase 1) | High | Use only built-in/trusted skills | Phase 2 roadmap |
| Secrets stored in plaintext config | Medium | 0600 file permissions; use secrets manager | Documented above |
| No per-destination webhook secrets | Low | Use strong shared secret; rotate regularly | Future enhancement |

---

## References

- [ADR-003: gRPC Contracts](docs/adr/adr-003-grpc-contracts.md)
- [ADR-007: Agentic AI Agent Model](docs/adr/adr-007-agentic-ai-agent-model.md)
- [ADR-011: Port Assignment](docs/adr/adr-011-port-assignment.md)
- [ADR-014: Security Architecture](docs/adr/adr-014-security-approach.md)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Security Best Practices](https://go.dev/doc/security)

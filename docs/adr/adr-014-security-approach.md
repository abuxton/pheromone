# ADR-014: Security Architecture

**Status**: Accepted  
**Date**: 2026-03-10  
**Author**: @copilot  

---

## Context

The Pheromone platform manages operating-system and workload configurations through
AI-capable agents communicating over gRPC. As the platform moves toward production
use, three classes of security concerns must be addressed:

1. **Transport security** — inter-component communication (agent↔server, server↔etcd)
   currently has optional TLS with no enforcement policy.
2. **Authentication & authorisation** — gRPC services accept connections from any
   client without verifying identity; access control exists only at the in-process
   skill layer.
3. **Vulnerability management** — no automated tooling exists to detect dependency
   CVEs or insecure code patterns.

A concurrent issue (#security-review) asked for:
- Documented security approach for communications, configuration, and access control
- Source-code security review with documented fixes or mitigations
- Automated vulnerability scanning as part of the CI/CD pipeline

---

## Decision

### D1 — Mandatory TLS for gRPC (Phase 2 implementation)

gRPC server and agent client **must** present and verify TLS certificates in all
production deployments. Mutual TLS (mTLS) is the target state:

- **Server**: loads cert+key from `server.tls_cert_file` / `server.tls_key_file`
  and passes `grpc.Creds(...)` to `grpc.NewServer` using `credentials.NewTLS(tlsConfig)`:
  ```go
  cert, err := tls.LoadX509KeyPair(certFile, keyFile)
  creds := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}})
  grpc.NewServer(grpc.Creds(creds))
  ```
- **Agent**: loads CA bundle from `agent.tls_ca_cert_file` and uses
  `credentials.NewClientTLSFromFile(...)` for the dial connection.

Until the gRPC wiring is implemented, the configuration fields exist but are not
applied at the transport layer. Operators **must** compensate with network isolation
(VPN, private subnet, firewall rules that allow only authorised agent IPs on ports
4426/4427).

### D2 — gRPC Authentication Interceptors (Phase 2 implementation)

Unary and streaming interceptors will validate the client identity on every RPC:

```
server interceptor chain:
  1. TLS peer certificate extraction  →  agent certificate CN / SAN
  2. Token / client-cert agent-ID binding
  3. AccessPolicy.Check()  →  per-skill authorisation
  4. Audit log entry (agent-ID, RPC method, allowed/denied)
```

Return `codes.Unauthenticated` for missing or invalid credentials;
`codes.PermissionDenied` for authenticated but unauthorised requests.

### D3 — Config File Permissions Hardened (implemented)

`writeConfig()` in `internal/config/generator.go` now writes files with mode
**0600** (owner-only read/write) and creates parent directories with mode **0750**.
This prevents other local users from reading webhook HMAC secrets and TLS key paths
stored in configuration.

### D4 — Vulnerability Scanning in CI (implemented)

A dedicated **Security Scan** workflow (`.github/workflows/security-scan.yml`) runs:

| Tool | Trigger | Purpose |
|------|---------|---------|
| `govulncheck` | Every push & PR | Known CVEs in Go module graph |
| `gosec` | Every push & PR | SAST: insecure code patterns |
| `dependency-review-action` | PRs only | Block high-severity new dependencies |

A scheduled weekly run (Monday 08:00 UTC) catches newly published CVEs in
existing dependencies.

SARIF output from `gosec` is uploaded to GitHub Advanced Security for triage in
the repository **Security → Code scanning alerts** tab.

Makefile targets `make govulncheck`, `make gosec`, and `make security-scan` allow
developers to run the same checks locally.

### D5 — SECURITY.md Created (implemented)

`SECURITY.md` at the repository root documents:

- Vulnerability disclosure process
- Secure communications approach (TLS, mTLS roadmap, webhook HMAC)
- Access control model (twin-level ACLs, gap: no network auth yet)
- Configuration security (file permissions, sensitive fields)
- Skill execution security (trusted-only model, sandbox roadmap)
- Logging guidelines (no secrets in logs)
- Vulnerability scanning procedures (CI + manual)
- Known gaps with severities and mitigations

---

## Consequences

### Positive

- Operators and contributors have a clear, documented security posture.
- Automated scanning catches dependency CVEs and insecure patterns before merge.
- Config file permission hardening (0600/0750) prevents local privilege escalation
  via secrets in config.
- SARIF upload enables GitHub Advanced Security dashboard for ongoing triage.
- The gap list is explicit — known risks are acknowledged and mitigated rather than
  silently ignored.

### Negative / Trade-offs

- `gosec` will flag `InsecureSkipVerify` in `internal/skill/spec.go`; this is an
  intentional developer opt-in (guarded by config) and should be suppressed with a
  `#nosec G402` annotation once reviewed.
- The permission change (0644→0600) may break deployments that rely on config files
  being world-readable (e.g., reading config as a non-owner service user). Operators
  must ensure the service user owns the config directory.

### Open Items (Phase 2)

- Implement gRPC server TLS wiring (D1)
- Implement gRPC authentication + authorisation interceptors (D2)
- Add per-destination webhook secrets
- Implement skill execution sandboxing (namespaces + seccomp)
- Support secret injection from environment variables or Vault

---

## References

- [ADR-003: gRPC Contracts](adr-003-grpc-contracts.md)
- [ADR-007: Agentic AI Agent Model](adr-007-agentic-ai-agent-model.md)
- [ADR-011: Port Assignment](adr-011-port-assignment.md)
- [SECURITY.md](../../SECURITY.md)
- [gosec](https://github.com/securego/gosec)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)
- [GitHub Actions: dependency-review-action](https://github.com/actions/dependency-review-action)
- [Go TLS credentials](https://pkg.go.dev/google.golang.org/grpc/credentials)

# Implementation Plan: User Access Control & Identity Provider Integration

**Branch**: `001-user-access-control` | **Date**: 2026-03-24 | **Spec**: [spec.md](../specs/001-user-access-control/spec.md)
**ADR**: [ADR-015](../docs/adr/adr-015-user-access-control-identity-provider.md)
**Input**: Feature specification from `specs/001-user-access-control/spec.md`

---

## Summary

Add a complete application-layer identity and access control system to the Pheromone management
server.  Phase 1 delivers local username/password authentication (bcrypt, etcd-backed), a
four-role RBAC model aligned with ADR-006 twin-level ACLs, RS256-signed JWT session tokens,
a gRPC interceptor chain extending ADR-014 D2, and structured audit logging.  Phase 2 extends
the `AuthProvider` plugin interface with LDAP, OIDC, and SAML 2.0 adapters.  Phase 3 adds
token revocation, SCIM provisioning, dynamic OPA-based RBAC, and multi-tenant namespace scoping.

---

## Technical Context

**Language/Version**: Go 1.22+ (primary), Protocol Buffers v3
**Primary Dependencies**:
  - `golang-jwt/jwt` v5 — JWT signing/validation (RS256 + HS256)
  - `golang.org/x/crypto/bcrypt` — password hashing (stdlib-adjacent, part of `x/crypto`)
  - `go-ldap/ldap` v3 — LDAP adapter (Phase 2)
  - `coreos/go-oidc` + `golang.org/x/oauth2` — OIDC adapter (Phase 2)
  - `crewjam/saml` — SAML 2.0 SP adapter (Phase 2)
  - `go.etcd.io/etcd/client/v3` — already in use (ADR-002)
  - `go.etcd.io/bbolt` — embedded DB fallback (already transitive dep via etcd)
**Storage**: etcd keyspace `/pheromone/users/*`; bbolt fallback (`data/pheromone.db`)
**Testing**: `go test ./... -race`; coverage target ≥ 80% for `internal/auth/`
**Target Platform**: Linux server (amd64, arm64); Vagrant multi-machine test environment (ADR-012)
**Project Type**: Server-side Go module; extends existing `cmd/server` and `internal/` layout
**Performance Goals**:
  - bcrypt login latency: < 500 ms p95 (cost=12 on target hardware)
  - JWT validation latency: < 1 ms (in-memory RSA key; no I/O in hot path)
  - Zero overhead on agent mTLS flows (interceptor short-circuits at Step 2)
**Constraints**:
  - Must not break existing gRPC agent contracts (ADR-003 backward compatibility)
  - Must not require external services in Phase 1 (air-gapped deployment)
  - Token TTL configurable but bounded: 5 min ≤ TTL ≤ 30 days
**Scale/Scope**: Supports ~100–1000 concurrent operators (not agents); human login rate far below
  bcrypt throughput ceiling

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Principle | Requirement | Status | Notes |
|-----------|-------------|--------|-------|
| **I — Layered Twin Architecture** | Auth fits management layer; agents unaffected | ✅ PASS | Interceptor distinguishes human vs. agent identity; ADR-006 skill ACLs preserved |
| **II — ADR-Driven / Observability** | ADR filed before implementation | ✅ PASS | ADR-015 created; all auth events emit structured log entries |
| **III — Language-Agnostic Protocols** | New `AuthService` uses gRPC/Protobuf | ✅ PASS | `proto/pheromone/v1/auth.proto` follows ADR-003 patterns |
| **IV — Smoke Tests / Quality Gates** | ≥ 70% coverage; integration tests required | ✅ PASS | `internal/auth/` targets 80%; interceptor chain smoke tests planned |
| **V — Git-Driven / ADR Workflow** | Branch name references ADR; squash commit format | ✅ PASS | Branch `001-user-access-control` created; PR will reference ADR-015 |

**Gate result**: ✅ All five principles satisfied — proceed to Phase 0.

---

## Project Structure

### Documentation (this feature)

```text
specs/001-user-access-control/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   ├── auth.proto       # AuthService gRPC contract
│   └── openapi.yaml     # REST endpoints (SAML ACS, OIDC callback, JWKS)
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created here)
```

### Source Code (repository root)

```text
proto/pheromone/v1/
└── auth.proto                        # AuthService + message definitions

internal/auth/
├── provider.go                       # AuthProvider interface + UserAttributes
├── store.go                          # UserRecord + etcd/bbolt CRUD
├── jwt.go                            # Token issuance + validation (RS256/HS256)
├── rbac.go                           # Role constants + RBACPolicy.Check()
├── interceptor.go                    # Unary + streaming gRPC interceptors
├── ratelimit.go                      # Login rate limiter (token bucket, per-IP)
├── local/
│   └── local.go                      # Local bcrypt AuthProvider
├── ldap/
│   └── ldap.go                       # LDAP AuthProvider (Phase 2)
├── oidc/
│   └── oidc.go                       # OIDC/OAuth2 AuthProvider (Phase 2)
├── saml/
│   └── saml.go                       # SAML 2.0 SP AuthProvider (Phase 2)
└── audit.go                          # Structured auth event logger

internal/auth/testdata/
├── rsa_test_key.pem
└── rsa_test_key_pub.pem

cmd/server/
└── main.go                           # Wire auth interceptors into grpc.NewServer()

cmd/cli/
└── user.go                           # `pheromone user` subcommands

tests/
├── integration/
│   └── auth_interceptor_test.go      # End-to-end interceptor smoke tests
└── unit/
    └── auth/                         # Mirrors internal/auth/ unit tests
```

**Structure Decision**: Single Go project layout, extending the existing `internal/` and `cmd/`
structure.  New `internal/auth/` package isolates all identity logic; adapters live in named
sub-packages to enable conditional compilation or separate review.

---

## Phase 0: Research

> **Status**: Complete — findings consolidated in `research.md`

### Research Questions Resolved

**R1 — JWT library selection (Go)**
- Decision: `golang-jwt/jwt` v5
- Rationale: Actively maintained fork of the widely-used `dgrijalva/jwt-go`; supports RS256, HS256,
  RS384, and ES256; no breaking API changes from v4 to v5; used by etcd itself.
- Alternatives: `lestrrat-go/jwx` (more complete JOSE; heavier dependency tree), `go-jose/go-jose`
  (comprehensive but complex API for Phase 1 needs).

**R2 — bcrypt cost factor for target hardware**
- Decision: cost=12 (default); configurable down to 10 for resource-constrained deployments.
- Rationale: bcrypt(cost=12) runs ~250–400 ms on modern amd64; ~600–900 ms on arm64 (Raspberry Pi 4).
  For interactive human login, sub-1-second latency is acceptable.  Cost=10 is the minimum that
  remains resistant to GPU attacks with 2026 hardware.
- Reference: OWASP Authentication Cheat Sheet recommends cost ≥ 10 (prefer 12+).

**R3 — etcd key layout for user records**
- Decision: `/pheromone/users/<username>` → JSON-encoded `UserRecord`
- Rationale: Consistent with existing `/pheromone/twins/*` pattern (ADR-002).  Prefix watch
  enables change notification for Phase 3 multi-server token revocation.
- bbolt fallback: bucket `users`, key = username bytes, value = same JSON encoding.

**R4 — RS256 vs HS256 default**
- Decision: RS256 default; HS256 opt-in.
- Rationale: RS256 (asymmetric) allows the public key to be served at `/auth/jwks` for
  third-party verification and OIDC federation without sharing the signing secret.  HS256 retained
  as an option for single-node deployments that cannot manage an RSA key pair.

**R5 — LDAP adapter library**
- Decision: `go-ldap/ldap` v3 (Phase 2).
- Rationale: The de-facto standard Go LDAP client; supports STARTTLS, paging, and group search.
  MIT licence; no CVEs in current advisory database as of research date.

**R6 — OIDC adapter library**
- Decision: `coreos/go-oidc` v3 + `golang.org/x/oauth2` (Phase 2).
- Rationale: `go-oidc` wraps the OIDC Discovery document and JWKS rotation automatically.
  Used by Kubernetes, Dex, and other CNCF projects; well-understood operational model.

**R7 — SAML 2.0 adapter library**
- Decision: `crewjam/saml` (Phase 2).
- Rationale: Pure-Go SP implementation; handles SP metadata generation, ACS assertion parsing,
  and attribute mapping.  Apache-2.0 licence.  Evaluate against `russellhaering/gosaml2` before
  Phase 2 starts (latter is more actively maintained as of 2026).

**R8 — OPA for dynamic RBAC (Phase 3)**
- Decision: Evaluate `open-policy-agent/opa` embedded Go library.
- Rationale: Replaces static role table with Rego policies evaluated at RPC time.  Enables
  attribute-based decisions (e.g., "operator role, but only on twin namespace matching user's
  team label").  Deferred to Phase 3; Phase 1 uses a simple lookup table.

---

## Phase 1: Design & Contracts

> **Status**: Complete — see `data-model.md`, `contracts/`, and `quickstart.md`

### Data Model

See `data-model.md` for full entity definitions.  Summary:

| Entity | Storage | Key fields |
|--------|---------|------------|
| `UserRecord` | etcd `/pheromone/users/<username>` | username, password_hash, role, disabled, created_at |
| `JWTConfig` | server config file | algorithm, private_key_file, secret_file, ttl |
| `AuthProviderConfig` | server config file | provider type + adapter-specific fields |
| `RBACPolicy` | in-memory (static table) | method → minimum required role |
| `AuditEvent` | structured log (stdout/file) | ts, event, sub, role, rpc, jti, source_ip |

### API Contracts

See `contracts/auth.proto` for full Protocol Buffer definitions.  Key service:

```protobuf
service AuthService {
  rpc Login(LoginRequest) returns (LoginResponse);              // exempt from auth interceptor
  rpc Logout(LogoutRequest) returns (LogoutResponse);           // invalidates client-side (Phase 1); deny-list (Phase 3)
  rpc ChangePassword(ChangePasswordRequest) returns (ChangePasswordResponse);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);  // requires: admin
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse); // requires: admin (operator/viewer), owner (admin)
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse); // requires: admin or self (password only)
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse); // requires: admin (operator/viewer), owner (admin)
}
```

HTTP endpoints (Phase 2, for SAML/OIDC callbacks):
- `GET /auth/jwks` — JWKS public key document (Phase 1, for future federation)
- `GET /auth/saml/metadata` — SP metadata (Phase 2)
- `POST /auth/saml/acs` — Assertion Consumer Service (Phase 2)
- `GET /auth/oidc/callback` — OIDC Authorization Code callback (Phase 2)

### RBAC Permission Table

| gRPC Method | viewer | operator | admin | owner |
|-------------|:------:|:--------:|:-----:|:-----:|
| `AuthService.Login` | exempt | exempt | exempt | exempt |
| `AuthService.ChangePassword` (self) | ✅ | ✅ | ✅ | ✅ |
| `AuthService.ListUsers` | ❌ | ❌ | ✅ | ✅ |
| `AuthService.CreateUser` (operator/viewer) | ❌ | ❌ | ✅ | ✅ |
| `AuthService.CreateUser` (admin) | ❌ | ❌ | ❌ | ✅ |
| `AuthService.DeleteUser` (operator/viewer) | ❌ | ❌ | ✅ | ✅ |
| `AuthService.DeleteUser` (admin) | ❌ | ❌ | ❌ | ✅ |
| `TwinControl.*` (read) | ✅ | ✅ | ✅ | ✅ |
| `TwinControl.*` (enforce/write) | ❌ | ✅ | ✅ | ✅ |
| `SkillService.DeleteSkill` | ❌ | ❌ | ✅ | ✅ |
| `AgentRegistry.*` | agent-only | agent-only | agent-only | agent-only |

---

## Implementation Phases

### Phase 1 — Local Auth MVP

**Scope**: D1 (local store) + D2 (RBAC) + D3 (JWT) + D5 (interceptor) + D6 (audit log)
**Target**: Single PR; all smoke tests green; ADR-015 Status → Accepted after review

#### Task Groups

**T1.1 — Proto & Code Generation**
- [ ] Define `proto/pheromone/v1/auth.proto` with `AuthService` messages
- [ ] Add `buf generate` target; regenerate Go stubs
- [ ] Verify backward compatibility of existing `.proto` files (no breaking changes)

**T1.2 — Data Layer (`internal/auth/store.go`)**
- [ ] Implement `UserRecord` struct + JSON serialisation
- [ ] etcd CRUD: `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `ListUsers`
- [ ] bbolt fallback: same interface, embedded database target `data/pheromone.db`
- [ ] First-run bootstrap: seed `owner` account if no users exist
- [ ] Unit tests ≥ 80% line coverage

**T1.3 — JWT Package (`internal/auth/jwt.go`)**
- [ ] Implement `Issuer`: `IssueToken(user UserRecord) (string, error)`
- [ ] Implement `Validator`: `ValidateToken(token string) (*Claims, error)`
- [ ] RS256 key loading from PEM file + in-memory cache
- [ ] HS256 fallback (config `algorithm: hs256`)
- [ ] TTL enforcement + `exp` claim validation
- [ ] Unit tests: valid token, expired token, wrong algorithm, tampered signature

**T1.4 — RBAC Policy (`internal/auth/rbac.go`)**
- [ ] Define `Role` type + hierarchy constants (viewer < operator < admin < owner)
- [ ] `RBACPolicy` with method-to-minimum-role map (see permission table above)
- [ ] `Check(identity Identity, method string) error`
- [ ] Special handling for agent identity (bypass human RBAC)
- [ ] Unit tests: all four roles against representative RPC methods

**T1.5 — gRPC Interceptors (`internal/auth/interceptor.go`)**
- [ ] `AuthUnaryInterceptor(validator, rbac)` — unary RPC gate
- [ ] `AuthStreamInterceptor(validator, rbac)` — streaming RPC gate
- [ ] JWT Bearer extraction from gRPC metadata
- [ ] mTLS agent-cert detection path (sets synthetic "agent" identity)
- [ ] Exempt RPC list (Login, Register, Heartbeat, health-check)
- [ ] Propagate `Identity` through context for downstream ACL use
- [ ] Unit tests: authenticated, unauthenticated, permission denied, agent bypass

**T1.6 — Auth Service Handler (`internal/auth/service.go`)**
- [ ] `Login`: verify credentials (bcrypt), issue JWT, emit `auth.login_*` events
- [ ] `Logout`: Phase 1 = no-op server-side; emit `auth.logout` event
- [ ] `ChangePassword`: verify current password; update hash; emit audit event
- [ ] `ListUsers`, `CreateUser`, `UpdateUser`, `DeleteUser`: CRUD with RBAC pre-check
- [ ] Rate limiter (T1.7) applied to `Login` handler

**T1.7 — Rate Limiter (`internal/auth/ratelimit.go`)**
- [ ] Token-bucket rate limiter: 10 requests/minute per source IP
- [ ] `grpc.peer` metadata used to extract source IP
- [ ] Emit `auth.login_locked` on rate limit exceeded
- [ ] Unit tests: within limit, at limit, exceeded

**T1.8 — Audit Logger (`internal/auth/audit.go`)**
- [ ] `AuditLogger` wrapping existing `logrus` logger
- [ ] Emit all events in taxonomy table (D6) with correct fields
- [ ] Never log raw passwords or token strings
- [ ] Unit tests: verify field presence/absence per event type

**T1.9 — Server Wiring (`cmd/server/main.go`)**
- [ ] Load `AuthProvider` (local by default) from config
- [ ] Instantiate `JWTValidator`, `RBACPolicy`, `AuditLogger`
- [ ] Register `AuthService` gRPC handler
- [ ] Add `AuthUnaryInterceptor` + `AuthStreamInterceptor` to server chain
- [ ] Add `GET /auth/jwks` HTTP endpoint (RS256 public key)
- [ ] Integration test: full interceptor chain with valid/invalid/missing token

**T1.10 — CLI (`cmd/cli/user.go`)**
- [ ] `pheromone user create <username> --role <role>`
- [ ] `pheromone user delete <username>`
- [ ] `pheromone user list`
- [ ] `pheromone user set-password <username>` (interactive prompt)
- [ ] `pheromone user set-role <username> --role <role>`
- [ ] Unit tests for CLI flag parsing

**T1.11 — Configuration Schema**
- [ ] Add `auth` section to server config YAML schema:
  ```yaml
  auth:
    provider: local          # local | ldap | oidc | saml
    jwt_algorithm: rs256     # rs256 | hs256
    jwt_private_key_file: /etc/pheromone/jwt.key
    jwt_public_key_file: /etc/pheromone/jwt.pub
    # jwt_secret_file: /etc/pheromone/jwt.secret  # hs256 only
    token_ttl: 8h
    bcrypt_cost: 12
    rate_limit_logins_per_minute: 10
  ```
- [ ] Validate config at startup; fail fast with descriptive error on missing key files
- [ ] Ensure key files created with mode 0600 (extends ADR-014 D3)

**T1.12 — Documentation & Testing**
- [ ] Update `SECURITY.md` to reflect Phase 1 auth implementation (close "gap" item)
- [ ] Add `docs/adr/adr-015-user-access-control-identity-provider.md` → already created ✅
- [ ] Update `docs/adr/INDEX.md` with ADR-015 entry
- [ ] Write `specs/001-user-access-control/quickstart.md` (operator bootstrap guide)
- [ ] Integration tests in `tests/integration/auth_interceptor_test.go`:
  - Valid JWT → `TwinControl` RPC succeeds
  - Missing JWT → `codes.Unauthenticated`
  - Expired JWT → `codes.Unauthenticated`
  - `viewer` role → `SkillService.DeleteSkill` → `codes.PermissionDenied`
  - Agent mTLS path → unaffected

---

### Phase 2 — External IdP Integration

**Scope**: D4 (AuthProvider adapters)
**Target**: Three separate PRs (LDAP, OIDC, SAML); each independently releasable

#### Phase 2a — LDAP Adapter

- [ ] Implement `internal/auth/ldap/ldap.go` satisfying `AuthProvider` interface
- [ ] LDAP bind authentication (user DN + password)
- [ ] Group search + group→role mapping from config
- [ ] `ldaps://` and STARTTLS transport; block plain `ldap://` by default
- [ ] Local shadow `UserRecord` created/updated on first login
- [ ] `auth.idp_login_success` / `auth.idp_login_failure` audit events
- [ ] Configuration documentation with AD and OpenLDAP examples
- [ ] Integration test against containerised OpenLDAP instance

#### Phase 2b — OIDC/OAuth 2.0 Adapter

- [ ] Implement `internal/auth/oidc/oidc.go`
- [ ] OIDC Discovery document fetch + JWKS rotation
- [ ] Authorization Code Flow with PKCE + `state` CSRF parameter
- [ ] `GET /auth/oidc/callback` HTTP endpoint (align with ADR-011 HTTP listener)
- [ ] `userinfo` endpoint claims → `UserAttributes.Groups`
- [ ] Configuration examples: Google, Azure AD, Keycloak, Okta
- [ ] Integration test against containerised Keycloak

#### Phase 2c — SAML 2.0 Adapter

- [ ] Implement `internal/auth/saml/saml.go`
- [ ] SP metadata endpoint: `GET /auth/saml/metadata`
- [ ] Assertion Consumer Service: `POST /auth/saml/acs`
- [ ] Attribute mapping from SAML assertion to `UserAttributes`
- [ ] Configuration examples: Okta, ADFS
- [ ] Integration test with `crewjam/saml` test IdP

---

### Phase 3 — Advanced IAM

**Scope**: Token revocation, SCIM, OPA RBAC, multi-tenant namespacing
**Target**: Separate feature branch (post-Phase 2)

- [ ] JWT deny-list in etcd (`/pheromone/auth/revoked/<jti>`) with TTL sweep
- [ ] Sliding refresh token (new JWT issued within last 25% of TTL window)
- [ ] SCIM 2.0 user/group provisioning endpoint
- [ ] OPA/Rego policy engine replacing static RBAC table
- [ ] Multi-tenant namespace scoping: `UserRecord.TwinNamespaceGlobs []string`
- [ ] Dedicated audit trail store with configurable retention
- [ ] Password expiry + forced rotation policy

---

## Acceptance Criteria

### Phase 1 MVP

| ID | Criterion | Verification |
|----|-----------|-------------|
| AC-001 | Unauthenticated gRPC client receives `codes.Unauthenticated` | Integration test T1.9 |
| AC-002 | Valid JWT bearer token grants access to permitted RPCs | Integration test T1.9 |
| AC-003 | `viewer` role cannot call write RPCs; receives `codes.PermissionDenied` | Integration test T1.9 |
| AC-004 | Agent mTLS flows unaffected by auth interceptor | Integration test T1.9 |
| AC-005 | `auth.login_success` / `auth.login_failure` logged on each attempt | Unit test T1.8 |
| AC-006 | `auth.access_denied` logged on each RBAC/ACL rejection | Unit test T1.8 |
| AC-007 | Passwords never appear in log output | Unit test T1.8 |
| AC-008 | JWT expired after configured TTL; re-login required | Unit test T1.3 |
| AC-009 | First-run bootstrap creates `owner` account | Unit test T1.2 |
| AC-010 | `pheromone user create/list/delete` CLI commands work end-to-end | CLI unit test T1.10 |
| AC-011 | `internal/auth/` unit test coverage ≥ 80% | `go test -coverprofile` |
| AC-012 | `govulncheck` clean on new dependencies | CI security-scan workflow |
| AC-013 | `gosec` clean on `internal/auth/` (no new findings) | CI security-scan workflow |

---

## Dependency & Risk Register

| ID | Risk | Probability | Impact | Mitigation |
|----|------|-------------|--------|-----------|
| R-001 | bcrypt latency too high on arm64 (Raspberry Pi) | Medium | Medium | Spike before Phase 1 merge; cost=10 config option available |
| R-002 | etcd compaction deletes user records | Low | High | Set etcd lease TTL = none for user records; add integration test |
| R-003 | JWT RSA key file not present at startup | Medium | High | Fail fast with descriptive error; `pheromone key generate` CLI command |
| R-004 | Stolen JWT valid until expiry (no revocation in Phase 1) | Low | Medium | Short default TTL (8h); TLS in transit; Phase 3 revocation mitigates fully |
| R-005 | Phase 2 SAML/OIDC needs HTTP listener; conflicts with existing port layout | Low | Medium | Align with ADR-011 HTTP notification listener; same listener, new routes |
| R-006 | `crewjam/saml` maintenance status (Phase 2) | Medium | Low | Evaluate `russellhaering/gosaml2` at Phase 2 start; decision in Phase 2 PR |

---

## References

- **ADR-015**: `docs/adr/adr-015-user-access-control-identity-provider.md`
- **Feature Spec**: `specs/001-user-access-control/spec.md`
- **Memory Spec**: `.specify/memory/spec-002-user-access-control.md`
- **ADR-002**: Server Architecture (etcd state store)
- **ADR-003**: gRPC Service Contracts
- **ADR-006**: Agent Lifecycle & Skill Framework (twin-level ACLs)
- **ADR-007**: Agentic AI Agent Model
- **ADR-014**: Security Architecture (TLS, interceptors, vulnerability scanning)
- **Constitution**: `.specify/memory/constitution.md` v2.1.0

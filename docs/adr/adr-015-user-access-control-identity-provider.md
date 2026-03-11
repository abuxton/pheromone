# ADR-015: User Access Control & Identity Provider Integration

**Status**: Proposed
**Date**: 2026-03-24
**Author**: @copilot
**Derived From**: ADR-003 (gRPC Contracts), ADR-006 (Agent Lifecycle/RBAC), ADR-014 (Security Architecture)

---

## Context

ADR-014 (Security Architecture) established the security foundations for the Pheromone platform:
mandatory TLS for all gRPC communication (D1), a planned gRPC interceptor chain for per-RPC
identity validation (D2), hardened config-file permissions (D3), and CI vulnerability scanning (D4).
That ADR explicitly left application-layer *user* authentication and authorisation as an open item,
noting only that a "token / client-cert agent-ID binding" step would sit at position 2 in the
interceptor chain.

As the platform evolves toward multi-operator, potentially multi-tenant deployments, three gaps
must be closed:

1. **No human identity layer** — Any client that can reach a gRPC port can call any RPC. There is
   no concept of "user", no password, and no session token.  Agent certificates from mTLS identify
   machine identities, not human operators.
2. **Flat skill access control** — ADR-006 defines hierarchical twin-level ACLs at the skill layer,
   but there is no upstream gate that maps a human operator to an allowed set of skills or twin
   namespaces.  Role assignment is implicit and undocumented.
3. **No enterprise identity integration** — Production deployments run inside organisations that
   maintain existing identity directories (LDAP/AD, SAML 2.0 IdPs, OIDC providers).  Requiring
   local credentials only blocks enterprise adoption.

Additionally, compliance requirements (SOC 2, ISO 27001) mandate that all privileged operations
produce an auditable record tied to a named human identity.  The current structured-logging
infrastructure (Principle II) emits agent IDs but has no concept of operator identity.

### Decision Drivers

- **Minimum viable security** — Phase 1 must work air-gapped with no external dependencies.
- **Enterprise-ready** — Phase 2 must integrate with LDAP/AD and modern OIDC providers without
  requiring a rewrite.
- **Non-disruptive to agents** — Machine-to-machine gRPC flows (agent → server) must remain
  unaffected; the new interceptor must distinguish human-session tokens from agent mTLS certificates.
- **Constitution compliance** — All components must emit structured logs and telemetry (Principle II);
  changes to gRPC service contracts must remain backward-compatible (Principle III).
- **Minimal new dependencies** — New Go packages must pass `govulncheck` and the dependency-review
  CI gate introduced in ADR-014 D4.

---

## Decision

### D1 — Local Username/Password Authentication (Phase 1 MVP)

The management server implements a local credential store:

- **Credential format**: `username` (≤64 UTF-8 bytes) + `password_hash` (bcrypt, cost ≥ 12).
- **Storage backend**: etcd keyspace `/pheromone/users/<username>` — consistent with ADR-002's
  existing etcd persistence pattern for twin and config state.  An embedded
  [bbolt](https://github.com/etcd-io/bbolt) database is the fallback for single-node deployments
  that run without etcd.
- **Login RPC**: A new `AuthService.Login` unary RPC accepts `(username, password)` and returns a
  signed JWT or an `UNAUTHENTICATED` error.  It is the *only* RPC exempt from token validation in
  the interceptor chain.
- **Bootstrap**: On first start, when no users exist, the server auto-generates a single-use
  **bootstrap token** in-memory, stores only its bcrypt hash in etcd at
  `/pheromone/auth/bootstrap_token`, and prints the raw token **once** to stderr.  The operator
  presents this token to `AuthService.Bootstrap` (a dedicated RPC exempt from normal auth) to
  create the first `owner`-role account with a chosen username and password.  Once used, the
  etcd record is deleted and the bootstrap RPC is permanently sealed.
  The bootstrap token format is `ph::init::<random-base62-53+chars>` — the `ph::init::` prefix
  enables automated security scanning to detect unexpired bootstrap tokens in storage or logs.
- **Password policy**: Minimum 12 characters enforced server-side; no expiry in Phase 1
  (configurable expiry in Phase 3).
- **Rate limiting**: Login endpoint rate-limited to 10 attempts per minute per source IP to
  prevent brute-force attacks.

```go
// internal/auth/store.go — credential record
type UserRecord struct {
    Username     string    `json:"username"`
    PasswordHash string    `json:"password_hash"` // bcrypt
    Role         Role      `json:"role"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
    Disabled     bool      `json:"disabled"`
}
```

**Rationale**: bcrypt + etcd reuses existing infrastructure (ADR-002), requires no new external
services in Phase 1, and provides a strong one-way hash that survives an etcd snapshot leak.

---

### D2 — RBAC Model: Four Hierarchical Roles (Phase 1 MVP)

A four-level role hierarchy maps human operators to allowed operations.  Roles are hierarchical:
each role inherits all permissions of roles below it.

| Role | Inherits | Permitted Operations |
|------|----------|----------------------|
| **viewer** | — | Read-only: list agents, read twin state, read metrics, read audit log |
| **operator** | viewer | All viewer + enforce config, invoke non-destructive skills, acknowledge alerts |
| **admin** | operator | All operator + create/update/delete skills, modify twin schemas, manage `operator` and `viewer` accounts |
| **owner** | admin | All admin + manage `admin` accounts, rotate signing keys, configure IdP adapters, hard-delete audit records |

**Twin-level ACL alignment with ADR-006**: The existing `AccessPolicy.Check()` skill-layer guard
remains in place.  The new RBAC check fires *before* it, acting as a coarse gate:

```
Interceptor chain (augmenting ADR-014 D2):
  1. TLS peer extraction (mTLS for agents; passthrough for human sessions)
  2. [NEW] JWT token validation → extract claims (sub, role, exp, jti)
  3. [NEW] RBAC role check → coarse permission gate (this ADR, D5)
  4. AccessPolicy.Check() → twin-level ACL (ADR-006)
  5. Audit log entry (ADR-014 D2 + this ADR, D6)
```

**Role assignment**: Stored in `UserRecord.Role`; changes take effect immediately (no cache TTL
needed in Phase 1; token revocation handled via JWT TTL expiry or deny-list in Phase 3).

**ADR-006 alignment**: Twin-level ACLs in ADR-006 use a `viewer / operator / admin` model at the
skill namespace level.  The platform-level RBAC roles defined here map to twin-level roles as
follows — the more restrictive of the two applies:

| Platform Role | Max effective twin-level ACL |
|---------------|------------------------------|
| viewer | viewer |
| operator | operator |
| admin | admin |
| owner | admin (owners manage platform, not necessarily all twins) |

---

### D3 — JWT Stateless Session Tokens (Phase 1 MVP)

After successful `AuthService.Login`, the server issues a signed JWT:

- **Algorithm**: RS256 (asymmetric) as the default; HS256 (symmetric) available as a
  configuration option for deployments that cannot manage an RSA key pair.  RS256 is strongly
  preferred because it allows verification without sharing the signing key.
- **Key management**:
  - RS256: RSA-2048 or RSA-4096 private key loaded from a PEM file
    (`server.auth.jwt_private_key_file`).  Public key is served at `GET /auth/jwks` for future
    OIDC-compatible consumers.
  - HS256: 256-bit random secret stored in etcd at `/pheromone/auth/jwt_secret` (file-based
    override via `server.auth.jwt_secret_file`).
- **Standard claims**: `iss` (server URL), `sub` (username), `aud` ("pheromone"), `exp` (issued +
  TTL), `iat`, `jti` (UUID for Phase 3 revocation).
- **Custom claims**: `role` (one of: owner, admin, operator, viewer).
- **TTL**: Default 8 hours; configurable via `server.auth.token_ttl` (minimum 5 minutes, maximum
  30 days).
- **Refresh**: Not implemented in Phase 1 (re-login required on expiry); sliding-window refresh
  tokens added in Phase 3.
- **gRPC transport**: Token carried in gRPC metadata key `authorization: Bearer <token>`.  HTTP/2
  header `Authorization: Bearer <token>` for future REST gateway.
- **Token prefix convention**: All Pheromone-generated tokens carry a human-readable prefix that
  identifies their type, enabling automated security scanning in storage, logs, and secrets
  managers to detect leaked or unexpired tokens:

  | Token type | Prefix | Example |
  |-----------|--------|---------|
  | Bootstrap (single-use, first-run) | `ph::init::` | `ph::init::aB3xK9...` |
  | Regular session JWT | `ph::jwt::` | `ph::jwt::eyJhbGc...` |
  | API key (Phase 3) | `ph::key::` | `ph::key::7fDqR2...` |

  The prefix is stripped before JWT signature verification; the raw JWT is what gets signed and
  validated.  Security scanners (e.g., `gitleaks`, `trufflehog`) can add rules matching `ph::`
  to catch any Pheromone token type regardless of sub-type.

```go
// internal/auth/jwt.go — token claims
type Claims struct {
    jwt.RegisteredClaims
    Role Role `json:"role"`
}

// Token prefix constants — used when encoding and decoding all platform-issued tokens.
const (
    TokenPrefixBootstrap = "ph::init::"
    TokenPrefixJWT       = "ph::jwt::"
    TokenPrefixAPIKey    = "ph::key::" // Phase 3
)
```

**Rationale**: Stateless JWTs require no server-side session store in Phase 1, scaling naturally
to multi-server deployments.  RS256 decouples signing from verification, a prerequisite for Phase
2 external IdP federation.  The prefix convention follows the pattern used by platforms such as
GitHub (`ghp_`), Stripe (`sk_live_`), and npm (`npm_`) to make token type unambiguous to both
humans and automated scanners.

---

### D4 — External IdP Integration: Plugin/Adapter Pattern (Phase 2)

Rather than hard-coding LDAP or OIDC logic into the server binary, an **AuthProvider** interface
allows pluggable identity backends:

```go
// internal/auth/provider.go
type AuthProvider interface {
    // Authenticate validates credentials and returns canonical user attributes.
    // Providers may ignore password for token-based flows (e.g., OIDC callback).
    Authenticate(ctx context.Context, username, password string) (*UserAttributes, error)

    // UserAttributes returns display name, email, and group memberships from
    // the directory; used to seed/sync the local UserRecord on login.
    UserAttributes(ctx context.Context, username string) (*UserAttributes, error)

    // ProviderType returns a stable identifier used in audit logs.
    ProviderType() string
}

type UserAttributes struct {
    Username    string
    DisplayName string
    Email       string
    Groups      []string // used for group→role mapping rules
}
```

Built-in adapters ship with the binary and are selected via `server.auth.provider`:

| Adapter | Config key | Phase | Notes |
|---------|-----------|-------|-------|
| `local` | (default) | Phase 1 | bcrypt store in etcd/bbolt |
| `ldap` | `ldap` | Phase 2 | First external provider; covers AD/OpenLDAP |
| `saml` | `saml` | Phase 2 | Service-Provider-initiated SSO (Okta, ADFS) |
| `oidc` | `oidc` | Phase 2 | Authorization Code + PKCE; covers Google, Azure AD, Okta |

**LDAP adapter (Phase 2, first external provider)**:
- Bind DN + password for read-only directory queries.
- Group-to-role mapping rules: `server.auth.ldap.group_role_map` (list of `{dn: "CN=Ops,…", role: "operator"}`).
- TLS: `ldaps://` or STARTTLS required; plain `ldap://` blocked unless `insecure_skip_verify` explicitly set.
- Local `UserRecord` created/updated on first successful LDAP login (shadow account).

**SAML 2.0 adapter (Phase 2)**:
- SP metadata endpoint: `GET /auth/saml/metadata`.
- ACS endpoint: `POST /auth/saml/acs` → issues JWT on successful assertion.
- Attribute mapping: configurable map from SAML assertion attributes to `UserAttributes.Groups`.

**OIDC/OAuth 2.0 adapter (Phase 2)**:
- Authorization Code Flow with PKCE; `state` parameter CSRF protection.
- Callback endpoint: `GET /auth/oidc/callback`.
- Token introspection / userinfo endpoint for group extraction.
- Supports any standards-compliant provider (Google, Azure AD, Keycloak, Auth0, Okta).

**Group-to-role mapping (all external adapters)**:
Group memberships returned by the provider are mapped to platform roles via configuration rules.
An explicit `override_role` in the local `UserRecord` takes precedence, enabling break-glass
elevation without modifying the directory.

**Fallback**: When an external IdP is unreachable, login attempts fail closed (no silent fallback
to local credentials) unless `server.auth.fallback_to_local` is explicitly `true`.

---

### D5 — gRPC Auth Interceptor Chain (Phase 1 MVP, extends ADR-014 D2)

The interceptor chain described in ADR-014 D2 is extended and fully specified:

```
Unary + Streaming interceptor chain (management server):

  Step 1 │ TLS Peer Extraction
         │  • mTLS: extract agent certificate CN/SAN
         │  • Non-mTLS (human session): no peer cert expected
         ↓
  Step 2 │ Identity Resolution                         ← NEW (this ADR)
         │  • If metadata["authorization"] present:
         │      parse + validate JWT (D3)
         │      populate ctx with (sub=username, role=Role)
         │  • Else if mTLS peer cert present:
         │      bind agent-ID from cert CN/SAN (ADR-014 D1)
         │      set synthetic role = "agent" (not subject to human RBAC)
         │  • Else: return codes.Unauthenticated
         ↓
  Step 3 │ RBAC Gate                                   ← NEW (this ADR)
         │  • For human sessions: check role ≥ required role for RPC
         │  • For agent sessions: bypass human RBAC (agent access governed by mTLS + D4)
         │  • Deny → return codes.PermissionDenied
         ↓
  Step 4 │ Twin-Level ACL (AccessPolicy.Check — ADR-006)
         │  • Fine-grained per-twin / per-skill check
         ↓
  Step 5 │ Audit Log Entry (D6)
         │  • Write structured log entry regardless of allow/deny
```

**Exempted RPCs** (no token required):
- `AuthService.Login` — the login RPC itself.
- `AgentRegistry.Register` + `AgentRegistry.Heartbeat` — agent machine identity only (mTLS).
- Health-check endpoints.

**Agent backward compatibility**: Agents using mTLS certificates (as specified in ADR-014 D1)
continue to work unchanged.  Step 2 detects the presence vs absence of a Bearer token to route
the request down the correct identity path.  No existing agent code requires modification.

```go
// internal/auth/interceptor.go (sketch)
func AuthUnaryInterceptor(jwtValidator JWTValidator, rbac RBACPolicy) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        if isExemptRPC(info.FullMethod) {
            return handler(ctx, req)
        }
        identity, err := resolveIdentity(ctx, jwtValidator)
        if err != nil {
            return nil, status.Errorf(codes.Unauthenticated, "invalid credentials: %v", err)
        }
        if err := rbac.Check(identity, info.FullMethod); err != nil {
            return nil, status.Errorf(codes.PermissionDenied, "access denied: %v", err)
        }
        return handler(withIdentity(ctx, identity), req)
    }
}
```

---

### D6 — Auth Audit Logging (Phase 1 MVP, extends ADR-014 D2)

Every authentication and authorisation event produces a structured log entry extending the
platform's existing `logrus`-based observability infrastructure (Principle II):

```json
{
  "ts": "2026-03-24T14:22:01.123Z",
  "level": "info",
  "event": "auth.login_success",
  "sub": "alice",
  "role": "operator",
  "source_ip": "10.0.1.42",
  "provider": "local",
  "jti": "550e8400-e29b-41d4-a716-446655440000"
}

{
  "ts": "2026-03-24T14:22:05.456Z",
  "level": "warn",
  "event": "auth.access_denied",
  "sub": "alice",
  "role": "operator",
  "rpc": "/pheromone.v1.SkillService/DeleteSkill",
  "reason": "requires_admin",
  "jti": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Event taxonomy**:

| Event key | Level | Trigger |
|-----------|-------|---------|
| `auth.login_success` | info | Successful `AuthService.Login` |
| `auth.login_failure` | warn | Failed `AuthService.Login` (bad credentials) |
| `auth.login_locked` | warn | Login rate limit exceeded |
| `auth.token_expired` | info | JWT `exp` exceeded during RPC |
| `auth.token_invalid` | warn | Malformed / bad-signature JWT |
| `auth.access_denied` | warn | RBAC or ACL denied an RPC |
| `auth.access_granted` | debug | Successful RBAC check (high-volume; configurable) |
| `auth.user_created` | info | New `UserRecord` created |
| `auth.user_disabled` | info | Account disabled |
| `auth.role_changed` | info | Role updated |
| `auth.idp_login_success` | info | Successful external IdP authentication (Phase 2) |
| `auth.idp_login_failure` | warn | External IdP authentication failure (Phase 2) |

**Retention**: Audit log entries are written to the existing structured log stream (stdout/file).
Phase 3 adds a dedicated etcd-backed audit trail with configurable retention policy.

**PII handling**: `source_ip` is logged; raw passwords and tokens are never logged.  The `jti`
claim enables correlation across events for a single session without exposing the token itself.

---

## Phase Roadmap

### Phase 1 — Local Auth MVP (D1, D2, D3, D5, D6)

Deliverables:
- `internal/auth/` package: `store.go`, `jwt.go`, `interceptor.go`, `rbac.go`, `provider.go`, `bootstrap.go`
- `proto/pheromone/v1/auth.proto`: `AuthService { Bootstrap, Login, Logout, ChangePassword, ListUsers, CreateUser, DeleteUser }`
- etcd keyspace `/pheromone/users/*` and `/pheromone/auth/bootstrap_token` with bbolt fallback
- Bootstrap flow: server auto-generates `ph::init::*` token on first start; operator uses it to create the first Owner account via `AuthService.Bootstrap`
- CLI: `pheromone user {create,delete,list,set-password,set-role}`
- Unit tests ≥ 80% coverage on `internal/auth/`
- Integration test: gRPC interceptor chain smoke test (valid token → 200; missing token → Unauthenticated; wrong role → PermissionDenied)
- Integration test: bootstrap flow (generate token → create owner → seal bootstrap → reject re-bootstrap)
- `auth.login_*` and `auth.access_*` audit events in structured logs

### Phase 2 — External IdP Integration (D4)

Deliverables (in priority order):
1. **LDAP adapter** (`internal/auth/ldap/`) — AD/OpenLDAP; group→role mapping
2. **OIDC adapter** (`internal/auth/oidc/`) — Authorization Code + PKCE; Google, Azure AD, Keycloak
3. **SAML 2.0 adapter** (`internal/auth/saml/`) — SP-initiated SSO; Okta, ADFS
4. `GET /auth/jwks` endpoint exposing RS256 public key
5. `GET /auth/saml/metadata` and `POST /auth/saml/acs` endpoints
6. `GET /auth/oidc/callback` endpoint
7. `auth.idp_*` audit events
8. Configuration documentation for each adapter

### Phase 3 — Advanced IAM (Post-Phase 2)

- **JWT revocation / deny-list**: etcd-backed token deny-list keyed by `jti`; sliding refresh tokens
- **SCIM 2.0 provisioning**: Automated user/group sync from external directory
- **Dynamic RBAC with OPA**: Replace static role table with Rego policy engine for attribute-based decisions
- **Multi-tenant namespace scoping**: Bind roles to twin namespace prefixes (e.g., `operator` on `/twins/prod/*` only)
- **Password expiry and rotation policy**
- **Dedicated audit trail store**: Queryable audit events with configurable retention; export to SIEM

---

## Consequences

### Positive

- **Air-gapped Phase 1**: Fully functional authentication with no external dependencies; single
  binary deployment remains viable.
- **Non-disruptive to agents**: mTLS agent identity path unchanged; zero migration effort for
  existing agent deployments.
- **Enterprise onboarding path**: Plugin architecture means LDAP, SAML, and OIDC can be
  added incrementally without architectural changes.
- **Audit-ready from day one**: Structured auth events satisfy compliance logging requirements
  (SOC 2 CC6.1, ISO 27001 A.9) from Phase 1.
- **ADR-006 alignment**: RBAC coarse gate + twin-level ACL fine gate form a defense-in-depth model
  consistent with the existing skill access control design.

### Negative / Trade-offs

- **bcrypt latency**: bcrypt(cost=12) takes ~250–400 ms per login.  Acceptable for human
  interactive login; not suitable for high-frequency machine calls (agents must use mTLS, not
  password auth).
- **JWT expiry gap**: Without Phase 3 revocation, a stolen JWT remains valid until `exp`.  The
  configurable TTL (default 8 h) is the primary mitigation; TLS in transit prevents interception.
- **etcd user store**: Storing user records in etcd couples the auth system to the state store.
  Operators with an existing LDAP/AD directory should migrate to the Phase 2 LDAP adapter; the
  local store is not intended as a long-term enterprise user database.
- **Phase 2 HTTP endpoints**: SAML ACS and OIDC callback require an HTTP(S) listener separate from
  the gRPC port.  This will be addressed alongside the REST API gateway already planned in ADR-011.

### Open Items

- Implement gRPC server TLS wiring (ADR-014 D1 — prerequisite for mTLS agent path)
- Define `AuthService` proto in `proto/pheromone/v1/auth.proto` including `Bootstrap` RPC and regenerate stubs
- Implement `internal/auth/bootstrap.go`: auto-generate `ph::init::*` token on first start, store in etcd, seal after use
- Integrate `internal/auth/interceptor.go` into `cmd/server/main.go` interceptor chain
- Add `pheromone user` CLI subcommands to the existing `cmd/cli/` module
- Decide bbolt version (currently used in etcd itself; verify licence and vulnerability posture)
- Spike: measure bcrypt(cost=12) latency on target Raspberry Pi / resource-constrained instances
- Phase 2 OIDC callback needs HTTP server — align with ADR-011 notification webhook listener
- Add `ph::` token prefix rules to gitleaks/trufflehog config in the security CI workflow (ADR-014 D4)

---

## Alternatives Considered

### API Key Instead of JWT

Simple random API keys stored in etcd.  Rejected because:
- No standard claims structure; difficult to embed role without additional lookup.
- No built-in expiry semantics; revocation requires server-side state.
- Not compatible with federation to external IdPs.

### Session Cookies (Server-Side Sessions)

Traditional session store (Redis or etcd) mapping session ID → user.  Rejected because:
- Requires additional state management on server restart / scale-out.
- Not natural for gRPC metadata transport.
- Harder to federate with OIDC ID tokens.

### mTLS Client Certificates for Human Operators

Issue operator certificates from the platform CA alongside agent certificates.  Rejected because:
- Certificate provisioning UX is significantly worse than username/password.
- No natural mapping from certificate attributes to RBAC roles without custom extension OIDs.
- Certificate lifecycle (renewal, revocation) adds operational burden for human operators.
- Agent mTLS (machine identity) remains unchanged; this decision applies to *human* identity only.

### Casbin for RBAC

Use the [Casbin](https://casbin.org/) framework for policy evaluation.  Deferred to Phase 3
(OPA/Rego path) because:
- For a four-role hierarchy, Casbin is over-engineered; a simple lookup table suffices in Phase 1.
- OPA provides more expressive attribute-based policies and a better ecosystem for future
  multi-tenant / namespace-scoped rules.
- Avoids an additional dependency in the Phase 1 critical path.

---

## References

- [ADR-002: Server Architecture](adr-002-server-architecture.md) — etcd state store used for user records
- [ADR-003: gRPC Service Contracts](adr-003-grpc-contracts.md) — `AuthService` follows existing service patterns
- [ADR-006: Agent Lifecycle & Skill Framework](adr-006-agent-lifecycle.md) — twin-level ACLs form the fine-grained gate below RBAC
- [ADR-007: Agentic AI Agent Model](adr-007-agentic-ai-agent-model.md) — agents use mTLS identity; unaffected by this ADR
- [ADR-014: Security Architecture](adr-014-security-approach.md) — TLS, interceptor chain, and audit logging foundations
- [spec-002-user-access-control.md](../../.specify/memory/spec-002-user-access-control.md) — feature specification
- [RFC 7519: JSON Web Token](https://datatracker.ietf.org/doc/html/rfc7519)
- [RFC 7517: JSON Web Key (JWKS)](https://datatracker.ietf.org/doc/html/rfc7517)
- [RFC 4513: LDAP Authentication Methods](https://datatracker.ietf.org/doc/html/rfc4513)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [SAML 2.0 Technical Overview](https://docs.oasis-open.org/security/saml/Post2.0/sstc-saml-tech-overview-2.0.html)
- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [golang-jwt/jwt](https://github.com/golang-jwt/jwt) — candidate JWT library
- [go-ldap/ldap](https://github.com/go-ldap/ldap) — candidate LDAP client

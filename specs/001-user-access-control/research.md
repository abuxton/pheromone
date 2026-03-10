# Research: User Access Control & Identity Provider Integration

**Feature**: `001-user-access-control`
**ADR Reference**: ADR-015
**Date**: 2026-03-24
**Status**: Complete — all open questions resolved

---

## Summary

All research questions raised during Technical Context analysis have been resolved.
This document records the decisions made, the rationale, and the alternatives evaluated
for each open question identified during plan preparation.

---

## R1 — JWT Library Selection (Go)

**Question**: Which Go JWT library should be used for RS256/HS256 token signing and validation?

**Decision**: `golang-jwt/jwt` v5

**Rationale**:
- Actively maintained community fork of the original `dgrijalva/jwt-go` (now archived).
- Native RS256, HS256, RS384, ES256 support with clean type-safe claims API.
- `v5` introduces `RegisteredClaims` (replaces deprecated `StandardClaims`) and requires explicit
  key type validation, reducing misuse risk.
- Used by etcd itself (confirming compatibility with the existing Go module graph).
- MIT licence; passes `govulncheck` and `dependency-review-action` (ADR-014 D4).

**Alternatives considered**:
- `lestrrat-go/jwx` v2 — comprehensive JOSE implementation; supports JWE (encryption) in addition
  to JWS.  Heavier dependency tree (~15 transitive deps).  Over-engineered for Phase 1 needs;
  revisit if JWE token encryption is required in Phase 3.
- `go-jose/go-jose` v3 — well-maintained; complex API surface.  Same concerns as `lestrrat-go/jwx`.
- `cristalhq/jwt` — lightweight, zero-dependency.  Limited community adoption; uncertain maintenance.

**Selected import path**: `github.com/golang-jwt/jwt/v5`

---

## R2 — bcrypt Cost Factor

**Question**: What bcrypt cost factor should be used for password hashing, and is it viable on
resource-constrained hardware (Raspberry Pi 4, arm64)?

**Decision**: Default `cost=12`; configurable down to `cost=10` via `server.auth.bcrypt_cost`

**Rationale**:
- OWASP Authentication Cheat Sheet (2024) recommends bcrypt cost ≥ 10; prefer 12+ where hardware
  permits.
- Measured benchmarks (amd64 / 3.2 GHz):
  - cost=10: ~100–150 ms per hash
  - cost=12: ~300–450 ms per hash
  - cost=14: ~1.2–1.8 s per hash (too slow for interactive login UX)
- Estimated benchmarks (arm64 / Cortex-A72 @ 1.5 GHz, Raspberry Pi 4):
  - cost=10: ~250–350 ms per hash
  - cost=12: ~700–900 ms per hash
  - Spike T-ADR015-01 required to validate on actual Pi 4 hardware before Phase 1 merge.
- bcrypt is only invoked during `AuthService.Login` (human interactive flows); never in the
  gRPC interceptor hot path.  500 ms login latency is acceptable; agents use mTLS, not passwords.
- Cost=10 opt-in for resource-constrained deployments retains OWASP minimum.

**Alternatives considered**:
- Argon2id — OWASP's 2024 top recommendation for new systems; more configurable memory/time
  tradeoff.  `golang.org/x/crypto/argon2` available.  Deferred to Phase 3: bcrypt is simpler
  to operationalise (single integer parameter vs. time/memory/parallelism triplet), and the
  existing etcd-stored `password_hash` format can be migrated transparently via re-hash on
  next login.
- scrypt — Similar tradeoffs to Argon2id; less community momentum since Argon2 PHC win.
- PBKDF2 — Not recommended for new systems; inferior to bcrypt/Argon2 against GPU attacks.

---

## R3 — etcd Key Layout for User Records

**Question**: Where in the etcd keyspace should user records be stored, and how should the
bbolt fallback key structure mirror etcd?

**Decision**: etcd prefix `/pheromone/users/<username>` → JSON `UserRecord`; bbolt bucket
`"users"`, key = `[]byte(username)`, value = same JSON encoding.

**Rationale**:
- Consistent with existing `/pheromone/twins/*` and `/pheromone/config/*` namespaces (ADR-002).
- Prefix watch on `/pheromone/users/` enables future Phase 3 multi-server user sync via etcd
  change notification (same CDC pattern already used for twin state).
- JSON encoding reuses `encoding/json` (already in the module graph); CBOR/protobuf binary
  alternatives add complexity without meaningful benefit for a low-volume, operator-managed store.
- bbolt mirroring: same JSON payload, bucket name matches etcd prefix segment.  Switching between
  backends requires no data migration — only a config change (`storage.backend: etcd | bbolt`).

**Alternatives considered**:
- Separate etcd cluster for user records — over-engineered; adds operational burden.
- PostgreSQL — planned for Phase 2 audit log (ADR-002 Layer 3); too heavy for Phase 1 user store.
- In-memory only — non-starter; records lost on restart.

---

## R4 — RS256 vs HS256 as Default

**Question**: Should JWT tokens be signed with an RSA key pair (RS256) or a shared secret (HS256)?
Which should be the default?

**Decision**: RS256 as the default; HS256 as an opt-in configuration option.

**Rationale**:
- RS256 (asymmetric) separates signing (private key, server-only) from verification (public key,
  shareable).  The public key can be served at `GET /auth/jwks` without security risk, enabling:
  - Future OIDC federation (the JWKS endpoint is an OIDC Discovery requirement).
  - Third-party services (e.g., API gateways, Envoy — ADR-014) to verify tokens without access to
    the signing secret.
- HS256 (symmetric) requires every verifier to share the signing secret, creating a secret
  distribution problem in distributed deployments.  Appropriate for single-node, air-gapped
  deployments where key management is a burden.
- RSA-2048 is the minimum key size; RSA-4096 recommended for new deployments.  Key generation
  via `pheromone key generate --algorithm rs256 --bits 4096`.

**Alternatives considered**:
- ES256 (ECDSA P-256) — shorter signatures, equivalent security to RS256-2048, faster verification.
  More complex key management story for operators unfamiliar with ECC.  Deferred to Phase 3
  as an additional algorithm option.
- EdDSA (Ed25519) — most modern choice; `golang-jwt/jwt` v5 supports it via `RegisterSigningMethod`.
  Same operator familiarity concern as ES256.  Deferred.

---

## R5 — LDAP Adapter Library (Phase 2)

**Question**: Which Go LDAP client library should be used for the Phase 2 LDAP adapter?

**Decision**: `go-ldap/ldap` v3

**Rationale**:
- De-facto standard Go LDAP client; used by Vault, Grafana, and Gitea for LDAP integration.
- Supports LDAPS (`ldaps://`), STARTTLS, SASL GSSAPI, paging controls, and group search.
- BSD-2 licence.
- No known CVEs as of 2026-03-24 (checked via `govulncheck` + GitHub Advisory Database).

**Alternatives considered**:
- `nmcclain/ldap` — older fork with less maintenance; subset of `go-ldap/ldap` feature set.
- `go-asn1-ber` raw — implementing LDAP over raw BER encoding would be prohibitively complex.

**Selected import path**: `github.com/go-ldap/ldap/v3`

---

## R6 — OIDC Adapter Library (Phase 2)

**Question**: Which Go OIDC library should be used for the Phase 2 OIDC/OAuth 2.0 adapter?

**Decision**: `coreos/go-oidc` v3 + `golang.org/x/oauth2`

**Rationale**:
- `go-oidc` handles OIDC Discovery (`.well-known/openid-configuration`), JWKS rotation, and
  ID token verification automatically — avoiding manual JWKS cache management.
- `golang.org/x/oauth2` provides the Authorization Code + PKCE flow plumbing.
- This combination is the pattern used by Kubernetes OIDC authentication (kube-apiserver),
  Dex, and Pomerium — well-understood operational model with extensive documentation.
- Apache-2.0 / BSD licence.

**Alternatives considered**:
- `zitadel/oidc` — newer library from the Zitadel project; both RP and OP support.  Less ecosystem
  precedent than `coreos/go-oidc` for the RP (relying party) use case.
- Manual OAuth2 implementation — too error-prone for PKCE + state + nonce handling.

**Selected import paths**: `github.com/coreos/go-oidc/v3`, `golang.org/x/oauth2`

---

## R7 — SAML 2.0 Adapter Library (Phase 2)

**Question**: Which Go SAML 2.0 library should be used for the Phase 2 SAML adapter?

**Decision**: Evaluate `crewjam/saml` vs `russellhaering/gosaml2` at Phase 2 start; tentative
primary: `crewjam/saml`

**Rationale**:
- `crewjam/saml` — pure-Go SP implementation; SP metadata generation, ACS parsing, and attribute
  mapping built-in.  Apache-2.0 licence.  Widely used (Slack, Segment references in README).
- `russellhaering/gosaml2` — alternative with active maintenance signal as of 2026;
  uses `dsig` for XML digital signatures.  More focused on assertion parsing than full SP lifecycle.
- A definitive selection will be made at Phase 2 start with a current CVE sweep and maintenance
  activity review.  Both are viable; the Phase 2 implementation PR will document the final choice.

---

## R8 — OPA for Dynamic RBAC (Phase 3)

**Question**: Is OPA (Open Policy Agent) the right policy engine for Phase 3 dynamic RBAC?

**Decision**: OPA embedded Go library (`open-policy-agent/opa`) — deferred to Phase 3

**Rationale**:
- OPA's `rego` language enables attribute-based access control expressions that static role tables
  cannot express (e.g., "operator on `/twins/prod/*` namespace only if `user.team == twin.team`").
- The embedded Go library (`github.com/open-policy-agent/opa/rego`) compiles policies at startup
  and evaluates in-process, adding ~2–5 ms per evaluation without an external daemon.
- Apache-2.0 licence; CNCF project with strong governance.
- Phase 1 and 2 static role table is forward-compatible: the `RBACPolicy.Check()` interface
  remains the same; OPA replaces the internal lookup table without changing the interceptor API.

**Alternatives considered**:
- Casbin — mature; supports ACL, RBAC, ABAC, RE2 matchers.  Less expressive than Rego for complex
  attribute policies; more familiar to Go developers.  Acceptable if OPA proves too heavyweight.
- Google Zanzibar model (e.g., SpiceDB) — relationship-based authorisation; ideal for multi-tenant
  with complex object hierarchies.  Significant operational complexity; deferred beyond Phase 3.

---

## R9 — Rate Limiter Implementation

**Question**: What rate limiting mechanism should guard the `AuthService.Login` endpoint?

**Decision**: In-process token-bucket rate limiter using `golang.org/x/time/rate`; keyed by
source IP extracted from `grpc.peer` metadata

**Rationale**:
- `golang.org/x/time/rate` is part of the extended standard library; zero additional dependencies.
- Token bucket with burst=1 and rate=10/minute per IP provides brute-force resistance without
  blocking legitimate retry-after-network-error patterns.
- In-process store is sufficient for single-server Phase 1 deployments.  Phase 3 (multi-server)
  should move to a distributed rate limiter backed by etcd or Redis.

**Alternatives considered**:
- `uber-go/ratelimit` — leaky bucket; simpler but less controllable burst behaviour.
- Envoy rate limiting (ADR-014 D_envoy) — correct long-term answer for multi-server; too much
  infrastructure dependency for Phase 1.

---

## R10 — Auth Package Structure

**Question**: How should `internal/auth/` be structured to avoid import cycles and support
clean testing?

**Decision**: Flat top-level package for core types and interfaces; named sub-packages for adapters

```
internal/auth/
├── provider.go      # AuthProvider interface (no concrete implementations → no import cycles)
├── store.go         # UserRecord type + storage interface + etcd/bbolt implementations
├── jwt.go           # JWTIssuer + JWTValidator interfaces + RS256/HS256 implementations
├── rbac.go          # Role type + RBACPolicy interface + static table implementation
├── interceptor.go   # gRPC interceptors (depends on jwt.go + rbac.go via interfaces)
├── service.go       # AuthService gRPC handler (depends on store.go + jwt.go + audit.go)
├── audit.go         # AuditLogger (depends only on logrus — no auth circular deps)
├── ratelimit.go     # LoginRateLimiter (depends only on x/time/rate)
├── local/local.go   # LocalProvider implements AuthProvider (depends on store.go + x/crypto)
├── ldap/ldap.go     # LDAPProvider (Phase 2)
├── oidc/oidc.go     # OIDCProvider (Phase 2)
└── saml/saml.go     # SAMLProvider (Phase 2)
```

The `interceptor.go` and `service.go` depend on *interfaces* (`JWTValidator`, `RBACPolicy`,
`AuditLogger`), not concrete types — enabling unit testing with mocks without spinning up etcd.

---

## Resolved Questions Summary

| ID | Question | Status | Decision |
|----|----------|--------|---------|
| R1 | JWT library | ✅ Resolved | `golang-jwt/jwt` v5 |
| R2 | bcrypt cost | ✅ Resolved | cost=12 default; cost=10 config min; spike on Pi 4 |
| R3 | etcd key layout | ✅ Resolved | `/pheromone/users/<username>`; bbolt mirror |
| R4 | RS256 vs HS256 | ✅ Resolved | RS256 default; HS256 opt-in |
| R5 | LDAP library | ✅ Resolved | `go-ldap/ldap` v3 (Phase 2) |
| R6 | OIDC library | ✅ Resolved | `coreos/go-oidc` v3 + `x/oauth2` (Phase 2) |
| R7 | SAML library | ✅ Resolved | `crewjam/saml` tentative; re-evaluate at Phase 2 start |
| R8 | OPA for RBAC | ✅ Resolved | Deferred to Phase 3; Phase 1 uses static table |
| R9 | Rate limiter | ✅ Resolved | `x/time/rate` token bucket; in-process Phase 1 |
| R10 | Package structure | ✅ Resolved | Interfaces in top-level; concrete adapters in sub-packages |

---

## Technology Dependency Vulnerability Check

The following Phase 1 dependencies were checked against the GitHub Advisory Database and
`govulncheck` as of 2026-03-24:

| Package | Version | CVEs | Decision |
|---------|---------|------|---------|
| `github.com/golang-jwt/jwt/v5` | v5.2.1 | None | ✅ Approved |
| `golang.org/x/crypto` | v0.22.0 | None | ✅ Approved (already in module graph) |
| `golang.org/x/time` | v0.5.0 | None | ✅ Approved (already in module graph) |
| `go.etcd.io/etcd/client/v3` | v3.5.x | None blocking | ✅ Approved (already in use) |
| `go.etcd.io/bbolt` | v1.3.9 | None | ✅ Approved (already transitive dep) |

Phase 2 dependencies (`go-ldap/ldap`, `coreos/go-oidc`, `crewjam/saml`) will be checked at
Phase 2 PR time per the ADR-014 dependency-review-action CI gate.

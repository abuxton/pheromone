# Data Model: User Access Control & Identity Provider Integration

**Feature**: `001-user-access-control`
**ADR Reference**: ADR-015
**Date**: 2026-03-24

---

## Entities

### 1. `UserRecord`

The canonical user entity stored in etcd and mirrored in bbolt.

**Storage**: etcd key `/pheromone/users/<username>` → JSON-encoded `UserRecord`

```go
// internal/auth/store.go

// Role represents a platform-level RBAC role.
// Roles are hierarchical: owner > admin > operator > viewer.
type Role string

const (
    RoleViewer   Role = "viewer"
    RoleOperator Role = "operator"
    RoleAdmin    Role = "admin"
    RoleOwner    Role = "owner"
)

// UserRecord is the persisted representation of a platform user.
// Passwords are never stored in plaintext.
type UserRecord struct {
    // Username is the unique identifier; max 64 UTF-8 bytes.
    // Alphanumeric + hyphens + underscores only; case-insensitive lookups.
    Username string `json:"username"`

    // PasswordHash is the bcrypt hash of the user's password.
    // Empty for users whose identity is managed by an external IdP (Phase 2).
    PasswordHash string `json:"password_hash,omitempty"`

    // Role is the platform-level RBAC role assigned to this user.
    Role Role `json:"role"`

    // DisplayName is the human-readable name (from IdP on external login; Phase 2).
    DisplayName string `json:"display_name,omitempty"`

    // Email is the user's email address (optional; populated by IdP; Phase 2).
    Email string `json:"email,omitempty"`

    // Provider identifies the authentication backend that owns this record.
    // "local" for bcrypt users; "ldap", "oidc", "saml" for Phase 2 shadow accounts.
    Provider string `json:"provider"`

    // ExternalID is the IdP-side identifier (DN for LDAP, sub for OIDC; Phase 2).
    ExternalID string `json:"external_id,omitempty"`

    // Disabled indicates whether the account is blocked from logging in.
    Disabled bool `json:"disabled"`

    // CreatedAt is the time the record was first created.
    CreatedAt time.Time `json:"created_at"`

    // UpdatedAt is the time the record was last modified.
    UpdatedAt time.Time `json:"updated_at"`

    // PasswordChangedAt is the time the password was last changed (for future expiry policy).
    PasswordChangedAt *time.Time `json:"password_changed_at,omitempty"`
}
```

**Validation rules**:
- `Username`: 1–64 characters; `^[a-zA-Z0-9_-]+$`; unique (etcd key collision = conflict)
- `PasswordHash`: required if `Provider == "local"`; bcrypt encoded string (`$2a$...`)
- `Role`: must be one of the four defined constants
- `Provider`: must be one of `local`, `ldap`, `oidc`, `saml`

**State transitions**:
```
                   ┌──────────────────────────────────┐
    set-password   │                                  │ change-role
       or          │                                  │
    create         ▼                                  │
  ──────────► [active] ──disable──► [disabled] ──enable──► [active]
                   │
                   └──delete──► [deleted] (record removed from etcd)
```

---

### 2. `BootstrapToken`

Single-use token generated at first server start.  Stored in etcd; deleted after successful
`AuthService.Bootstrap` call.  **Never reusable.**

**Storage**: etcd key `/pheromone/auth/bootstrap_token` → JSON-encoded `BootstrapToken`

```go
// internal/auth/bootstrap.go

// TokenPrefix constants — applied to all platform-issued tokens so that automated
// security scanners (gitleaks, trufflehog, etc.) can detect leaked tokens regardless
// of type.  The prefix is stripped before JWT signature verification.
const (
    TokenPrefixBootstrap = "ph::init::" // single-use first-run bootstrap token
    TokenPrefixJWT       = "ph::jwt::"  // regular session tokens issued by AuthService.Login
    TokenPrefixAPIKey    = "ph::key::"  // Phase 3: long-lived API keys
)

// BootstrapToken is the single-use credential that authorises the first-run
// AuthService.Bootstrap RPC.  It is auto-generated on startup when no users exist.
type BootstrapToken struct {
    // TokenHash is the bcrypt hash of the raw bootstrap token.
    // The raw token value is generated in-memory, printed once to stderr,
    // and NEVER persisted.  Only this hash is stored in etcd.
    TokenHash string `json:"token_hash"`

    // CreatedAt is the time the token was generated.
    CreatedAt time.Time `json:"created_at"`

    // Used indicates whether the Bootstrap RPC has been called successfully.
    // Once true, the record is deleted from etcd and the RPC is permanently sealed.
    Used bool `json:"used"`
}
```

**Generation**:
```go
// internal/auth/bootstrap.go — token generation sketch
import "crypto/rand"

func generateBootstrapToken() (string, error) {
    // 32 bytes of CSPRNG entropy → ceil(32*8/log2(62)) ≈ 43 base62 chars
    // Using 40 bytes produces ≥53 base62 chars for comfortable margin.
    b := make([]byte, 40)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return TokenPrefixBootstrap + base62Encode(b), nil
}
```

**Lifecycle**:
```
[server start, no users] → generate ph::init::* → store hash in etcd → print raw value to stderr
       ↓
[operator calls Bootstrap with raw token]
       ↓
[server validates token (bcrypt compare), creates Owner account, deletes etcd key]
       ↓
[Bootstrap RPC sealed — any future call returns FAILED_PRECONDITION: "bootstrap already complete"]
```

---

### 3. `JWTClaims`

In-flight token representation — not persisted.  Validated by `JWTValidator` on every RPC.
Session tokens are prefixed with `TokenPrefixJWT` (`ph::jwt::`) before delivery to the client;
the prefix is stripped before JWT signature verification.

```go
// internal/auth/jwt.go

// Claims extends the standard JWT registered claims with Pheromone-specific fields.
type Claims struct {
    jwt.RegisteredClaims
    // Role is the RBAC role at time of token issuance.
    // If the user's role changes after issuance, the new role takes effect on next login.
    Role Role `json:"role"`
}

// Standard claim values set by the Issuer:
//   iss: server base URL (e.g., "https://pheromone.example.com")
//   sub: username
//   aud: []string{"pheromone"}
//   exp: time.Now().Add(config.TokenTTL)
//   iat: time.Now()
//   jti: uuid.NewString()  -- for Phase 3 revocation via deny-list
```

---

### 4. `Identity`

Runtime context object propagated through the gRPC interceptor chain.  Never persisted.

```go
// internal/auth/interceptor.go

// IdentityType distinguishes human operator sessions from machine agent sessions.
type IdentityType string

const (
    IdentityHuman IdentityType = "human"
    IdentityAgent IdentityType = "agent"
)

// Identity carries the resolved caller identity through the request context.
type Identity struct {
    // Type distinguishes human JWT sessions from agent mTLS sessions.
    Type IdentityType

    // Subject is the username (human) or agent certificate CN/SAN (agent).
    Subject string

    // Role is the RBAC role (human only; agents use a synthetic "agent" role
    // that bypasses human RBAC and flows directly to twin-level ACL).
    Role Role

    // TokenID is the JWT jti claim (human only; used in Phase 3 revocation checks).
    TokenID string

    // Provider is the AuthProvider that authenticated this session (human only).
    Provider string
}

// contextKey is the unexported key type used to store Identity in context.
type contextKey struct{}

func withIdentity(ctx context.Context, id Identity) context.Context {
    return context.WithValue(ctx, contextKey{}, id)
}

func identityFromContext(ctx context.Context) (Identity, bool) {
    id, ok := ctx.Value(contextKey{}).(Identity)
    return id, ok
}
```

---

### 5. `AuditEvent`

Structured log record emitted for every authentication and authorisation event.  Not persisted
in Phase 1 (written to stdout/structured log); Phase 3 adds etcd-backed audit trail.

```go
// internal/auth/audit.go

// AuditEvent is the schema of a single auth audit log entry.
// All fields map to logrus fields in the emitted JSON log line.
type AuditEvent struct {
    // Timestamp of the event (RFC3339Nano).
    Timestamp time.Time `logrus:"ts"`

    // EventType is the stable event key (e.g., "auth.login_success").
    EventType string `logrus:"event"`

    // Subject is the username or agent ID involved.
    Subject string `logrus:"sub"`

    // Role is the subject's role at event time (empty for pre-auth events).
    Role Role `logrus:"role,omitempty"`

    // SourceIP is the client IP address (gRPC peer address).
    SourceIP string `logrus:"source_ip,omitempty"`

    // Provider is the AuthProvider used ("local", "ldap", "oidc", "saml").
    Provider string `logrus:"provider,omitempty"`

    // RPCMethod is the gRPC full method name for access-control events.
    RPCMethod string `logrus:"rpc,omitempty"`

    // DenyReason is the human-readable reason for access_denied events.
    DenyReason string `logrus:"reason,omitempty"`

    // TokenID is the JWT jti claim (for session correlation across events).
    TokenID string `logrus:"jti,omitempty"`

    // Level is the log severity: "info", "warn", "debug".
    Level string `logrus:"level"`
}
```

**Invariant**: `AuditEvent` never contains raw passwords, tokens, or private keys.

---

### 6. `AuthConfig` (configuration schema)

The `auth` section added to the server YAML configuration file.

```go
// internal/config/config.go (extension)

type AuthConfig struct {
    // Provider selects the authentication backend.
    // "local" is the default; Phase 2 adds "ldap", "oidc", "saml".
    Provider string `yaml:"provider" default:"local"`

    // JWTAlgorithm selects the signing algorithm.
    // "rs256" (default, asymmetric) or "hs256" (symmetric).
    JWTAlgorithm string `yaml:"jwt_algorithm" default:"rs256"`

    // JWTPrivateKeyFile is the path to the PEM-encoded RSA private key (RS256 only).
    // File must exist and be mode 0600.
    JWTPrivateKeyFile string `yaml:"jwt_private_key_file"`

    // JWTPublicKeyFile is the path to the PEM-encoded RSA public key (RS256 only).
    JWTPublicKeyFile string `yaml:"jwt_public_key_file"`

    // JWTSecretFile is the path to the HS256 signing secret file (HS256 only).
    // File must exist and be mode 0600. Contains a raw 256-bit random secret.
    JWTSecretFile string `yaml:"jwt_secret_file"`

    // TokenTTL is the JWT token validity duration.
    // Default: 8h. Minimum: 5m. Maximum: 720h (30 days).
    TokenTTL time.Duration `yaml:"token_ttl" default:"8h"`

    // BcryptCost is the bcrypt work factor for password hashing.
    // Default: 12. Minimum: 10. Maximum: 16.
    BcryptCost int `yaml:"bcrypt_cost" default:"12"`

    // RateLimitLoginsPerMinute is the maximum Login RPCs per source IP per minute.
    // Default: 10. Set to 0 to disable rate limiting (not recommended).
    RateLimitLoginsPerMinute int `yaml:"rate_limit_logins_per_minute" default:"10"`

    // LDAP holds LDAP adapter configuration (Phase 2; ignored when provider != "ldap").
    LDAP *LDAPConfig `yaml:"ldap,omitempty"`

    // OIDC holds OIDC adapter configuration (Phase 2; ignored when provider != "oidc").
    OIDC *OIDCConfig `yaml:"oidc,omitempty"`

    // SAML holds SAML 2.0 adapter configuration (Phase 2; ignored when provider != "saml").
    SAML *SAMLConfig `yaml:"saml,omitempty"`

    // FallbackToLocal controls whether local credentials are tried when an external
    // IdP is unreachable. Default: false (fail closed).
    FallbackToLocal bool `yaml:"fallback_to_local" default:"false"`
}

// Phase 2 adapter configs — defined here for schema completeness; fields TBD in Phase 2 PRs.
type LDAPConfig struct {
    URL         string            `yaml:"url"`           // ldaps://ad.example.com:636
    BindDN      string            `yaml:"bind_dn"`
    BindPassFile string           `yaml:"bind_password_file"` // mode 0600
    UserBase    string            `yaml:"user_base_dn"`
    GroupBase   string            `yaml:"group_base_dn"`
    GroupRoleMap []GroupRoleRule  `yaml:"group_role_map"`
    InsecureSkipVerify bool       `yaml:"insecure_skip_verify"` // #nosec G402 — explicit opt-in
}

type GroupRoleRule struct {
    DN   string `yaml:"dn"`
    Role Role   `yaml:"role"`
}

type OIDCConfig struct {
    IssuerURL    string `yaml:"issuer_url"`
    ClientID     string `yaml:"client_id"`
    ClientSecret string `yaml:"client_secret_file"` // mode 0600
    RedirectURL  string `yaml:"redirect_url"`
    Scopes       []string `yaml:"scopes"` // default: ["openid", "profile", "email", "groups"]
}

type SAMLConfig struct {
    IDPMetadataURL string `yaml:"idp_metadata_url"`
    SPEntityID     string `yaml:"sp_entity_id"`
    SPCertFile     string `yaml:"sp_cert_file"`  // mode 0600
    SPKeyFile      string `yaml:"sp_key_file"`   // mode 0600
    AttributeMap   map[string]string `yaml:"attribute_map"` // SAML attr → UserAttributes field
}
```

---

## RBAC Permission Matrix

The static role-to-method mapping used by `RBACPolicy` in Phase 1.

| gRPC Full Method | viewer | operator | admin | owner | agent |
|-----------------|:------:|:--------:|:-----:|:-----:|:-----:|
| `AuthService/Login` | exempt | exempt | exempt | exempt | n/a |
| `AuthService/Logout` | ✅ | ✅ | ✅ | ✅ | n/a |
| `AuthService/ChangePassword` (own account) | ✅ | ✅ | ✅ | ✅ | n/a |
| `AuthService/ListUsers` | ❌ | ❌ | ✅ | ✅ | n/a |
| `AuthService/CreateUser` (role≤operator) | ❌ | ❌ | ✅ | ✅ | n/a |
| `AuthService/CreateUser` (role=admin) | ❌ | ❌ | ❌ | ✅ | n/a |
| `AuthService/UpdateUser` (own account) | ✅ | ✅ | ✅ | ✅ | n/a |
| `AuthService/UpdateUser` (other account, role≤operator) | ❌ | ❌ | ✅ | ✅ | n/a |
| `AuthService/UpdateUser` (other account, role=admin) | ❌ | ❌ | ❌ | ✅ | n/a |
| `AuthService/DeleteUser` (role≤operator) | ❌ | ❌ | ✅ | ✅ | n/a |
| `AuthService/DeleteUser` (role=admin) | ❌ | ❌ | ❌ | ✅ | n/a |
| `AgentRegistry/Register` | n/a | n/a | n/a | n/a | ✅ |
| `AgentRegistry/Heartbeat` | n/a | n/a | n/a | n/a | ✅ |
| `TwinControl/SyncTwinState` (read path) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `TwinControl/SyncTwinState` (write path) | ❌ | ✅ | ✅ | ✅ | ✅ |
| `TelemetryStream/StreamMetrics` (read) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `SkillService/ListSkills` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `SkillService/DeploySkill` | ❌ | ✅ | ✅ | ✅ | ✅ |
| `SkillService/DeleteSkill` | ❌ | ❌ | ✅ | ✅ | n/a |
| `SkillService/UpdateSkillPolicy` | ❌ | ❌ | ✅ | ✅ | n/a |

**Key**: ✅ allowed, ❌ denied, exempt = no auth required, n/a = not applicable to this identity type

**Notes**:
- `write path` for `TwinControl/SyncTwinState` = messages containing `ConfigAction` commands.
- Agent access bypasses the human RBAC gate entirely; the twin-level `AccessPolicy.Check()`
  (ADR-006) governs what a given agent certificate can do at the skill level.
- The permission table above is the Phase 1 static implementation.  Phase 3 replaces this with
  an OPA policy file while keeping the same `RBACPolicy.Check()` interface.

---

## Entity Relationship Diagram

```
┌───────────────────────────────┐
│         UserRecord            │
│ (etcd: /pheromone/users/<id>) │
│                               │
│  username (PK)                │
│  password_hash                │
│  role ──────────────────────┐ │
│  provider                   │ │
│  external_id                │ │
│  disabled                   │ │
│  created_at                 │ │
│  updated_at                 │ │
└───────────────────────────────┘
                                │
                    ┌───────────▼──────────┐
                    │        Role          │
                    │  viewer              │
                    │  operator (> viewer) │
                    │  admin (> operator)  │
                    │  owner (> admin)     │
                    └──────────────────────┘
                                │
                    maps to permission set via
                    ┌───────────▼──────────┐
                    │    RBACPolicy        │
                    │  method → min role   │
                    │  (static table P1;   │
                    │   OPA policy P3)     │
                    └──────────────────────┘

                         at login time
                    ┌───────────────────────┐
UserRecord ────────►│     JWTClaims         │
                    │  iss, sub, aud,       │
                    │  exp, iat, jti, role  │
                    └───────────────────────┘
                              │
                              │ on every RPC (gRPC metadata)
                    ┌─────────▼─────────────┐
                    │      Identity         │
                    │  type, subject,       │
                    │  role, token_id       │
                    │  (request context)    │
                    └───────────────────────┘
                              │
                     ┌────────▼────────────────┐
                     │      AuditEvent          │
                     │  (structured log line)   │
                     └──────────────────────────┘
```

---

## Storage Layout Summary

| Data | Backend | Key/Bucket | Encoding | TTL |
|------|---------|-----------|----------|-----|
| `UserRecord` | etcd | `/pheromone/users/<username>` | JSON | None (permanent until deleted) |
| `UserRecord` (fallback) | bbolt | bucket: `users`, key: `<username>` | JSON | None |
| JWT signing key (RS256) | File system | `server.auth.jwt_private_key_file` | PEM, mode 0600 | None |
| JWT secret (HS256) | File system | `server.auth.jwt_secret_file` | Raw bytes, mode 0600 | None |
| JWT tokens | Client-side only | gRPC metadata `authorization` | Bearer string | `exp` claim |
| Audit events (P1) | Structured log | stdout / log file | JSON (logrus) | OS log rotation |
| Audit events (P3) | etcd | `/pheromone/audit/auth/<jti>` | JSON | Configurable retention |
| JWT deny-list (P3) | etcd | `/pheromone/auth/revoked/<jti>` | JSON | TTL = original `exp` |

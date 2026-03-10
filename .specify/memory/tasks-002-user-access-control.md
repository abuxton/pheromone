# Tasks: User Access Control & Identity Provider Integration

**Feature**: `001-user-access-control`
**ADR**: [ADR-015](../../docs/adr/adr-015-user-access-control-identity-provider.md)
**Input**: [spec.md](spec.md) · [plan.md](plan.md) · [data-model.md](data-model.md) · [contracts/auth.proto](contracts/auth.proto)
**Generated**: 2026-03-24
**Phase 1 MVP Scope**: User Stories 1–3 (Bootstrap, User Management, Authentication)

---

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Parallelisable — different files, no blocking dependencies within phase
- **[US1–US5]**: User story traceability label
- **Effort**: Time estimate in engineer-hours
- **Labels**: `type:spike|feature|test|docs` · `phase:1|2|3`
- All file paths are relative to repository root

---

## Phase 1: Setup & Research Spikes

**Purpose**: Validate library choices, security assumptions, and lay the proto/config scaffolding
before any implementation work begins. All spike tasks can run in parallel.

---

- [ ] T001 [P] Research spike — JWT library selection and RS256 key-management strategy in `specs/001-user-access-control/research.md`
  - **Effort**: 4h
  - **Dependencies**: none
  - **Labels**: `type:spike` `phase:1`
  - **Acceptance Criteria**:
    - Confirm `golang-jwt/jwt` v5 has no open CVEs (run `govulncheck`)
    - Document comparison against `lestrrat-go/jwx` v2 and `go-jose/go-jose` v3
    - Validate RS256 token issue + validation round-trip in a throwaway Go test
    - Record bcrypt(cost=12) median latency on CI runner and arm64 target hardware
    - Output: updated `research.md` §R1–R4 with final decisions and benchmark figures

---

- [ ] T002 [P] Research spike — threat model for local auth (brute-force, token theft, bootstrap abuse) in `specs/001-user-access-control/research.md`
  - **Effort**: 3h
  - **Dependencies**: none
  - **Labels**: `type:spike` `phase:1`
  - **Acceptance Criteria**:
    - Identify top 5 threat vectors relevant to Phase 1 local auth
    - Map each threat to a mitigation already in the design (rate limiter, bcrypt, short TTL, etc.)
    - Flag any gap requiring a design change before implementation
    - Output: threat-model subsection appended to `research.md`

---

- [ ] T003 [P] Define `proto/pheromone/v1/auth.proto` — AuthService + all request/response messages
  - **Effort**: 4h
  - **Dependencies**: none
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Service definition includes: `Login`, `Logout`, `ChangePassword`, `ListUsers`, `CreateUser`, `UpdateUser`, `DeleteUser`
    - All messages include field numbers, proto comments, and validation annotations
    - File matches the contract defined in `specs/001-user-access-control/contracts/auth.proto`
    - `buf lint` passes with zero warnings
    - No breaking changes to existing `pheromone.v1` services (verified with `buf breaking`)

---

- [ ] T004 [P] Configure `buf generate` target for `auth.proto` and regenerate Go stubs in `internal/gen/pheromone/v1/`
  - **Effort**: 1h
  - **Dependencies**: T003
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `buf generate` produces `auth_grpc.pb.go` and `auth.pb.go` under `internal/gen/pheromone/v1/`
    - Generated code compiles with `go build ./...` (zero errors)
    - `buf.gen.yaml` updated if needed; new targets documented in Makefile

---

- [ ] T005 [P] Add `AuthConfig` struct to `internal/config/config.go` and wire into server config YAML schema
  - **Effort**: 2h
  - **Dependencies**: none
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `AuthConfig` matches the schema in `specs/001-user-access-control/data-model.md` §AuthConfig
    - Fields: `Provider`, `JWTAlgorithm`, `JWTPrivateKeyFile`, `JWTPublicKeyFile`, `JWTSecretFile`, `TokenTTL`, `BcryptCost`, `RateLimitLoginsPerMinute`
    - Default values applied via struct tags: `provider: local`, `jwt_algorithm: rs256`, `token_ttl: 8h`, `bcrypt_cost: 12`
    - Phase 2 stub sub-configs (`LDAPConfig`, `OIDCConfig`, `SAMLConfig`) defined but empty
    - `go vet ./internal/config/...` passes

---

**Checkpoint — Phase 1 Complete**: Research spikes done, proto stubs generated, config schema
defined. Phase 2 foundational work can now begin.

---

## Phase 2: Foundational Infrastructure

**Purpose**: Core data structures, persistence layer, and shared interfaces that every user story
depends on. Must be complete before any story-specific work begins.

⚠️ **CRITICAL**: No user-story phase (3, 4, or 5) can start until this phase is complete.

---

- [ ] T006 Implement `UserRecord` struct, `Role` type, validation helpers, and JSON serialisation in `internal/auth/store.go`
  - **Effort**: 3h
  - **Dependencies**: T005
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `UserRecord` matches data-model.md definition exactly (all fields, json tags, omitempty rules)
    - `Role` constants: `RoleViewer`, `RoleOperator`, `RoleAdmin`, `RoleOwner`
    - `ValidateUsername()` enforces 1–64 chars, `^[a-zA-Z0-9_-]+$` regex
    - `ValidatePassword()` enforces minimum 12 characters
    - `Role.IsValid()` returns false for unknown role strings
    - `go test ./internal/auth/... -run TestUserRecord` passes

---

- [ ] T007 Implement etcd CRUD for `UserRecord` — `CreateUser`, `GetUser`, `UpdateUser`, `DeleteUser`, `ListUsers` in `internal/auth/store.go`
  - **Effort**: 5h
  - **Dependencies**: T006
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Key pattern: `/pheromone/users/<username>` (lowercase normalised)
    - `CreateUser` returns `codes.AlreadyExists` if username taken (etcd compare-and-swap)
    - `GetUser` returns `codes.NotFound` for missing keys
    - `UpdateUser` uses etcd optimistic concurrency (no silent overwrites)
    - `ListUsers` uses prefix watch `/pheromone/users/`; returns paginated results
    - All operations use `context.Context`; cancelled contexts propagate correctly
    - Unit tests use etcd mock/in-memory client; no live etcd required for `go test`

---

- [ ] T008 [P] Implement bbolt fallback store satisfying the same `UserStore` interface in `internal/auth/store.go`
  - **Effort**: 3h
  - **Dependencies**: T007
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `bboltUserStore` satisfies the same `UserStore` interface as the etcd implementation
    - Bucket: `users`; key: `[]byte(username)`; value: JSON-encoded `UserRecord`
    - Selected automatically when etcd is unavailable (config `provider: local` + no etcd endpoint)
    - `go test ./internal/auth/... -run TestBboltStore` passes without a live etcd instance

---

- [ ] T009 [P] Implement `AuthProvider` interface and `UserAttributes` struct in `internal/auth/provider.go`
  - **Effort**: 2h
  - **Dependencies**: T005
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Interface methods: `Authenticate(ctx, username, password) (*UserAttributes, error)`, `UserAttributes(ctx, username) (*UserAttributes, error)`, `ProviderType() string`
    - `UserAttributes` struct: `Username`, `DisplayName`, `Email`, `Groups []string`
    - Interface is the single contract that all adapters (local, LDAP, OIDC, SAML) implement
    - File compiles; `go vet ./internal/auth/...` clean

---

- [ ] T010 [P] Implement `AuditEvent` struct and `AuditLogger` backed by logrus in `internal/auth/audit.go`
  - **Effort**: 3h
  - **Dependencies**: T006
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `AuditEvent` matches data-model.md definition (fields: `Timestamp`, `EventType`, `Subject`, `Role`, `SourceIP`, `Provider`, `RPCMethod`, `DenyReason`, `TokenID`, `Level`)
    - `AuditLogger.Emit(event AuditEvent)` writes a logrus JSON log line with all non-empty fields
    - Invariant: no raw passwords, no token strings, no private key material in any emitted field
    - Helper constructors for each event type in the taxonomy (e.g., `EmitLoginSuccess`, `EmitAccessDenied`)
    - Unit tests verify: (a) required fields present, (b) sensitive fields never appear in output

---

**Checkpoint — Phase 2 Complete**: Data layer, interfaces, and audit infrastructure are solid.
User story phases 3, 4, and 5 may now proceed — in parallel if team capacity allows.

---

## Phase 3: User Story 1 — Platform Owner Bootstrap (Priority: P1) 🎯 MVP Entry Point

**Goal**: A freshly deployed server detects zero users, seeds an `owner` account with a
one-time random password printed to stderr, and seals the bootstrap path permanently.

**Independent Test**: Start server with empty etcd, observe one-time password on stderr, authenticate
as that owner via `Login` RPC, confirm `codes.PermissionDenied` on a second bootstrap attempt.

---

- [ ] T011 [US1] Implement first-run bootstrap — detect zero users, seed owner account, print one-time password in `internal/auth/store.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `Bootstrap(ctx context.Context, store UserStore, logger AuditLogger) error` function
    - Check for existing users via `ListUsers`; skip if any user exists (idempotent)
    - Generate random username `admin` + 24-char cryptographically random password
    - Hash password with bcrypt (cost from config, default 12); store as `owner` role `UserRecord`
    - Print one-time credentials to stderr with `WARN` level: `"First-run bootstrap: username=admin password=<REDACTED_IN_LOGS>"`
    - Emit `AuditEvent{EventType: "auth.bootstrap_complete"}` via `AuditLogger`
    - `go test ./internal/auth/... -run TestBootstrap` passes against mock store

---

- [ ] T012 [US1] Implement bootstrap seal guard — reject re-run when users already exist in `internal/auth/store.go`
  - **Effort**: 2h
  - **Dependencies**: T011
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `Bootstrap()` returns `codes.FailedPrecondition` with message `"bootstrap already complete: users exist"` when any UserRecord is present
    - No account is created or modified when the guard fires
    - Guard check is atomic (single etcd transaction); no TOCTOU race
    - Unit test: pre-seed one user, call `Bootstrap()`, assert error and no new user created

---

- [ ] T013 [US1] Implement `pheromone user set-password <username>` interactive CLI in `cmd/cli/user.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T006
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Prompts for new password twice (no echo); validates they match and meet policy
    - Calls `AuthService.ChangePassword` gRPC or writes directly to store when called pre-server (bootstrap mode)
    - Rejects passwords shorter than 12 characters with a policy explanation
    - `--generate` flag generates and prints a strong random password without prompting
    - `pheromone user set-password --help` output documents all flags

---

- [ ] T014 [US1] Unit tests for bootstrap flow in `tests/unit/auth/bootstrap_test.go`
  - **Effort**: 2h
  - **Dependencies**: T011, T012
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - Test: fresh store → bootstrap → owner record created with correct role and bcrypt hash
    - Test: non-empty store → bootstrap → `FailedPrecondition` returned, no mutation
    - Test: bootstrap with weak password → rejected before store write
    - Test: audit event emitted on successful bootstrap
    - Coverage ≥ 80% for bootstrap code paths (`go test -coverprofile`)

---

**Checkpoint — US1 Complete**: First-run bootstrap works end-to-end. An `owner` account can
authenticate. US2 (user management) and US3 (auth sessions) can now proceed in parallel.

---

## Phase 4: User Story 2 — Administrator Manages Users and Assigns Roles (Priority: P1)

**Goal**: An admin/owner can create, read, update, and delete user accounts, assign roles, and
disable accounts. The last owner guard prevents accidental lockout.

**Independent Test**: With an owner account from US1, create an operator account, verify login,
change role to viewer, verify operator actions rejected, disable account, verify login fails.

---

- [ ] T015 [P] [US2] Implement `RBACPolicy` with static permission matrix and `Check(identity, method)` in `internal/auth/rbac.go`
  - **Effort**: 4h
  - **Dependencies**: T006 (Role type)
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `RBACPolicy` struct with method-to-minimum-role lookup map (data-model.md RBAC matrix)
    - `Check(identity Identity, method string) error` returns `nil` or `codes.PermissionDenied`
    - Special-case handling: agent `IdentityType` bypasses human RBAC entirely
    - Exempt methods list: `AuthService/Login`, `AgentRegistry/Register`, `AgentRegistry/Heartbeat`, health-check
    - Role hierarchy respected: `owner > admin > operator > viewer` (each inherits all lower-role permissions)
    - All 20 rows of the data-model.md permission table covered

---

- [ ] T016 [US2] Implement `AuthService.CreateUser` with RBAC pre-check and last-owner guard in `internal/auth/service.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T015, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Admin may create accounts with role ≤ operator; owner may create any role
    - Returns `codes.PermissionDenied` for insufficient role (e.g. admin creating admin)
    - Returns `codes.AlreadyExists` if username taken
    - Returns `codes.InvalidArgument` if password fails policy or username invalid
    - Emits `AuditEvent{EventType: "auth.user_created"}` on success
    - `go test ./internal/auth/... -run TestCreateUser` passes

---

- [ ] T017 [US2] Implement `AuthService.ListUsers` in `internal/auth/service.go`
  - **Effort**: 2h
  - **Dependencies**: T007, T015
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Requires `admin` or `owner` role; returns `codes.PermissionDenied` otherwise
    - Returns paginated list of `UserRecord` objects (password hashes NEVER included in response)
    - Optional filter by `Role` and `Disabled` status via request fields
    - `go test ./internal/auth/... -run TestListUsers` passes

---

- [ ] T018 [US2] Implement `AuthService.UpdateUser` — role changes, enable/disable — with privilege-escalation guard in `internal/auth/service.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T015, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Any role may update their own `DisplayName`; only admin/owner may change another user's fields
    - Admin may not promote a user to admin or higher; that requires owner
    - Disabling own account is rejected (`codes.InvalidArgument`)
    - Emits `AuditEvent{EventType: "auth.user_updated"}` including old and new role values
    - `go test ./internal/auth/... -run TestUpdateUser` passes

---

- [ ] T019 [US2] Implement `AuthService.DeleteUser` with last-owner guard in `internal/auth/service.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T015, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Admin may delete accounts with role ≤ operator; owner required to delete admin accounts
    - If deleting the last active owner: return `codes.FailedPrecondition` with `"at least one owner must remain"`
    - Last-owner check is atomic (etcd transaction); no TOCTOU race
    - Emits `AuditEvent{EventType: "auth.user_deleted"}`
    - Self-deletion returns `codes.InvalidArgument`
    - `go test ./internal/auth/... -run TestDeleteUser` passes

---

- [ ] T020 [P] [US2] Implement `pheromone user create`, `pheromone user list`, `pheromone user delete`, and `pheromone user set-role` CLI subcommands in `cmd/cli/user.go`
  - **Effort**: 4h
  - **Dependencies**: T016, T017, T018, T019
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `pheromone user create <username> --role <role> --password <file|->` creates a user via gRPC
    - `pheromone user list [--role <role>]` prints a table of users (no password hashes)
    - `pheromone user delete <username>` prompts for confirmation before calling `DeleteUser`
    - `pheromone user set-role <username> --role <role>` calls `UpdateUser`
    - All commands read `--server` flag or `PHEROMONE_SERVER` env var for gRPC endpoint
    - `--help` on each subcommand shows usage, flags, and examples

---

- [ ] T021 [P] [US2] Unit tests for RBAC permission matrix in `tests/unit/auth/rbac_test.go`
  - **Effort**: 3h
  - **Dependencies**: T015
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - Table-driven tests: every row in the data-model.md RBAC matrix has a passing test case
    - Tests cover: allowed path (correct role), denied path (insufficient role), agent bypass, exempt RPC list
    - Coverage ≥ 80% for `internal/auth/rbac.go`
    - `go test -race ./internal/auth/... -run TestRBAC` passes

---

- [ ] T022 [P] [US2] Unit tests for user management service handlers in `tests/unit/auth/service_test.go`
  - **Effort**: 4h
  - **Dependencies**: T016, T017, T018, T019
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - `CreateUser`: valid, AlreadyExists, privilege-escalation blocked, invalid username
    - `ListUsers`: admin sees all users, viewer gets PermissionDenied, password hash absent
    - `UpdateUser`: self-update (own display name), cross-user update (admin→operator), last-admin escalation blocked
    - `DeleteUser`: standard delete, last-owner guard, self-delete rejected
    - All tests use mock `UserStore`; no live etcd required
    - Coverage ≥ 80% for service handler code paths

---

**Checkpoint — US2 Complete**: Full user CRUD lifecycle works. Admin/owner can manage the entire
user directory. US3 (authentication) can proceed.

---

## Phase 5: User Story 3 — Operator Authenticates and Performs Role-Scoped Actions (Priority: P1)

**Goal**: Users log in with username/password, receive a signed JWT, and present it on every gRPC
call. The interceptor chain validates the token and enforces RBAC before the handler runs.

**Independent Test**: Create an operator account, log in via `Login` RPC, use returned token to call
`TwinControl.SyncTwinState` (allowed) and `AuthService.CreateUser` (PermissionDenied). Force token
expiry (short TTL config), verify Unauthenticated is returned. Verify agent mTLS path unaffected.

---

- [ ] T023 [US3] Implement JWT `Issuer` — RS256 key loading (PEM) + `IssueToken(user UserRecord) (string, error)` in `internal/auth/jwt.go`
  - **Effort**: 4h
  - **Dependencies**: T006, T005
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Loads RSA private key from `AuthConfig.JWTPrivateKeyFile` at startup; caches in memory
    - Fails fast with descriptive error if key file missing or not mode 0600
    - Issued token includes standard claims: `iss`, `sub`, `aud=["pheromone"]`, `exp`, `iat`, `jti` (UUID)
    - Custom claim: `role` (one of the four Role constants)
    - Token TTL from `AuthConfig.TokenTTL`; enforces minimum 5m and maximum 720h bounds
    - `go test ./internal/auth/... -run TestIssueToken` passes

---

- [ ] T024 [P] [US3] Implement JWT `Validator` — `ValidateToken(token string) (*Claims, error)` with RS256 verification in `internal/auth/jwt.go`
  - **Effort**: 3h
  - **Dependencies**: T023
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Loads RSA public key from `AuthConfig.JWTPublicKeyFile` (separate from private key)
    - Returns `codes.Unauthenticated` for: expired token, invalid signature, wrong algorithm, missing claims
    - Validates `aud` claim contains `"pheromone"`
    - Returns typed `*Claims` struct (not raw `map[string]interface{}`) on success
    - Hot-path: no file I/O; key cached in memory after first load
    - `go test ./internal/auth/... -run TestValidateToken` passes (valid, expired, tampered, wrong-alg cases)

---

- [ ] T025 [P] [US3] Implement HS256 fallback signing/validation in `internal/auth/jwt.go`
  - **Effort**: 2h
  - **Dependencies**: T023
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Selected when `AuthConfig.JWTAlgorithm == "hs256"`
    - Loads secret from `AuthConfig.JWTSecretFile`; validates file is ≥ 32 bytes and mode 0600
    - Same `IssueToken` / `ValidateToken` interface as RS256; callers are algorithm-agnostic
    - Returns `codes.InvalidArgument` at startup if algorithm value is not `"rs256"` or `"hs256"`
    - Unit tests: HS256 round-trip, HS256 token rejected by RS256 validator

---

- [ ] T026 [US3] Implement local bcrypt `AuthProvider` — `Authenticate()` validates bcrypt hash in `internal/auth/local/local.go`
  - **Effort**: 3h
  - **Dependencies**: T007, T009, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `ProviderType()` returns `"local"`
    - `Authenticate()` calls `bcrypt.CompareHashAndPassword`; returns `codes.Unauthenticated` on mismatch
    - Rejects login for `UserRecord.Disabled == true` with `codes.FailedPrecondition`
    - Never logs or returns the plaintext password in any error path
    - `UserAttributes()` reads from `UserStore`; returns current role and display name
    - `go test ./internal/auth/local/... -run TestLocalProvider` passes

---

- [ ] T027 [US3] Implement login rate limiter (10 req/min per source IP, token bucket) in `internal/auth/ratelimit.go`
  - **Effort**: 3h
  - **Dependencies**: T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Token-bucket rate limiter: 10 requests/minute per source IP (configurable via `AuthConfig.RateLimitLoginsPerMinute`)
    - Source IP extracted from `grpc.Peer` metadata in the context
    - Exceeding limit returns `codes.ResourceExhausted`
    - Emits `AuditEvent{EventType: "auth.login_rate_limited"}` when limit exceeded
    - Setting `RateLimitLoginsPerMinute: 0` disables rate limiting (not recommended; logs a startup warning)
    - `go test ./internal/auth/... -run TestRateLimit` passes (within limit, at limit, exceeded cases)

---

- [ ] T028 [US3] Implement `AuthService.Login` — credential validation, JWT issuance, rate limiting, audit events in `internal/auth/service.go`
  - **Effort**: 4h
  - **Dependencies**: T023, T026, T027, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Rate limiter applied before any bcrypt work (fast-path rejection)
    - Calls `AuthProvider.Authenticate()`; on success calls `Issuer.IssueToken()`
    - Returns `LoginResponse{Token: <jwt>, ExpiresAt: <timestamp>}` on success
    - Emits `AuditEvent{EventType: "auth.login_success"}` including role and source IP
    - Emits `AuditEvent{EventType: "auth.login_failure"}` on wrong credentials (no password in event)
    - `go test ./internal/auth/... -run TestLogin` passes (success, wrong-password, disabled-account, rate-limited)

---

- [ ] T029 [P] [US3] Implement `AuthService.Logout` — Phase 1 audit-only no-op in `internal/auth/service.go`
  - **Effort**: 1h
  - **Dependencies**: T028, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Extracts `jti` from the caller's JWT (via `Identity` in context)
    - Emits `AuditEvent{EventType: "auth.logout"}` with subject and token ID
    - Returns `LogoutResponse{}` (success); never fails for authenticated callers
    - Phase 3 TODO comment: `// TODO(phase3): add jti to etcd deny-list at /pheromone/auth/revoked/<jti>`
    - `go test ./internal/auth/... -run TestLogout` passes

---

- [ ] T030 [P] [US3] Implement `AuthService.ChangePassword` — current-password verification, bcrypt rehash, audit event in `internal/auth/service.go`
  - **Effort**: 2h
  - **Dependencies**: T026, T028, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Caller must prove current password (bcrypt verify) before setting a new one
    - Admin/owner may change any user's password without proving the current password (admin override path)
    - New password validated against policy (≥ 12 characters)
    - `UserRecord.PasswordChangedAt` updated on success
    - Emits `AuditEvent{EventType: "auth.password_changed"}`
    - `go test ./internal/auth/... -run TestChangePassword` passes

---

- [ ] T031 [US3] Implement `AuthUnaryInterceptor` and `AuthStreamInterceptor` — JWT extraction, validation, RBAC gate, identity propagation in `internal/auth/interceptor.go`
  - **Effort**: 5h
  - **Dependencies**: T024, T015, T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Extracts `Authorization: Bearer <token>` from gRPC metadata
    - Calls `Validator.ValidateToken()`; returns `codes.Unauthenticated` on failure
    - Detects agent mTLS path (peer certificate present, no Bearer token): sets `Identity{Type: IdentityAgent}`, skips human RBAC
    - Calls `RBACPolicy.Check(identity, fullMethod)` for human sessions; returns `codes.PermissionDenied` on denial
    - Stores resolved `Identity` in context via `withIdentity(ctx, id)` for downstream handlers
    - Emits `AuditEvent{EventType: "auth.access_denied"}` on RBAC failure including `RPCMethod` and `DenyReason`
    - Zero overhead on agent mTLS flows (interceptor short-circuits before any JWT parse)
    - `go test ./internal/auth/... -run TestInterceptor` passes (authenticated, unauthenticated, permission-denied, agent-bypass)

---

- [ ] T032 [US3] Wire auth interceptors, `AuthService`, and JWKS endpoint into `cmd/server/main.go`
  - **Effort**: 3h
  - **Dependencies**: T031, T028, T023, T005
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `grpc.NewServer()` call wraps both `AuthUnaryInterceptor` and `AuthStreamInterceptor` into the chain (after existing TLS interceptor from ADR-014)
    - `AuthService` handler registered via `pb.RegisterAuthServiceServer()`
    - Server loads `AuthConfig` at startup; fails fast with descriptive error if JWT key files missing
    - `GET /auth/jwks` HTTP endpoint returns RS256 public key as JWKS JSON document (for future federation)
    - Existing agent gRPC contracts continue to work (no regression)
    - `go build ./cmd/server/...` succeeds

---

- [ ] T033 [P] [US3] Implement `pheromone key generate` CLI command for RSA key pair generation in `cmd/cli/key.go`
  - **Effort**: 2h
  - **Dependencies**: T005
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `pheromone key generate --algorithm rs256 --bits <2048|4096> --private-key <path> --public-key <path>`
    - Generated files written with mode 0600 (private) and 0644 (public)
    - Refuses to overwrite existing key files without `--force` flag
    - Prints checksum of generated files for verification
    - `--help` output includes usage example matching quickstart.md §Step 1

---

- [ ] T034 [P] [US3] Unit tests for JWT service in `tests/unit/auth/jwt_test.go`
  - **Effort**: 4h
  - **Dependencies**: T023, T024, T025
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - RS256: valid token round-trip, expired token rejected, tampered signature rejected
    - RS256: wrong algorithm (HS256 token) rejected by RS256 validator
    - HS256: valid token round-trip, short secret rejected at startup
    - TTL enforcement: token with TTL < 5m rejected at issue time; TTL > 720h rejected
    - Claims structure: `role`, `sub`, `aud`, `jti` all present in issued token
    - Coverage ≥ 80% for `internal/auth/jwt.go`
    - `go test -race ./internal/auth/... -run TestJWT` passes

---

- [ ] T035 [P] [US3] Unit tests for interceptors in `tests/unit/auth/interceptor_test.go`
  - **Effort**: 4h
  - **Dependencies**: T031
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - Unary: valid JWT + permitted role → handler called
    - Unary: valid JWT + denied role → `codes.PermissionDenied`; audit event emitted
    - Unary: missing token → `codes.Unauthenticated`
    - Unary: expired token → `codes.Unauthenticated`
    - Streaming: same four cases as unary
    - Agent mTLS path: handler called; identity type is `IdentityAgent`; no JWT required
    - `Identity` correctly stored in context and readable by handler
    - Coverage ≥ 80% for `internal/auth/interceptor.go`

---

- [ ] T036 [US3] Integration smoke tests for the full interceptor chain in `tests/integration/auth_interceptor_test.go`
  - **Effort**: 6h
  - **Dependencies**: T032, T028, T036 (unit tests passing)
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - Spins up an in-process gRPC server with auth interceptors and a mock `TwinControl` handler
    - Test: valid JWT (operator role) → `TwinControl.SyncTwinState` (read) → `codes.OK`
    - Test: missing JWT → `codes.Unauthenticated`
    - Test: expired JWT → `codes.Unauthenticated`
    - Test: viewer role → `SkillService.DeleteSkill` → `codes.PermissionDenied`
    - Test: agent mTLS cert (no JWT) → `AgentRegistry.Register` → `codes.OK` (agent path unaffected)
    - Test: owner creates an admin, admin creates an operator, operator logs in (full lifecycle)
    - Tests run via `go test ./tests/integration/... -run TestAuthInterceptor`
    - All tests pass with `go test -race`

---

**Checkpoint — US3 Complete**: Full Phase 1 MVP delivered. Local auth + RBAC + JWT + interceptor
chain all working end-to-end. Coverage target ≥ 70% verified. Ready for audit review.

---

## Phase 6: User Story 4 — Administrator Configures External Identity Provider (Priority: P2)

**Goal**: Enterprise admins connect Pheromone to LDAP/AD, SAML 2.0, or OIDC provider. Users log
in with corporate credentials; group-to-role mapping applied automatically.

**Status**: Phase 2 — NOT part of Phase 1 MVP. Stub tasks defined here for planning continuity.

---

- [ ] T037 [P] [US4] Implement LDAP `AuthProvider` — bind auth, group search, group→role mapping in `internal/auth/ldap/ldap.go`
  - **Effort**: 8h
  - **Dependencies**: T009 (AuthProvider interface), T007 (UserStore for shadow records)
  - **Labels**: `type:feature` `phase:2`
  - **Acceptance Criteria**:
    - `ProviderType()` returns `"ldap"`
    - LDAP bind authentication using user DN + password
    - Group search and group→role mapping via `LDAPConfig.GroupRoleMap`
    - `ldaps://` and STARTTLS supported; plain `ldap://` blocked by default
    - Shadow `UserRecord` created/updated in etcd on first login
    - Emits `auth.idp_login_success` / `auth.idp_login_failure` audit events
    - Integration test against containerised OpenLDAP
    - `govulncheck` clean on `go-ldap/ldap` v3

---

- [ ] T038 [P] [US4] Implement OIDC `AuthProvider` — Authorization Code + PKCE, JWKS rotation in `internal/auth/oidc/oidc.go`
  - **Effort**: 8h
  - **Dependencies**: T009
  - **Labels**: `type:feature` `phase:2`
  - **Acceptance Criteria**:
    - OIDC Discovery document fetch + JWKS auto-rotation via `coreos/go-oidc` v3
    - Authorization Code flow with PKCE + `state` CSRF parameter
    - `GET /auth/oidc/callback` HTTP endpoint (aligned with ADR-011 HTTP listener)
    - `userinfo` endpoint claims → `UserAttributes.Groups`
    - Break-glass path: local owner account still works when OIDC provider unreachable
    - Integration test against containerised Keycloak

---

- [ ] T039 [P] [US4] Implement SAML 2.0 `AuthProvider` — SP metadata, ACS, attribute mapping in `internal/auth/saml/saml.go`
  - **Effort**: 8h
  - **Dependencies**: T009
  - **Labels**: `type:feature` `phase:2`
  - **Acceptance Criteria**:
    - SP metadata endpoint: `GET /auth/saml/metadata`
    - Assertion Consumer Service: `POST /auth/saml/acs`
    - Attribute mapping via `SAMLConfig.AttributeMap`
    - `crewjam/saml` or `russellhaering/gosaml2` — evaluate and record choice in `research.md` §R7 update
    - Configuration examples for Okta and ADFS in `docs/idp/`
    - Integration test using built-in test IdP

---

**Checkpoint — US4 Complete**: Enterprise SSO works. Users from LDAP, OIDC, and SAML IdPs can
authenticate and receive role-mapped session tokens.

---

## Phase 7: User Story 5 — Security Auditor Reviews Authentication and Authorization Events (Priority: P2)

**Goal**: Every auth/authz event is captured in a structured audit log with tamper-indication.
Auditors can query, filter, and export event records for compliance reporting.

**Status**: Phase 1 delivers structured stdout logging (T010); Phase 2 adds queryable audit trail.

---

- [ ] T040 [US5] Complete `AuditEvent` taxonomy — all event types, field coverage, emit helpers in `internal/auth/audit.go`
  - **Effort**: 3h
  - **Dependencies**: T010
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - All event types implemented: `auth.login_success`, `auth.login_failure`, `auth.login_rate_limited`, `auth.logout`, `auth.bootstrap_complete`, `auth.user_created`, `auth.user_updated`, `auth.user_deleted`, `auth.password_changed`, `auth.access_denied`, `auth.token_expired`
    - Each event type has a dedicated emit helper (e.g. `EmitLoginSuccess(ctx, subject, provider, sourceIP)`)
    - All helpers enforce the invariant: no raw passwords, no token strings, no private key material
    - Structured output parseable by `jq` for ad-hoc filtering
    - Unit tests: verify correct field values for each event type

---

- [ ] T041 [P] [US5] Integrate audit events across all auth operations — verify no gap in coverage in `internal/auth/`
  - **Effort**: 2h
  - **Dependencies**: T040, T028, T029, T030, T016, T019
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - Code review checklist: every `AuthService` handler emits at least one `AuditEvent`
    - Every `RBACPolicy.Check()` denial path emits `auth.access_denied`
    - `grep -r "AuditLogger.Emit" internal/auth/` shows coverage of all handlers
    - No audit event contains `password`, `token`, `secret`, or `key` as a value

---

- [ ] T042 [P] [US5] Validate audit log output format with `jq` query examples in `specs/001-user-access-control/quickstart.md`
  - **Effort**: 2h
  - **Dependencies**: T040, T041
  - **Labels**: `type:docs` `phase:1`
  - **Acceptance Criteria**:
    - Quickstart appendix includes `jq` one-liners for: filter by event type, filter by subject, count failures by source IP
    - All `jq` examples verified against real log output from integration tests
    - Output schema documented (field names, types, example values)

---

---

## Final Phase: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, security hardening, and coverage validation across all user stories.

---

- [ ] T043 [P] Generate RSA-2048 test key pair and commit to `internal/auth/testdata/`
  - **Effort**: 1h
  - **Dependencies**: T033
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `internal/auth/testdata/rsa_test_key.pem` (private, mode 0600) and `rsa_test_key_pub.pem` (public) committed
    - Files are test-only keys — clearly labelled with a `DO NOT USE IN PRODUCTION` header comment
    - Used by all JWT unit and integration tests (no dynamic generation needed in test hot path)

---

- [ ] T044 [P] Update `SECURITY.md` — document Phase 1 auth implementation, close the "gap" items from ADR-014 in `SECURITY.md`
  - **Effort**: 2h
  - **Dependencies**: T032
  - **Labels**: `type:docs` `phase:1`
  - **Acceptance Criteria**:
    - ADR-014 "open item" for user auth explicitly marked as closed with link to ADR-015
    - Sections added: Credential Policy, JWT Token Lifecycle, RBAC Model, Audit Events, Key Management
    - Rate limiting and brute-force protection documented
    - Phase 2 (IdP) and Phase 3 (revocation, OPA) roadmap items listed

---

- [ ] T045 [P] Update `docs/adr/INDEX.md` — add ADR-015 entry with status, date, and links in `docs/adr/INDEX.md`
  - **Effort**: 30m
  - **Dependencies**: none
  - **Labels**: `type:docs` `phase:1`
  - **Acceptance Criteria**:
    - ADR-015 row added to the index table with: number, title, status (Proposed → Accepted post-review), date, derived-from ADRs
    - Link to `adr-015-user-access-control-identity-provider.md` is correct
    - `markdownlint` passes on updated file

---

- [ ] T046 [P] Update `README.md` — add Authentication & Access Control section with quickstart link in `README.md`
  - **Effort**: 1h
  - **Dependencies**: T044
  - **Labels**: `type:docs` `phase:1`
  - **Acceptance Criteria**:
    - New §Authentication section covers: how to generate keys, how to bootstrap, how to log in via CLI
    - Links to `specs/001-user-access-control/quickstart.md` for full operator guide
    - Links to `docs/adr/adr-015-user-access-control-identity-provider.md` for architecture reference

---

- [ ] T047 Run `govulncheck` on all new dependencies; resolve any findings in `go.mod` / `go.sum`
  - **Effort**: 1h
  - **Dependencies**: T023, T026 (new deps imported)
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `govulncheck ./...` exits 0 on CI
    - Any findings documented in PR description with resolution or accepted-risk justification
    - Specifically validates: `golang-jwt/jwt` v5, `golang.org/x/crypto`

---

- [ ] T048 Run `gosec` on `internal/auth/` and resolve all new findings in `internal/auth/`
  - **Effort**: 1h
  - **Dependencies**: T031, T032
  - **Labels**: `type:feature` `phase:1`
  - **Acceptance Criteria**:
    - `gosec ./internal/auth/...` exits 0 with zero new findings vs. baseline
    - Any `#nosec` annotations include a comment justifying the exception
    - Specifically checks for: hardcoded credentials, insecure TLS configs, weak random usage

---

- [ ] T049 Verify `internal/auth/` unit test coverage ≥ 70% (target 80%) via coverage report
  - **Effort**: 2h
  - **Dependencies**: T014, T021, T022, T034, T035
  - **Labels**: `type:test` `phase:1`
  - **Acceptance Criteria**:
    - `go test -coverprofile=coverage.out ./internal/auth/...` and `go tool cover -func=coverage.out`
    - Total coverage for `internal/auth/` package ≥ 70% (hard gate); ≥ 80% (target)
    - Any uncovered paths documented with rationale (e.g. unreachable error branches)
    - Coverage report artifact uploaded in CI

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup & Spikes)
    └── Phase 2 (Foundational) ─ BLOCKS everything below
            ├── Phase 3 (US1: Bootstrap)    ─ BLOCKS US2 (admin user management needs an owner to exist)
            │       └── Phase 4 (US2: User Management)   ─ can proceed in parallel with Phase 5
            ├── Phase 5 (US3: Authentication & Interceptors)  ─ can proceed after Phase 2
            ├── Phase 6 (US4: External IdP) ─ Phase 2 only; depends on Phase 5 complete
            └── Phase 7 (US5: Audit Review) ─ overlaps Phase 5 (audit logger already in Phase 2)
Final Phase (Polish & Docs) ─ depends on all Phase 1 MVP phases (3, 4, 5) complete
```

### User Story Dependencies

| Story | Depends On | Parallel With | Notes |
|-------|-----------|---------------|-------|
| US1 (Bootstrap) | Phase 2 complete | — | Entry point for all others |
| US2 (User Mgmt) | US1 complete | US3 | Needs owner account from US1 |
| US3 (Auth) | Phase 2 complete | US2 | Core JWT + interceptor work; independent of US2 |
| US4 (IdP) | US3 complete | — | Phase 2; builds on auth provider interface |
| US5 (Audit) | Phase 2 (AuditLogger) | All stories | Audit events emitted throughout; review story is additive |

### Critical Path to Phase 1 MVP

```
T001,T002 (spikes) → T003 (proto) → T004 (codegen) → T005 (config)
    → T006 (UserRecord) → T007 (etcd CRUD) → T009 (RBAC skeleton) → T010 (AuditLogger)
    → T011 (Bootstrap) → T015 (RBACPolicy) → T023 (JWT Issuer) → T026 (LocalProvider)
    → T027 (RateLimit) → T028 (Login RPC) → T031 (Interceptors) → T032 (Server wiring)
    → T036 (Integration tests) → T049 (Coverage gate) → T047 (govulncheck) → T048 (gosec)
```

**Estimated Phase 1 MVP total**: ~95 engineer-hours  
**Critical path length** (sequential): ~42 engineer-hours  
**Parallelisable savings**: ~53 engineer-hours (with 2–3 engineers working in parallel)

---

## Parallel Execution Examples

### Phase 2 Parallelism (Foundational)

```bash
# All can start after T005 (AuthConfig) is done:
Task T006: UserRecord struct + validation          (internal/auth/store.go)
Task T009: AuthProvider interface                  (internal/auth/provider.go)
Task T010: AuditEvent + AuditLogger                (internal/auth/audit.go)
```

### Phase 5 Parallelism (Authentication)

```bash
# After T023 (JWT Issuer) is done:
Task T024: JWT Validator                           (internal/auth/jwt.go)
Task T025: HS256 fallback                          (internal/auth/jwt.go - different function group)
Task T034: JWT unit tests                          (tests/unit/auth/jwt_test.go)
```

### Polish Phase Parallelism

```bash
# All can run in parallel after Phase 1 MVP is code-complete:
Task T043: Generate test key pair
Task T044: Update SECURITY.md
Task T045: Update ADR INDEX.md
Task T046: Update README.md
Task T047: govulncheck
Task T048: gosec
```

---

## Implementation Strategy

### MVP First (Stories 1–3 Only)

1. Complete Phase 1: Setup & Spikes (T001–T005)
2. Complete Phase 2: Foundational (T006–T010) — blocks everything
3. Complete Phase 3: US1 Bootstrap (T011–T014)
4. Run US1 checkpoint: fresh-server bootstrap works end-to-end
5. Complete Phase 4 (US2) + Phase 5 (US3) in parallel
6. Run integration smoke tests (T036)
7. **STOP and VALIDATE**: full Phase 1 MVP independent of Phase 2 IdP work
8. Polish (Final Phase) before merging to main

### Incremental Delivery

| Increment | Stories Delivered | Validates |
|-----------|------------------|-----------|
| After Phase 3 | US1 only | Bootstrap + owner account |
| After Phase 4 | US1 + US2 | Full user lifecycle |
| After Phase 5 | US1–US3 | **Phase 1 MVP complete** |
| After Phase 6 | US1–US4 | Enterprise SSO |
| After Phase 7 | US1–US5 | Full compliance audit trail |

### Parallel Team Strategy (2 engineers)

Once Phase 2 (Foundational) is complete:
- **Engineer A**: US2 (User Management) — T015 → T016 → T017 → T018 → T019 → T020 → T021 → T022
- **Engineer B**: US3 (Authentication) — T023 → T024 → T026 → T028 → T031 → T032 → T034 → T035
- Both merge to feature branch; T036 (integration tests) run after both threads complete

---

## Task Summary

| Phase | Story | Tasks | Effort | Type |
|-------|-------|-------|--------|------|
| 1 — Setup & Spikes | — | T001–T005 | 14h | spike + feature |
| 2 — Foundational | — | T006–T010 | 16h | feature |
| 3 — Bootstrap | US1 | T011–T014 | 10h | feature + test |
| 4 — User Management | US2 | T015–T022 | 26h | feature + test |
| 5 — Authentication | US3 | T023–T036 | 39h | feature + test |
| 6 — External IdP | US4 | T037–T039 | 24h | feature (phase 2) |
| 7 — Audit Trail | US5 | T040–T042 | 7h | feature + docs |
| Final — Polish | — | T043–T049 | 9h | docs + test |
| **Total** | | **49 tasks** | **~145h** | |
| **Phase 1 MVP** | US1–US3 | T001–T036 + T040–T041 + T043–T049 | **~115h** | |

---

## Notes

- `[P]` tasks touch different files and have no intra-phase blocking dependencies; safe to run concurrently
- `[US#]` label maps each task to its user story for traceability and independent testability
- Phase 2 tasks (T037–T039) are listed for planning completeness; do NOT implement during Phase 1 MVP
- Coverage gate (T049) is a hard blocker for PR merge: `internal/auth/` must reach ≥ 70%
- Security scans (T047, T048) must be clean before merge; no new `gosec` findings
- JWT test keys in `testdata/` must never be used outside of `go test` contexts
- Bootstrap credentials printed to stderr must be changed immediately post-deploy (documented in quickstart.md)

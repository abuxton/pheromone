# Feature Specification: User Access Control & Identity Provider Integration

**Feature Branch**: `001-user-access-control`
**Created**: 2026-03-10
**Status**: Draft
**Derived From**: ADR-003 (gRPC Contracts), ADR-006 (Agent Lifecycle/RBAC), ADR-014 (Security Architecture), Spec-001 User Story 5
**Input**: "User Access Control and Identity Provider Integration for the Pheromone digital twin platform — including username:password management, RBAC (owner/admin/operator/viewer), extensible IdP integration (LDAP, SAML 2.0, SCIM, OAuth 2.0/OIDC), gRPC interceptor integration, session management, and audit logging"

---

## Context & Motivation

Pheromone's ADR-014 security architecture (Phase 2) explicitly deferred application-layer authentication and authorization via gRPC interceptors. Spec-001 User Story 5 ("Multi-Tenant Twin Namespacing with RBAC") was also designated a Phase 2 concern. This specification formalises both deferred items as a cohesive feature, establishing the access control foundation required before Pheromone can be deployed in shared or production environments where multiple operators manage overlapping fleets of agents and twins.

The feature must integrate with the interceptor hook points described in ADR-014 (D2) and complement the mTLS transport security already specified for agent↔server communication in ADR-003. User-facing access control operates at the application layer, above mTLS, and governs what authenticated human users and service accounts are permitted to do once connected.

---

## User Scenarios & Testing

### User Story 1 — Platform Owner: Bootstrap Initial Admin Access (Priority: P1)

When a Pheromone server is deployed for the first time, there are no user accounts. The platform owner (the person or automation responsible for initial deployment) needs to create the first privileged account using a username and password, from which all subsequent user management flows.

**Why this priority**: Without a bootstrap path, no user can log in to perform any management task. This is the entry point for the entire access control model and must be delivered first.

**Independent Test**: Deploy a fresh Pheromone server with no pre-existing accounts. Run the bootstrap command (or first-run wizard) with a chosen username and password. Verify the resulting account has the Owner role and can successfully authenticate and call a protected gRPC endpoint. All other stories depend on this working first.

**Acceptance Scenarios**:

1. **Given** a freshly deployed Pheromone server with no user accounts, **When** the operator runs the bootstrap command with a valid username and strong password, **Then** an Owner account is created, the bootstrap path is sealed (cannot be re-run), and the account can authenticate immediately
2. **Given** an Owner account that exists, **When** someone attempts to re-run the bootstrap command, **Then** the system rejects it with a clear error explaining that bootstrapping is already complete
3. **Given** a bootstrap attempt with a password that does not meet the minimum strength requirement, **When** the command is submitted, **Then** the system rejects it with an explanation of the unmet criteria, and no account is created

---

### User Story 2 — Administrator: Manage Users and Assign Roles (Priority: P1)

An administrator needs to create user accounts for each team member, assign appropriate roles (Owner, Admin, Operator, or Viewer), and manage the full account lifecycle — including modifying roles, disabling compromised accounts, and removing departed users.

**Why this priority**: Role assignment is the core of the RBAC model. Without it, all users share the same permissions, which is unsuitable for any shared deployment.

**Independent Test**: With an existing Admin account, create a new Operator account, verify it can perform operator-permitted actions, then modify it to Viewer, verify the operator actions are now rejected, then disable the account, verify all further logins fail. The full lifecycle can be exercised against a single test account without any external dependencies.

**Acceptance Scenarios**:

1. **Given** an authenticated Admin user, **When** they create a new user account and assign the Operator role, **Then** the account is active and the user can log in with the assigned role immediately
2. **Given** an Operator account, **When** an Admin changes the role to Viewer, **Then** operations requiring Operator permission are rejected for that user within 5 seconds of the change
3. **Given** an Admin disabling a user account, **When** the disabled user attempts to authenticate, **Then** authentication is rejected with a clear "account disabled" message and the event is recorded in the audit log
4. **Given** only one Owner account in the system, **When** an Admin attempts to delete or demote that account, **Then** the system refuses and explains that at least one Owner must always exist
5. **Given** a Viewer attempting to create a new user, **When** the request is submitted, **Then** the system rejects it with `PERMISSION_DENIED` and logs the attempt

---

### User Story 3 — Operator: Authenticate and Perform Role-Scoped Actions (Priority: P1)

An operator authenticates with their username and password, receives a session credential, and uses it to interact with the Pheromone API for permitted actions — such as pushing configuration to agents or inspecting twin state — while being blocked from admin operations like user management.

**Why this priority**: This is the everyday access path for the most common user type. The session credential mechanism must be proven before building IdP integration on top of it.

**Independent Test**: Create an Operator account, log in via the authentication endpoint, receive a session token, call a twin configuration push endpoint (should succeed), call a user management endpoint (should be rejected with `PERMISSION_DENIED`). Token expiry behaviour can be tested in isolation by setting a short TTL.

**Acceptance Scenarios**:

1. **Given** a valid username and password for an Operator account, **When** the user authenticates, **Then** a session token is returned with a stated expiry time
2. **Given** an Operator with a valid session token, **When** they call the `TwinControl.SyncTwinState` gRPC method, **Then** the call succeeds and the twin operation is performed
3. **Given** an Operator with a valid session token, **When** they call a user management RPC (e.g., `CreateUser`), **Then** the call is rejected with `PERMISSION_DENIED` and an audit event is recorded
4. **Given** a session token that has expired, **When** a user presents it in a gRPC call, **Then** the call is rejected with `UNAUTHENTICATED` and the user must re-authenticate
5. **Given** a valid session token within its active window, **When** the user requests a token refresh, **Then** a new token with a fresh expiry is issued without requiring the password again

---

### User Story 4 — Administrator: Configure an External Identity Provider (Priority: P2)

An enterprise administrator needs to connect Pheromone to an existing corporate identity system (LDAP directory, SAML 2.0 identity provider, or OAuth 2.0/OIDC provider) so that employees can log in with their existing corporate credentials without a separate Pheromone password. The administrator also maps external groups to Pheromone roles.

**Why this priority**: Enterprise adoption requires SSO. Without IdP integration, every corporate deployment requires a parallel user directory, increasing operational overhead and security risk. This builds on the working local auth from Stories 1–3.

**Independent Test**: Configure a test LDAP server (or mock OIDC provider) via the Pheromone admin interface. Map an LDAP group to the Operator role. Authenticate as an LDAP user and verify the correct role is applied. Verify that a local break-glass account still works when the LDAP server is unreachable.

**Acceptance Scenarios**:

1. **Given** an Admin who has configured an LDAP IdP with a group-to-role mapping, **When** an LDAP user in the mapped group authenticates, **Then** they receive a session token bearing the mapped Pheromone role
2. **Given** a configured SAML 2.0 IdP, **When** a user completes the SAML SSO flow, **Then** Pheromone creates or updates their local profile and issues a session token
3. **Given** a configured OIDC provider, **When** a user completes the OAuth 2.0 authorization code flow, **Then** Pheromone accepts the OIDC ID token and issues a session token bearing the role mapped from the user's claims
4. **Given** an external IdP is temporarily unavailable, **When** a locally stored Owner account attempts to authenticate via username/password, **Then** the login succeeds (break-glass path remains available)
5. **Given** an LDAP user whose group membership has been removed (i.e., they no longer map to any Pheromone role), **When** they attempt to authenticate, **Then** the login is rejected with a "no role mapping found" message
6. **Given** multiple IdP configurations active simultaneously (e.g., LDAP for internal users, OIDC for contractors), **When** users from each IdP authenticate, **Then** each user receives the correct role from their respective IdP mapping

---

### User Story 5 — Security Auditor: Review Authentication and Authorization Events (Priority: P2)

A security auditor or compliance officer needs a complete, tamper-evident record of every authentication and authorization event — who logged in, from where, what operations were attempted, whether they were permitted or denied — to support incident investigation and regulatory compliance.

**Why this priority**: Audit logging is a hard requirement for enterprise and regulated environments. It also provides the feedback loop for detecting misuse and validating that RBAC enforcement is working correctly.

**Independent Test**: Perform a series of login attempts (successful and failed), role-restricted API calls (permitted and denied), and a role change. Retrieve the audit log and verify all events are present with correct fields. Attempt to delete or modify an audit entry and verify the system rejects it.

**Acceptance Scenarios**:

1. **Given** a successful authentication event, **When** the audit log is queried, **Then** an entry exists with: timestamp, username, source IP, authentication method (local/LDAP/SAML/OIDC), and outcome (success)
2. **Given** a failed authentication attempt, **When** the audit log is queried, **Then** an entry exists with the attempted username, source IP, failure reason, and timestamp — and no sensitive credential data is included
3. **Given** a `PERMISSION_DENIED` response to a gRPC call, **When** the audit log is queried, **Then** an entry records the user identity, the RPC method attempted, the resource targeted, and the denial reason
4. **Given** a role change event, **When** the audit log is queried, **Then** an entry records the Admin who made the change, the affected user, the old role, the new role, and the timestamp
5. **Given** an audit log entry, **When** an Admin attempts to delete or modify it, **Then** the system rejects the operation — audit records are read-only once written

---

### User Story 6 — DevOps Team: Multi-Tenant Twin Namespace Access Scoping (Priority: P3)

A DevOps team manages multiple environments (development, staging, production) within a shared Pheromone deployment. Each team member should only see and act on twins in the namespaces assigned to their role. A staging operator must not be able to view or modify production twins, even if they are a valid Pheromone user.

**Why this priority**: Multi-tenancy is required for enterprise deployments managing multiple environments or business units from a single server instance. This completes Spec-001 User Story 5 and is the most complex story; it builds on all previous stories.

**Independent Test**: Create two namespaces (staging, production). Assign a user to the Operator role scoped to the staging namespace only. Verify that twin queries and configuration pushes to staging succeed, while the same operations targeting production twins return `PERMISSION_DENIED`. The namespace scope must be enforced independently of the role level.

**Acceptance Scenarios**:

1. **Given** a user with Operator role scoped to the staging namespace, **When** they query twin state for a production twin, **Then** the system returns `PERMISSION_DENIED` and records the attempt in the audit log
2. **Given** a user with Operator role scoped to the staging namespace, **When** they push a configuration change to a staging twin, **Then** the operation succeeds
3. **Given** an Owner (unrestricted), **When** they query or modify any twin regardless of namespace, **Then** all operations succeed — Owners are not namespace-scoped
4. **Given** a Viewer scoped to the production namespace, **When** they attempt to push a configuration change to a production twin, **Then** the operation is rejected due to insufficient role — namespace access does not grant write permission beyond the role level

---

### Edge Cases

- **What happens when an agent presents a valid mTLS certificate but no application-layer session token?** The gRPC interceptor MUST reject the call with `UNAUTHENTICATED`. mTLS authenticates the transport channel; the application token authenticates the user or service account. Both are required.
- **What happens when a session token is valid but the user's account has been disabled since issuance?** Token validation MUST check current account status; a token for a disabled account MUST be rejected with `UNAUTHENTICATED`. Account state is authoritative over token contents.
- **What happens when an LDAP group mapping changes and an already-authenticated user is mid-session?** The role change takes effect on the next token issuance or refresh. In-flight requests with the old token continue to their expiry (up to configurable maximum). Admins can force immediate revocation for high-urgency situations.
- **What happens if the only Owner account's password is lost?** An out-of-band recovery path (e.g., a signed recovery token generated at bootstrap time, or direct server-side CLI command) MUST exist to reset access without deleting audit history.
- **What happens when two IdP sources return conflicting role claims for the same user (e.g., user is in both LDAP and OIDC)?** The system MUST apply a deterministic precedence rule (configurable per deployment; default: highest-privilege role wins). The resolution is recorded in the audit log.
- **What happens if the audit log storage is full or unavailable?** Authentication and authorization MUST continue to function. Audit events that cannot be persisted MUST be written to the server's stderr log as a fallback; a health alert MUST be raised.
- **What happens when a SCIM provisioning request attempts to create a user that already exists locally?** The system MUST reconcile by updating the existing account rather than creating a duplicate, and log the reconciliation event.

---

## Requirements

### Functional Requirements

#### User Account Management

- **FR-001**: System MUST support full lifecycle management of user accounts: create, read, update (role, display name, credential), and deactivate (soft-delete; account history and audit trail are preserved)
- **FR-002**: System MUST enforce globally unique usernames within a deployment
- **FR-003**: System MUST require passwords for local accounts to meet a configurable minimum strength policy; the out-of-the-box default MUST enforce at least 12 characters with mixed character classes
- **FR-004**: System MUST store local passwords using an industry-standard adaptive hashing algorithm; plaintext or reversibly encrypted passwords MUST NOT be stored anywhere in the system
- **FR-005**: System MUST provide a first-run bootstrap path that creates the initial Owner account; this path MUST be sealed after first use
- **FR-006**: System MUST prevent the deactivation, deletion, or role-demotion of the last remaining Owner account
- **FR-007**: System SHOULD support SCIM 2.0 for automated user provisioning and deprovisioning from external directories, so that account lifecycle follows the source-of-truth IdP without manual synchronisation

#### Role-Based Access Control

- **FR-008**: System MUST implement exactly four built-in roles with the following capability hierarchy (each role includes all capabilities of the roles below it):
  - **Owner**: unrestricted access including server configuration, IdP setup, and Owner-level account management
  - **Admin**: full user and role management; cannot modify Owner accounts or server-level configuration
  - **Operator**: read and write access to twin models, agent configuration, and configuration enforcement; no user management
  - **Viewer**: read-only access to twin state, metrics, and audit logs; no write operations
- **FR-009**: System MUST enforce RBAC on every gRPC RPC call via the ADR-014 interceptor chain; no RPC handler MUST execute before the caller's role and permissions are validated
- **FR-010**: System MUST support namespace-scoped role assignments — a user may hold a role restricted to one or more twin namespaces, limiting their access to twins within those namespaces only
- **FR-011**: System MUST allow an Owner or Admin to assign, modify, or revoke role assignments for any non-Owner user account at any time
- **FR-012**: System MUST return gRPC status `PERMISSION_DENIED` (per ADR-014 D2) for any request where the caller's role does not permit the requested action, regardless of the validity of their session token

#### Session Management

- **FR-013**: System MUST issue a stateless, cryptographically signed session token upon successful authentication (local or via IdP); the token encodes user identity, role(s), namespace scopes, and expiry time
- **FR-014**: Session token expiry MUST be configurable at the server level; the default MUST be 8 hours for interactive sessions
- **FR-015**: System MUST support session token refresh within the active window, issuing a new token with a fresh expiry without requiring re-entry of the password
- **FR-016**: System MUST support explicit token revocation (logout), causing the revoked token to be rejected on subsequent requests within seconds of revocation
- **FR-017**: Revoked or expired tokens MUST be rejected with gRPC status `UNAUTHENTICATED` (per ADR-014 D2)
- **FR-018**: Session tokens MUST be transmitted as gRPC metadata on each call; embedding tokens in message payloads MUST NOT be used as the primary mechanism

#### Identity Provider Integration

- **FR-019**: System MUST support local username/password authentication as the always-available baseline identity provider; this path MUST remain functional even when external IdPs are configured or unavailable
- **FR-020**: System MUST support LDAP / Active Directory integration, allowing users in a configured directory to authenticate with their directory credentials
- **FR-021**: System MUST support SAML 2.0 Service Provider integration, enabling federated SSO with enterprise identity systems
- **FR-022**: System MUST support OAuth 2.0 / OIDC integration, accepting ID tokens from configured OIDC providers (including cloud identity providers such as Google Workspace, Azure AD, Okta)
- **FR-023**: System MUST provide an IdP group/role mapping configuration that translates external directory groups or OIDC claims to Pheromone roles; unmapped identities MUST be denied access
- **FR-024**: System MUST allow multiple IdP configurations to be active simultaneously, each with its own group mapping rules
- **FR-025**: System MUST record which IdP was used for authentication in every audit log entry
- **FR-026**: When an external IdP is misconfigured or unreachable, the local authentication path MUST remain available as a break-glass mechanism; the system MUST log and alert on IdP connectivity failures

#### Audit Logging

- **FR-027**: System MUST write an immutable audit log entry for every authentication event: successful login, failed login, logout, token refresh, and token revocation — including timestamp, username, source IP, authentication method, and outcome
- **FR-028**: System MUST write an immutable audit log entry for every authorization decision where access is denied: RPC method attempted, user identity, role at time of attempt, resource or namespace targeted, and denial reason
- **FR-029**: System MUST write an immutable audit log entry for every administrative action: user creation, role assignment change, account deactivation, IdP configuration change — including the acting Admin's identity, the target, old and new values, and timestamp
- **FR-030**: Audit log entries MUST NOT contain secret material (passwords, token values, private keys)
- **FR-031**: Audit log retention period MUST be configurable; the default MUST be 90 days; entries within the retention window MUST NOT be deletable by any user through the application API
- **FR-032**: Audit logs MUST be queryable by time range, user identity, event type, and outcome to support incident investigation

#### gRPC Interceptor Integration (ADR-014 D2)

- **FR-033**: The authentication interceptor MUST execute as the first interceptor in the gRPC unary and streaming interceptor chains, before any authorisation or business logic
- **FR-034**: The authorisation interceptor MUST execute immediately after the authentication interceptor and before any RPC handler, enforcing the caller's role against the requested method and resource
- **FR-035**: Interceptor rejection MUST return the appropriate gRPC status code without invoking the downstream handler or leaking internal state
- **FR-036**: The interceptor framework MUST propagate the verified user identity and role into the RPC request context so that downstream handlers can reference them for resource-level decisions (e.g., namespace filtering)

---

### Key Entities

- **User**: Represents an authenticated identity on the platform. Attributes: unique username, display name, account status (active/disabled), authentication type (local/LDAP/SAML/OIDC), creation timestamp, last login timestamp. A User is distinct from an Agent (which authenticates via mTLS per ADR-003).

- **Role**: A named, built-in permission profile. The four roles form a strict hierarchy: Owner → Admin → Operator → Viewer. Roles are not user-defined; the set is fixed. Attributes: role name, ordered privilege level, list of permitted RPC methods/resource operations.

- **RoleAssignment**: A binding of a User to a Role, with an optional namespace scope. A user may have multiple RoleAssignments (e.g., Operator in namespace "staging", Viewer in namespace "production"). Attributes: user reference, role, namespace scope (null = unrestricted), assigned-by (Admin user reference), assigned-at timestamp.

- **SessionToken**: A stateless, signed credential representing a successfully authenticated session. Encodes: user identity, effective roles and namespace scopes, authentication method, issued-at and expiry timestamps. Not stored server-side (stateless design), but the revocation list tracks explicitly revoked token identifiers.

- **IdentityProvider**: Configuration record for a connected external IdP. Attributes: unique name, type (LDAP/SAML/OIDC/SCIM), connection parameters (host, port, base DN, client ID, metadata URL, etc. — all sensitive fields encrypted at rest), status (active/disabled), group-to-role mapping rules.

- **GroupRoleMapping**: A rule within an IdentityProvider configuration that maps an external group or claim value to a Pheromone role, optionally scoped to a namespace. Attributes: IdP reference, external group/claim name, target Pheromone role, optional namespace scope.

- **AuditEvent**: An immutable record of an authentication or authorization event. Attributes: event ID, event type (login/logout/permission-denied/role-change/idp-config-change/etc.), timestamp (UTC), actor username, source IP, affected resource, outcome (success/failure), additional context (old/new values for change events). Audit events are append-only; no update or delete operation exists.

- **TwinNamespace**: A logical grouping of digital twins under a named boundary (e.g., "production", "staging"). Users with namespace-scoped RoleAssignments can only access twins within their assigned namespaces. Namespaces are managed by Owners and Admins.

---

## Success Criteria

### Measurable Outcomes

**Bootstrap & Authentication**

- **SC-001**: A platform owner with no prior Pheromone experience can deploy a fresh server, complete the bootstrap, create an Admin account, and verify successful authentication in under 10 minutes using only the provided documentation
- **SC-002**: Local username/password authentication completes and returns a session token in under 500 milliseconds under normal server load
- **SC-003**: LDAP and OIDC authentication flows complete and return a session token in under 2 seconds (excluding network round-trip to the external IdP)

**Access Control Enforcement**

- **SC-004**: Every unauthorized gRPC call is rejected with `PERMISSION_DENIED` and an audit entry is recorded; zero unauthorized calls reach an RPC handler in any test scenario
- **SC-005**: Role and namespace scope changes applied by an Admin take effect for all subsequent gRPC calls within 5 seconds of the change, without requiring the affected user to re-authenticate
- **SC-006**: Token validation and RBAC enforcement add no more than 50 milliseconds of overhead to any gRPC call under normal operating load (measured as latency delta between calls with and without interceptors)
- **SC-007**: The system correctly enforces namespace isolation in 100% of tested scenarios — a user scoped to namespace "staging" cannot read or write any twin in namespace "production"

**Identity Provider Integration**

- **SC-008**: An Admin can configure a new LDAP or OIDC provider from scratch and verify that a test user can authenticate through it within 30 minutes, using only the admin interface and provider documentation
- **SC-009**: When an external IdP is unavailable, local Owner/Admin break-glass authentication succeeds within normal response time limits, with no degradation of local auth

**Audit Logging**

- **SC-010**: 100% of authentication events (both successful and failed) and 100% of `PERMISSION_DENIED` responses appear in the audit log — no events are silently dropped under normal operating conditions
- **SC-011**: Audit log entries for any event are available for query within 1 second of the event occurring
- **SC-012**: An auditor can retrieve all access events for a specific user within a 30-day window in under 5 seconds for deployments with up to 10,000 audit entries per day

**Scale & Resilience**

- **SC-013**: The system sustains 1,000 concurrent authenticated sessions with no degradation in access control enforcement or audit log completeness
- **SC-014**: The system correctly handles 100 simultaneous authentication requests (a mix of local, LDAP, and OIDC) without session token collisions or audit log gaps

---

## Assumptions

1. **Transport security is pre-established**: Agent↔server communication is secured by mTLS (ADR-003, ADR-014 D1). Application-layer access control applies to human users and management API clients, not to agent gRPC streams, which continue to use mTLS certificates for identity. Agents do not have user accounts.

2. **Single-server deployment for Phase 2**: Stateless session tokens are sufficient for a single Pheromone server instance. Token revocation uses an in-process revocation list. Horizontal scaling of the auth layer (shared revocation store, distributed session state) is deferred to Phase 3.

3. **SCIM is optional in Phase 2**: Not all enterprise environments require automated provisioning; manual user management with LDAP/OIDC group mapping is acceptable. SCIM is specified as SHOULD (FR-007) rather than MUST, with full MUST status targeted for Phase 3.

4. **Static group-to-role mapping**: IdP group-to-role translation is defined in a configuration file or admin UI. Dynamic policy evaluation (e.g., attribute-based access control via OPA) is deferred to Phase 3.

5. **Audit log storage co-located with server**: Phase 2 audit logs are persisted in the same data store as the rest of the platform. Dedicated SIEM export (Splunk, Elastic, etc.) is a Phase 3 integration.

6. **Four built-in roles are sufficient for Phase 2**: Custom role definitions (user-defined permission sets) are explicitly out of scope for this feature. The Owner/Admin/Operator/Viewer hierarchy covers the overwhelming majority of operational use cases identified in spec-001 Story 5.

7. **Token format is an implementation decision**: The specification requires stateless, cryptographically signed tokens. JWT (JSON Web Token) with HS256 or RS256 is the expected implementation but is not mandated by this spec so as not to constrain future cryptographic choices.

---

## Dependencies & Related Work

| Reference | Type | Relevance |
|-----------|------|-----------|
| ADR-014: Security Architecture | Blocking dependency | Defines the gRPC interceptor hook points (D2) that this feature implements |
| ADR-003: gRPC Contracts | Blocking dependency | gRPC status codes (`UNAUTHENTICATED`, `PERMISSION_DENIED`) and interceptor integration pattern |
| ADR-006: Agent Lifecycle/RBAC | Informing | Twin-level skill access control (TwinLevelOS/TwinLevelWorkload hierarchy) is the agent-side analogue to this user-side RBAC |
| Spec-001 User Story 5 | Feature origin | Multi-tenant RBAC namespacing was deferred to Phase 2; this spec delivers it |
| ADR-014 Phase 2 Open Items | Implementation gate | mTLS wiring (D1) and interceptor implementation (D2) must be completed in the same phase; this feature is co-dependent |

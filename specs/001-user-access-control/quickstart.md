# Quickstart: User Access Control & Identity Provider Integration

**Feature**: `001-user-access-control` | **ADR**: ADR-015 | **Phase**: 1 MVP

This guide covers bootstrapping authentication for a new Pheromone management server,
creating users and logging in with the gRPC CLI or a custom client.

---

## Prerequisites

- Pheromone management server built with Phase 1 auth (branch `001-user-access-control`)
- `pheromone` CLI binary in PATH
- (RS256 only) An RSA key pair generated in advance — see Step 1

---

## Step 1: Generate JWT Signing Keys

**RS256 (recommended)**

```bash
# Generate a 4096-bit RSA key pair
pheromone key generate --algorithm rs256 --bits 4096 \
  --private-key /etc/pheromone/jwt.key \
  --public-key  /etc/pheromone/jwt.pub

# Verify permissions (must be 0600)
ls -la /etc/pheromone/jwt.key
# -rw------- 1 pheromone pheromone 3272 Mar 24 2026 /etc/pheromone/jwt.key
```

**HS256 (single-node / air-gapped)**

```bash
# Generate a 256-bit random secret
openssl rand -hex 32 > /etc/pheromone/jwt.secret
chmod 0600 /etc/pheromone/jwt.secret
```

---

## Step 2: Configure the Server

Add the `auth` section to your server configuration file (default: `/etc/pheromone/server.yaml`):

**RS256 configuration** (recommended):

```yaml
# /etc/pheromone/server.yaml
auth:
  provider: local
  jwt_algorithm: rs256
  jwt_private_key_file: /etc/pheromone/jwt.key
  jwt_public_key_file: /etc/pheromone/jwt.pub
  token_ttl: 8h
  bcrypt_cost: 12
  rate_limit_logins_per_minute: 10
```

**HS256 configuration** (single-node only):

```yaml
auth:
  provider: local
  jwt_algorithm: hs256
  jwt_secret_file: /etc/pheromone/jwt.secret
  token_ttl: 8h
```

---

## Step 3: First-Run Bootstrap

On first start, if no users exist in etcd, the server creates a default `owner` account
with a random password and prints it **once** to stderr:

```
WARN[2026-03-24T12:00:00Z] First-run bootstrap: created owner account
     username=admin password=Xk9mP2...  (CHANGE IMMEDIATELY)
```

Immediately change the bootstrap password:

```bash
pheromone user set-password admin
# Enter current password: <paste bootstrap password>
# Enter new password: <your secure password>
# Confirm new password: <repeat>
# ✓ Password updated for user: admin
```

---

## Step 4: Create Operator Accounts

```bash
# Log in as admin (owner)
pheromone login --server localhost:4426 --username admin
# Password: <your password>
# ✓ Logged in as admin (owner) — token valid until 2026-03-24T20:00:00Z

# Create a read-only account
pheromone user create alice --role viewer
# ✓ Created user: alice (viewer)

# Create an operator account
pheromone user create bob --role operator
# ✓ Created user: bob (operator)

# Create another admin
pheromone user create carol --role admin
# ✓ Created user: carol (admin)

# Set initial passwords (interactive prompts)
pheromone user set-password alice
pheromone user set-password bob
pheromone user set-password carol
```

---

## Step 5: Log In and Use the API

**CLI login** (stores token in `~/.config/pheromone/token`):

```bash
pheromone login --server localhost:4426 --username bob
# Password:
# ✓ Logged in as bob (operator) — token expires at 2026-03-24T20:00:00Z

# All subsequent CLI commands automatically use the stored token
pheromone twin list
pheromone skill list
```

**gRPC direct** (grpcurl):

```bash
# Login
TOKEN=$(grpcurl -plaintext -d '{"username":"bob","password":"<password>"}' \
  localhost:4426 pheromone.v1.AuthService/Login | jq -r .token)

# Use the token
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  localhost:4426 pheromone.v1.TwinControl/...
```

**Go client example**:

```go
// Login
conn, _ := grpc.Dial("localhost:4426", grpc.WithTransportCredentials(insecure.NewCredentials()))
authClient := pheromone_v1.NewAuthServiceClient(conn)

resp, err := authClient.Login(ctx, &pheromone_v1.LoginRequest{
    Username: "bob",
    Password: "my-secure-password",
})
// resp.Token contains the JWT bearer token

// Use token in subsequent calls
md := metadata.New(map[string]string{
    "authorization": "Bearer " + resp.Token,
})
ctx = metadata.NewOutgoingContext(ctx, md)

// Now call any other service
twinClient := pheromone_v1.NewTwinControlClient(conn)
// ...
```

---

## Step 6: Verify Access Control

Test that RBAC is working correctly:

```bash
# As alice (viewer) — should succeed
pheromone twin list

# As alice (viewer) — should fail with PermissionDenied
pheromone skill deploy myskill.tar.gz
# Error: rpc error: code = PermissionDenied desc = access denied: requires_operator

# As bob (operator) — skill deploy should succeed
pheromone login --username bob --server localhost:4426
pheromone skill deploy myskill.tar.gz
# ✓ Skill deployed

# As bob (operator) — user management should fail
pheromone user list
# Error: rpc error: code = PermissionDenied desc = access denied: requires_admin
```

---

## Step 7: Review Audit Logs

Auth events are written to the server's structured log output (JSON format):

```bash
# Filter auth events from server logs
journalctl -u pheromone-server -f | jq 'select(.event | startswith("auth."))'

# Example successful login
# {"ts":"2026-03-24T12:05:00Z","level":"info","event":"auth.login_success",
#  "sub":"bob","role":"operator","source_ip":"10.0.1.5","provider":"local",
#  "jti":"550e8400-e29b-41d4-a716-446655440000"}

# Example access denied
# {"ts":"2026-03-24T12:06:00Z","level":"warn","event":"auth.access_denied",
#  "sub":"alice","role":"viewer","rpc":"/pheromone.v1.SkillService/DeploySkill",
#  "reason":"requires_operator","jti":"660f9511-f30c-52e5-b827-557766551111"}
```

---

## JWKS Endpoint (RS256 only)

The RS256 public key is available for third-party JWT verification:

```bash
curl https://pheromone.example.com/auth/jwks
# {
#   "keys": [
#     {"kty":"RSA","use":"sig","alg":"RS256","kid":"2026-03-24","n":"sA6...","e":"AQAB"}
#   ]
# }
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `UNAUTHENTICATED` on all RPCs | Token missing or expired | Re-login: `pheromone login` |
| `UNAUTHENTICATED` immediately after login | Clock skew > 1 min | Sync server and client clocks (NTP) |
| `PERMISSION_DENIED` on allowed operation | Wrong role assigned | `pheromone user list` to verify role |
| Login rejected with correct password | Account disabled | `pheromone user enable <username>` (admin) |
| `RESOURCE_EXHAUSTED` on Login | Rate limit exceeded (10/min) | Wait 60 seconds and retry |
| Server fails to start: "jwt key file not found" | Key file missing or wrong path | Verify `jwt_private_key_file` path; check file mode 0600 |
| Server fails to start: "permission denied reading jwt key" | File permissions too restrictive | `chmod 0600 /etc/pheromone/jwt.key && chown pheromone /etc/pheromone/jwt.key` |

---

## Phase 2: External IdP (Preview)

When Phase 2 is available, switch to LDAP, OIDC, or SAML by updating `server.auth.provider`
and providing the adapter-specific configuration block.  See ADR-015 D4 for the configuration
schema.

```yaml
# LDAP example (Phase 2)
auth:
  provider: ldap
  jwt_algorithm: rs256
  jwt_private_key_file: /etc/pheromone/jwt.key
  jwt_public_key_file: /etc/pheromone/jwt.pub
  token_ttl: 8h
  fallback_to_local: false    # fail closed if LDAP unreachable
  ldap:
    url: ldaps://ad.example.com:636
    bind_dn: "CN=pheromone-svc,OU=ServiceAccounts,DC=example,DC=com"
    bind_password_file: /etc/pheromone/ldap-bind.secret
    user_base_dn: "OU=Users,DC=example,DC=com"
    group_base_dn: "OU=Groups,DC=example,DC=com"
    group_role_map:
      - dn: "CN=PheromoneOwners,OU=Groups,DC=example,DC=com"
        role: owner
      - dn: "CN=PheromoneAdmins,OU=Groups,DC=example,DC=com"
        role: admin
      - dn: "CN=PheromoneOps,OU=Groups,DC=example,DC=com"
        role: operator
      - dn: "CN=PheromoneViewers,OU=Groups,DC=example,DC=com"
        role: viewer
```

---

## References

- ADR-015: `docs/adr/adr-015-user-access-control-identity-provider.md`
- Data Model: `specs/001-user-access-control/data-model.md`
- gRPC Contract: `specs/001-user-access-control/contracts/auth.proto`
- HTTP Contract: `specs/001-user-access-control/contracts/openapi.yaml`
- SECURITY.md: `SECURITY.md`

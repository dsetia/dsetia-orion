# API Design Document

# Scope 

Add a human-user authentication layer (users, password hashing, JWT issuance, tenant-scoped middleware) to the Orion apis server. Existing sensor authentication (X-API-KEY \+ X-DEVICE-ID) is unchanged.  
---

# 1\. Goals

- Store users in PostgreSQL with bcrypt-hashed passwords, tied to a specific tenant or marked as system-wide administrators.  
- Issue short-lived signed JWTs on successful login; provide a refresh-token mechanism for session continuity.  
- Enforce tenant scoping entirely server-side: UI handlers derive the effective tenant\_id from the validated JWT claim, never from a client-supplied value.  
- Define two roles — **security\_analyst** and **system\_admin**  
  - Security\_analyst: monitoring dashboards, threat detection, configuring AI models  
  - System\_admin: Administrates the tenant, create new users, manage the agent   
- Provide useradm tool for user lifecycle management (create, list, delete, reset password).

---

# 2\. Out of Scope

- OAuth2 / OIDC / SSO integration (future work).  
- Fine-grained RBAC beyond the two roles defined here.  
- UI or frontend implementation.  
- Changes to the sensor-facing endpoints (/v1/authenticate/, /v1/updates/, /v1/status/, /v1/download/).  
- Nginx-level JWT validation (JWT validation stays in the Go layer).  
- Full CRUD operations on existing HNDR tables (tenants, devices, api\_keys, hndr\_sw, hndr\_rules, threatintel, status, version). Such access is available via dbtool.

---

# 3\. Schema Changes

## 3.1 users Table

```sql
CREATE TABLE users (
    user_id            UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          BIGINT  NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email              TEXT    NOT NULL UNIQUE,
    password_hash      TEXT    NOT NULL,           -- bcrypt, cost 12
    role               TEXT    NOT NULL
                          CHECK (role IN ('security_analyst', 'system_admin')),
    is_active          BOOLEAN NOT NULL DEFAULT true,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    failed_attempts    INT         NOT NULL DEFAULT 0,
    lockout_until      TIMESTAMPTZ
);

CREATE INDEX idx_users_tenant   ON users(tenant_id);
CREATE INDEX idx_users_email_lower ON users(LOWER(email));

CREATE TABLE login_audit_log (
    id             BIGSERIAL PRIMARY KEY,
    user_id        UUID REFERENCES users(user_id) ON DELETE SET NULL,
    email          TEXT NOT NULL,
    success        BOOLEAN NOT NULL,
    ip_address     TEXT,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_login_audit_user    ON login_audit_log(user_id);
CREATE INDEX idx_login_audit_email   ON login_audit_log(LOWER(email));
CREATE INDEX idx_login_audit_created ON login_audit_log(created_at DESC);
```

Example failure\_reason are: “unknown user”, “inactive\_user”, “incorrect\_password”, “account\_locked”

## 3.2 refresh\_tokens Table

Access JWTs are short-lived and stateless. Refresh tokens are long-lived and stored so they can be individually revoked (logout, compromise response).

```sql
CREATE TABLE refresh_tokens (
    token_id     UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID      NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash   TEXT      NOT NULL UNIQUE,   -- SHA-256 hex of the opaque token value
    expires_at   TIMESTAMP NOT NULL,
    revoked      BOOLEAN   NOT NULL DEFAULT false,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP
);

CREATE INDEX idx_refresh_tokens_user    ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash    ON refresh_tokens(token_hash);
```

The actual refresh token value sent to the client is a cryptographically random opaque string (32-byte, base64url-encoded). Only its SHA-256 digest is stored in the database.

## 3.3 Migration

The two new tables are additive. They can be applied to an existing database via:

```shell
db/migrate_v2_to_v3.sh
```

A new file db/user\_auth\_schema.sql will hold only these additions (not the full schema) so it can be run against production without recreating the existing schema.  
---

# 4\. New Configuration

A new config/auth.json file carries JWT key material and token lifetimes:

```json
{
    "jwt_secret":               "<256-bit hex or base64-encoded secret>",
    "access_token_ttl_minutes": 15,
    "refresh_token_ttl_days":   7
}
```

jwt\_secret is an HS256 HMAC secret. It must be at least 32 bytes of entropy, generated at deploy time (e.g. openssl rand \-hex 32) and kept out of version control. A config/auth.example.json with a placeholder value will be committed instead.

The apis server will accept a new \-auth-config flag (defaulting to config/auth.json) alongside the existing \-config flag for db.json.

A new AuthConfig struct in common/ mirrors this JSON.  
---

# 5\. New API Endpoints

All new endpoints live under /v1/ma/. Sensor endpoints under /v1/authenticate/, /v1/updates/, /v1/status/, and /v1/download/ are untouched.

## 5.1 Auth Endpoints (unauthenticated)

| Method | Path | Description |
| :---- | :---- | :---- |
| POST | /v1/ma/auth/login | Validate credentials, issue access JWT \+ refresh token |

## 5.2 Auth Endpoints (authenticated)

| Method | Path | Description |
| :---- | :---- | :---- |
| POST | /v1/ma/auth/refresh | Exchange a valid refresh token for a new access JWT |
| POST | /v1/ma/auth/logout | Revoke the caller's refresh token (requires valid access JWT) |

## 5.3 Application Endpoints

All routes below require a valid Authorization: Bearer \<access\_jwt\> header. Tenant scoping is enforced by middleware (§6).

| Method | Path | Description |
| :---- | :---- | :---- |
| GET | /v1/ma/me | Return calling user's profile (from JWT context, no DB hit needed). This can be used to confirm session and load user context |
| GET | /v1/ma/devices | List network agents |
| GET | /v1/ma/devices/{device\_id} | Get network agent details. |
| GET | /v1/ma/versions | List reported network agent versions of software, rules and threat intel. |
| GET | /v1/ma/status | List network agent statuses. |
| GET | /v1/ma/users | List UI users. |
| POST | /v1/ma/users | Create a user for this tenant.  |
| DELETE | /v1/ma/users/{user\_id} | Delete a user from this tenant. |
| PUT | /v1/ma/users/{user\_id}/password | Reset a user's password |

## 5.3 User creation

### 5.3.1 system\_admin

The first system\_admin role user must be created via dbtool/useradm with direct DB access. It can’t be deleted by the user. The initial system admin user can create additional system\_admin users using the REST API.

### 5.3.2 security\_analyst 

Security analysts user can be created via useradm tool by ops team or by the REST API by system administrator

| Path | Who uses it | When |
| :---- | :---- | :---- |
| useradm insert-user \-role system\_admin \-tenant-id {id} | Ops | Initial provisioning of a tenant's first system admin |
| useradm insert-user \-role security\_analyst \-tenant-id {id} | Ops | As needed |
| POST /v1/ma/users with a valid JWT | Logged-in system\_admin | Self-service user management via the UI |

---

# 6\. JWT Design

## 6.1 Token Types

| Token | Lifetime | Storage | Revocable |
| :---- | :---- | :---- | :---- |
| Access JWT | 15 min (configurable) | Client memory only | No (short-lived) |
| Refresh token | 7 days (configurable) | Client (cookie or local storage) \+ SHA-256 in DB | Yes (revoked \= true) |

## 6.2 Access JWT Claims

Algorithm: **HS256** (HMAC-SHA256) with the shared secret from auth.json.

```json
{
    "sub":       "<user_id UUID>",
    "email":     "user@example.com",
    "role":      "security_analyst",
    "tenant_id": 1234,
    "iat":       1700000000,
    "exp":       1700000900
}
```

- sub is the UUID from users.user\_id.  
- tenant\_id is always present for both roles. All users are tenant-scoped.  
- No sensitive data (password hash, full DB row) is included.

## 6.3 Token Issuance (POST /v1/ma/auth/login)

Request:

```json
{ "email": "user@example.com", "password": "plaintext" }
```

Server-side:

1. Look up users by email. If not found → insert audit log (failure\_reason \= "unknown\_user") → 401 Unauthorized.  
2. If is\_active \= false → insert audit log (failure\_reason \= "inactive\_user") → 401 Unauthorized.  
3. If NOW() \< lockout\_until → insert audit log (failure\_reason \= "account\_locked") → 401 Unauthorized.  
4. Run bcrypt.CompareHashAndPassword(stored\_hash, provided\_password). On mismatch:  
   - Call RecordFailedAttempt(userID) — increments failed\_attempts; if failed\_attempts \>= lockoutThreshold, sets lockout\_until \= NOW() \+ lockoutDuration.  
   - Insert audit log (failure\_reason \= "incorrect\_password").  
   - Return 401 Unauthorized.  
5. Call ClearLockout(userID) — resets failed\_attempts \= 0, clears lockout\_until.  
6. Insert audit log (success \= true).  
7. Sign access JWT with claims above.  
8. Generate 32-byte random opaque refresh token; store SHA-256(token) in refresh\_tokens.  
9. Return:

```json
{
    "access_token":  "<jwt>",
    "token_type":    "Bearer",
    "expires_in":    900,
    "refresh_token": "<opaque>"
}
```

Steps 1–4 always return the same 401 Unauthorized response body regardless of the actual failure reason, preventing user enumeration. The precise reason is recorded in login\_audit\_log for internal audit use only.

## 6.4 Token Refresh (POST /v1/ma/auth/refresh)

Request:

```json
{ "refresh_token": "<opaque>" }
```

Server-side:

1. Compute SHA-256(provided\_token), look up in refresh\_tokens.  
2. Reject if not found, revoked \= true, or expires\_at \< now.  
3. Load the associated users row; reject if is\_active \= false.  
4. Update last\_used\_at \= now.  
5. Issue a new access JWT. **Do not rotate the refresh token** (rotation adds complexity with no meaningful security gain in this model; revocation on logout covers the primary threat).  
6. Return the same shape as the login response (without refresh\_token).

## 6.5 Logout (POST /v1/ma/auth/logout)

Requires valid access to JWT. Sets refresh\_tokens.revoked \= true for all non-expired refresh tokens belonging to the calling user. Returns 204 No Content.

---

# 7\. Middleware Design

The new API uses a context based middleware chain implemented in apis/middleware.go. Identity and tenant scope are propagated through the request context. `requireJWT` wraps handlers at route registration time. It validates the `Authorization: Bearer <token>` header, verifies the JWT signature and expiry and stores the resulting claims in the request context. All downstream handlers read the caller’s identity (user ID, email, role, tenant ID) exclusively from this context value.

## 7.1 Existing Sensor Middleware (unchanged)

The existing authenticate() method in apis.go remains exactly as-is, applied only to the sensor endpoints. It reads X-API-KEY \+ X-DEVICE-ID, validates via db.ValidateAPIKey(), and checks the URL {tenant\_id} against the credential's tenant\_id.

## 7.2 UI JWT Middleware

One component handles authentication for every API request.

```
Request → requireJWT → handler                                               
```

### **requireJWT**

Applied to every `/v1/ma/` route except the unauthenticated auth endpoints. Validates the token and makes the caller's identity available to everything downstream.

```go
func (s *Server) requireJWT(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Read Authorization header, expect "Bearer <token>".
        //    Missing or malformed → 401 {"error": "invalid or expired token"}
        // 2. Parse and verify: HS256 signature, exp, iat.
        //    Any failure → 401 (same message, no detail leaked)
        // 3. Store the parsed UserClaims in context under a private key type.
        // 4. Call next.
    }
}
```

After requireJWT runs, the context carries a UserClaims value containing UserID, Email, Role, and TenantID. All downstream middleware and handlers read identity exclusively from this context value — never from request headers or body.

The context key is a private unexported type to prevent collisions with other packages. claimsFromContext is a convenience wrapper so callers don't need to know about the key:

```go
type contextKey int
const claimsKey contextKey = iota

func claimsFromContext(ctx context.Context) *UserClaims {
    v, _ := ctx.Value(claimsKey).(*UserClaims)
    return v
}
```

Handlers read TenantID directly from the UserClaims already in context (set by requireJWT). Handlers never read tenant identity from the URL or request body.

## 7.3 Route Registration

Routes are registered using Go 1.22+ method-prefixed patterns (`"METHOD /path"`). The mux handles method dispatch, returning 405 automatically for unregistered methods. Each route is registered individually — there is no catch-all handler.

```go
// Existing sensor routes (unchanged)
http.HandleFunc("/v1/authenticate/", server.handleAuthenticate)
http.HandleFunc("/v1/updates/",      server.handleUpdates)
http.HandleFunc("/v1/status/",       server.handleStatus)
http.HandleFunc("/v1/healthcheck",   server.handleHealthCheck)

// Management API auth — unauthenticated
http.HandleFunc("/v1/ma/auth/login",   server.handleUserLogin)
http.HandleFunc("/v1/ma/auth/refresh", server.handleAccessTokenRefresh)
http.HandleFunc("/v1/ma/auth/logout",  server.requireJWT(server.handleUserLogout))

// Identity
http.HandleFunc("GET /v1/ma/me", server.requireJWT(server.handleMe))

// Devices
http.HandleFunc("GET /v1/ma/devices",             server.requireJWT(server.handleListDevices))
http.HandleFunc("GET /v1/ma/devices/{device_id}", server.requireJWT(server.handleGetDevice))

// Versions
http.HandleFunc("GET /v1/ma/versions", server.requireJWT(server.handleListVersions))

// Status
http.HandleFunc("GET /v1/ma/status", server.requireJWT(server.handleListStatus))

// Users
http.HandleFunc("GET    /v1/ma/users",                    server.requireJWT(server.handleListUsers))
http.HandleFunc("POST   /v1/ma/users",                    server.requireJWT(server.handleCreateUser))
http.HandleFunc("DELETE /v1/ma/users/{user_id}",          server.requireJWT(server.handleDeleteUser))
http.HandleFunc("PUT    /v1/ma/users/{user_id}/password", server.requireJWT(server.handleResetPassword))
```

Path parameters (e.g. `{device_id}`, `{user_id}`) are extracted inside handlers via `r.PathValue("device_id")` — no manual string splitting. Each handler reads the tenant ID directly from the JWT claims already in context:

```go
tenantID := claimsFromContext(r.Context()).TenantID
```

Role enforcement is per-operation inside each handler function, not at the routing layer. Adding new resources means adding new `http.HandleFunc` registrations and handler functions — no central dispatcher needs to change.

## 7.4 Constants in apis/[auth.go](http://auth.go)

These could be configurable via auth.json but constants for this iteration.

```textproto
const lockoutThreshold = 3
const lockoutDuration  = 10 * time.Minute
```

---

# 8\. Role Access Matrix

| Resource | system\_admin | security\_analyst |
| :---- | :---- | :---- |
| View one user (GET /v1/ma/users/{id}) | Yes | No |
| List network agents | Yes | Yes |
| View network agent detail | Yes | Yes |
| List network agent versions / status | Yes | Yes |
| List users | Yes | No |
| Create security\_analyst user | Yes | No |
| Create system\_admin user | Yes; first one using useradm tool | No |
| Delete a user | Yes | No |
| Reset another user's password | Yes | No |
| Reset own password (PUT /v1/ma/users/{own\_user\_id}/password) | Yes | Yes |
| View /v1/ma/me | Yes | Yes |

---

# 9\. Useradm tool (wrapper over dbtool for the following operations)

## 9.1 dbtool new operations

The existing dbtool binary gains the following \-op values. These are the authoritative implementations — they connect directly to PostgreSQL and have no dependency on auth.json.

| Operation | Flags | Description |
| :---- | :---- | :---- |
| insert-user | \-email, \-role, \-tenant-id | Prompts for password on stdin; stores bcrypt hash; password min length is 12 |
| list-users |  | Lists users |
| delete-user | \-user-id or \-email | Deletes user and cascades refresh tokens |
| reset-user-password | \-user-id or \-email | Prompts for new password on stdin |
| deactivate-user | \-user-id or \-email | Sets is\_active \= false without deleting |
| list-login-audit | \-email or \-user-id (optional) |  |

Password input uses golang.org/x/term when stdin is a terminal (no echo, secure interactive use). When stdin is a pipe or file, it falls back to a plain line read, allowing scripted and automated callers to supply the password via stdin (e.g. `echo "$PW" | dbtool ...`). The useradm binary does not require auth.json since it never issues JWTs — it only manages the users and refresh\_tokens tables via direct DB access.

## 9.2 useradm shell wrapper

`useradm` is a thin shell script that wraps `dbtool` and exposes only the user management operations above. It exists to give ops a purpose-limited, user-facing interface without exposing the full `dbtool` surface (tenant management, device management, rules, etc.).

```shell
#!/usr/bin/env bash
# useradm — user management wrapper around dbtool
# Usage: useradm <operation> [flags]
# Supported operations: insert-user, list-users, delete-user,
#                       reset-user-password, deactivate-user, list-login-audit

ALLOWED_OPS="insert-user list-users delete-user reset-user-password deactivate-user list-login-audit"
OP="$1"

if ! echo "$ALLOWED_OPS" | grep -qw "$OP"; then
    echo "useradm: unknown operation '$OP'" >&2
    echo "Supported: $ALLOWED_OPS" >&2
    exit 1
fi

exec dbtool -op "$@"
```

useradm lives at utils/useradm.sh and is installed via make install-utils. It requires dbtool to be on PATH and accepts the same \-db config flag that dbtool does (passed through via "$@").  
---

# 10\. New Go Dependencies

All added to apis/go.mod:

| Package | Purpose |
| :---- | :---- |
| github.com/golang-jwt/jwt/v5 | JWT signing and verification |
| golang.org/x/crypto | bcrypt password hashing |

Added to db/go.mod (for dbtool):

| Package | Purpose |
| :---- | :---- |
| golang.org/x/crypto | bcrypt for insert-user / reset-user-password |
| golang.org/x/term | Password prompting without echo |

---

# 11\. Nginx Changes

No nginx changes are required. The /v1/ma/ namespace falls through the existing catch-all:

```
location /v1/ {
    proxy_pass http://apis-container:8080/v1/;
}
```

JWT validation is handled entirely within the Go layer. No auth\_request sub-block is needed for UI routes.

---

# 12\. Updated File Inventory

| File | Change |
| :---- | :---- |
| db/schema\_pg\_v3.sql | New schema version — adds users, refresh\_tokens, and login\_audit\_log tables |
| db/migrate\_v2\_to\_v3.sh | Migration script from schema v2 to v3 |
| common/types.go | New shared config and claims types: AuthConfig (JWT secret, TTLs) and UserClaims (JWT payload) |
| apis/auth.go | New — login, token refresh, and logout handlers; JWT signing helpers; lockout and TTL constants |
| apis/auth\_db.go | New — DB access functions used exclusively by auth handlers (user lookup, refresh token CRUD, lockout tracking) |
| apis/resources.go | New — all /v1/ma/ resource handlers; role enforcement per operation |
| apis/middleware.go | New — JWT validation middleware and context helpers |
| apis/dbutil.go | Extended — user/audit types and DB functions shared with dbtool (user CRUD, audit log, refresh token listing) |
| apis/apis.go | Extended — \-auth-config flag; register all /v1/ma/ routes with JWT middleware |
| apis/auth\_test.go | New — unit tests covering login, lockout, token refresh, logout, /me, and role enforcement |
| apis/go.mod / apis/go.sum | Add github.com/golang-jwt/jwt/v5, golang.org/x/crypto |
| db/dbtool.go | Extended — user management and audit log operations (insert, list, delete, reset password, deactivate, list audit log, list refresh tokens) |
| db/go.mod / db/go.sum | Add golang.org/x/crypto, golang.org/x/term |
| utils/useradm.sh | New — shell wrapper exposing only user management operations from dbtool |
| utils/test\_ui\_auth.sh | New — end-to-end REST test script for all /v1/ma/ endpoints |
| config/auth.example.json | New — example auth config with placeholder secret and default TTL values |

# 

---

# 13\. Open Questions

1. **Refresh token delivery mechanism** — Should the refresh token be returned in the JSON body (client stores it) or as an HttpOnly Secure cookie (browser manages it, XSS-resistant)? Cookie-based is strongly preferred for browser-originated UIs. Decision deferred to UI team.  
     
2. **Multi-device sessions** — Currently one refresh token is issued per login. Should multiple concurrent refresh tokens per user be allowed (multiple devices), or should login invalidate all prior refresh tokens? Current design allows multiple concurrent sessions.  
     
3. **Password policy** — Minimum length, complexity, expiry? None is enforced in this design beyond a minimum 12-character length checked at creation time.

4. **Rate limiting** — Login endpoint should be rate-limited to prevent brute force. Current design has no rate limiting. Recommend nginx limit\_req zone on /v1/ma/auth/login as a follow-on.


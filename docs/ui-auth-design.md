# UI User Authentication — Design Document

**Branch:** `173-add-support-for-ui-users`
**Scope:** Add a human-user authentication layer (UI users, password hashing, JWT issuance, tenant-scoped middleware) to the Orion `apis` server. Existing sensor authentication (`X-API-KEY` + `X-DEVICE-ID`) is unchanged.

---

## 1. Goals

- Store UI users in PostgreSQL with bcrypt-hashed passwords, tied to a specific tenant or marked as system-wide administrators.
- Issue short-lived signed JWTs on successful login; provide a refresh-token mechanism for session continuity.
- Enforce tenant scoping entirely server-side: UI handlers derive the effective `tenant_id` from the validated JWT claim, never from a client-supplied value.
- Define two roles — **tenant_admin** (scoped to one tenant) and **system_admin** (unrestricted) — with separate access surfaces.
- Provide `dbtool` operations for user lifecycle management (create, list, delete, reset password) so operators can bootstrap the system without a running UI.

---

## 2. Non-Goals

- OAuth2 / OIDC / SSO integration (future work).
- Fine-grained RBAC beyond the two roles defined here.
- UI or frontend implementation.
- Changes to the sensor-facing endpoints (`/v1/authenticate/`, `/v1/updates/`, `/v1/status/`, `/v1/download/`).
- Nginx-level JWT validation (JWT validation stays in the Go layer).

---

## 3. Schema Changes

### 3.1 `ui_users` Table

```sql
CREATE TABLE ui_users (
    user_id       UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     BIGINT  REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,           -- bcrypt, cost 12
    role          TEXT    NOT NULL
                          CHECK (role IN ('tenant_admin', 'system_admin')),
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- tenant_admin must belong to a tenant; system_admin must not
    CONSTRAINT chk_role_tenant_consistency CHECK (
        (role = 'tenant_admin' AND tenant_id IS NOT NULL) OR
        (role = 'system_admin' AND tenant_id IS NULL)
    )
);

CREATE INDEX idx_ui_users_tenant   ON ui_users(tenant_id);
CREATE INDEX idx_ui_users_email    ON ui_users(email);
```

`tenant_id` is `NULL` for `system_admin` users. The `CHECK` constraint ensures this invariant at the DB layer.

### 3.2 `refresh_tokens` Table

Access JWTs are short-lived and stateless. Refresh tokens are long-lived and stored so they can be individually revoked (logout, compromise response).

```sql
CREATE TABLE refresh_tokens (
    token_id     UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID      NOT NULL REFERENCES ui_users(user_id) ON DELETE CASCADE,
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

### 3.3 Migration

The two new tables are additive. They can be applied to an existing database via:

```bash
psql $DSN -f db/ui_auth_schema.sql
```

A new file `db/ui_auth_schema.sql` will hold only these additions (not the full schema) so it can be run against production without recreating the existing schema.

---

## 4. New Configuration

A new `config/auth.json` file carries JWT key material and token lifetimes:

```json
{
    "jwt_secret":               "<256-bit hex or base64-encoded secret>",
    "access_token_ttl_minutes": 15,
    "refresh_token_ttl_days":   7
}
```

`jwt_secret` is an HS256 HMAC secret. It must be at least 32 bytes of entropy, generated at deploy time (e.g. `openssl rand -hex 32`) and kept out of version control. A `config/auth.example.json` with a placeholder value will be committed instead.

The `apis` server will accept a new `-auth-config` flag (defaulting to `config/auth.json`) alongside the existing `-config` flag for `db.json`.

A new `AuthConfig` struct in `common/` mirrors this JSON.

---

## 5. New API Endpoints

All new endpoints live under `/v1/ui/`. Sensor endpoints under `/v1/authenticate/`, `/v1/updates/`, `/v1/status/`, and `/v1/download/` are untouched.

### 5.1 Auth Endpoints (unauthenticated)

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/ui/auth/login` | Validate credentials, issue access JWT + refresh token |
| `POST` | `/v1/ui/auth/refresh` | Exchange a valid refresh token for a new access JWT |
| `POST` | `/v1/ui/auth/logout` | Revoke the caller's refresh token (requires valid access JWT) |

### 5.2 Authenticated Endpoints

All routes below require a valid `Authorization: Bearer <access_jwt>` header. Tenant scoping is enforced by middleware (§6).

**Identity**

| Method | Path | Roles | Description |
|--------|------|-------|-------------|
| `GET` | `/v1/ui/me` | any | Return calling user's profile (from JWT context, no DB hit needed) |

**System admin — tenant management** (`system_admin` only)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/ui/tenants` | List all tenants |
| `GET` | `/v1/ui/tenants/{tenant_id}` | Get one tenant |
| `POST` | `/v1/ui/tenants` | Create a tenant |
| `DELETE` | `/v1/ui/tenants/{tenant_id}` | Delete a tenant |

**Tenant-scoped resources** (`tenant_admin` for their own tenant; `system_admin` for any)

All paths below use a single URL pattern. The `{tenant_id}` segment accepts either a real tenant ID or the reserved string `_all`.

- **Real tenant ID** — scoped to that tenant. `tenant_admin` may only use their own JWT tenant_id; any mismatch returns `403`. `system_admin` may use any real tenant ID.
- **`_all`** — `system_admin` only; returns the cross-tenant aggregate. `tenant_admin` always receives `403` for `_all`. Only meaningful on `GET` (list) operations; `POST`, `PUT`, `DELETE` with `_all` return `400 Bad Request`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/ui/tenants/{tenant_id}/devices` | List devices. `_all` returns every device across all tenants. |
| `GET` | `/v1/ui/tenants/{tenant_id}/devices/{device_id}` | Get device detail. Real tenant ID only. |
| `GET` | `/v1/ui/tenants/{tenant_id}/versions` | List reported device versions. |
| `GET` | `/v1/ui/tenants/{tenant_id}/status` | List device statuses. |
| `GET` | `/v1/ui/tenants/{tenant_id}/rules` | List rule versions. |
| `GET` | `/v1/ui/tenants/{tenant_id}/users` | List UI users. `_all` returns every user across all tenants. |
| `POST` | `/v1/ui/tenants/{tenant_id}/users` | Create a `tenant_admin` user for this tenant. Real tenant ID only. |
| `DELETE` | `/v1/ui/tenants/{tenant_id}/users/{user_id}` | Delete a user from this tenant. Real tenant ID only. |
| `PUT` | `/v1/ui/tenants/{tenant_id}/users/{user_id}/password` | Reset a user's password. Real tenant ID only. |

`system_admin` users are not tied to a tenant. To create one, a `system_admin` caller uses `POST /v1/ui/tenants/{tenant_id}/users` with `"role": "system_admin"` in the body; the `{tenant_id}` in the path is validated to ensure it matches the `tenant_id` in the request body (which must be absent or null for `system_admin`). `tenant_admin` callers cannot create `system_admin` users; the server rejects any role elevation attempt with `403`.

---

## 6. JWT Design

### 6.1 Token Types

| Token | Lifetime | Storage | Revocable |
|-------|----------|---------|-----------|
| Access JWT | 15 min (configurable) | Client memory only | No (short-lived) |
| Refresh token | 7 days (configurable) | Client (cookie or local storage) + SHA-256 in DB | Yes (`revoked = true`) |

### 6.2 Access JWT Claims

Algorithm: **HS256** (HMAC-SHA256) with the shared secret from `auth.json`.

```json
{
    "sub":       "<user_id UUID>",
    "email":     "user@example.com",
    "role":      "tenant_admin",
    "tenant_id": 1234,
    "iat":       1700000000,
    "exp":       1700000900
}
```

- `tenant_id` is **omitted** (or `null`) for `system_admin` users.
- `sub` is the UUID from `ui_users.user_id`.
- No sensitive data (password hash, full DB row) is included.

### 6.3 Token Issuance (`POST /v1/ui/auth/login`)

Request:
```json
{ "email": "user@example.com", "password": "plaintext" }
```

Server-side:
1. Look up `ui_users` by `email`.
2. If not found or `is_active = false` → `401 Unauthorized` (same error either way, no user enumeration).
3. Run `bcrypt.CompareHashAndPassword(stored_hash, provided_password)`.
4. On mismatch → `401 Unauthorized`.
5. Sign access JWT with claims above.
6. Generate 32-byte random opaque refresh token; store `SHA-256(token)` in `refresh_tokens`.
7. Return:

```json
{
    "access_token":  "<jwt>",
    "token_type":    "Bearer",
    "expires_in":    900,
    "refresh_token": "<opaque>"
}
```

### 6.4 Token Refresh (`POST /v1/ui/auth/refresh`)

Request:
```json
{ "refresh_token": "<opaque>" }
```

Server-side:
1. Compute `SHA-256(provided_token)`, look up in `refresh_tokens`.
2. Reject if not found, `revoked = true`, or `expires_at < now`.
3. Load the associated `ui_users` row; reject if `is_active = false`.
4. Update `last_used_at = now`.
5. Issue a new access JWT. **Do not rotate the refresh token** (rotation adds complexity with no meaningful security gain in this model; revocation on logout covers the primary threat).
6. Return same shape as login response (without `refresh_token`).

### 6.5 Logout (`POST /v1/ui/auth/logout`)

Requires valid access JWT. Sets `refresh_tokens.revoked = true` for all non-expired refresh tokens belonging to the calling user. Returns `204 No Content`.

---

## 7. Middleware Design

### 7.1 Existing Sensor Middleware (unchanged)

The existing `authenticate()` method in `apis.go` remains exactly as-is, applied only to the sensor endpoints. It reads `X-API-KEY` + `X-DEVICE-ID`, validates via `db.ValidateAPIKey()`, and checks the URL `{tenant_id}` against the credential's `tenant_id`.

### 7.2 New UI JWT Middleware

A new `requireJWT(next http.HandlerFunc) http.HandlerFunc` middleware:

1. Reads `Authorization` header, expects `Bearer <token>`.
2. Parses and validates the JWT: signature, `exp`, `iat`.
3. On any failure → `401 Unauthorized` with `{"error": "invalid or expired token"}`.
4. Stores the parsed claims in `context.Context` under a private key type.
5. Calls `next`.

A role-enforcement layer wraps specific routes:

```
requireJWT → requireRole("system_admin") → handler
requireJWT → requireTenantScope          → handler
```

`requireTenantScope` middleware:
1. Extracts the raw `{tenant_id}` path variable (string).
2. Reads `role` and `tenant_id` claim from context (populated by `requireJWT`).
3. If the path variable is `"_all"`:
   - `role == system_admin` → set `effective_tenant_id = nil` in context (signals aggregate query to handler).
   - `role == tenant_admin` → `403 Forbidden` unconditionally.
4. If the path variable is a numeric tenant ID:
   - `role == system_admin` → set `effective_tenant_id` from the URL value.
   - `role == tenant_admin` → compare URL value against JWT `tenant_id` claim; mismatch → `403 Forbidden`; match → set `effective_tenant_id` from JWT claim (not the URL, as a defence-in-depth measure).
5. For `POST`, `PUT`, `DELETE` requests, if the path variable is `"_all"` → `400 Bad Request` before any role check.

**Handlers never read `tenant_id` from the request URL or body directly.** They always call `tenantIDFromContext(ctx)`, which returns `(int64, bool)` — `bool` is `false` when the effective scope is `_all`. This makes the scoping guarantee structural rather than dependent on each handler getting it right. Handlers that do not support aggregate mode (e.g. single-resource GET by device ID) assert `bool == true` and return `400` if not.

### 7.3 Route Registration

Routes will be organized into three groups at server startup:

```
# Unauthenticated sensor routes (existing)
mux.HandleFunc("/v1/authenticate/{tenant_id}", s.handleAuthenticate)
mux.HandleFunc("/v1/updates/{tenant_id}",      s.handleUpdates)
mux.HandleFunc("/v1/status/{tenant_id}",       s.handleStatus)
mux.HandleFunc("/v1/healthcheck",              s.handleHealthCheck)

# UI auth (unauthenticated)
mux.HandleFunc("POST /v1/ui/auth/login",       s.handleUILogin)
mux.HandleFunc("POST /v1/ui/auth/refresh",     s.handleUIRefresh)
mux.HandleFunc("POST /v1/ui/auth/logout",      requireJWT(s.handleUILogout))

# UI authenticated — identity and tenant management
mux.HandleFunc("GET /v1/ui/me",                         requireJWT(s.handleUIMe))
mux.HandleFunc("GET /v1/ui/tenants",                    requireJWT(requireRole("system_admin", s.handleListTenants)))
mux.HandleFunc("POST /v1/ui/tenants",                   requireJWT(requireRole("system_admin", s.handleCreateTenant)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}",        requireJWT(requireRole("system_admin", s.handleGetTenant)))
mux.HandleFunc("DELETE /v1/ui/tenants/{tenant_id}",     requireJWT(requireRole("system_admin", s.handleDeleteTenant)))

# Tenant-scoped resources — {tenant_id} accepts a real ID or "_all"
# requireTenantScope enforces role rules and sets effective_tenant_id in context
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/devices",
    requireJWT(requireTenantScope(s.handleUIListDevices)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/devices/{device_id}",
    requireJWT(requireTenantScope(s.handleUIGetDevice)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/versions",
    requireJWT(requireTenantScope(s.handleUIListVersions)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/status",
    requireJWT(requireTenantScope(s.handleUIListStatus)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/rules",
    requireJWT(requireTenantScope(s.handleUIListRules)))
mux.HandleFunc("GET /v1/ui/tenants/{tenant_id}/users",
    requireJWT(requireTenantScope(s.handleUIListUsers)))
mux.HandleFunc("POST /v1/ui/tenants/{tenant_id}/users",
    requireJWT(requireTenantScope(s.handleUICreateUser)))
mux.HandleFunc("DELETE /v1/ui/tenants/{tenant_id}/users/{user_id}",
    requireJWT(requireTenantScope(s.handleUIDeleteUser)))
mux.HandleFunc("PUT /v1/ui/tenants/{tenant_id}/users/{user_id}/password",
    requireJWT(requireTenantScope(s.handleUIResetPassword)))
```

When a new resource is added in a future release, it gets **one route registration** using the same `requireTenantScope` wrapper. The `_all` aggregate behaviour is automatically available to `system_admin` with no additional route.

---

## 8. Role Access Matrix

| Resource | `system_admin` | `tenant_admin` |
|---|---|---|
| List all tenants (`GET /v1/ui/tenants`) | Yes | No |
| Create / delete tenant | Yes | No |
| View one tenant (`GET /v1/ui/tenants/{id}`) | Yes | No |
| List devices — own tenant | Yes | Yes |
| List devices — all tenants (`_all`) | Yes | No (403) |
| View device detail | Yes | Yes (own tenant only) |
| List versions / status / rules — own tenant | Yes | Yes |
| List versions / status / rules — all tenants (`_all`) | Yes | No (403) |
| List users — own tenant | Yes | Yes |
| List users — all tenants (`_all`) | Yes | No (403) |
| Create `tenant_admin` in own tenant | Yes | Yes (own tenant only) |
| Create `system_admin` | Yes | No (403) |
| Delete / reset password for user in own tenant | Yes | Yes (own tenant only) |
| Use `_all` on any write (`POST`/`PUT`/`DELETE`) | No (400) | No (400) |
| View `/v1/ui/me` | Yes | Yes |

---

## 9. `dbtool` Additions

New `-op` values added to `dbtool`:

| Operation | Flags | Description |
|---|---|---|
| `insert-ui-user` | `-email`, `-role`, `-tenant-id` (omit for system_admin) | Prompts for password on stdin; stores bcrypt hash |
| `list-ui-users` | `-tenant-id` (optional) | Lists users, optionally filtered by tenant |
| `delete-ui-user` | `-user-id` or `-email` | Deletes user and cascades refresh tokens |
| `reset-ui-user-password` | `-user-id` or `-email` | Prompts for new password on stdin |
| `deactivate-ui-user` | `-user-id` or `-email` | Sets `is_active = false` without deleting |

Password prompting (no echo) uses `golang.org/x/term`. The `dbtool` binary does not require `auth.json` since it never issues JWTs — it only manages the `ui_users` and `refresh_tokens` tables via direct DB access.

---

## 10. New Go Dependencies

All added to `apis/go.mod`:

| Package | Purpose |
|---|---|
| `github.com/golang-jwt/jwt/v5` | JWT signing and verification |
| `golang.org/x/crypto` | `bcrypt` password hashing |

Added to `db/go.mod` (for `dbtool`):

| Package | Purpose |
|---|---|
| `golang.org/x/crypto` | `bcrypt` for `insert-ui-user` / `reset-ui-user-password` |
| `golang.org/x/term` | Password prompting without echo |

---

## 11. Nginx Changes

No nginx changes are required. The `/v1/ui/` namespace falls through the existing catch-all:

```nginx
location /v1/ {
    proxy_pass http://apis-container:8080/v1/;
}
```

JWT validation is handled entirely within the Go layer. No `auth_request` sub-block is needed for UI routes.

---

## 12. Security Considerations

**Password storage:** bcrypt with cost factor 12. The `password_hash` column is never returned by any API endpoint. Login responses return tokens only.

**Timing attacks:** Login returns `401` with the same message and delay whether the email is unknown or the password is wrong, preventing user enumeration. Use `bcrypt.CompareHashAndPassword` (constant-time) in both branches; when the user is not found, run a dummy bcrypt comparison against a static sentinel hash to normalise timing.

**JWT secret rotation:** Changing `jwt_secret` in `auth.json` and restarting `apis` immediately invalidates all outstanding access JWTs. Refresh tokens survive (they are validated by DB lookup, not by JWT signature). Planned rotation: issue new secret, deploy, let users re-authenticate via refresh tokens.

**Refresh token security:** The opaque token value is only ever held in memory or transmitted over TLS; only its SHA-256 digest is persisted. A DB compromise does not expose reusable tokens.

**Tenant boundary:** The `chk_role_tenant_consistency` CHECK constraint and the `requireTenantScope` middleware provide defence-in-depth. A `tenant_admin` JWT cannot access another tenant's data even if the URL is manipulated.

**`system_admin` bootstrap:** The first `system_admin` user must be created via `dbtool` with direct DB access. There is intentionally no API endpoint for self-service `system_admin` creation.

**Refresh token expiry cleanup:** A periodic job (cron or at login time) can prune `refresh_tokens` rows where `expires_at < now`. This is a maintenance concern, not a security requirement, since expired tokens are always rejected regardless.

---

## 13. File Inventory

Files to be created or modified:

| File | Change |
|---|---|
| `db/ui_auth_schema.sql` | New — additive schema migration (ui_users, refresh_tokens) |
| `common/types.go` | Add `AuthConfig`, `UserClaims` structs |
| `apis/ui_auth.go` | New — login/refresh/logout handlers + JWT helpers |
| `apis/ui_handlers.go` | New — all `/v1/ui/` resource handlers |
| `apis/middleware.go` | New — `requireJWT`, `requireRole`, `requireTenantScope` |
| `apis/apis.go` | Add `-auth-config` flag, load `AuthConfig`, register new routes |
| `apis/go.mod` / `apis/go.sum` | Add `golang-jwt/jwt/v5`, `golang.org/x/crypto` |
| `db/dbutil.go` | Add `User`, `RefreshToken` DB methods |
| `db/dbtool.go` | Add new `-op` cases for user management |
| `db/go.mod` / `db/go.sum` | Add `golang.org/x/crypto`, `golang.org/x/term` |
| `config/auth.example.json` | New — example auth config (placeholder secret) |
| `apis/ui_auth_test.go` | New — unit tests for login, refresh, logout, middleware |

---

## 14. Open Questions

1. **Refresh token delivery mechanism** — Should the refresh token be returned in the JSON body (client stores it) or as an `HttpOnly Secure` cookie (browser manages it, XSS-resistant)? Cookie-based is strongly preferred for browser-originated UIs. Decision deferred to UI team.

2. **Multi-device sessions** — Currently one refresh token is issued per login. Should multiple concurrent refresh tokens per user be allowed (multiple devices), or should login invalidate all prior refresh tokens? Current design allows multiple concurrent sessions.

3. **Password policy** — Minimum length, complexity, expiry? None enforced in this design beyond a minimum 12-character length checked at creation time.

4. **Audit log** — Should login attempts (success and failure) be logged to a DB table or just to structured application logs? Recommend structured logs for now; DB audit table can be added later.

5. **Rate limiting** — Login endpoint should be rate-limited to prevent brute force. Current design has no rate limiting. Recommend nginx `limit_req` zone on `/v1/ui/auth/login` as a follow-on.

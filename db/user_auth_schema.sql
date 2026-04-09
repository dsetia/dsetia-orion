-- ===================================================================
-- USER AUTH SCHEMA — additive migration
-- Apply to an existing database:
--   psql $DSN -f db/user_auth_schema.sql
-- ===================================================================

-- UI users table.
-- Every user is tied to a tenant; tenant_id is never NULL.
-- role is enforced to one of two values by a CHECK constraint.
CREATE TABLE users (
    user_id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       BIGINT      NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email           TEXT        NOT NULL UNIQUE,
    password_hash   TEXT        NOT NULL,
    role            TEXT        NOT NULL CHECK (role IN ('security_analyst', 'system_admin')),
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    failed_attempts INT         NOT NULL DEFAULT 0,
    lockout_until   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_tenant      ON users(tenant_id);
CREATE INDEX idx_users_email_lower ON users(LOWER(email));

-- Refresh tokens table.
-- The actual token is never stored; only its SHA-256 hex digest is kept.
-- Revocation is performed by setting revoked = true.
CREATE TABLE refresh_tokens (
    token_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked      BOOLEAN     NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_hash ON refresh_tokens(token_hash);

-- Login audit log.
-- Records every login attempt (success and failure).
-- user_id is nullable because the user may not exist for unknown-email attempts.
-- failure_reason is one of: unknown_user, inactive_user, account_locked, incorrect_password.
-- On success, failure_reason is empty and success = true.
CREATE TABLE login_audit_log (
    id             BIGSERIAL   PRIMARY KEY,
    user_id        UUID        REFERENCES users(user_id) ON DELETE SET NULL,
    email          TEXT        NOT NULL,
    success        BOOLEAN     NOT NULL,
    ip_address     TEXT,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_login_audit_user    ON login_audit_log(user_id);
CREATE INDEX idx_login_audit_email   ON login_audit_log(LOWER(email));
CREATE INDEX idx_login_audit_created ON login_audit_log(created_at DESC);

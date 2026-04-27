#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Migrate V2 -> V3: schema changes applied by this script (all idempotent):
#
#   1. devices:  ADD COLUMN location TEXT
#   2. devices:  ADD CONSTRAINT chk_device_name_length
#                  CHECK (char_length(device_name) BETWEEN 1 AND 128)
#   3. devices:  ADD CONSTRAINT chk_location_length
#                  CHECK (char_length(location) <= 255)
#   4. tenants:  ADD CONSTRAINT chk_tenant_name_length
#                  CHECK (char_length(tenant_name) BETWEEN 1 AND 128)
#
# Constraint names match what schema_pg_v3.sql produces for fresh installs.
# Reads DB connection parameters from a JSON config file.
# =============================================================================

SCRIPT_NAME=$(basename "$0")

usage() {
    cat <<EOF
Usage: $SCRIPT_NAME -c /path/to/db-config.json

Options:
  -c, --config   Path to JSON config file (required)
  -h, --help     Show this help message

Example:
  $SCRIPT_NAME -c /opt/config/db.json

Expected config format:
{
  "host": "localhost",
  "port": 5432,
  "user": "pguser",
  "password": "pgpass",
  "dbname": "orion",
  "sslmode": "disable"
}
EOF
    exit 1
}

# Parse command line arguments
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

if [[ -z "$CONFIG_FILE" ]]; then
    echo "Error: Config file is required (-c /path/to/config.json)"
    usage
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "Error: Config file not found: $CONFIG_FILE"
    exit 1
fi

# Read values from JSON using jq (make sure jq is installed)
if ! command -v jq &> /dev/null; then
    echo "Error: 'jq' is required but not installed."
    echo "On Ubuntu/Debian: sudo apt-get install jq"
    echo "On macOS: brew install jq"
    exit 1
fi

HOST=$(jq -r '.host' "$CONFIG_FILE")
PORT=$(jq -r '.port' "$CONFIG_FILE")
USER=$(jq -r '.user' "$CONFIG_FILE")
PASSWORD=$(jq -r '.password' "$CONFIG_FILE")
DBNAME=$(jq -r '.dbname' "$CONFIG_FILE")
SSLMODE=$(jq -r '.sslmode // "prefer"' "$CONFIG_FILE")   # default to prefer if missing

# Basic validation
if [[ "$HOST" == "null" || "$USER" == "null" || "$PASSWORD" == "null" || "$DBNAME" == "null" ]]; then
    echo "Error: Missing required fields (host, user, password, dbname) in config file"
    exit 1
fi

# Export password for psql
export PGPASSWORD="$PASSWORD"

echo "Connecting to: $USER@$HOST:$PORT/$DBNAME (sslmode=$SSLMODE)"

# Run the migration
psql -h "$HOST" -p "$PORT" -U "$USER" -d "$DBNAME" --set=sslmode="$SSLMODE" <<'EOF'
DO $$
BEGIN
    -- -----------------------------------------------------------------
    -- Step 1: Add location column to devices (original v2->v3 change)
    -- -----------------------------------------------------------------
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name   = 'devices'
          AND column_name  = 'location'
    ) THEN
        ALTER TABLE devices ADD COLUMN location TEXT;
        RAISE NOTICE 'Step 1: Added column "location" to table "devices"';
    ELSE
        RAISE NOTICE 'Step 1: Column "location" already exists in "devices" — skipped';
    END IF;

    -- -----------------------------------------------------------------
    -- Step 2: devices.device_name length constraint (1-128 chars)
    -- -----------------------------------------------------------------
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname   = 'chk_device_name_length'
          AND conrelid  = 'devices'::regclass
    ) THEN
        ALTER TABLE devices
            ADD CONSTRAINT chk_device_name_length
            CHECK (char_length(device_name) BETWEEN 1 AND 128);
        RAISE NOTICE 'Step 2: Added constraint "chk_device_name_length" to "devices"';
    ELSE
        RAISE NOTICE 'Step 2: Constraint "chk_device_name_length" already exists — skipped';
    END IF;

    -- -----------------------------------------------------------------
    -- Step 3: devices.location length constraint (<= 255 chars)
    -- -----------------------------------------------------------------
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname   = 'chk_location_length'
          AND conrelid  = 'devices'::regclass
    ) THEN
        ALTER TABLE devices
            ADD CONSTRAINT chk_location_length
            CHECK (char_length(location) <= 255);
        RAISE NOTICE 'Step 3: Added constraint "chk_location_length" to "devices"';
    ELSE
        RAISE NOTICE 'Step 3: Constraint "chk_location_length" already exists — skipped';
    END IF;

    -- -----------------------------------------------------------------
    -- Step 4: tenants.tenant_name length constraint (1-128 chars)
    -- -----------------------------------------------------------------
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname   = 'chk_tenant_name_length'
          AND conrelid  = 'tenants'::regclass
    ) THEN
        ALTER TABLE tenants
            ADD CONSTRAINT chk_tenant_name_length
            CHECK (char_length(tenant_name) BETWEEN 1 AND 128);
        RAISE NOTICE 'Step 4: Added constraint "chk_tenant_name_length" to "tenants"';
    ELSE
        RAISE NOTICE 'Step 4: Constraint "chk_tenant_name_length" already exists — skipped';
    END IF;
END $$;

-- Show final constraint state for verification
SELECT conname, contype, pg_get_constraintdef(oid) AS definition
FROM   pg_constraint
WHERE  conrelid IN ('devices'::regclass, 'tenants'::regclass)
  AND  conname  IN ('chk_device_name_length', 'chk_location_length', 'chk_tenant_name_length')
ORDER BY conrelid::text, conname;

-- Optional: show current schema of devices table for verification
\dt+ devices

-- =========================================================================
-- User auth tables (users, refresh_tokens, login_audit_log)
-- =========================================================================

CREATE TABLE IF NOT EXISTS users (
    user_id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       BIGINT      NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email           TEXT        NOT NULL,
    password_hash   TEXT        NOT NULL,
    role            TEXT        NOT NULL CHECK (role IN ('security_analyst', 'system_admin')),
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    failed_attempts INT         NOT NULL DEFAULT 0,
    lockout_until   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS        idx_users_tenant      ON users(tenant_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_lower ON users(LOWER(email));

CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked      BOOLEAN     NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);

CREATE TABLE IF NOT EXISTS login_audit_log (
    id             BIGSERIAL   PRIMARY KEY,
    user_id        UUID,
    email          TEXT        NOT NULL,
    success        BOOLEAN     NOT NULL,
    ip_address     TEXT,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_audit_user    ON login_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_login_audit_email   ON login_audit_log(LOWER(email));
CREATE INDEX IF NOT EXISTS idx_login_audit_created ON login_audit_log(created_at DESC);

DO $$
BEGIN
    RAISE NOTICE 'User auth tables (users, refresh_tokens, login_audit_log) are present.';
END $$;
EOF

if [[ $? -eq 0 ]]; then
    echo ""
    echo "Migration completed successfully (4 steps applied)."
else
    echo ""
    echo "Migration failed. Check the output above for errors."
    exit 1
fi

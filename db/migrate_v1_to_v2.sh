
#!/bin/bash
#
# Copyright (c) 2025 SecurITe
# All rights reserved.
#
# This source code is the property of SecurITe.
# Unauthorized copying, modification, or distribution of this file,
# via any medium is strictly prohibited unless explicitly authorized
# by SecurITe.
#
# This software is proprietary and confidential.
#
# Copyright (c) 2025 SecurITe
# File Owner: deepinder@securite.world
#

# migrate_v1_to_v2.sh
# Works on real V1 → V2 migration

set -euo pipefail

# ===================================================================
# Argument parsing
# ===================================================================
DRY_RUN=false
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --config) CONFIG_FILE="$2"; shift 2 ;;
        --dry-run|-n) DRY_RUN=true; shift ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

[[ -z "$CONFIG_FILE" ]] && { echo "Error: --config is required"; exit 1; }
[[ ! -f "$CONFIG_FILE" ]] && { echo "Error: Config file not found: $CONFIG_FILE"; exit 1; }

command -v jq &> /dev/null || { echo "Error: jq is required"; exit 1; }

# Load config
PGHOST=$(jq -r '.host // "postgres"' "$CONFIG_FILE")
PGPORT=$(jq -r '.port // 5432' "$CONFIG_FILE")
PGUSER=$(jq -r '.user // "pguser"' "$CONFIG_FILE")
PGPASSWORD=$(jq -r '.password // "pgpass"' "$CONFIG_FILE")
PGDB=$(jq -r '.dbname // "pgdb"' "$CONFIG_FILE")
TARGET_ENV=$(jq -r '.environment' "$CONFIG_FILE")

VALID_ENVS=("private-staging" "private-prod" "aws-prod" "gcloud-prod" "azure-prod")
if ! printf '%s\n' "${VALID_ENVS[@]}" | grep -Fx "$TARGET_ENV" > /dev/null; then
    echo "Invalid environment: $TARGET_ENV"
    exit 1
fi

export PGPASSWORD
BACKUP_DIR="backups/db_migration_backup_$(date +%Y%m%d_%H%M%S)"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
header() { echo -e "${GREEN}========================================${NC}"; echo -e "${GREEN}$1${NC}"; echo -e "${GREEN}========================================${NC}"; }
step()   { echo -e "${YELLOW}→ $1${NC}"; }
ok()     { echo -e "${GREEN}✓ $1${NC}"; }
err()    { echo -e "${RED}✗ $1${NC}"; }

# ===================================================================
# Core functions — FIXED ORDER & CONTENT
# ===================================================================
check_prerequisites() {
    step "Checking prerequisites..."
    command -v psql &> /dev/null || { err "psql not found"; exit 1; }
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT 1" &> /dev/null || { err "Cannot connect"; exit 1; }
    mkdir -p "$BACKUP_DIR"
    ok "OK"
}

preview() {
    header "PREVIEW"
    step "Checking schema..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c '\dt tenants' &> /dev/null || { err "No tenants table → not V1"; exit 1; }
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT 1 FROM information_schema.columns WHERE table_name='tenants' AND column_name='environment'" -t -A | grep -q 1 && { err "Already migrated"; exit 1; }

    count=$(psql -t -A -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT COUNT(*) FROM tenants")
    echo "Found $count tenant(s) → will be moved to environment: $TARGET_ENV"

    case "$TARGET_ENV" in
        private-staging)  range="1–1000"     ;;
        private-prod)     range="1001–10000" ;;
        aws-prod)         range="11000–20000";;
        gcloud-prod)      range="21000–30000";;
        azure-prod)       range="31000–40000";;
    esac
    echo -e "Reserved range for $TARGET_ENV: $range"
}

backup() {
    step "Backing up database..."
    pg_dump -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" > "$BACKUP_DIR/full_dump.sql"
    for t in tenants devices api_keys hndr_sw hndr_rules threatintel status version; do
        psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "\COPY $t TO '$BACKUP_DIR/$t.csv' CSV HEADER" 2>/dev/null || true
    done
    ok "Backup saved to $BACKUP_DIR"
}

# FIXED: Create table first, then insert
create_tenant_id_blocks() {
    step "Creating tenant_id_blocks table and inserting ranges..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
CREATE TABLE IF NOT EXISTS tenant_id_blocks (
    environment TEXT PRIMARY KEY,
    start_id BIGINT NOT NULL,
    end_id BIGINT NOT NULL,
    description TEXT,
    CHECK (end_id >= start_id),
    CHECK (start_id > 0)
);

INSERT INTO tenant_id_blocks (environment, start_id, end_id, description) VALUES
    ('private-staging',  1,     1000,   'Private staging tenants'),
    ('private-prod',     1001,  10000,  'Private production tenants'),
    ('aws-prod',         11000, 20000,  'AWS production tenants'),
    ('gcloud-prod',      21000, 30000,  'GCloud production tenants'),
    ('azure-prod',       31000, 40000,  'Azure production tenants')
ON CONFLICT (environment) DO NOTHING;
EOF
    ok "tenant_id_blocks created"
}

create_sequences() {
    step "Creating sequences..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
CREATE SEQUENCE IF NOT EXISTS seq_private_staging_tenant_id MINVALUE 1    MAXVALUE 1000   START 1;
CREATE SEQUENCE IF NOT EXISTS seq_private_prod_tenant_id    MINVALUE 1001 MAXVALUE 10000  START 1001;
CREATE SEQUENCE IF NOT EXISTS seq_aws_prod_tenant_id        MINVALUE 11000 MAXVALUE 20000 START 11000;
CREATE SEQUENCE IF NOT EXISTS seq_gcloud_prod_tenant_id     MINVALUE 21000 MAXVALUE 30000 START 21000;
CREATE SEQUENCE IF NOT EXISTS seq_azure_prod_tenant_id      MINVALUE 31000 MAXVALUE 40000 START 31000;
EOF
    ok "Sequences ready"
}

drop_fks() {
    step "Dropping foreign keys..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
ALTER TABLE IF EXISTS devices    DROP CONSTRAINT IF EXISTS devices_tenant_id_fkey;
ALTER TABLE IF EXISTS api_keys   DROP CONSTRAINT IF EXISTS api_keys_tenant_id_fkey;
ALTER TABLE IF EXISTS hndr_rules DROP CONSTRAINT IF EXISTS hndr_rules_tenant_id_fkey;
ALTER TABLE IF EXISTS status     DROP CONSTRAINT IF EXISTS status_tenant_id_fkey;

-- Only drop version FK if table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'version') THEN
        ALTER TABLE version DROP CONSTRAINT IF EXISTS version_tenant_id_fkey;
    END IF;
END $$;
EOF
    ok "FKs dropped"
}

migrate_tenants() {
    step "Migrating tenants table (assigning to $TARGET_ENV)..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<EOF
BEGIN;

CREATE TABLE tenants_backup AS SELECT * FROM tenants;
DROP TABLE tenants CASCADE;

CREATE TABLE tenants (
    tenant_id BIGINT PRIMARY KEY,
    tenant_name TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment) REFERENCES tenant_id_blocks(environment)
);

INSERT INTO tenants (tenant_id, tenant_name, environment, created_at, updated_at)
SELECT tenant_id::bigint, tenant_name, '$TARGET_ENV', created_at, updated_at
FROM tenants_backup;

COMMIT;
EOF
    ok "Tenants migrated"
}

create_version_table_if_missing() {
    step "Checking for version table..."
    local exists=$(psql -t -A -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c \
        "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'version')")

    if [[ "$exists" == "t" ]]; then
        ok "Version table exists"
        return
    fi

    step "Creating missing version table..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
CREATE TABLE version (
    device_id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    software TEXT NOT NULL,
    rules TEXT NOT NULL,
    threatintel TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index
CREATE INDEX IF NOT EXISTS idx_version_tenant ON version(tenant_id);
EOF
    ok "Version table created"
}

widen_columns() {
    step "Converting tenant_id columns to BIGINT..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
ALTER TABLE devices    ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE api_keys   ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE hndr_rules ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE status     ALTER COLUMN tenant_id TYPE BIGINT;

-- Only widen version.tenant_id if table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'version') THEN
        ALTER TABLE version ALTER COLUMN tenant_id TYPE BIGINT;
    END IF;
END $$;
EOF
    ok "Columns widened"
}

restore_fks() {
    step "Restoring foreign keys..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
ALTER TABLE devices    ADD CONSTRAINT devices_tenant_id_fkey    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
ALTER TABLE api_keys   ADD CONSTRAINT api_keys_tenant_id_fkey   FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
ALTER TABLE hndr_rules ADD CONSTRAINT hndr_rules_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
ALTER TABLE status     ADD CONSTRAINT status_tenant_id_fkey     FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

-- Only restore version FK if table exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'version') THEN
        ALTER TABLE version ADD CONSTRAINT version_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
    END IF;
END $$;
EOF
    ok "FKs restored"
}

create_functions_triggers_view() {
    step "Creating functions, triggers and view..."
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<'EOF'
-- Function to get next tenant ID
CREATE OR REPLACE FUNCTION get_next_tenant_id(env TEXT) RETURNS BIGINT AS $$
DECLARE seq_name TEXT := 'seq_' || replace(env, '-', '_') || '_tenant_id';
BEGIN
    RETURN nextval(seq_name);
END;
$$ LANGUAGE plpgsql;

-- Validation trigger
CREATE OR REPLACE FUNCTION validate_tenant_id_range() RETURNS TRIGGER AS $$
DECLARE r tenant_id_blocks%ROWTYPE;
BEGIN
    SELECT * INTO r FROM tenant_id_blocks WHERE environment = NEW.environment;
    IF NEW.tenant_id < r.start_id OR NEW.tenant_id > r.end_id THEN
        RAISE EXCEPTION 'tenant_id % out of range for environment %', NEW.tenant_id, NEW.environment;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_tenant_id_range ON tenants;
CREATE TRIGGER trg_validate_tenant_id_range
    BEFORE INSERT OR UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION validate_tenant_id_range();

-- Monitoring view
CREATE OR REPLACE VIEW tenant_allocation_status AS
SELECT
    b.environment,
    b.start_id,
    b.end_id,
    b.end_id - b.start_id + 1 AS total_capacity,
    COUNT(t.tenant_id) AS allocated,
    b.end_id - b.start_id + 1 - COUNT(t.tenant_id) AS remaining,
    ROUND(100.0 * COUNT(t.tenant_id) / (b.end_id - b.start_id + 1), 2) AS utilization_pct
FROM tenant_id_blocks b
LEFT JOIN tenants t ON t.environment = b.environment
GROUP BY b.environment, b.start_id, b.end_id;
EOF
    ok "Functions & view created"
}

set_sequence() {
    step "Setting sequence value for $TARGET_ENV..."
    local seq=$(echo "$TARGET_ENV" | tr '-' '_')
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" <<EOF
DO \$\$
DECLARE
    max_id BIGINT := COALESCE((SELECT MAX(tenant_id) FROM tenants WHERE environment = '$TARGET_ENV'), 0);
    min_val BIGINT := (SELECT start_id - 1 FROM tenant_id_blocks WHERE environment = '$TARGET_ENV');
BEGIN
    PERFORM setval('seq_${seq}_tenant_id', GREATEST(max_id, min_val));
END \$\$;
EOF
    ok "Sequence set"
}

verify() {
    step "Verification..."
    echo -e "\n=== tenant_allocation_status ==="
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT * FROM tenant_allocation_status;"
    echo -e "\n=== Sample tenants ==="
    psql -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" -d "$PGDB" -c "SELECT tenant_id, tenant_name, environment FROM tenants LIMIT 10;"
    ok "Migration complete and verified"
}

# ===================================================================
# Main
# ===================================================================
main() {
    header "SecurITe DB Migration V1 → V2"
    echo "Config: $CONFIG_FILE"
    echo "Target environment: $TARGET_ENV"
    echo "Database: $PGDB@$PGHOST:$PGPORT"
    $DRY_RUN && echo -e "${YELLOW}=== DRY RUN MODE ===${NC}"

    read -p "Continue? (yes/no): " ans
    [[ "$ans" != "yes" ]] && exit 0

    check_prerequisites
    preview

    if $DRY_RUN; then
        header "DRY RUN FINISHED — no changes made"
        exit 0
    fi

    echo -e "\n${RED}FINAL CONFIRMATION${NC}"
    read -p "Type 'MIGRATE NOW' to proceed: " confirm
    [[ "$confirm" != "MIGRATE NOW" ]] && echo "Cancelled" && exit 0

    backup
    create_tenant_id_blocks
    create_sequences
    drop_fks
    migrate_tenants
    create_version_table_if_missing
    widen_columns
    restore_fks
    create_functions_triggers_view
    set_sequence
    verify

    header "MIGRATION SUCCESSFUL!"
    echo "All tenants are now in environment: $TARGET_ENV"
    echo "Backup location: $BACKUP_DIR"
}

main

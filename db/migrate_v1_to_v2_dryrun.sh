#!/bin/bash
#
# migrate_v1_to_v2.sh
# Migration script from Schema V1 to V2 (Simplified - single environment field)
# Now with --dry-run support and detailed preview/conflict detection
#
# Copyright (c) 2025 SecurITe
# File Owner: deepinder@securite.world
#

set -e

# ===================================================================
# Argument handling
# ===================================================================
DRY_RUN=false
if [[ "$1" == "--dry-run" || "$1" == "-n" ]]; then
    DRY_RUN=true
fi

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration (same as original)
PGHOST="${DB_HOST:-postgres}"
PGPORT="${DB_PORT:-5432}"
PGUSER="${DB_USER:-pguser}"
PGPASSWORD="${DB_PASSWORD:-pgpass}"
PGDB="${DB_NAME:-pgdb}"
BACKUP_DIR="${BACKUP_DIR:-/tmp/db_migration_backup}"

export PGPASSWORD

# ===================================================================
# Helper functions (unchanged)
# ===================================================================
print_header() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${GREEN}========================================${NC}"
}

print_step() {
    echo -e "${YELLOW}➜ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ ERROR: $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

check_prerequisites() {
    print_step "Checking prerequisites..."
    
    # Check if psql is available
    if ! command -v psql &> /dev/null; then
        print_error "psql not found. Please install PostgreSQL client."
        exit 1
    fi
    
    # Check database connection
    if ! psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT 1" &> /dev/null; then
        print_error "Cannot connect to database. Please check credentials."
        exit 1
    fi
    
    # Create backup directory
    mkdir -p $BACKUP_DIR
    
    print_success "Prerequisites check passed"
}

backup_data() {
    print_step "Backing up existing data..."
    
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    
    # Backup tenants
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "\COPY tenants TO '$BACKUP_DIR/tenants_$TIMESTAMP.csv' CSV HEADER" || true
    
    # Backup devices
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "\COPY devices TO '$BACKUP_DIR/devices_$TIMESTAMP.csv' CSV HEADER" || true
    
    # Backup api_keys
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "\COPY api_keys TO '$BACKUP_DIR/api_keys_$TIMESTAMP.csv' CSV HEADER" || true
    
    # Backup hndr_rules
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "\COPY hndr_rules TO '$BACKUP_DIR/hndr_rules_$TIMESTAMP.csv' CSV HEADER" || true
    
    # Backup status
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "\COPY status TO '$BACKUP_DIR/status_$TIMESTAMP.csv' CSV HEADER" || true
    
    # Create full database dump
    pg_dump -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        > "$BACKUP_DIR/full_backup_$TIMESTAMP.sql"
    
    print_success "Backup completed at: $BACKUP_DIR"
    echo "  - Timestamp: $TIMESTAMP"
}

# ===================================================================
# NEW: Detailed preview function
# ===================================================================
preview_migration() {
    print_header "MIGRATION PREVIEW / IMPACT ANALYSIS"

    print_step "Analyzing current tenants table..."

    # CORRECT: Check if tenants table exists and has old-style integer tenant_id
    if ! psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c '\dt tenants' >/dev/null 2>&1
    then
        print_error "No 'tenants' table found in database '$PGDB'."
        echo "    This script only works on V1 schema databases that have a 'tenants' table."
        exit 1
    fi

    # Additional safety: confirm tenant_id is still INTEGER (not already migrated)
    if psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "
        SELECT data_type FROM information_schema.columns
        WHERE table_name='tenants' AND column_name='tenant_id'
    " -t -A | grep -iq "big"; then
        print_error "tenant_id is already BIGINT → this database appears to be ALREADY MIGRATED to V2!"
        echo "    No action needed."
        exit 1
    fi

    tenant_count=$(psql -t -A -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT COUNT(*) FROM tenants;")

    echo -e "${YELLOW}Found $tenant_count existing tenant(s). All will be migrated to the 'private-prod' environment with their current tenant_id preserved.${NC}"

    if [ "$tenant_count" -gt 0 ]; then
        echo ""
        echo "=== CURRENT TENANTS (will become private-prod) ==="
        psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "
            SELECT tenant_id::bigint AS tenant_id, tenant_name, created_at, updated_at
            FROM tenants
            ORDER BY tenant_id;"
        echo ""

        max_id=$(psql -t -A -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT COALESCE(MAX(tenant_id)::text, '0') FROM tenants;")
        min_id=$(psql -t -A -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT COALESCE(MIN(tenant_id)::text, '0') FROM tenants;")

        echo "Current tenant_id range: $min_id – $max_id"
        echo ""
    else
        max_id=0
        echo "No existing tenants → next tenant_id will be 1001"
        echo ""
    fi

    # Planned next tenant_id for private-prod
    planned_setval=$max_id
    if [ $max_id -lt 1001 ]; then
        planned_setval=1000
    fi
    next_private_prod=$((planned_setval + 1))

    echo -e "Next tenant_id for ${GREEN}private-prod${NC} will be → ${GREEN}$next_private_prod${NC}"

    # Conflict detection
    echo ""
    print_step "Checking for potential primary key conflicts with reserved ranges..."

    has_conflict=false

    # Staging, AWS, GCloud, Azure ranges
    declare -a envs=("private-staging" "aws-prod" "gcloud-prod" "azure-prod")
    declare -a starts=(1 11000 21000 31000)
    declare -a ends=(1000 20000 30000 40000)

    for i in "${!envs[@]}"; do
        env="${envs[i]}"
        start="${starts[i]}"
        end_="${ends[i]}"
	echo "Checking env=$env, start=$start, end=$end_"
        conflicting=$(psql -t -A -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT COUNT(*) FROM tenants WHERE tenant_id >= $start AND tenant_id <= $end_")

        if [ "$conflicting" -gt 0 ]; then
            print_error "CONFLICT: $conflicting tenant(s) have tenant_id in $env range ($start–$end_)"
            has_conflict=true
            # Show the offending IDs (optional, but helpful)
            echo "    Affected tenant_id(s):"
            psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT tenant_id, tenant_name FROM tenants WHERE tenant_id >= $start AND tenant_id <= $end_ ORDER BY tenant_id;"
            echo ""
        fi
    done

    # Out-of-range for private-prod
    out_of_range=$(psql -t -A -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT COUNT(*) FROM tenants WHERE tenant_id < 1001 OR tenant_id > 10000;")

    if [ "$out_of_range" -gt 0 ]; then
        print_error "WARNING: $out_of_range tenant(s) are outside the private-prod range (1001–10000)."
        echo "    These tenants will still be usable, but any UPDATE on them will fail after migration due to the validation trigger."
        echo "    Affected tenant_id(s):"
        psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB -c "SELECT tenant_id, tenant_name FROM tenants WHERE tenant_id < 1001 OR tenant_id > 10000 ORDER BY tenant_id;"
        echo ""
    fi

    if $has_conflict; then
        echo -e "${RED}One or more primary key conflicts detected. If you plan to use other environments (staging, aws, gcloud, azure), you must manually re-assign conflicting tenant_ids before migration.${NC}"
        echo ""
    fi

    # Simulated tenant_allocation_status view
    echo ""
    print_step "Simulated tenant_allocation_status after migration"
    printf "%-18s | %8s | %8s | %13s | %10s | %18s | %12s\n" "environment" "start_id" "end_id" "total_capacity" "allocated" "remaining_capacity" "utilization_percent"
    printf "%-18s | %8s | %8s | %13s | %10s | %18s | %12s\n" "------------------" "--------" "-------" "-------------" "----------" "------------------" "------------------" 

    # private-staging
    printf "%-18s | %8d | %8d | %13d | %10d | %18d | %11.2f%%\n" "private-staging" 1 1000 1000 0 1000 0.00

    # private-prod
    prod_cap=9000
    prod_remaining=$((prod_cap - tenant_count))
    (( prod_remaining < 0 )) && prod_remaining=0
    prod_util=$(awk 'BEGIN {printf "%.2f", 100.0 * '"$tenant_count"' / 9000 }')
    printf "%-18s | %8d | %8d | %13d | %10d | %18d | %11.2f%%\n" "private-prod" 1001 10000 $prod_cap $tenant_count $prod_remaining "$prod_util"

    # aws, gcloud, azure
    other_cap=9001
    # 9001 slots each
    printf "%-18s | %8d | %8d | %13d | %10d | %18d | %11.2f%%\n" "aws-prod" 11000 20000 $other_cap 0 $other_cap 0.00
    printf "%-18s | %8d | %8d | %13d | %10d | %18d | %11.2f%%\n" "gcloud-prod" 21000 30000 $other_cap 0 $other_cap 0.00
    printf "%-18s | %8d | %8d | %13d | %10d | %18d | %11.2f%%\n" "azure-prod" 31000 40000 $other_cap 0 $other_cap 0.00

    echo ""
}

# ===================================================================
# All original functions unchanged (copy all)
# ===================================================================
create_tenant_id_blocks() {
create_tenant_id_blocks() {
    print_step "Creating tenant_id_blocks table..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
-- Create tenant ID blocks table (simplified - single environment field)
CREATE TABLE IF NOT EXISTS tenant_id_blocks (
    environment TEXT PRIMARY KEY,
    start_id BIGINT NOT NULL,
    end_id BIGINT NOT NULL,
    description TEXT,
    CHECK (end_id >= start_id),
    CHECK (start_id > 0)
);

-- Insert predefined ID blocks
INSERT INTO tenant_id_blocks (environment, start_id, end_id, description) VALUES
    ('private-staging',  1,     1000,   'Private staging tenants (1-1000)'),
    ('private-prod',     1001,  10000,  'Private production tenants (1001-10000)'),
    ('aws-prod',         11000, 20000,  'AWS production tenants (11000-20000)'),
    ('gcloud-prod',      21000, 30000,  'GCloud production tenants (21000-30000)'),
    ('azure-prod',       31000, 40000,  'Azure production tenants (31000-40000)')
ON CONFLICT (environment) DO NOTHING;
EOF
    
    print_success "tenant_id_blocks table created"
}

create_sequences() {
    print_step "Creating sequences for ID allocation..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
CREATE SEQUENCE IF NOT EXISTS seq_private_staging_tenant_id
    START WITH 1 INCREMENT BY 1 MINVALUE 1 MAXVALUE 1000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_private_prod_tenant_id
    START WITH 1001 INCREMENT BY 1 MINVALUE 1001 MAXVALUE 10000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_aws_prod_tenant_id
    START WITH 11000 INCREMENT BY 1 MINVALUE 11000 MAXVALUE 20000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_gcloud_prod_tenant_id
    START WITH 21000 INCREMENT BY 1 MINVALUE 21000 MAXVALUE 30000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_azure_prod_tenant_id
    START WITH 31000 INCREMENT BY 1 MINVALUE 31000 MAXVALUE 40000 NO CYCLE;
EOF
    
    print_success "Sequences created"
}

drop_foreign_keys() {
    print_step "Dropping foreign key constraints..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_tenant_id_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_tenant_id_fkey;
ALTER TABLE hndr_rules DROP CONSTRAINT IF EXISTS hndr_rules_tenant_id_fkey;
ALTER TABLE status DROP CONSTRAINT IF EXISTS status_tenant_id_fkey;
EOF
    
    print_success "Foreign key constraints dropped"
}

migrate_tenants_table() {
    print_step "Migrating tenants table..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
BEGIN;

-- Create backup table
CREATE TABLE tenants_backup AS SELECT * FROM tenants;

-- Drop old tenants table
DROP TABLE tenants CASCADE;

-- Create new tenants table with BIGINT and single environment field
CREATE TABLE tenants (
    tenant_id BIGINT PRIMARY KEY,
    tenant_name TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment) REFERENCES tenant_id_blocks(environment)
);

-- Migrate existing tenants (assuming private-prod environment)
INSERT INTO tenants (tenant_id, tenant_name, environment, created_at, updated_at)
SELECT tenant_id, tenant_name, 'private-prod', created_at, updated_at
FROM tenants_backup;

COMMIT;
EOF
    
    print_success "Tenants table migrated"
}

alter_related_tables() {
    print_step "Altering related tables to use BIGINT..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE api_keys ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE hndr_rules ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE status ALTER COLUMN tenant_id TYPE BIGINT;
EOF
    
    print_success "Related tables altered"
}

restore_foreign_keys() {
    print_step "Restoring foreign key constraints..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices ADD CONSTRAINT devices_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE api_keys ADD CONSTRAINT api_keys_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE hndr_rules ADD CONSTRAINT hndr_rules_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE status ADD CONSTRAINT status_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
EOF
    
    print_success "Foreign key constraints restored"
}

create_functions_and_triggers() {
    print_step "Creating helper functions and triggers..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
-- Function to get next tenant ID (simplified - single parameter)
CREATE OR REPLACE FUNCTION get_next_tenant_id(env TEXT)
RETURNS BIGINT AS $$
DECLARE
    next_id BIGINT;
    seq_name TEXT;
BEGIN
    -- Construct sequence name from environment (replace hyphen with underscore)
    seq_name := 'seq_' || replace(env, '-', '_') || '_tenant_id';
    
    -- Get next value from the appropriate sequence
    EXECUTE format('SELECT nextval(%L)', seq_name) INTO next_id;
    
    RETURN next_id;
END;
$$ LANGUAGE plpgsql;

-- Function to validate tenant ID range (simplified)
CREATE OR REPLACE FUNCTION validate_tenant_id_range()
RETURNS TRIGGER AS $$
DECLARE
    block_start BIGINT;
    block_end BIGINT;
BEGIN
    -- Get the ID range for this environment
    SELECT start_id, end_id INTO block_start, block_end
    FROM tenant_id_blocks
    WHERE environment = NEW.environment;
    
    IF NEW.tenant_id < block_start OR NEW.tenant_id > block_end THEN
        RAISE EXCEPTION 'Tenant ID % is outside the valid range [%--%] for environment=%',
            NEW.tenant_id, block_start, block_end, NEW.environment;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to validate tenant ID range
CREATE TRIGGER trg_validate_tenant_id_range
    BEFORE INSERT OR UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION validate_tenant_id_range();
EOF
    
    print_success "Functions and triggers created"
}

create_monitoring_view() {
    print_step "Creating monitoring view..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
CREATE OR REPLACE VIEW tenant_allocation_status AS
SELECT 
    b.environment,
    b.start_id,
    b.end_id,
    b.end_id - b.start_id + 1 AS total_capacity,
    COUNT(t.tenant_id) AS allocated_count,
    b.end_id - b.start_id + 1 - COUNT(t.tenant_id) AS remaining_capacity,
    ROUND(100.0 * COUNT(t.tenant_id) / (b.end_id - b.start_id + 1), 2) AS utilization_percent
FROM tenant_id_blocks b
LEFT JOIN tenants t ON b.environment = t.environment
GROUP BY b.environment, b.start_id, b.end_id
ORDER BY b.start_id;
EOF
    
    print_success "Monitoring view created"
}

set_sequence_values() {
    print_step "Setting sequence current values..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
DO $$
DECLARE
    max_tenant_id BIGINT;
BEGIN
    -- Get max existing tenant ID
    SELECT COALESCE(MAX(tenant_id), 1000) INTO max_tenant_id FROM tenants;
    
    -- Set sequence to start after the max existing ID
    IF max_tenant_id < 1001 THEN
        max_tenant_id := 1000;
    END IF;
    
    EXECUTE format('SELECT setval(%L, %s, true)', 'seq_private_prod_tenant_id', max_tenant_id);
    
    RAISE NOTICE 'Set seq_private_prod_tenant_id to %', max_tenant_id;
END $$;
EOF
    
    print_success "Sequence values set"
}
    print_step "Creating tenant_id_blocks table..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
-- Create tenant ID blocks table (simplified - single environment field)
CREATE TABLE IF NOT EXISTS tenant_id_blocks (
    environment TEXT PRIMARY KEY,
    start_id BIGINT NOT NULL,
    end_id BIGINT NOT NULL,
    description TEXT,
    CHECK (end_id >= start_id),
    CHECK (start_id > 0)
);

-- Insert predefined ID blocks
INSERT INTO tenant_id_blocks (environment, start_id, end_id, description) VALUES
    ('private-staging',  1,     1000,   'Private staging tenants (1-1000)'),
    ('private-prod',     1001,  10000,  'Private production tenants (1001-10000)'),
    ('aws-prod',         11000, 20000,  'AWS production tenants (11000-20000)'),
    ('gcloud-prod',      21000, 30000,  'GCloud production tenants (21000-30000)'),
    ('azure-prod',       31000, 40000,  'Azure production tenants (31000-40000)')
ON CONFLICT (environment) DO NOTHING;
EOF
    
    print_success "tenant_id_blocks table created"
}

create_sequences() {
    print_step "Creating sequences for ID allocation..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
CREATE SEQUENCE IF NOT EXISTS seq_private_staging_tenant_id
    START WITH 1 INCREMENT BY 1 MINVALUE 1 MAXVALUE 1000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_private_prod_tenant_id
    START WITH 1001 INCREMENT BY 1 MINVALUE 1001 MAXVALUE 10000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_aws_prod_tenant_id
    START WITH 11000 INCREMENT BY 1 MINVALUE 11000 MAXVALUE 20000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_gcloud_prod_tenant_id
    START WITH 21000 INCREMENT BY 1 MINVALUE 21000 MAXVALUE 30000 NO CYCLE;

CREATE SEQUENCE IF NOT EXISTS seq_azure_prod_tenant_id
    START WITH 31000 INCREMENT BY 1 MINVALUE 31000 MAXVALUE 40000 NO CYCLE;
EOF
    
    print_success "Sequences created"
}

drop_foreign_keys() {
    print_step "Dropping foreign key constraints..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_tenant_id_fkey;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_tenant_id_fkey;
ALTER TABLE hndr_rules DROP CONSTRAINT IF EXISTS hndr_rules_tenant_id_fkey;
ALTER TABLE status DROP CONSTRAINT IF EXISTS status_tenant_id_fkey;
EOF
    
    print_success "Foreign key constraints dropped"
}

migrate_tenants_table() {
    print_step "Migrating tenants table..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
BEGIN;

-- Create backup table
CREATE TABLE tenants_backup AS SELECT * FROM tenants;

-- Drop old tenants table
DROP TABLE tenants CASCADE;

-- Create new tenants table with BIGINT and single environment field
CREATE TABLE tenants (
    tenant_id BIGINT PRIMARY KEY,
    tenant_name TEXT NOT NULL UNIQUE,
    environment TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment) REFERENCES tenant_id_blocks(environment)
);

-- Migrate existing tenants (assuming private-prod environment)
INSERT INTO tenants (tenant_id, tenant_name, environment, created_at, updated_at)
SELECT tenant_id, tenant_name, 'private-prod', created_at, updated_at
FROM tenants_backup;

COMMIT;
EOF
    
    print_success "Tenants table migrated"
}

alter_related_tables() {
    print_step "Altering related tables to use BIGINT..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE api_keys ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE hndr_rules ALTER COLUMN tenant_id TYPE BIGINT;
ALTER TABLE status ALTER COLUMN tenant_id TYPE BIGINT;
EOF
    
    print_success "Related tables altered"
}

restore_foreign_keys() {
    print_step "Restoring foreign key constraints..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
ALTER TABLE devices ADD CONSTRAINT devices_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE api_keys ADD CONSTRAINT api_keys_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE hndr_rules ADD CONSTRAINT hndr_rules_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;

ALTER TABLE status ADD CONSTRAINT status_tenant_id_fkey 
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id) ON DELETE CASCADE;
EOF
    
    print_success "Foreign key constraints restored"
}

create_functions_and_triggers() {
    print_step "Creating helper functions and triggers..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
-- Function to get next tenant ID (simplified - single parameter)
CREATE OR REPLACE FUNCTION get_next_tenant_id(env TEXT)
RETURNS BIGINT AS $$
DECLARE
    next_id BIGINT;
    seq_name TEXT;
BEGIN
    -- Construct sequence name from environment (replace hyphen with underscore)
    seq_name := 'seq_' || replace(env, '-', '_') || '_tenant_id';
    
    -- Get next value from the appropriate sequence
    EXECUTE format('SELECT nextval(%L)', seq_name) INTO next_id;
    
    RETURN next_id;
END;
$$ LANGUAGE plpgsql;

-- Function to validate tenant ID range (simplified)
CREATE OR REPLACE FUNCTION validate_tenant_id_range()
RETURNS TRIGGER AS $$
DECLARE
    block_start BIGINT;
    block_end BIGINT;
BEGIN
    -- Get the ID range for this environment
    SELECT start_id, end_id INTO block_start, block_end
    FROM tenant_id_blocks
    WHERE environment = NEW.environment;
    
    IF NEW.tenant_id < block_start OR NEW.tenant_id > block_end THEN
        RAISE EXCEPTION 'Tenant ID % is outside the valid range [%--%] for environment=%',
            NEW.tenant_id, block_start, block_end, NEW.environment;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to validate tenant ID range
CREATE TRIGGER trg_validate_tenant_id_range
    BEFORE INSERT OR UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION validate_tenant_id_range();
EOF
    
    print_success "Functions and triggers created"
}

create_monitoring_view() {
    print_step "Creating monitoring view..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
CREATE OR REPLACE VIEW tenant_allocation_status AS
SELECT 
    b.environment,
    b.start_id,
    b.end_id,
    b.end_id - b.start_id + 1 AS total_capacity,
    COUNT(t.tenant_id) AS allocated_count,
    b.end_id - b.start_id + 1 - COUNT(t.tenant_id) AS remaining_capacity,
    ROUND(100.0 * COUNT(t.tenant_id) / (b.end_id - b.start_id + 1), 2) AS utilization_percent
FROM tenant_id_blocks b
LEFT JOIN tenants t ON b.environment = t.environment
GROUP BY b.environment, b.start_id, b.end_id
ORDER BY b.start_id;
EOF
    
    print_success "Monitoring view created"
}

set_sequence_values() {
    print_step "Setting sequence current values..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF'
DO $$
DECLARE
    max_tenant_id BIGINT;
BEGIN
    -- Get max existing tenant ID
    SELECT COALESCE(MAX(tenant_id), 1000) INTO max_tenant_id FROM tenants;
    
    -- Set sequence to start after the max existing ID
    IF max_tenant_id < 1001 THEN
        max_tenant_id := 1000;
    END IF;
    
    EXECUTE format('SELECT setval(%L, %s, true)', 'seq_private_prod_tenant_id', max_tenant_id);
    
    RAISE NOTICE 'Set seq_private_prod_tenant_id to %', max_tenant_id;
END $$;
EOF
    
    print_success "Sequence values set"
}

verify_migration() {
    print_step "Verifying migration..."
    
    echo ""
    echo "Tenant Allocation Status:"
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "SELECT * FROM tenant_allocation_status;"
    
    echo ""
    echo "Tenant Count:"
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "SELECT environment, COUNT(*) as count FROM tenants GROUP BY environment;"
    
    echo ""
    echo "Sample Tenants:"
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB \
        -c "SELECT tenant_id, tenant_name, environment FROM tenants LIMIT 5;"
    
    print_success "Migration verification complete"
}

cleanup() {
    print_step "Cleaning up temporary tables..."
    
    psql -h $PGHOST -p $PGPORT -U $PGUSER -d $PGDB << 'EOF' 2>/dev/null || true
DROP TABLE IF EXISTS tenants_backup;
EOF
    
    print_success "Cleanup complete"
}

# ===================================================================
# Main
# ===================================================================
main() {
    print_header "Database Migration: Schema V1 → V2 (with dry-run support)"

    echo "Database: $PGDB@$PGHOST:$PGPORT"
    echo "Backup directory: $BACKUP_DIR"
    echo ""

    if $DRY_RUN; then
        echo -e "${YELLOW}=== DRY RUN MODE ENABLED (no changes will be made) ===${NC}"
        echo ""
    fi

    proceed_text="perform the migration"
    if $DRY_RUN; then proceed_text="preview the migration impact"; fi

    echo "This script will $proceed_text."
    echo ""
    read -p "Do you want to continue? (type 'yes' to confirm): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Cancelled."
        exit 0
    fi

    check_prerequisites

    preview_migration   # <--- ALWAYS show preview first

    if $DRY_RUN; then
        print_header "DRY RUN completed – database unchanged"
        echo "If the preview looks good, run without '--dry-run' to apply the migration."
        exit 0
    fi

    # Final safety gate for real migration
    echo ""
    echo -e "${RED}FINAL CONFIRMATION: This will permanently modify the database.${NC}"
    echo "A full backup will be created in $BACKUP_DIR"
    echo ""
    read -p "Type 'YES' to proceed with the migration: " final_confirm
    if [ "$final_confirm" != "YES" ]; then
        echo "Migration cancelled."
        exit 0
    fi

    print_step "Creating backup..."
    backup_data

    create_tenant_id_blocks
    create_sequences
    drop_foreign_keys
    migrate_tenants_table
    alter_related_tables
    restore_foreign_keys
    create_functions_and_triggers
    create_monitoring_view
    set_sequence_values
    verify_migration
    cleanup

    print_header "Migration Completed Successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Update your application config to include 'environment' (e.g. \"environment\": \"private-prod\")"
    echo "2. Test tenant creation with your dbtool or API"
    echo "3. Run 'SELECT * FROM tenant_allocation_status;' to monitor usage"
    echo ""
    echo "Backups saved to: $BACKUP_DIR"
}

main "$@"

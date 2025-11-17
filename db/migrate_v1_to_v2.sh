#!/bin/bash
#
# migrate_v1_to_v2.sh
# Migration script from Schema V1 to V2 (Simplified - single environment field)
#
# Copyright (c) 2025 SecurITe
# File Owner: deepinder@securite.world
#

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PGHOST="${DB_HOST:-postgres}"
PGPORT="${DB_PORT:-5432}"
PGUSER="${DB_USER:-pguser}"
PGPASSWORD="${DB_PASSWORD:-pgpass}"
PGDB="${DB_NAME:-pgdb}"
BACKUP_DIR="${BACKUP_DIR:-/tmp/db_migration_backup}"

export PGPASSWORD

# Functions
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

# Main execution
main() {
    print_header "Database Migration: Schema V1 → V2 (Simplified)"
    echo "This script will migrate your database to support multi-environment tenant management"
    echo ""
    echo "Database: $PGDB@$PGHOST:$PGPORT"
    echo "Backup directory: $BACKUP_DIR"
    echo ""
    echo "SIMPLIFIED SCHEMA:"
    echo "  - Single 'environment' field (no tenant_type)"
    echo "  - Environments: private-staging, private-prod, aws-prod, gcloud-prod, azure-prod"
    echo ""
    
    read -p "Do you want to proceed? (yes/no): " confirm
    if [ "$confirm" != "yes" ]; then
        echo "Migration cancelled."
        exit 0
    fi
    
    echo ""
    check_prerequisites
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
    
    echo ""
    print_header "Migration Completed Successfully!"
    echo ""
    echo "Next steps:"
    echo "1. Update your db.json config file with 'environment' field"
    echo "   Example: {\"environment\": \"private-prod\"}"
    echo "2. Test tenant creation with: dbtool -db /opt/config/db.json -op insert-tenant -tenant-name 'test-tenant'"
    echo "3. Verify with: dbtool -db /opt/config/db.json -op list-tenants"
    echo ""
    echo "Backups saved to: $BACKUP_DIR"
}

# Run main function
main

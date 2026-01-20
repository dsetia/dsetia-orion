#!/bin/bash

# ===================================================================
# move-tenant.sh - Move a tenant to a different environment ID block
# ===================================================================
# Usage: move-tenant.sh --db <db-config-path> --tenant-id <id> --environment <env> [--dry-run]
# ===================================================================

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
DB_CONFIG=""
TENANT_ID=""
TARGET_ENV=""
DRY_RUN=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --db)
            DB_CONFIG="$2"
            shift 2
            ;;
        --tenant-id)
            TENANT_ID="$2"
            shift 2
            ;;
        --environment)
            TARGET_ENV="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Validate required parameters
if [[ -z "$DB_CONFIG" || -z "$TENANT_ID" || -z "$TARGET_ENV" ]]; then
    echo -e "${RED}Error: Missing required parameters${NC}"
    echo ""
    echo "Usage: $0 --db <db-config-path> --tenant-id <id> --environment <env> [--dry-run]"
    echo ""
    echo "Options:"
    echo "  --db <path>           Path to database config JSON file"
    echo "  --tenant-id <id>      ID of tenant to move"
    echo "  --environment <env>   Target environment (e.g., private-prod, aws-prod)"
    echo "  --dry-run             Show what would be done without making changes"
    echo ""
    echo "Example:"
    echo "  $0 --db /opt/config/db.json --tenant-id 7 --environment private-prod"
    exit 1
fi

# Validate DB config file exists
if [[ ! -f "$DB_CONFIG" ]]; then
    echo -e "${RED}Error: Database config file not found: $DB_CONFIG${NC}"
    exit 1
fi

# Parse database configuration
parse_db_config() {
    local config_file="$1"
    
    # Try jq first (more reliable)
    if command -v jq &> /dev/null; then
        DB_HOST=$(jq -r '.host // "localhost"' "$config_file")
        DB_PORT=$(jq -r '.port // 5432' "$config_file")
        DB_USER=$(jq -r '.user // "postgres"' "$config_file")
        DB_PASS=$(jq -r '.password // ""' "$config_file")
        DB_NAME=$(jq -r '.dbname // "postgres"' "$config_file")
    else
        # Fallback to grep/sed
        DB_HOST=$(grep -o '"host"[[:space:]]*:[[:space:]]*"[^"]*"' "$config_file" | sed 's/.*"\([^"]*\)".*/\1/' || echo "localhost")
        DB_PORT=$(grep -o '"port"[[:space:]]*:[[:space:]]*[0-9]*' "$config_file" | sed 's/.*:[[:space:]]*\([0-9]*\).*/\1/' || echo "5432")
        DB_USER=$(grep -o '"user"[[:space:]]*:[[:space:]]*"[^"]*"' "$config_file" | sed 's/.*"\([^"]*\)".*/\1/' || echo "postgres")
        DB_PASS=$(grep -o '"password"[[:space:]]*:[[:space:]]*"[^"]*"' "$config_file" | sed 's/.*"\([^"]*\)".*/\1/' || echo "")
        DB_NAME=$(grep -o '"dbname"[[:space:]]*:[[:space:]]*"[^"]*"' "$config_file" | sed 's/.*"\([^"]*\)".*/\1/' || echo "postgres")
    fi
    
    export PGHOST="$DB_HOST"
    export PGPORT="$DB_PORT"
    export PGUSER="$DB_USER"
    export PGPASSWORD="$DB_PASS"
    export PGDATABASE="$DB_NAME"
}

# Execute SQL query
psql_exec() {
    psql -t -A "$@"
}

# Execute SQL and return result
psql_query() {
    psql -t -A -c "$1"
}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Tenant Environment Migration${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Parse database config
parse_db_config "$DB_CONFIG"

echo -e "${YELLOW}Database Connection:${NC}"
echo "  Host: $PGHOST:$PGPORT"
echo "  Database: $PGDATABASE"
echo "  User: $PGUSER"
echo ""

# Verify database connection
if ! psql -c "SELECT 1" &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to database${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Database connection successful${NC}"
echo ""

# Get current tenant information
echo -e "${YELLOW}Fetching current tenant information...${NC}"

TENANT_INFO=$(psql_query "SELECT tenant_id, tenant_name, environment FROM tenants WHERE tenant_id = $TENANT_ID")

if [[ -z "$TENANT_INFO" ]]; then
    echo -e "${RED}Error: Tenant ID $TENANT_ID not found${NC}"
    exit 1
fi

CURRENT_NAME=$(echo "$TENANT_INFO" | cut -d'|' -f2)
CURRENT_ENV=$(echo "$TENANT_INFO" | cut -d'|' -f3)

echo -e "  Current Tenant ID: ${BLUE}$TENANT_ID${NC}"
echo -e "  Tenant Name: ${BLUE}$CURRENT_NAME${NC}"
echo -e "  Current Environment: ${BLUE}$CURRENT_ENV${NC}"
echo -e "  Target Environment: ${GREEN}$TARGET_ENV${NC}"
echo ""

# Check if already in target environment
if [[ "$CURRENT_ENV" == "$TARGET_ENV" ]]; then
    echo -e "${YELLOW}Warning: Tenant is already in environment '$TARGET_ENV'${NC}"
    exit 0
fi

# Validate target environment exists
TARGET_ENV_INFO=$(psql_query "SELECT start_id, end_id FROM tenant_id_blocks WHERE environment = '$TARGET_ENV'")

if [[ -z "$TARGET_ENV_INFO" ]]; then
    echo -e "${RED}Error: Target environment '$TARGET_ENV' not found in tenant_id_blocks${NC}"
    exit 1
fi

TARGET_START=$(echo "$TARGET_ENV_INFO" | cut -d'|' -f1)
TARGET_END=$(echo "$TARGET_ENV_INFO" | cut -d'|' -f2)

echo -e "${YELLOW}Target Environment Details:${NC}"
echo -e "  ID Range: ${BLUE}$TARGET_START - $TARGET_END${NC}"
echo ""

# Count related records
echo -e "${YELLOW}Counting related records...${NC}"

DEVICE_COUNT=$(psql_query "SELECT COUNT(*) FROM devices WHERE tenant_id = $TENANT_ID")
APIKEY_COUNT=$(psql_query "SELECT COUNT(*) FROM api_keys WHERE tenant_id = $TENANT_ID")
RULES_COUNT=$(psql_query "SELECT COUNT(*) FROM hndr_rules WHERE tenant_id = $TENANT_ID")
STATUS_COUNT=$(psql_query "SELECT COUNT(*) FROM status WHERE tenant_id = $TENANT_ID")
VERSION_COUNT=$(psql_query "SELECT COUNT(*) FROM version WHERE tenant_id = $TENANT_ID")

echo "  Devices: $DEVICE_COUNT"
echo "  API Keys: $APIKEY_COUNT"
echo "  Rules: $RULES_COUNT"
echo "  Status Records: $STATUS_COUNT"
echo "  Version Records: $VERSION_COUNT"
echo ""

# Get new tenant ID from target environment
echo -e "${YELLOW}Allocating new tenant ID from target environment...${NC}"

NEW_TENANT_ID=$(psql_query "SELECT get_next_tenant_id('$TARGET_ENV')")

if [[ -z "$NEW_TENANT_ID" ]]; then
    echo -e "${RED}Error: Failed to allocate new tenant ID${NC}"
    exit 1
fi

echo -e "  New Tenant ID: ${GREEN}$NEW_TENANT_ID${NC}"
echo ""

# Verify new ID is in correct range
if [[ $NEW_TENANT_ID -lt $TARGET_START || $NEW_TENANT_ID -gt $TARGET_END ]]; then
    echo -e "${RED}Error: New tenant ID $NEW_TENANT_ID is outside valid range [$TARGET_START-$TARGET_END]${NC}"
    exit 1
fi

# Show migration plan
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Migration Plan${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${YELLOW}Changes to be made:${NC}"
echo -e "  0. Temporarily rename old tenant to avoid name conflict"
echo -e "  1. Create new tenant record (ID: ${GREEN}$NEW_TENANT_ID${NC}, Env: ${GREEN}$TARGET_ENV${NC})"
echo -e "  2. Update ${BLUE}$DEVICE_COUNT${NC} device(s)"
echo -e "  3. Update ${BLUE}$APIKEY_COUNT${NC} API key(s)"
echo -e "  4. Update ${BLUE}$RULES_COUNT${NC} rule(s)"
echo -e "  5. Update ${BLUE}$STATUS_COUNT${NC} status record(s)"
echo -e "  6. Update ${BLUE}$VERSION_COUNT${NC} version record(s)"
echo -e "  7. Delete old tenant record (ID: ${RED}$TENANT_ID${NC})"
echo ""

if [[ "$DRY_RUN" == true ]]; then
    echo -e "${YELLOW}DRY RUN MODE - No changes will be made${NC}"
    echo ""
    echo -e "${GREEN}Migration plan validated successfully${NC}"
    exit 0
fi

# Confirm before proceeding
echo -e "${YELLOW}Warning: This operation will change the tenant ID from $TENANT_ID to $NEW_TENANT_ID${NC}"
echo -e "${YELLOW}All related records will be updated to reference the new tenant ID.${NC}"
echo ""
read -p "Do you want to proceed? (yes/no): " CONFIRM

if [[ "$CONFIRM" != "yes" ]]; then
    echo -e "${YELLOW}Migration cancelled${NC}"
    exit 0
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Executing Migration${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Execute migration in a transaction
MIGRATION_SQL=$(cat <<EOF
BEGIN;

-- Step 0: Temporarily rename old tenant to avoid UNIQUE constraint violation
UPDATE tenants
SET tenant_name = tenant_name || '-migrating-$TENANT_ID-to-$NEW_TENANT_ID'
WHERE tenant_id = $TENANT_ID;

-- Step 1: Create new tenant record
INSERT INTO tenants (tenant_id, tenant_name, environment, created_at, updated_at)
SELECT $NEW_TENANT_ID,
       REPLACE(tenant_name, '-migrating-$TENANT_ID-to-$NEW_TENANT_ID', ''),
       '$TARGET_ENV',
       created_at,
       updated_at
FROM tenants
WHERE tenant_id = $TENANT_ID;

-- Step 2: Update devices
UPDATE devices 
SET tenant_id = $NEW_TENANT_ID 
WHERE tenant_id = $TENANT_ID;

-- Step 3: Update api_keys
UPDATE api_keys 
SET tenant_id = $NEW_TENANT_ID 
WHERE tenant_id = $TENANT_ID;

-- Step 4: Update hndr_rules
UPDATE hndr_rules 
SET tenant_id = $NEW_TENANT_ID 
WHERE tenant_id = $TENANT_ID;

-- Step 5: Update status
UPDATE status 
SET tenant_id = $NEW_TENANT_ID 
WHERE tenant_id = $TENANT_ID;

-- Step 6: Update version
UPDATE version 
SET tenant_id = $NEW_TENANT_ID 
WHERE tenant_id = $TENANT_ID;

-- Step 7: Delete old tenant record (with temporary name)
DELETE FROM tenants WHERE tenant_id = $TENANT_ID;

COMMIT;
EOF
)

echo -e "${YELLOW}Executing migration transaction...${NC}"

if psql -c "$MIGRATION_SQL"; then
    echo ""
    echo -e "${GREEN}✓ Migration completed successfully${NC}"
    echo ""
    
    # Verify migration
    echo -e "${YELLOW}Verifying migration...${NC}"
    
    NEW_TENANT_INFO=$(psql_query "SELECT tenant_id, tenant_name, environment FROM tenants WHERE tenant_id = $NEW_TENANT_ID")
    
    if [[ -n "$NEW_TENANT_INFO" ]]; then
        echo -e "${GREEN}✓ New tenant record verified${NC}"
        
        NEW_DEVICE_COUNT=$(psql_query "SELECT COUNT(*) FROM devices WHERE tenant_id = $NEW_TENANT_ID")
        NEW_APIKEY_COUNT=$(psql_query "SELECT COUNT(*) FROM api_keys WHERE tenant_id = $NEW_TENANT_ID")
        NEW_RULES_COUNT=$(psql_query "SELECT COUNT(*) FROM hndr_rules WHERE tenant_id = $NEW_TENANT_ID")
        NEW_STATUS_COUNT=$(psql_query "SELECT COUNT(*) FROM status WHERE tenant_id = $NEW_TENANT_ID")
        NEW_VERSION_COUNT=$(psql_query "SELECT COUNT(*) FROM version WHERE tenant_id = $NEW_TENANT_ID")
        
        echo ""
        echo -e "${YELLOW}Post-migration record counts:${NC}"
        echo "  Devices: $NEW_DEVICE_COUNT (expected: $DEVICE_COUNT)"
        echo "  API Keys: $NEW_APIKEY_COUNT (expected: $APIKEY_COUNT)"
        echo "  Rules: $NEW_RULES_COUNT (expected: $RULES_COUNT)"
        echo "  Status Records: $NEW_STATUS_COUNT (expected: $STATUS_COUNT)"
        echo "  Version Records: $NEW_VERSION_COUNT (expected: $VERSION_COUNT)"
        echo ""
        
        # Verify counts match
        if [[ "$NEW_DEVICE_COUNT" == "$DEVICE_COUNT" ]] && \
           [[ "$NEW_APIKEY_COUNT" == "$APIKEY_COUNT" ]] && \
           [[ "$NEW_RULES_COUNT" == "$RULES_COUNT" ]] && \
           [[ "$NEW_STATUS_COUNT" == "$STATUS_COUNT" ]] && \
           [[ "$NEW_VERSION_COUNT" == "$VERSION_COUNT" ]]; then
            echo -e "${GREEN}✓ All record counts verified${NC}"
        else
            echo -e "${RED}⚠ Warning: Record count mismatch detected${NC}"
        fi
    else
        echo -e "${RED}⚠ Warning: Could not verify new tenant record${NC}"
    fi
    
    # Verify old tenant is gone
    OLD_TENANT_CHECK=$(psql_query "SELECT COUNT(*) FROM tenants WHERE tenant_id = $TENANT_ID")
    if [[ "$OLD_TENANT_CHECK" == "0" ]]; then
        echo -e "${GREEN}✓ Old tenant record removed${NC}"
    else
        echo -e "${RED}⚠ Warning: Old tenant record still exists${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${GREEN}Migration Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo -e "  Tenant Name: ${BLUE}$CURRENT_NAME${NC}"
    echo -e "  Old Tenant ID: ${RED}$TENANT_ID${NC} (${RED}$CURRENT_ENV${NC})"
    echo -e "  New Tenant ID: ${GREEN}$NEW_TENANT_ID${NC} (${GREEN}$TARGET_ENV${NC})"
    echo ""
    echo -e "${GREEN}✓ Tenant successfully moved to $TARGET_ENV environment${NC}"
    
else
    echo ""
    echo -e "${RED}✗ Migration failed${NC}"
    echo -e "${YELLOW}Transaction was rolled back - no changes were made${NC}"
    exit 1
fi

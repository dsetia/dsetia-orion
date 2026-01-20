#!/bin/bash

# ===================================================================
# tenant-info.sh - Display comprehensive tenant information
# ===================================================================
# Usage: tenant-info.sh --db <db-config-path> --tenant-id <id>
# ===================================================================

set -euo pipefail

# Color codes
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

DB_CONFIG=""
TENANT_ID=""

# Parse arguments
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
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ -z "$DB_CONFIG" || -z "$TENANT_ID" ]]; then
    echo "Usage: $0 --db <db-config-path> --tenant-id <id>"
    exit 1
fi

# Parse database configuration
if command -v jq &> /dev/null; then
    DB_HOST=$(jq -r '.host // "localhost"' "$DB_CONFIG")
    DB_PORT=$(jq -r '.port // 5432' "$DB_CONFIG")
    DB_USER=$(jq -r '.user // "postgres"' "$DB_CONFIG")
    DB_PASS=$(jq -r '.password // ""' "$DB_CONFIG")
    DB_NAME=$(jq -r '.database // "postgres"' "$DB_CONFIG")
else
    DB_HOST=$(grep -o '"host"[[:space:]]*:[[:space:]]*"[^"]*"' "$DB_CONFIG" | sed 's/.*"\([^"]*\)".*/\1/' || echo "localhost")
    DB_PORT=$(grep -o '"port"[[:space:]]*:[[:space:]]*[0-9]*' "$DB_CONFIG" | sed 's/.*:[[:space:]]*\([0-9]*\).*/\1/' || echo "5432")
    DB_USER=$(grep -o '"user"[[:space:]]*:[[:space:]]*"[^"]*"' "$DB_CONFIG" | sed 's/.*"\([^"]*\)".*/\1/' || echo "postgres")
    DB_PASS=$(grep -o '"password"[[:space:]]*:[[:space:]]*"[^"]*"' "$DB_CONFIG" | sed 's/.*"\([^"]*\)".*/\1/' || echo "")
    DB_NAME=$(grep -o '"database"[[:space:]]*:[[:space:]]*"[^"]*"' "$DB_CONFIG" | sed 's/.*"\([^"]*\)".*/\1/' || echo "postgres")
fi

export PGHOST="$DB_HOST"
export PGPORT="$DB_PORT"
export PGUSER="$DB_USER"
export PGPASSWORD="$DB_PASS"
export PGDATABASE="$DB_NAME"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Tenant Information Report${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get tenant info
TENANT_INFO=$(psql -t -A -c "SELECT tenant_id, tenant_name, environment, created_at, updated_at FROM tenants WHERE tenant_id = $TENANT_ID")

if [[ -z "$TENANT_INFO" ]]; then
    echo "Error: Tenant ID $TENANT_ID not found"
    exit 1
fi

TENANT_NAME=$(echo "$TENANT_INFO" | cut -d'|' -f2)
ENVIRONMENT=$(echo "$TENANT_INFO" | cut -d'|' -f3)
CREATED_AT=$(echo "$TENANT_INFO" | cut -d'|' -f4)
UPDATED_AT=$(echo "$TENANT_INFO" | cut -d'|' -f5)

echo -e "${YELLOW}Tenant Details:${NC}"
echo "  Tenant ID: $TENANT_ID"
echo "  Name: $TENANT_NAME"
echo "  Environment: $ENVIRONMENT"
echo "  Created: $CREATED_AT"
echo "  Updated: $UPDATED_AT"
echo ""

# Get environment block info
ENV_INFO=$(psql -t -A -c "SELECT start_id, end_id, description FROM tenant_id_blocks WHERE environment = '$ENVIRONMENT'")
START_ID=$(echo "$ENV_INFO" | cut -d'|' -f1)
END_ID=$(echo "$ENV_INFO" | cut -d'|' -f2)
DESCRIPTION=$(echo "$ENV_INFO" | cut -d'|' -f3)

echo -e "${YELLOW}Environment Block:${NC}"
echo "  Range: $START_ID - $END_ID"
echo "  Description: $DESCRIPTION"
echo ""

# Count related records
echo -e "${YELLOW}Related Records:${NC}"
psql -c "
SELECT 
    'Devices' as table_name,
    COUNT(*) as count
FROM devices WHERE tenant_id = $TENANT_ID
UNION ALL
SELECT 'API Keys', COUNT(*) FROM api_keys WHERE tenant_id = $TENANT_ID
UNION ALL
SELECT 'Rules', COUNT(*) FROM hndr_rules WHERE tenant_id = $TENANT_ID
UNION ALL
SELECT 'Status', COUNT(*) FROM status WHERE tenant_id = $TENANT_ID
UNION ALL
SELECT 'Version', COUNT(*) FROM version WHERE tenant_id = $TENANT_ID;
"

echo ""
echo -e "${YELLOW}Devices:${NC}"
psql -c "SELECT device_id, device_name, hndr_sw_version FROM devices WHERE tenant_id = $TENANT_ID;"

echo ""
echo -e "${YELLOW}API Keys:${NC}"
psql -c "SELECT api_key, device_id, is_active FROM api_keys WHERE tenant_id = $TENANT_ID;"

echo ""
echo -e "${YELLOW}Available Target Environments:${NC}"
psql -c "
SELECT 
    environment,
    start_id,
    end_id,
    end_id - start_id + 1 as capacity,
    (SELECT COUNT(*) FROM tenants WHERE tenants.environment = tenant_id_blocks.environment) as used,
    end_id - start_id + 1 - (SELECT COUNT(*) FROM tenants WHERE tenants.environment = tenant_id_blocks.environment) as available
FROM tenant_id_blocks
WHERE environment != '$ENVIRONMENT'
ORDER BY start_id;
"

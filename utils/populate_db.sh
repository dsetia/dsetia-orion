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

# Configuration
CONFIG_DIR="config"
TENANT_ID=1
TENANT_NAME="tenant1"
VALID_API_KEY="key1"
INVALID_API_KEY="invalid-key"
DEVICE_ID="dev1"
DEVICE_NAME="Device 1"

# Print usage/help
usage() {
    echo "Usage: $0 [db-config-path]"
    echo "  Populate the database"
    echo "  db-config-path: Path to DB config JSON (default: $CONFIG_DIR/apis_config.json)"
    exit 1
}

[[ $# -lt 1 ]] && usage

# config file
dbpath=${1:-"$CONFIG/apis_config.json"}

# remember to init the db first 
# - orion/db/init_db.sh - sqliet3 
# - orion/db/init_pg.sh - postgres

echo "DB path is $dbpath"

dbtool -db $dbpath -op insert-tenant -tenant-name $TENANT_NAME
dbtool -db $dbpath -op insert-device -tenant-id $TENANT_ID -device-id $DEVICE_ID -device-name $DEVICE_NAME -hndr-sw-version $DEVICE_VERSION
dbtool -db $dbpath -op insert-api-key -tenant-id $TENANT_ID -device-id $DEVICE_ID -api-key $VALID_API_KEY

dbtool -db $dbpath -op list-tenants
dbtool -db $dbpath -op list-devices
dbtool -db $dbpath -op list-api-keys
dbtool -db $dbpath -op list-hndr-sw
dbtool -db $dbpath -op list-hndr-rules
dbtool -db $dbpath -op list-threat-intel

#!/bin/bash

# config file
dbpath=${1:-"config/apis_config.json"}

# Configuration
TENANT_ID=1
TENANT_NAME="tenant1"
VALID_API_KEY="key1"
INVALID_API_KEY="invalid-key"
DEVICE_ID="dev1"
DEVICE_NAME="Device 1"
DEVICE_IMAGE_VERSION="v1.2.2"
GLOBAL_IMAGE_VERSION="v1.2.3"
TENANT_RULES_VERSION="r1.2.3"
GLOBAL_THREAT_VERSION="2025.04.10.1523"

# remember to init the db first 
# - orion/db/init_db.sh - sqliet3 
# - orion/db/init_pg.sh - postgres

echo "DB path is $dbpath"

dbtool -db $dbpath -op insert-tenant -tenant-name $TENANT_NAME
dbtool -db $dbpath -op insert-device -tenant-id $TENANT_ID -device-id $DEVICE_ID -device-name $DEVICE_NAME -hndr-sw-version $DEVICE_VERSION
dbtool -db $dbpath -op insert-api-key -tenant-id $TENANT_ID -device-id $DEVICE_ID -api-key $VALID_API_KEY
dbtool -db $dbpath -op insert-hndr-sw -sw-version $GLOBAL_IMAGE_VERSION -sw-size 1024 -sw-sha256 sw-sha256
dbtool -db $dbpath -op insert-hndr-rules -tenant-id $TENANT_ID -rules-version $TENANT_RULES_VERSION -rules-size 512 -rules-sha256 rules-sha256
dbtool -db $dbpath -op insert-threat-intel -ti-version $GLOBAL_THREAT_VERSION -ti-size 256 -ti-sha256 ti-sha256

dbtool -db $dbpath -op list-tenants
dbtool -db $dbpath -op list-devices
dbtool -db $dbpath -op list-api-keys
dbtool -db $dbpath -op list-hndr-sw
dbtool -db $dbpath -op list-hndr-rules
dbtool -db $dbpath -op list-threat-intel

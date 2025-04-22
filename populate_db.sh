#!/bin/bash

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

# config file
CONFIG_FILE="config/apis_config.json"
SCHEMA_FILE="db/schema_pg.sql"

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

# Read config values into variables
host=$(jq -r '.host' "$CONFIG_FILE")
port=$(jq -r '.port' "$CONFIG_FILE")
user=$(jq -r '.user' "$CONFIG_FILE")
password=$(jq -r '.password' "$CONFIG_FILE")
dbname=$(jq -r '.dbname' "$CONFIG_FILE")
sslmode=$(jq -r '.sslmode' "$CONFIG_FILE")
dbpath="postgres://$user:$password@$host:$port/$dbname?sslmode=$sslmode"
"postgres://pguser:pgpass@localhost:5432/pgdb?sslmode=disable"

# Print config and ask for confirmation
echo "✅ Loaded configuration:"
echo "  Host     : $host"
echo "  Port     : $port"
echo "  User     : $user"
echo "  Password : $password"
echo "  DB Name  : $dbname"
echo "  SSL Mode : $sslmode"
echo "  DB Path  : $dbpath"
echo

read -p "❓ Proceed with this configuration? [y/N] " confirm
confirm=${confirm,,}  # to lowercase

if [[ "$confirm" != "y" && "$confirm" != "yes" ]]; then
    echo "❌ Aborting."
    exit 1
fi

export PGPASSWORD=$password

cd db
./dbtool -db $dbpath -op insert-tenant -tenant-name $TENANT_NAME
./dbtool -db $dbpath -op insert-device -tenant-id $TENANT_ID -device-id $DEVICE_ID -device-name $DEVICE_NAME -hndr-sw-version $DEVICE_VERSION
./dbtool -db $dbpath -op insert-api-key -tenant-id $TENANT_ID -device-id $DEVICE_ID -api-key $VALID_API_KEY
./dbtool -db $dbpath -op insert-hndr-sw -sw-version $GLOBAL_IMAGE_VERSION -sw-size 1024 -sw-sha256 sw-sha256
./dbtool -db $dbpath -op insert-hndr-rules -tenant-id $TENANT_ID -rules-version $TENANT_RULES_VERSION -rules-size 512 -rules-sha256 rules-sha256
./dbtool -db $dbpath -op insert-threat-intel -ti-version $GLOBAL_THREAT_VERSION -ti-size 256 -ti-sha256 ti-sha256

./dbtool -db $dbpath -op list-tenants
./dbtool -db $dbpath -op list-devices
./dbtool -db $dbpath -op list-api-keys
./dbtool -db $dbpath -op list-hndr-sw
./dbtool -db $dbpath -op list-hndr-rules
./dbtool -db $dbpath -op list-threat-intel

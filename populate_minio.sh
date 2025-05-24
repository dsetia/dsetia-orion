#!/bin/bash

CONFIG_DIR="config"
DB_CONFIG_FILE=${1:-"$CONFIG_DIR/apis_config.json"}
MINIO_CONFIG_FILE=${2:-"$CONFIG_DIR/minio_config.json"}

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

adminuser=$(jq -r '.user' "$MINIO_CONFIG_FILE")
adminpass=$(jq -r '.password' "$MINIO_CONFIG_FILE")
endpoint=$(jq -r '.endpoint' "$MINIO_CONFIG_FILE")

# Print config and ask for confirmation
echo "✅ Loaded configuration:"
echo "  User     : $adminuser"
echo "  Password : $adminpass"
echo "  Endpoint : $endpoint"

read -p "❓ Proceed with this configuration? [y/N] " confirm
confirm=${confirm,,}  # to lowercase

if [[ "$confirm" != "y" && "$confirm" != "yes" ]]; then
    echo "❌ Aborting."
    exit 1
fi

mc alias set myminio http://$endpoint $adminuser $adminpass

objupdater -type software -dbconfig $DB_CONFIG_FILE -minioconfig $MINIO_CONFIG_FILE -file minio/hndr-sw-v1.2.3.tar.gz 
objupdater -type rules -dbconfig $DB_CONFIG_FILE -minioconfig $MINIO_CONFIG_FILE -file minio/hndr-rules-r1.2.3.tar.gz -tenantid 1
objupdater -type threatintel -dbconfig $DB_CONFIG_FILE -minioconfig $MINIO_CONFIG_FILE -file minio/threatintel-2025.04.10.1523.tar.gz

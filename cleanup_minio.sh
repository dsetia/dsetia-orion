#!/bin/bash

CONFIG_FILE="config/minio_config.json"

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

adminuser=$(jq -r '.minio_root_user' "$CONFIG_FILE")
adminpass=$(jq -r '.minio_root_password' "$CONFIG_FILE")

# Print config and ask for confirmation
echo "✅ Loaded configuration:"
echo "  User     : $adminuser"
echo "  Password : $adminpass"

read -p "❓ Proceed with this configuration? [y/N] " confirm
confirm=${confirm,,}  # to lowercase

if [[ "$confirm" != "y" && "$confirm" != "yes" ]]; then
    echo "❌ Aborting."
    exit 1
fi

mc alias set myminio http://localhost:9000 $adminuser $adminpass

# delete bucket and objects
mc rb --force myminio/software
mc rb --force myminio/rules
mc rb --force myminio/threatintel


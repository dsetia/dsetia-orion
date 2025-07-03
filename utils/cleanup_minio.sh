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

CONFIG_DIR="config"
MINIO_CONFIG_FILE=${1:-"$CONFIG_DIR/minio.json"}

# Print usage/help
usage() {
    echo "Usage: $0 [minio-config-path]"
    echo "  Cleanup minio store"
    echo "  minio-config-path: Path to Minio config (default: $CONFIG_DIR/minio.json)"
    exit 1
}

[[ $# -lt 1 ]] && usage

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

# delete bucket and objects
mc rb --force myminio/software
mc rb --force myminio/rules
mc rb --force myminio/threatintel
mc rb --force myminio/config
mc rb --force myminio/provisioner
mc rb --force myminio/sensor


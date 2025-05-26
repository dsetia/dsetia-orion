#!/bin/bash

CONFIG_DIR="config"
CONFIG_FILE=${1:-"$CONFIG_DIR/minio_config.json"}

# Print usage/help
usage() {
    echo "Usage: $0 [minio-config-path]"
    echo "  Initialize minio store"
    echo "  minio-config-path: Path to Minio config (default: $CONFIG_DIR/minio_config.sql)"
    exit 1
}

[[ $# -lt 1 ]] && usage

if ! command -v jq &>/dev/null; then
    echo "❌ 'jq' is required but not installed. Please run: sudo apt-get install jq"
    exit 1
fi

adminuser=$(jq -r '.user' "$CONFIG_FILE")
adminpass=$(jq -r '.password' "$CONFIG_FILE")
endpoint=$(jq -r '.endpoint' "$CONFIG_FILE")

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

# seed minio
mc mb myminio/software
mc mb myminio/rules
mc mb myminio/threatintel
mc mb myminio/config
mc mb myminio/provisioner
mc mb myminio/sensor

# Allow anonymous access for nginx
# This is equivamt to a policy like:
#{
#  "Version": "2012-10-17",
#  "Statement": [
#    {
#      "Effect": "Allow",
#      "Principal": "*",
#      "Action": ["s3:GetObject"],
#      "Resource": ["arn:aws:s3:::software/*"]
#    }
#  ]
#}
mc anonymous set download myminio/software
mc anonymous set download myminio/rules
mc anonymous set download myminio/threatintel
mc anonymous set download myminio/config
mc anonymous set download myminio/provisioner
mc anonymous set download myminio/sensor

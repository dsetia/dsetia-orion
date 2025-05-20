#!/bin/bash

CONFIG_FILE="config/minio_config.json"

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

#mc cp hndr-sw-v1.2.3.tar.gz myminio/software/
#mc cp hndr-rules-r1.2.3.tar.gz myminio/rules/1/hndr-rules-r1.2.3.tar.gz
#mc cp threatintel-2025.04.10.1523.tar.gz myminio/threatintel/
objupdater -type software -dbconfig config/db_dev_config.json -minioconfig config/minio_config.json -file minio/hndr-sw-v1.2.3.tar.gz 
objupdater -type rules -dbconfig config/db_dev_config.json -minioconfig config/minio_config.json -file minio/hndr-rules-r1.2.3.tar.gz -tenantid 1
objupdater -type threatintel -dbconfig config/db_dev_config.json -minioconfig config/minio_config.json -file minio/threatintel-2025.04.10.1523.tar.gz

# Add API user
mc admin user add myminio apiuser apiuserpassword
mc alias set local http://localhost:9000 apiuser apiuserpassword

# Create policy
echo '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::software/*", "arn:aws:s3:::rules/*", "arn:aws:s3:::threatintel/*"]
    }
  ]
}' > apiuser_policy.json

# Apply policy
mc admin policy create myminio apiuser-policy apiuser_policy.json
mc admin policy attach myminio apiuser-policy --user apiuser

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

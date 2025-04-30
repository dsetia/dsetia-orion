#!/bin/bash

mc alias set myminio http://localhost:9000 minioadmin minioadmin

# seed minio
mc mb myminio/software
mc mb myminio/rules
mc mb myminio/threatintel

cd ./minio
mc cp hndr-sw-v1.2.3.tar.gz myminio/software/
mc cp hndr-rules-r1.2.3.tar.gz myminio/rules/1
mc cp threatintel-2025.04.10.1523.tar.gz myminio/threatintel/

# Add API user
mc admin user add myminio apiuser apipass
mc alias set local http://localhost:9000 apiuser apipass

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

mc policy set download local/software
mc policy set download local/rules
mc policy set download local/threatintel

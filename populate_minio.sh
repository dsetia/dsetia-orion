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
mc admin user add myminio apiuser apiuserpassword

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

# Allow anonymous access for authenticated requests
# allows s3:GetObject only for requests with a Referer header from the API server,

echo '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::software/*", "arn:aws:s3:::rules/*", "arn:aws:s3:::threatintel/*"],
      "Condition": {
        "StringEquals": {
          "aws:Referer": "http://apis-container:8080"
        }
      }
    }
  ]
}' > bucket_policy.json
mc anonymous set-json bucket_policy.json myminio/software
mc anonymous set-json bucket_policy.json myminio/rules
mc anonymous set-json bucket_policy.json myminio/threatintel

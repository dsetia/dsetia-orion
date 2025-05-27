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
#

CONFIG_DIR="config"
DBPATH=${1:-"$CONFIG_DIR/db_dev_config.json"}
MINIO_PATH=${1:-"$CONFIG_DIR/minio_config.json"}

# Configuration
COMPOSE_FILE="docker-compose.yml"
NETWORK_NAME="securite-net"
NGINX_PORT=80
NGINX_SSL_PORT=443
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
API_PORT=8080
INVALID_API_KEY="invalid-key"
TEST_FILE_IMAGE="software/hndr-sw-v1.2.3.tar.gz"
TEST_FILE_THREATINTEL="threatintel/threatintel-2025.04.10.1523.tar.gz"

MINIO_CONFIG_FILE="config/minio_config.json"
minioadminuser=$(jq -r '.user' "$MINIO_CONFIG_FILE")
minioadminpass=$(jq -r '.password' "$MINIO_CONFIG_FILE")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}SUCCESS: $2${NC}"
    else
        echo -e "${RED}FAILURE: $2${NC}"
        exit 1
    fi
}

# Check if docker-compose is running
echo "Checking docker-compose status..."
sudo docker-compose -f "$COMPOSE_FILE" ps | grep "Up" >/dev/null
print_status $? "Docker-compose services are running"

# 3. Verify containers are running
echo "Checking container status..."
sudo docker ps --filter "name=nginx" --filter "status=running" -q | grep . >/dev/null
print_status $? "Nginx container is running"
sudo docker ps --filter "name=minio" --filter "status=running" -q | grep . >/dev/null
print_status $? "MinIO container is running"
sudo docker ps --filter "name=apis-container" --filter "status=running" -q | grep . >/dev/null
print_status $? "API server container is running"
sudo docker ps --filter "name=postgres" --filter "status=running" -q | grep . >/dev/null
print_status $? "Postgres container is running"

# 4. Test MinIO health check
echo "Testing MinIO health endpoint..."
curl -s -o /dev/null -w "%{http_code}" "http://localhost:$MINIO_API_PORT/minio/health/live" | grep -q 200
print_status $? "MinIO health check passed"
echo "Verifying MinIO files..."
sudo sudo docker exec minio sh -c "mc alias set myminio http://localhost:9000 $minioadminuser $minioadminpass && mc ls myminio/$TEST_FILE_IMAGE"
print_status $? "MinIO image file exists"

# Test API server healthcheck (direct)
echo "Testing API server health endpoint (direct)..."
curl -s -o /dev/null -w "%{http_code}" "http://localhost:$API_PORT/v1/healthcheck" | grep -q 200
print_status $? "API server health check (direct) passed"

# Test API server healthcheck (nginx)
echo "Testing API server health endpoint (nginx)..."
curl -k -s -o /dev/null -w "%{http_code}" "https://localhost:$NGINX_SSL_PORT/v1/healthcheck" | grep -q 200
print_status $? "API server health check (nginx) passed"

# populate DB
TENANT_NAME="tenant-$$"
DEVICE_NAME="device-name-$$"
VALID_DEVICE_ID="device-id-$$"
VALID_API_KEY="api-key-$$"
DEVICE_VERSION="v1.2.3"
OUTPUT=$(dbtool -db $DBPATH -op insert-tenant -tenant-name $TENANT_NAME)
echo "$OUTPUT"
TENANT_ID=$(echo "$OUTPUT" | grep -oE 'ID=[0-9]+' | cut -d= -f2)
dbtool -db $DBPATH -op insert-device -tenant-id $TENANT_ID -device-id $VALID_DEVICE_ID -device-name $DEVICE_NAME -hndr-sw-version $DEVICE_VERSION
dbtool -db $DBPATH -op insert-api-key -tenant-id $TENANT_ID -device-id $VALID_DEVICE_ID -api-key $VALID_API_KEY
TEST_FILE_RULES="rules/$TENANT_ID/hndr-rules-r1.2.3.tar.gz"

objupdater -type software -dbconfig $DBPATH -minioconfig $MINIO_PATH -file minio/hndr-sw-v1.2.3.tar.gz
objupdater -type rules -dbconfig $DBPATH -minioconfig $MINIO_PATH -file minio/hndr-rules-r1.2.3.tar.gz -tenantid $TENANT_ID
objupdater -type threatintel -dbconfig $DBPATH -minioconfig $MINIO_PATH -file minio/threatintel-2025.04.10.1523.tar.gz

# 5. Test API server authentication (valid credentials)
echo "Testing API server with valid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$API_PORT/v1/authenticate/$TENANT_ID" | grep -q 200
print_status $? "API server valid authentication passed"
# 6. Test API server authentication (invalid credentials)
echo "Testing API server with invalid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $INVALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$API_PORT/v1/authenticate/$TENANT_ID" | grep -q 401
print_status $? "API server invalid authentication rejected"
# 7. Test API server updates API
curl -k -s -o /dev/null -w "%{http_code}" -X POST -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" -d '{"image_version":"v1.2.2","rules_version":"2025.03.01","threatfeed_version":"2025.04.01.001"}' "https://localhost:$NGINX_SSL_PORT/v1/updates/$TENANT_ID" | grep -q 200
print_status $? "API server updates POST passed"

# 8. Test Nginx proxy to MinIO (invalid credentials)
echo "Testing Nginx proxy to MinIO with invalid credentials..."
curl -k -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $INVALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "https://localhost:$NGINX_SSL_PORT/v1/download/$TENANT_ID/$TEST_FILE_IMAGE" | grep -q 401
print_status $? "Nginx proxy to MinIO with invalid credentials rejected"
# 9. Test Nginx proxy to MinIO (image)
echo "Testing Nginx proxy to MinIO (images) with valid credentials..."
curl -k -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "https://localhost:$NGINX_SSL_PORT/v1/download/$TENANT_ID/$TEST_FILE_IMAGE" | grep -q 200
print_status $? "Nginx download of software passed"
# 10. Test Nginx proxy to MinIO (threatintel)
echo "Testing Nginx proxy to MinIO (threatintel) with valid credentials..."
curl -k -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "https://localhost:$NGINX_SSL_PORT/v1/download/$TENANT_ID/$TEST_FILE_THREATINTEL" | grep -q 200
print_status $? "Nginx download of threatintel passed"
# 11. Test Nginx proxy to MinIO (rules)
echo "Testing Nginx proxy to MinIO (rules) with valid credentials..."
curl -k -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "https://localhost:$NGINX_SSL_PORT/v1/download/$TENANT_ID/$TEST_FILE_RULES" | grep -q 200
print_status $? "Nginx download of rules passed"
# Test API server status API
echo "Testing API server status endpoint ..."
curl -k -s -o /dev/null -w "%{http_code}" -X POST -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" -d '{"software": {"status":"success"},"rules": {"status":"failure"},"threatintel":{"status":"success"}}' "https://localhost:$NGINX_SSL_PORT/v1/status/$TENANT_ID" | grep -q 200
print_status $? "API server status end point passed"

echo -e "${GREEN}All sanity tests passed!${NC}"

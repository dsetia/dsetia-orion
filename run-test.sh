#!/bin/bash

# Configuration
COMPOSE_FILE="docker-compose.yml"
NETWORK_NAME="securite-net"
NGINX_PORT=80
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
API_PORT=8080
VALID_API_KEY="key1"
VALID_DEVICE_ID="dev1"
INVALID_API_KEY="invalid-key"
TEST_FILE_IMAGE="images/hndr-sw-v1.2.3.tar.gz"
TEST_FILE_THREATINTEL="threatintel/threatintel-2025.04.10.1523.tar.gz"
TEST_FILE_RULES="rules/1/hndr-rules-r1.2.3.tar.gz"

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

# 3. Verify containers are running
echo "Checking container status..."
sudo docker ps --filter "name=nginx" --filter "status=running" -q | grep . >/dev/null
print_status $? "Nginx container is running"
sudo docker ps --filter "name=minio" --filter "status=running" -q | grep . >/dev/null
print_status $? "MinIO container is running"
sudo docker ps --filter "name=apis-container" --filter "status=running" -q | grep . >/dev/null
print_status $? "API server container is running"

# 4. Test MinIO health check
echo "Testing MinIO health endpoint..."
curl -s -o /dev/null -w "%{http_code}" "http://localhost:$MINIO_API_PORT/minio/health/live" | grep -q 200
print_status $? "MinIO health check passed"

# 5. Test API server authentication (valid credentials)
echo "Testing API server with valid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$API_PORT/v1/authenticate/1" | grep -q 200
print_status $? "API server valid authentication passed"
# 6. Test API server authentication (invalid credentials)
echo "Testing API server with invalid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $INVALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$API_PORT/v1/authenticate/1" | grep -q 401
print_status $? "API server invalid authentication rejected"
# 7. Test API server updates API
curl -s -o /dev/null -w "%{http_code}" POST -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: dev1" -d '{"image_version":"v1.2.2","rules_version":"2025.03.01","threatfeed_version":"2025.04.01.001"}' "http://localhost/v1/updates/1" | grep -q 200
print_status $? "API server updates POST passed"

# 8. Test Nginx proxy to MinIO (invalid credentials)
echo "Testing Nginx proxy to MinIO with invalid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $INVALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$NGINX_PORT/v1/download/1/$TEST_FILE_IMAGE" | grep -q 401
print_status $? "Nginx proxy to MinIO with invalid credentials rejected"
# 9. Test Nginx proxy to MinIO (image)
echo "Testing Nginx proxy to MinIO (images) with valid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$NGINX_PORT/v1/download/1/$TEST_FILE_IMAGE" | grep -q 200
print_status $? "Nginx download of image passed"
# 10. Test Nginx proxy to MinIO (threatintel)
echo "Testing Nginx proxy to MinIO (threatintel) with valid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$NGINX_PORT/v1/download/1/$TEST_FILE_THREATINTEL" | grep -q 200
print_status $? "Nginx download of threatintel passed"
# 11. Test Nginx proxy to MinIO (rules)
echo "Testing Nginx proxy to MinIO (rules) with valid credentials..."
curl -s -o /dev/null -w "%{http_code}" -H "X-API-KEY: $VALID_API_KEY" -H "X-DEVICE-ID: $VALID_DEVICE_ID" "http://localhost:$NGINX_PORT/v1/download/1/$TEST_FILE_RULES" | grep -q 200
print_status $? "Nginx download of rules passed"

echo -e "${GREEN}All sanity tests passed!${NC}"

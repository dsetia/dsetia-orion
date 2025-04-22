#!/bin/bash

# Configuration
COMPOSE_FILE="docker-compose.yml"
NETWORK_NAME="securite-net"
NGINX_PORT=80
MINIO_API_PORT=9000
MINIO_CONSOLE_PORT=9001
API_PORT=8080
VALID_API_KEY="a04cfb37-b7f5-4039-8538-a1cd961b298a"
VALID_DEVICE_ID="dev1"
INVALID_API_KEY="invalid-key"
TEST_FILE="images/hndr-sw-v1.2.3.tar.gz"

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

# 1. Ensure Docker network exists
echo "Checking for Docker network: $NETWORK_NAME..."
sudo docker network inspect "$NETWORK_NAME" >/dev/null 2>&1 || sudo docker network create "$NETWORK_NAME"
print_status $? "Docker network $NETWORK_NAME is ready"

# 2. Launch containers
echo "Starting containers with Docker Compose..."
sudo docker-compose -f "$COMPOSE_FILE" up -d --build
print_status $? "Containers started"

# Wait for containers to be healthy (up to 30 seconds)
echo "Waiting for containers to be ready..."
sleep 5
for i in {1..6}; do
    nginx_status=$(docker inspect --format='{{.State.Status}}' nginx 2>/dev/null)
    minio_status=$(docker inspect --format='{{.State.Status}}' minio 2>/dev/null)
    apis_status=$(docker inspect --format='{{.State.Status}}' apis-container 2>/dev/null)
    if [[ "$nginx_status" == "running" && "$minio_status" == "running" && "$apis_status" == "running" ]]; then
        break
    fi
    echo "Waiting... ($i/6)"
    sleep 5
done

# 3. Verify containers are running
echo "Checking container status..."
sudo docker ps --filter "name=nginx" --filter "status=running" -q | grep . >/dev/null
print_status $? "Nginx container is running"
sudo docker ps --filter "name=minio" --filter "status=running" -q | grep . >/dev/null
print_status $? "MinIO container is running"
sudo docker ps --filter "name=apis-container" --filter "status=running" -q | grep . >/dev/null
print_status $? "API server container is running"

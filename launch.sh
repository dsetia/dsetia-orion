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

OVERRIDE_OPT=""
if [[ -n "$1" ]]; then
  OVERRIDE_OPT="-f $COMPOSE_FILE -f $1"
else
  OVERRIDE_OPT="-f $COMPOSE_FILE"
fi

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

# 1. Launch containers
echo "Starting containers with Docker Compose..."
docker-compose $OVERRIDE_OPT up -d --build
print_status $? "Containers started"

# 2. Ensure Docker network exists
echo "Checking for Docker network: $NETWORK_NAME..."
docker network inspect "$NETWORK_NAME" >/dev/null 2>&1 || docker network create "$NETWORK_NAME"
print_status $? "Docker network $NETWORK_NAME is ready"


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
docker ps --filter "name=nginx" --filter "status=running" -q | grep . >/dev/null
print_status $? "Nginx container is running"
docker ps --filter "name=minio" --filter "status=running" -q | grep . >/dev/null
print_status $? "MinIO container is running"
docker ps --filter "name=apis-container" --filter "status=running" -q | grep . >/dev/null
print_status $? "API server container is running"
docker ps --filter "name=kafka" --filter "status=running" -q | grep . >/dev/null
print_status $? "Kafka (Redpanda) container is running"

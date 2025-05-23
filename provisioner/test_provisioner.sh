#!/bin/bash
set -e

# Print usage/help
usage() {
    echo "Usage: $0 [tenant-name] [device-name]"
    echo "  Test deployment for specific tenant and device"
    echo "  tenant-name (default tenant1"
    echo "  device-name (default dev1"
    exit 1
}

[[ $# -lt 2 ]] && usage
TENANT_NAME=${1:-"tenant1"}
DEVICE_NAME=${2:-"dev1"}

# Configuration
TARBALL_PKG="sensor-provision.tar.gz"
SENSOR_IMAGE="sensor:latest"
LOG_DIR="./test_logs"
API_URL="http://localhost:8080"  # Adjust to match nginx or api server port
SENSOR_PKG="sensor-provision.tar.gz"
PROVISIONER_PKG="package.tar.gz"

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

# Validate API connectivity
echo "Validating API connectivity..."
if curl -s -k "$API_URL" > /dev/null; then
    print_status $? "API connectivity validated successfully"
else
    print_status $? "Error: API connectivity failed"
    exit 1
fi

# Clean up previous runs
sudo docker rm -f sensor || true
rm -rf "$LOG_DIR" "$TARBALL_PKG"
mkdir -p "$LOG_DIR" "$LOG_DIR/suricata" "$LOG_DIR/updater"

echo "Provisioning tenant and sensor"
OUTPUT=$(provisioner -config=./config/provision-config.json -db=../config/db_dev_config.json -op provision-tenant -tenant-name "$TENANT_NAME")
provisioner -config=./config/provision-config.json -db=../config/db_dev_config.json -op provision-sensor -tenant-name "$TENANT_NAME" --device-name "$DEVICE_NAME" -minio=../config/minio_config.json

echo "$OUTPUT"
TENANT_ID=$(echo "$OUTPUT" | grep -oE 'ID=[0-9]+' | cut -d= -f2)

# echo "Uploading images to minio"
objupdater -type software -dbconfig ../config/db_dev_config.json -minioconfig ../config/minio_config.json -file ../minio/hndr-sw-v1.2.3.tar.gz
objupdater -type rules -dbconfig ../config/db_dev_config.json -minioconfig ../config/minio_config.json -file ../minio/hndr-rules-r1.2.3.tar.gz -tenantid $TENANT_ID
objupdater -type threatintel -dbconfig ../config/db_dev_config.json -minioconfig ../config/minio_config.json -file ../minio/threatintel-2025.04.10.1523.tar.gz

# Build and upload provisioner and sensor tarball
echo "Building provisioner packages"
./create-tarball.sh provisioner
echo "uploading provisioner packages"
mc cp $PROVISIONER_PKG myminio/provisioner/$PROVISIONER_PKG .
echo "Provisioner tarball uploaded to MinIO at provisioner/$PROVISIONER_PKG"

echo "Building sensor packages"
# sensor build needs device specific sensor-config.json
mc cp myminio/config/$TENANT_ID/sensor-config.json .
./create-tarball.sh sensor $TENANT_ID
echo "uploading sensor packages"
mc cp $SENSOR_PKG myminio/sensor/$TENANT_ID/$SENSOR_PKG
echo "Sensor tarball uploaded to MinIO at sensor/$TENANT_ID/$SENSOR_PKG"

# Build and start sensor container
echo "Building and starting sensor container..."
sudo docker build -f Dockerfile.sensor -t "$SENSOR_IMAGE" .
sudo docker run --name sensor \
  -v "$(pwd)/test_logs/suricata:/var/log/suricata" \
  -v "$(pwd)/test_logs/updater:/var/log/updater" \
  --network host \
  -d "$SENSOR_IMAGE"

# Wait for sensor to stabilize
echo "Waiting for sensor to stabilize..."
sleep 20

# Validate sensor logs
echo "Validating sensor logs..."
sudo docker logs sensor > "$LOG_DIR/sensor.log"
if grep -q "Sensor initialization complete" "$LOG_DIR/sensor.log" &&
   grep -q "hndr entered RUNNING state" "$LOG_DIR/sensor.log" &&
   grep -q "updater entered RUNNING state" "$LOG_DIR/sensor.log"; then
    print_status $? "Sensor logs validated successfully"
else
    print_status $? "Error: Sensor logs validation failed"
    cat "$LOG_DIR/sensor.log"
    exit 1
fi

# Validate suricata logs
echo "Validating suricata logs..."
if [ -f "$LOG_DIR/suricata/suricata.log" ] &&
   grep -q "Hello World" "$LOG_DIR/suricata/suricata.log"; then
    print_status $? "Suricata logs validated successfully"
else
    print_status $? "Error: Suricata logs validation failed"
    ls -l "$LOG_DIR"
    exit 1
fi

# Validate updater logs
echo "Validating updater logs..."
if [ -f "$LOG_DIR/updater/updater.log" ] &&
   grep -q "Status update sent successfully" "$LOG_DIR/updater/updater.log"; then
    print_status $? "Updater logs validated successfully"
else
    print_status $? "Error: Updater logs validation failed"
    ls -l "$LOG_DIR"
    cat "$LOG_DIR/updater.log" || true
    exit 1
fi


# Clean up
echo "Cleaning up..."
sudo docker rm -f sensor

echo "All tests passed successfully!"

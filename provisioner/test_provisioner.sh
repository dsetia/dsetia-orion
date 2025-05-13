#!/bin/bash
set -e

# Configuration
TARBALL_PATH="./sensor-provision.tar.gz"
SENSOR_IMAGE="sensor:latest"
LOG_DIR="./test_logs"
API_URL="http://localhost:8080"  # Adjust to match nginx or api server port

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

# Clean up previous runs
sudo docker rm -f sensor || true
rm -rf "$LOG_DIR" "$TARBALL_PATH"
mkdir -p "$LOG_DIR" "$LOG_DIR/suricata" "$LOG_DIR/updater"

# Build provisioner tarball
echo "Building provisioner tarball..."
./create-tarball.sh ./config/provision-config.json ../config/db_dev_config.json "$TARBALL_PATH"

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

# Validate API connectivity
echo "Validating API connectivity..."
if curl -s -k "$API_URL" > /dev/null; then
    print_status $? "API connectivity validated successfully"
else
    print_status $? "Error: API connectivity failed"
    exit 1
fi

# Clean up
echo "Cleaning up..."
sudo docker rm -f sensor
#rm -rf "$LOG_DIR" "$TARBALL_PATH"

echo "All tests passed successfully!"

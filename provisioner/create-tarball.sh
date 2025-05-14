#!/bin/bash
set -e

# Parameters
CONFIG_FILE=${1:-./config/provision-config.json}
DB_FILE=${2:-../config/db_dev_config.json}
TARBALL_OUTPUT=${3:-sensor-provision.tar.gz}

echo "Config $CONFIG_FILE, Output $TARBALL_OUTPUT"

# Validate config file
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Config file $CONFIG_FILE does not exist"
    exit 1
fi

# Generate config files
# sensor-config.json, updater-config.json, and hndr-config.json.

go build -o provision-sensor
./provision-sensor -config="$CONFIG_FILE" -db="$DB_FILE" -op provision-tenant -tenant-name "tenant1"
./provision-sensor -config="$CONFIG_FILE" -db="$DB_FILE" -op provision-sensor -tenant-name "tenant1" -device-name "Device 1"

# Build updater binary locally
# The CGO_ENABLED=0 flag disables CGO, ensuring a statically linked 
# binary that avoids GLIBC dependencies, making it compatible with 
# AlmaLinux 8 (GLIBC 2.28).
cd ../updater
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o updater
cd ../provisioner

# TODO
# 1. Bring in suricata config file (target: /opt/hndr/suricata/suricata.yaml)

# Create tarball
mkdir -p sensor-provision
cp config/sensor-config.json sensor-provision/
cp config/updater-config.json sensor-provision/
cp config/hndr-config.json sensor-provision/
cp ../updater/updater sensor-provision/
cp init-sensor.sh sensor-provision/
cp supervisor/updater.conf sensor-provision/
cp supervisor/hndr.conf sensor-provision/
chmod +x sensor-provision/init-sensor.sh
# Add Suricata binary
cp ./hello_world.sh sensor-provision/suricata
cp test_deployment.sh clean_deployment.sh sensor-provision/
tar -czf "$TARBALL_OUTPUT" sensor-provision
#rm -rf sensor-provision

echo "Tarball created at $TARBALL_OUTPUT"

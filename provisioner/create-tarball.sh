#!/bin/bash
set -e

# Parameters
TARBALL_OUTPUT=${1:-sensor-provision.tar.gz}

echo "Output $TARBALL_OUTPUT"

# Generate config files
# sensor-config.json, updater-config.json, and hndr-config.json.

# TODO
# 1. Bring in suricata config file (target: /opt/hndr/suricata/suricata.yaml)

# Create tarball
mkdir -p sensor-provision
cp config/sensor-config.json sensor-provision/
cp config/updater-config.json sensor-provision/
cp config/hndr-config.json sensor-provision/
cp /usr/local/bin/updater sensor-provision/
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

#!/bin/bash
set -e

# Parameters
TARBALL_OUTPUT=${1:-sensor-provision.tar.gz}

echo "Output $TARBALL_OUTPUT"

# Include:
# config files updater-config.json, and hndr-config.json.
# supervisor files for hndr and updater
# binaries for hndr and updater
# init script

# TODO
# 1. Bring in suricata config file (target: /opt/hndr/suricata/suricata.yaml)

# Create tarball
mkdir -p sensor-provision
cp config/updater-config.json sensor-provision/
cp config/hndr-config.json sensor-provision/
cp /usr/local/bin/updater sensor-provision/
cp init-sensor.sh sensor-provision/
cp supervisor/updater.conf sensor-provision/
cp supervisor/hndr.conf sensor-provision/
chmod +x sensor-provision/init-sensor.sh
# Add dummy Suricata binary
cp ./hello_world.sh sensor-provision/suricata
cp test_deployment.sh clean_deployment.sh sensor-provision/
tar -czf "$TARBALL_OUTPUT" sensor-provision
#rm -rf sensor-provision

echo "Tarball created at $TARBALL_OUTPUT"

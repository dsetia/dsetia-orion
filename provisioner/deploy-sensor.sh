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
# usage: deploy-sensor.sh sensor_hostname sensor-provision.tar.gz

# deploy tarball on the sensor

set -e
SENSOR_HOST=$1
TARBALL_PATH=$2
scp "$TARBALL_PATH" "user@$SENSOR_HOST:/tmp/"
ssh "user@$SENSOR_HOST" "
    cd /tmp &&
    tar -xzf sensor-provision.tar.gz &&
    cd sensor-provision &&
    chmod +x init-sensor.sh &&
    ./init-sensor.sh
"

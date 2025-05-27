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
set -e

# entry point running provisioner UT locally
# launches various services in the sensor container

# Run init-sensor.sh (assumes it’s in /app/sensor-provision)
cd /app/sensor-provision
./init-sensor.sh

# Start supervisord
# -n to run in foreground for testing
exec /usr/bin/supervisord -c /etc/supervisord.conf -n

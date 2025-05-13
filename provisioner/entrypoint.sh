#!/bin/bash
set -e

# entry point running provisioner UT locally
# launches various services in the sensor container

# Run init-sensor.sh (assumes it’s in /app/sensor-provision)
cd /app/sensor-provision
./init-sensor.sh

# Start supervisord
# -n to run in foreground for testing
exec /usr/bin/supervisord -c /etc/supervisord.conf -n

#!/bin/bash
set -e

# debug entry point running provisioner UT locally
# drops to sensor shell for testing

# Run init-sensor.sh
cd /app/sensor-provision
./init-sensor.sh

# Start a shell for debugging
exec /bin/bash

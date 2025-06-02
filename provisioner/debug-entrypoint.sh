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

# debug entry point running provisioner UT locally
# drops to sensor shell for testing

# Run init-sensor.sh
cd /app/sensor-provision
./init-sensor.sh

# Start a shell for debugging
exec /bin/bash

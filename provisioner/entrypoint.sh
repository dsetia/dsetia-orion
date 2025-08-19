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

# Patch supervisord.conf inside containers if missing required sections
if ! grep -q "\[unix_http_server\]" /etc/supervisord.conf; then
    cat <<EOF >> /etc/supervisord.conf

[unix_http_server]
file=/tmp/supervisor.sock
chmod=0700

[rpcinterface:supervisor]
supervisor.rpcinterface_factory=supervisor.rpcinterface:make_main_rpcinterface

[supervisorctl]
serverurl=unix:///tmp/supervisor.sock
EOF
fi


# Start supervisord
# -n to run in foreground for testing
exec /usr/bin/supervisord -c /etc/supervisord.conf -n

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

set -e  # Exit immediately if a command exits with a non-zero status



# Print usage/help
usage() {
    echo "Usage: $0 {init|populate|stop|launch|status|restart|test}"
    exit 1
}

init() {
    # DB init directly from outside the docker network
    ./init_db.sh ./config/db_dev.json
}

populate() {
    # DB populate directly from outside the docker network
    ./populate_db.sh ./config/db_dev.json
}

launch() {
    echo "[start] Starting stub"
    ./launch.sh
}

stop() {
    echo "[stop] Stopping services"
    docker compose down
}

restart() {
    stop
    launch
}

test() {
    echo "[test] Run tests"
    ./run_test.sh
}

status() {
    echo "[status] Show status"
    sudo docker ps
}

# Main logic: dispatch to function
if [ $# -ne 1 ]; then
    usage
fi

case "$1" in
    init) init ;;
    populate) populate ;;
    stop) stop ;;
    test) test ;;
    launch) launch ;;
    restart) restart ;;
    status) status ;;
    *) usage ;;
esac


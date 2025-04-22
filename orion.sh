#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status



# Print usage/help
usage() {
    echo "Usage: $0 {init|populate|stop|test|launch}"
    exit 1
}

init() {
    # Copy contents of your original init.sh here
    ./init_db.sh
}

populate() {
    # Copy contents of your original init.sh here
    ./populate_db.sh
}

launch() {
    echo "[start] Starting stub"
    ./launch.sh
}

stop() {
    echo "[stop] Stopping services"
    ./stop.sh
}

test() {
    echo "[test] Run tests"
    ./run_test.sh
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
    *) usage ;;
esac


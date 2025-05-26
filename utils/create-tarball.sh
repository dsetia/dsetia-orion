#!/bin/bash

set -euo pipefail

# Constants
SENSOR_PKG="sensor-provision.tar.gz"
PROVISIONER_PKG="package.tar.gz"
TMP_DIR="/tmp/sensor-provision-build"
MINIO_ALIAS="myminio_$$" # Unique alias using PID to avoid conflicts
CONFIG_DIR="config"
BIN_DIR="/usr/local/bin"
SUPERVISOR_DIR="supervisor"

# Default parameters
MINIO_CONFIG=${2:-"$CONFIG_DIR/minio_config.json"}
TENANT_ID=${3:-"1"}

# Print usage/help
usage() {
    echo "Usage: $0 {sensor|provisioner} [tenant-id]"
    echo "  sensor: Build sensor package"
    echo "  provisioner: Build provisioner package"
    echo "  tenant-id: Tenant ID for sensor config (default: 1)"
    exit 1
}

# Log message with timestamp
log() {
    local level=$1
    shift
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $@"
}

# Error and exit
error() {
    log "ERROR" "$@"
    exit 1
}

# Validate dependencies
check_deps() {
    for cmd in jq ; do
        if ! command -v "$cmd" &>/dev/null; then
            error "'$cmd' is required but not installed. Please install it (e.g., 'sudo apt-get install $cmd' for jq, or download mc from https://min.io/docs/minio/linux/reference/minio-mc.html)."
        fi
    done
}

# Validate tenant ID
validate_tenant_id() {
    if ! [[ "$TENANT_ID" =~ ^[0-9]+$ ]] || [[ "$TENANT_ID" -le 0 ]]; then
        error "Invalid tenant ID '$TENANT_ID'; must be a positive integer"
    fi
}

# Build provisioner tarball
build_provisioner_package() {
    log "INFO" "Building provisioner package"

    # Validate input files
    local files=(
        "$CONFIG_DIR/updater-config.json"
        "$CONFIG_DIR/hndr-config.json"
        "$CONFIG_DIR/suricata.yaml"
        "$BIN_DIR/updater"
        "init-sensor.sh"
        "$SUPERVISOR_DIR/updater.conf"
        "$SUPERVISOR_DIR/hndr.conf"
        "hello_world.sh" # Dummy Suricata binary
        "test_deployment.sh"
        "clean_deployment.sh"
    )
    for file in "${files[@]}"; do
        if [[ ! -f "$file" ]]; then
            error "Required file '$file' does not exist"
        fi
    done

    # Create temporary directory
    mkdir -p "$TMP_DIR/sensor-provision" || error "Failed to create directory $TMP_DIR/sensor-provision"
    trap 'rm -rf "$TMP_DIR"; log "INFO" "Cleaned up temporary directory $TMP_DIR"' EXIT

    # Copy files
    cp "$CONFIG_DIR/updater-config.json" "$CONFIG_DIR/hndr-config.json" "$TMP_DIR/sensor-provision/" || error "Failed to copy config files"
    cp "$BIN_DIR/updater" "$TMP_DIR/sensor-provision/" || error "Failed to copy updater binary"
    cp "init-sensor.sh" "$TMP_DIR/sensor-provision/" || error "Failed to copy init-sensor.sh"
    cp "$SUPERVISOR_DIR/updater.conf" "$SUPERVISOR_DIR/hndr.conf" "$TMP_DIR/sensor-provision/" || error "Failed to copy supervisor configs"
    cp "hello_world.sh" "$TMP_DIR/sensor-provision/suricata" || error "Failed to copy dummy Suricata binary"
    cp "test_deployment.sh" "clean_deployment.sh" "$TMP_DIR/sensor-provision/" || error "Failed to copy deployment scripts"

    # Copy Suricata config
    cp "$CONFIG_DIR/suricata.yaml" "$TMP_DIR/sensor-provision/" || error "Failed to copy Suricata config"

    # Set permissions
    chmod +x "$TMP_DIR/sensor-provision/init-sensor.sh" || error "Failed to set executable permission on init-sensor.sh"

    # Create tarball
    tar -czf "$PROVISIONER_PKG" -C "$TMP_DIR" sensor-provision || error "Failed to create tarball $PROVISIONER_PKG"
    log "INFO" "Provisioner tarball created at $PROVISIONER_PKG"
}

# Build sensor package
build_sensor_package() {
    log "INFO" "Building sensor package"

    # Clean up temporary directory
    rm -rf "$TMP_DIR" || error "Failed to clean up $TMP_DIR"
    mkdir -p "$TMP_DIR" || error "Failed to create $TMP_DIR"

    # provisioner package should be in current directory
    # Extract provisioner package
    tar -xzf "$PROVISIONER_PKG" -C "$TMP_DIR" || error "Failed to extract $PROVISIONER_PKG"

    # tenant-specific sensor config should be in current directory
    local sensor_config="sensor-config.json"
    cp "$sensor_config" "$TMP_DIR/sensor-provision/" || error "Failed to copy sensor config"

    # Create sensor tarball
    tar -czf "$SENSOR_PKG" -C "$TMP_DIR" sensor-provision || error "Failed to create sensor tarball $SENSOR_PKG"
    log "INFO" "Sensor tarball created at $SENSOR_PKG"
}

# Main logic
check_deps
[[ $# -lt 1 ]] && usage
validate_tenant_id

case "$1" in
    sensor)
        build_sensor_package
        ;;
    provisioner)
        build_provisioner_package
        ;;
    *)
        usage
        ;;
esac

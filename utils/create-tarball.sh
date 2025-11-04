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

set -eo pipefail

# Constants
SENSOR_PKG="sensor-provision.tar.gz"
PROVISIONER_PKG="package.tar.gz"
TMP_DIR="/tmp/sensor-provision-build"
MINIO_ALIAS="myminio_$$" # Unique alias using PID to avoid conflicts
BIN_DIR="/usr/local/bin"

# Default parameters
CONFIG_DIR=${2:-"../config"}
TENANT_ID=${3:-"1"}
MINIO_CONFIG=$CONFIG_DIR/minio.json
SUPERVISOR_DIR="$CONFIG_DIR/supervisor"
LOGROTATE_DIR="$CONFIG_DIR/logrotate.d"

# Print usage/help
usage() {
    echo "Usage: $0 {sensor|provisioner} [cfg-dir] [tenant-id] [device-id]"
    echo "  sensor: Build sensor package"
    echo "  provisioner: Build provisioner package"
    echo "  cfg-dir: Name of config directory"
    echo "  tenant-id: Tenant ID for sensor (default: 1)"
    echo "  device-id: Device ID for sensor"
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

# Validate and load MinIO config
load_minio_config() {
    if [[ ! -f "$MINIO_CONFIG" ]]; then
        error "MinIO config file '$MINIO_CONFIG' does not exist"
    fi

    ADMIN_USER=$(jq -r '.user // empty' "$MINIO_CONFIG") || error "Failed to parse 'user' from $MINIO_CONFIG"
    ADMIN_PASS=$(jq -r '.password // empty' "$MINIO_CONFIG") || error "Failed to parse 'password' from $MINIO_CONFIG"
    ENDPOINT=$(jq -r '.endpoint // empty' "$MINIO_CONFIG") || error "Failed to parse 'endpoint' from $MINIO_CONFIG"

    [[ -z "$ADMIN_USER" ]] && error "MinIO user is empty in $MINIO_CONFIG"
    [[ -z "$ADMIN_PASS" ]] && error "MinIO password is empty in $MINIO_CONFIG"
    [[ -z "$ENDPOINT" ]] && error "MinIO endpoint is empty in $MINIO_CONFIG"

    log "INFO" "Loaded MinIO configuration:"
    log "INFO" "  User     : $ADMIN_USER"
    log "INFO" "  Endpoint : $ENDPOINT"
}

# Set up MinIO alias
setup_minio_alias() {
    log "INFO" "Setting up MinIO alias '$MINIO_ALIAS'"
    if ! mc alias set "$MINIO_ALIAS" "http://$ENDPOINT" "$ADMIN_USER" "$ADMIN_PASS" &>/dev/null; then
        error "Failed to set MinIO alias for http://$ENDPOINT"
    fi
}

# Validate tenant ID
validate_tenant_id() {
    if ! [[ "$TENANT_ID" =~ ^[0-9]+$ ]] || [[ "$TENANT_ID" -le 0 ]]; then
        error "Invalid tenant ID '$TENANT_ID'; must be a positive integer"
    fi
}

# Validate device ID
validate_device_id() {
    if [ -z "$DEVICE_ID" ]; then
        error "Missing device ID"
    fi
    if ! [[ "$DEVICE_ID" =~ ^[a-zA-Z0-9-]+$ ]]; then
        error "Invalid device ID '$DEVICE_ID'"
    fi
}

# Build provisioner tarball
build_provisioner_package() {
    log "INFO" "Building provisioner package"

    # Validate input files
    local files=(
        "$CONFIG_DIR/provisioner/hndr-config.json"
        "$BIN_DIR/updater"
        "$CONFIG_DIR/scripts/init-sensor.sh"
        "$SUPERVISOR_DIR/updater.conf"
        "$SUPERVISOR_DIR/hndr.conf"
        "$CONFIG_DIR/scripts/hello_world.sh" # Dummy Suricata binary
        "$CONFIG_DIR/scripts/test_deployment.sh"
        "$CONFIG_DIR/scripts/clean_deployment.sh"
        "$CONFIG_DIR/filebeat.yml"
        "$LOGROTATE_DIR/securite"
    )
    for file in "${files[@]}"; do
        if [[ ! -f "$file" ]]; then
            error "Required file '$file' does not exist"
        fi
    done

    # Create temporary directory
    mkdir -p "$TMP_DIR/sensor-provision" || error "Failed to create directory $TMP_DIR/sensor-provision"
    trap 'rm -rf "$TMP_DIR"; log "INFO" "Cleaned up temporary directory $TMP_DIR"' EXIT

    # Download updater-config.json from MinIo
    if ! mc cp "$MINIO_ALIAS/provisioner/updater-config.json" "$TMP_DIR/sensor-provision/updater-config.json" &>/dev/null; then
        error "Failed to download updater-config.json from MinIO"
    fi

    # Copy files
    cp "$CONFIG_DIR/provisioner/hndr-config.json" "$TMP_DIR/sensor-provision/" || error "Failed to copy config files"
    cp "$BIN_DIR/updater" "$TMP_DIR/sensor-provision/" || error "Failed to copy updater binary"
    cp "$CONFIG_DIR/scripts/init-sensor.sh" "$TMP_DIR/sensor-provision/" || error "Failed to copy init-sensor.sh"
    cp "$SUPERVISOR_DIR/updater.conf" "$SUPERVISOR_DIR/hndr.conf" "$TMP_DIR/sensor-provision/" || error "Failed to copy supervisor configs"
    cp "$SUPERVISOR_DIR/filebeat.conf" "$TMP_DIR/sensor-provision/" || error "Failed to copy supervisor configs"
    cp "$CONFIG_DIR/scripts/hello_world.sh" "$TMP_DIR/sensor-provision/suricata" || error "Failed to copy dummy Suricata binary"
    cp "$CONFIG_DIR/scripts/test_deployment.sh" "$CONFIG_DIR/scripts/clean_deployment.sh" "$TMP_DIR/sensor-provision/" || error "Failed to copy deployment scripts"
    cp "$CONFIG_DIR/filebeat.yml" "$TMP_DIR/sensor-provision/" || error "Failed to copy filebeat.yml"
    cp "$LOGROTATE_DIR/securite" "$TMP_DIR/sensor-provision/" || error "Failed to copy logrotate configs"

    # Set permissions
    chmod +x "$TMP_DIR/sensor-provision/init-sensor.sh" || error "Failed to set executable permission on init-sensor.sh"

    # Create tarball
    tar -czf "$PROVISIONER_PKG" -C "$TMP_DIR" sensor-provision || error "Failed to create tarball $PROVISIONER_PKG"
    log "INFO" "Provisioner tarball created at $PROVISIONER_PKG"

    # Upload to MinIO
    if ! mc cp "$PROVISIONER_PKG" "$MINIO_ALIAS/provisioner/$PROVISIONER_PKG" &>/dev/null; then
        error "Failed to upload $PROVISIONER_PKG to MinIO at $MINIO_ALIAS/provisioner/$PROVISIONER_PKG"
    fi
    log "INFO" "Provisioner tarball uploaded to MinIO at provisioner/$PROVISIONER_PKG"
}

# Build sensor package
build_sensor_package() {
    log "INFO" "Building sensor package for $TENANT_ID/$DEVICE_ID"

    # Clean up temporary directory
    rm -rf "$TMP_DIR" || error "Failed to clean up $TMP_DIR"
    mkdir -p "$TMP_DIR" || error "Failed to create $TMP_DIR"

    # Download provisioner package
    if ! mc cp "$MINIO_ALIAS/provisioner/$PROVISIONER_PKG" "$TMP_DIR/$PROVISIONER_PKG" &>/dev/null; then
        error "Failed to download $PROVISIONER_PKG from MinIO"
    fi

    # Extract provisioner package
    cd "$TMP_DIR" || error "Failed to change to $TMP_DIR"
    tar -xzf "$PROVISIONER_PKG" || error "Failed to extract $PROVISIONER_PKG"

    # Download tenant-specific sensor config
    local sensor_config="sensor-config.json"
    if ! mc cp "$MINIO_ALIAS/config/$TENANT_ID/$DEVICE_ID/$sensor_config" "sensor-provision/$sensor_config" &>/dev/null; then
        error "Failed to download sensor-config.json for tenant $TENANT_ID from MinIO"
    fi

    # Create sensor tarball
    tar -czf "$SENSOR_PKG" sensor-provision || error "Failed to create sensor tarball $SENSOR_PKG"
    log "INFO" "Sensor tarball created at $SENSOR_PKG"

    # Upload to MinIO
    if ! mc cp "$SENSOR_PKG" "$MINIO_ALIAS/sensor/$TENANT_ID/$DEVICE_ID/$SENSOR_PKG" &>/dev/null; then
        error "Failed to upload $SENSOR_PKG to MinIO at $MINIO_ALIAS/sensor/$TENANT_ID/$SENSOR_PKG"
    fi
    log "INFO" "Sensor tarball uploaded to MinIO at sensor/$TENANT_ID/$DEVICE_ID/$SENSOR_PKG"

    # Clean up
    cd - >/dev/null || error "Failed to return to original directory"
}

# Main logic
check_deps
[[ $# -lt 2 ]] && usage
load_minio_config
setup_minio_alias
validate_tenant_id

case "$1" in
    sensor)
        DEVICE_ID=${4:-""}
        validate_device_id
        build_sensor_package
        ;;
    provisioner)
        build_provisioner_package
        ;;
    *)
        usage
        ;;
esac

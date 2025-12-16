#!/bin/bash

set -euo pipefail

# ============================================================================
# add-new-tenant-sensor.sh
# 
# Description: Automated script to create tenant, provision sensor, and 
#              generate sensor package in one workflow
#
# Usage: add-new-tenant-sensor.sh --tenant-name <name> --device-name <name> [--config-dir <path>]
# ============================================================================

# Color codes for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Default values
CONFIG_DIR="/opt/config"
TENANT_NAME=""
DEVICE_NAME=""

# Temporary files for capturing output
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

TENANT_OUTPUT="$TEMP_DIR/tenant_output.log"
SENSOR_OUTPUT="$TEMP_DIR/sensor_output.log"
TARBALL_OUTPUT="$TEMP_DIR/tarball_output.log"

# ============================================================================
# Helper Functions
# ============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

usage() {
    cat <<EOF
Usage: $0 --tenant-name <name> --device-name <name> [--config-dir <path>]

Required Parameters:
  --tenant-name <name>    Name of the tenant to create/use
  --device-name <name>    Name of the sensor device to provision

Optional Parameters:
  --config-dir <path>     Configuration directory (default: /opt/config)
  -h, --help             Show this help message

Description:
  This script automates the complete workflow for adding a new tenant and sensor:
  1. Creates tenant (if doesn't exist) and retrieves tenant ID
  2. Provisions sensor device and retrieves device ID
  3. Creates sensor deployment package (tarball)

Example:
  $0 --tenant-name "acme-corp" --device-name "sensor-01"
  $0 --tenant-name "test-tenant" --device-name "lobby-camera" --config-dir "/custom/config"

EOF
    exit 1
}

# ============================================================================
# Parameter Parsing
# ============================================================================

parse_arguments() {
    if [ $# -eq 0 ]; then
        usage
    fi

    while [ $# -gt 0 ]; do
        case "$1" in
            --tenant-name)
                TENANT_NAME="$2"
                shift 2
                ;;
            --device-name)
                DEVICE_NAME="$2"
                shift 2
                ;;
            --config-dir)
                CONFIG_DIR="$2"
                shift 2
                ;;
            -h|--help)
                usage
                ;;
            *)
                log_error "Unknown parameter: $1"
                usage
                ;;
        esac
    done

    # Validate required parameters
    if [ -z "$TENANT_NAME" ]; then
        log_error "Missing required parameter: --tenant-name"
        usage
    fi

    if [ -z "$DEVICE_NAME" ]; then
        log_error "Missing required parameter: --device-name"
        usage
    fi
}

# ============================================================================
# Validation Functions
# ============================================================================

validate_config_files() {
    local provisioner_config="$CONFIG_DIR/provisioner/provision-config.json"
    local db_config="$CONFIG_DIR/db.json"
    local minio_config="$CONFIG_DIR/minio.json"

    log_info "Validating configuration files..."

    if [ ! -f "$provisioner_config" ]; then
        log_error "Provisioner config not found: $provisioner_config"
        return 1
    fi

    if [ ! -f "$db_config" ]; then
        log_error "Database config not found: $db_config"
        return 1
    fi

    if [ ! -f "$minio_config" ]; then
        log_error "MinIO config not found: $minio_config"
        return 1
    fi

    log_success "All configuration files found"
    return 0
}

validate_commands() {
    log_info "Checking required commands..."

    local missing_commands=()

    if ! command -v provisioner &> /dev/null; then
        missing_commands+=("provisioner")
    fi

    if ! command -v create-tarball.sh &> /dev/null; then
        missing_commands+=("create-tarball.sh")
    fi

    if [ ${#missing_commands[@]} -gt 0 ]; then
        log_error "Missing required commands: ${missing_commands[*]}"
        log_error "Please ensure all commands are in your PATH"
        return 1
    fi

    log_success "All required commands available"
    return 0
}

# ============================================================================
# Step Execution Functions
# ============================================================================

step1_create_tenant() {
    log_info "Step 1: Creating tenant '$TENANT_NAME'..."
    
    local cmd="provisioner \
        -config=$CONFIG_DIR/provisioner/provision-config.json \
        -db=$CONFIG_DIR/db.json \
        -op provision-tenant \
        -tenant-name \"$TENANT_NAME\""

    echo ""
    echo "Command: $cmd"
    echo ""

    if ! eval "$cmd" 2>&1 | tee "$TENANT_OUTPUT"; then
        log_error "Failed to create tenant"
        return 1
    fi

    # Extract tenant ID from output
    if TENANT_ID=$(grep -oP 'ID=\K\d+' "$TENANT_OUTPUT" | head -1); then
        log_success "Tenant provisioned: ID=$TENANT_ID"
        echo "$TENANT_ID"
        return 0
    else
        log_error "Failed to extract tenant ID from output"
        return 1
    fi
}

step2_provision_sensor() {
    local tenant_id="$1"
    
    log_info "Step 2: Provisioning sensor '$DEVICE_NAME' for tenant ID=$tenant_id..."
    
    local cmd="provisioner \
        -config=$CONFIG_DIR/provisioner/provision-config.json \
        -db=$CONFIG_DIR/db.json \
        -minio=$CONFIG_DIR/minio.json \
        -op provision-sensor \
        -tenant-name \"$TENANT_NAME\" \
        --device-name \"$DEVICE_NAME\""

    echo ""
    echo "Command: $cmd"
    echo ""

    if ! eval "$cmd" 2>&1 | tee "$SENSOR_OUTPUT"; then
        log_error "Failed to provision sensor"
        return 1
    fi

    # Extract device ID (UUID) from output
    # Looking for pattern: "10/6eacc59d-7dac-4f42-9267-0dd8a3c6772b/sensor-config.json"
    if DEVICE_ID=$(grep -oP '\d+/\K[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' "$SENSOR_OUTPUT" | head -1); then
        log_success "Sensor provisioned: Device ID=$DEVICE_ID"
        echo "$DEVICE_ID"
        return 0
    else
        log_error "Failed to extract device ID from output"
        return 1
    fi
}

step3_create_tarball() {
    local tenant_id="$1"
    local device_id="$2"
    
    log_info "Step 3: Creating sensor package tarball..."
    
    local cmd="create-tarball.sh sensor $CONFIG_DIR $tenant_id $device_id"

    echo ""
    echo "Command: $cmd"
    echo ""

    if ! $cmd 2>&1 | tee "$TARBALL_OUTPUT"; then
        log_error "Failed to create sensor tarball"
        return 1
    fi

    # Check if tarball was created
    if [ -f "sensor-provision.tar.gz" ]; then
        log_success "Sensor tarball created: sensor-provision.tar.gz"
        return 0
    else
        log_error "Tarball file not found after creation"
        return 1
    fi
}

# ============================================================================
# Confirmation Display
# ============================================================================

display_summary_and_confirm() {
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "                    OPERATION SUMMARY"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "  Tenant Name:      $TENANT_NAME"
    echo "  Device Name:      $DEVICE_NAME"
    echo "  Config Directory: $CONFIG_DIR"
    echo ""
    echo "Operations to perform:"
    echo "  1. Create/verify tenant '$TENANT_NAME'"
    echo "  2. Provision sensor device '$DEVICE_NAME'"
    echo "  3. Generate sensor deployment package"
    echo ""
    echo "Configuration files:"
    echo "  • $CONFIG_DIR/provisioner/provision-config.json"
    echo "  • $CONFIG_DIR/db.json"
    echo "  • $CONFIG_DIR/minio.json"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo ""

    read -p "$(echo -e "${YELLOW}Proceed with tenant and sensor provisioning? [y/N]:${NC} ")" -n 1 -r
    echo ""
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_warning "Operation cancelled by user"
        exit 0
    fi
    
    echo ""
}

# ============================================================================
# Main Execution
# ============================================================================

main() {
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "           Add New Tenant & Sensor Workflow"
    echo "════════════════════════════════════════════════════════════════"
    echo ""

    # Parse arguments
    parse_arguments "$@"

    # Validate environment
    if ! validate_commands; then
        exit 1
    fi

    if ! validate_config_files; then
        exit 1
    fi

    # Display summary and get confirmation
    display_summary_and_confirm

    # Execute workflow
    log_info "Starting tenant and sensor provisioning workflow..."
    echo ""

    # Step 1: Create tenant
    if ! TENANT_ID=$(step1_create_tenant); then
        log_error "Workflow failed at Step 1: Tenant creation"
        exit 1
    fi
    echo ""

    # Step 2: Provision sensor
    if ! DEVICE_ID=$(step2_provision_sensor "$TENANT_ID"); then
        log_error "Workflow failed at Step 2: Sensor provisioning"
        exit 1
    fi
    echo ""

    # Step 3: Create tarball
    if ! step3_create_tarball "$TENANT_ID" "$DEVICE_ID"; then
        log_error "Workflow failed at Step 3: Tarball creation"
        exit 1
    fi
    echo ""

    # Final summary
    echo "════════════════════════════════════════════════════════════════"
    echo "                 WORKFLOW COMPLETED SUCCESSFULLY"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    log_success "Tenant Name:    $TENANT_NAME"
    log_success "Tenant ID:      $TENANT_ID"
    log_success "Device Name:    $DEVICE_NAME"
    log_success "Device ID:      $DEVICE_ID"
    log_success "Package:        sensor-provision.tar.gz"
    echo ""
    echo "Next steps:"
    echo "  • Deploy sensor-provision.tar.gz to target device"
    echo "  • Extract and run installation on device"
    echo "  • Verify sensor connectivity and configuration"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
}

# Run main function
main "$@"

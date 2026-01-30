#!/bin/bash
set -e

# Script metadata
SCRIPT_NAME="$(basename "$0")"
VERSION="1.0.0"
BACKUP_DIR="backups/orion_$(date +%Y%m%d_%H%M%S)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
header() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}$1${NC}"
    echo -e "${GREEN}========================================${NC}"
}
step()   { echo -e "${YELLOW}→ $1${NC}"; }
ok()     { echo -e "${GREEN}✓ $1${NC}"; }
err()    { echo -e "${RED}✗ $1${NC}"; }
info()   { echo -e "${BLUE}ℹ $1${NC}"; }

# Error handler
error_exit() {
    err "$1"
    exit 1
}

# Usage information
usage() {
    cat << EOF
Usage: $SCRIPT_NAME [OPTIONS] ROLE

Install Orion components based on server role.

ROLES:
  mgmt      Management server (dbtool, objupdater, MinIO setup)
  apis      API server (apis service)

OPTIONS:
  -h, --help           Show this help message
  -v, --version        Show version information
  -d, --dry-run        Show what would be done without executing
  --skip-services      Skip service restarts (useful for testing)

EXAMPLES:
  $SCRIPT_NAME mgmt                Install management components
  $SCRIPT_NAME apis                Install API server components
  $SCRIPT_NAME --dry-run mgmt      Preview management installation

NOTES:
  - Backups are always saved to: backups/YYYYMMDD_HHMMSS/
  - Script must be run from the extracted tarball directory
  - Root/sudo privileges required for copying to /opt and /usr/local/bin
  - MinIO configuration read from: /opt/config/minio.json

EOF
    exit 0
}

# Version information
version() {
    echo "$SCRIPT_NAME version $VERSION"
    exit 0
}

# Validate role
validate_role() {
    local role=$1
    case "$role" in
        mgmt|apis)
            return 0
            ;;
        *)
            error_exit "Invalid role: '$role'. Must be 'mgmt' or 'apis'."
            ;;
    esac
}

# Pre-flight checks
preflight_checks() {
    step "Running pre-flight checks..."

    # Check if running from correct directory
    if [[ ! -d "opt/bin" ]]; then
        error_exit "Must run from extracted tarball directory (opt/bin not found)"
    fi

    # Check for required tools
    local missing_tools=()
    for tool in cp mkdir supervisorctl; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done

    # Check for jq (needed for JSON parsing)
    if ! command -v jq &> /dev/null; then
        missing_tools+=("jq")
    fi

    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        error_exit "Missing required tools: ${missing_tools[*]}"
    fi

    ok "Pre-flight checks passed"
}

# Backup existing files
backup_mgmt() {
    step "Backing up existing management files..."
    mkdir -p "$BACKUP_DIR"

    local files=(
        "/usr/local/bin/dbtool"
        "/usr/local/bin/objupdater"
    )

    local backed_up=0
    for file in "${files[@]}"; do
        if [[ -f "$file" ]]; then
            cp "$file" "$BACKUP_DIR/"
	    backed_up=$((backed_up + 1))
            info "  Backed up: $file"
        fi
    done

    if [[ $backed_up -gt 0 ]]; then
        ok "Backed up $backed_up file(s) to $BACKUP_DIR"
    else
        info "No existing files to backup"
    fi
}

backup_apis() {
    step "Backing up existing API server files..."
    mkdir -p "$BACKUP_DIR"

    local files=(
        "/usr/local/bin/apis"
    )

    local backed_up=0
    for file in "${files[@]}"; do
        if [[ -f "$file" ]]; then
            cp "$file" "$BACKUP_DIR/"
	    backed_up=$((backed_up + 1))
            info "  Backed up: $file"
        fi
    done

    if [[ $backed_up -gt 0 ]]; then
        ok "Backed up $backed_up file(s) to $BACKUP_DIR"
    else
        info "No existing files to backup"
    fi
}

# Configure MinIO alias from minio.json
configure_minio_alias() {
    step "Configuring MinIO alias..."

    local minio_config="/opt/config/minio.json"

    if [[ ! -f "$minio_config" ]]; then
        err "MinIO configuration not found: $minio_config"
        return 1
    fi

    # Parse JSON configuration
    local user=$(jq -r '.user' "$minio_config")
    local password=$(jq -r '.password' "$minio_config")
    local endpoint=$(jq -r '.endpoint' "$minio_config")
    local usessl=$(jq -r '.usessl' "$minio_config")

    # Construct endpoint URL
    local protocol="http"
    if [[ "$usessl" == "true" ]]; then
        protocol="https"
    fi
    local minio_url="${protocol}://${endpoint}"

    info "  Endpoint: $minio_url"
    info "  User: $user"

    # Set MinIO alias
    if mc alias set myminio "$minio_url" "$user" "$password" &> /dev/null; then
        ok "MinIO alias 'myminio' configured successfully"
    else
        err "Failed to configure MinIO alias"
        return 1
    fi
}

# Install management components
install_mgmt() {
    header "Installing Management Components"

    step "Installing binaries..."
    cp opt/bin/* /usr/local/bin/
    ok "Binaries installed"

    # Check if mc (MinIO Client) is available
    if command -v mc &> /dev/null; then
        configure_minio_alias
    else
        info "MinIO Client (mc) not found - skipping alias configuration"
        info "Install mc from: https://min.io/docs/minio/linux/reference/minio-mc.html"
    fi

    ok "Management installation complete"
}

# Install API server components
install_apis() {
    header "Installing API Server Components"

    if [[ "$SKIP_SERVICES" != "true" ]]; then
        step "Stopping apis service..."
        if supervisorctl status apis &> /dev/null; then
            supervisorctl stop apis
            ok "Service stopped"
        else
            info "Service not running or not found in supervisord"
        fi
    fi

    step "Installing binary..."
    cp opt/bin/apis /usr/local/bin/
    chmod +x /usr/local/bin/apis
    ok "Binary installed"

    if [[ "$SKIP_SERVICES" != "true" ]]; then
        step "Starting apis service..."
        supervisorctl start apis
        sleep 2
        if supervisorctl status apis | grep -q RUNNING; then
            ok "Service started successfully"
        else
            err "Service failed to start - check logs with: supervisorctl tail apis"
        fi
    else
        info "Skipping service restart (--skip-services flag)"
    fi

    ok "API server installation complete"
}

# Main installation logic
install() {
    local role=$1

    preflight_checks

    echo ""
    read -p "Do you want to proceed with deploying as $role role? (yes/no): " CONFIRM
    if [[ "$CONFIRM" != "yes" ]]; then
        echo -e "${YELLOW}Deployment cancelled${NC}"
        exit 0
    fi
    echo ""

    case "$role" in
        mgmt)
            backup_mgmt
            install_mgmt
            ;;
        apis)
            backup_apis
            install_apis
            ;;
    esac
}

# Dry run mode
dry_run() {
    local role=$1

    header "DRY RUN MODE - No changes will be made"

    echo -e "\n${YELLOW}Would perform the following actions for role: $role${NC}\n"

    case "$role" in
        mgmt)
            echo "1. Backup existing files to: $BACKUP_DIR"
            echo "   - /usr/local/bin/dbtool"
            echo "   - /usr/local/bin/objupdater"
            echo ""
            echo "2. Install binaries:"
            echo "   opt/bin/dbtool → /usr/local/bin/"
            echo "   opt/bin/objupdater → /usr/local/bin/"
            echo ""
            echo "3. Configure MinIO alias from /opt/config/minio.json:"
            if [[ -f "/opt/config/minio.json" ]]; then
                local endpoint=$(jq -r '.endpoint' "/opt/config/minio.json")
                local user=$(jq -r '.user' "/opt/config/minio.json")
                local usessl=$(jq -r '.usessl' "/opt/config/minio.json")
                local protocol="http"
                [[ "$usessl" == "true" ]] && protocol="https"
                echo "   mc alias set myminio ${protocol}://${endpoint} ${user} ********"
            else
                echo "   (minio.json not found)"
            fi
            ;;
        apis)
            echo "1. Backup existing files to: $BACKUP_DIR"
            echo "   - /usr/local/bin/apis"
            echo ""
            echo "2. Stop apis service (supervisorctl stop apis)"
            echo ""
            echo "3. Install binary:"
            echo "   opt/bin/apis → /usr/local/bin/"
            echo ""
            echo "4. Start apis service (supervisorctl start apis)"
            ;;
    esac

    echo ""
    info "Run without --dry-run to execute these actions"
}

# Parse command line arguments
ROLE=""
DRY_RUN=false
SKIP_SERVICES=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -v|--version)
            version
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        --skip-services)
            SKIP_SERVICES=true
            shift
            ;;
        mgmt|apis)
            if [[ -n "$ROLE" ]]; then
                error_exit "Multiple roles specified. Only one role allowed."
            fi
            ROLE=$1
            shift
            ;;
        *)
            error_exit "Unknown option: $1. Use --help for usage information."
            ;;
    esac
done

# Validate we have a role
if [[ -z "$ROLE" ]]; then
    echo -e "${RED}Error: No role specified${NC}\n"
    usage
fi

validate_role "$ROLE"

# Execute based on mode
if [[ "$DRY_RUN" == "true" ]]; then
    dry_run "$ROLE"
else
    header "Orion Installer v$VERSION"
    info "Role: $ROLE"
    echo ""

    install "$ROLE"

    echo ""
    header "Installation Complete"
    info "Backup location: $BACKUP_DIR"
fi

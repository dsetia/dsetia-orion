#!/usr/bin/env bash
#
# Copyright (c) 2026 SecurITe
# All rights reserved.
#
# This source code is the property of SecurITe.
# Unauthorized copying, modification, or distribution of this file,
# via any medium is strictly prohibited unless explicitly authorized
# by SecurITe.
#
# This software is proprietary and confidential.
#
# File Owner:       deepinder@securite.world
# Created On:       01/30/2026

set -e

# Script metadata
SCRIPT_NAME="$(basename "$0")"
VERSION="1.0.0"

# GitHub API endpoint
GITHUB_API="https://api.github.com"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Logging functions
err()  { echo -e "${RED}✗ $1${NC}" >&2; }
ok()   { echo -e "${GREEN}✓ $1${NC}"; }
info() { echo -e "${YELLOW}→ $1${NC}"; }

# Error handler
error_exit() {
    err "$1"
    exit 1
}

# Usage information
usage() {
    cat << EOF
Usage: $SCRIPT_NAME [OPTIONS] REPO VERSION FILE OUTPUT

Download assets from GitHub releases (works with private repositories).

ARGUMENTS:
  REPO          Repository in format: owner/repo (e.g., cyberparticle/orion)
  VERSION       Release version tag (e.g., v1.2.3) or "latest"
  FILE          Asset filename to download (e.g., deployment.tar.gz)
  OUTPUT        Output filename to save the asset

OPTIONS:
  -h, --help    Show this help message
  -v, --version Show version information

ENVIRONMENT VARIABLES:
  GITHUB_TOKEN  GitHub personal access token (required)
                Create at: https://github.com/settings/tokens

PREREQUISITES:
  - curl
  - wget
  - jq

EXAMPLES:
  # Download latest release
  export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
  $SCRIPT_NAME cyberparticle/orion latest deployment.tar.gz output.tar.gz

  # Download specific version
  export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
  $SCRIPT_NAME cyberparticle/orion v2.1.1 deployment-20260129-1.tar.gz my_app.tar.gz

  # With token inline (not recommended for scripts in git)
  GITHUB_TOKEN="ghp_xxxx" $SCRIPT_NAME owner/repo latest app.tar.gz output.tar.gz

NOTES:
  - Personal access tokens need 'repo' scope for private repositories
  - Script will abort if GITHUB_TOKEN environment variable is not set
  - Use "latest" as VERSION to get the most recent release

EOF
    exit 0
}

# Version information
show_version() {
    echo "$SCRIPT_NAME version $VERSION"
    exit 0
}

# Check prerequisites
check_prerequisites() {
    local missing_tools=()

    for tool in curl wget jq; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done

    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        error_exit "Missing required tools: ${missing_tools[*]}"
    fi
}

# GitHub API curl wrapper
gh_curl() {
    curl -H "Authorization: token $GITHUB_TOKEN" \
         -H "Accept: application/vnd.github.v3.raw" \
         -s \
         "$@"
}

# Download release asset
download_release() {
    local repo=$1
    local version=$2
    local file=$3
    local output=$4

    info "Fetching release information from $repo..."

    # Construct jq parser based on version
    local parser
    if [[ "$version" == "latest" ]]; then
        # GitHub returns releases sorted by creation date (newest first)
        parser=".[0].assets | map(select(.name == \"$file\"))[0].id"
    else
        parser=". | map(select(.tag_name == \"$version\"))[0].assets | map(select(.name == \"$file\"))[0].id"
    fi

    # Get asset ID
    local asset_id
    asset_id=$(gh_curl "$GITHUB_API/repos/$repo/releases" | jq "$parser")

    if [[ "$asset_id" == "null" ]] || [[ -z "$asset_id" ]]; then
        if [[ "$version" == "latest" ]]; then
            error_exit "Asset '$file' not found in latest release"
        else
            error_exit "Asset '$file' not found in release '$version'"
        fi
    fi

    info "Found asset ID: $asset_id"
    info "Downloading to: $output"

    # Download the asset
    if wget -q --show-progress \
            --auth-no-challenge \
            --header="Accept: application/octet-stream" \
            "https://$GITHUB_TOKEN:@api.github.com/repos/$repo/releases/assets/$asset_id" \
            -O "$output"; then
        ok "Download complete: $output"

        # Show file size
        if [[ -f "$output" ]]; then
            local size=$(du -h "$output" | cut -f1)
            info "File size: $size"
        fi
    else
        error_exit "Download failed"
    fi
}

# Main script
main() {
    # Check for GitHub token
    if [[ -z "${GITHUB_TOKEN:-}" ]]; then
        error_exit "GITHUB_TOKEN environment variable is not set.

Create a token at: https://github.com/settings/tokens
Then set it with: export GITHUB_TOKEN=\"your_token_here\""
    fi

    # Parse arguments
    if [[ $# -eq 0 ]]; then
        usage
    fi

    case "$1" in
        -h|--help)
            usage
            ;;
        -v|--version)
            show_version
            ;;
    esac

    # Validate argument count
    if [[ $# -ne 4 ]]; then
        err "Error: Invalid number of arguments"
        echo ""
        usage
    fi

    local repo=$1
    local version=$2
    local file=$3
    local output=$4

    # Validate repository format
    if [[ ! "$repo" =~ ^[^/]+/[^/]+$ ]]; then
        error_exit "Invalid repository format: '$repo'. Expected: owner/repo"
    fi

    # Check prerequisites
    check_prerequisites

    # Download the release
    download_release "$repo" "$version" "$file" "$output"
}

main "$@"

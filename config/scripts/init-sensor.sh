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

# part of sensor deployment tarball; performs initialization 
# followed by launching hndr and updater services

# Create directories from updater-config.json
mkdir -p /opt/hndr-1/bin
mkdir -p /opt/hndr-2/bin
ln -sf /opt/hndr-1 /opt/hndr
mkdir -p /var/lib/suricata/rules
mkdir -p /var/log/suricata /var/log/updater
mkdir -p /opt/hndr/bin /opt/hndr/suricata
mkdir -p /opt/updater/bin /opt/updater/config
mkdir -p /opt/hndr/var/lib/suricata

# Install supervisor (if not already installed)
if ! command -v supervisorctl &> /dev/null; then
    echo "Installing supervisor..."
    # Detect OS
    OS_ID=$(grep -w ID /etc/os-release | cut -d'=' -f2 | tr -d '"')
    if [ "$OS_ID" = "almalinux" ]; then
        sudo dnf install -y epel-release || {
            echo "Failed to install epel-release from extras. Attempting direct download..."
            sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
        }
    elif [ "$OS_ID" = "rhel" ]; then
        sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
        if command -v subscription-manager &> /dev/null; then
            sudo subscription-manager repos --enable codeready-builder-for-rhel-9-$(arch)-rpms
        fi
    else
        echo "Unsupported OS: $OS_ID. Attempting direct EPEL installation..."
        sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm
    fi
    sudo dnf install -y supervisor
fi

# Install filebeat (if not already installed)
if [ ! -f /.dockerenv ]; then
  if ! command -v filebeat &> /dev/null; then
    echo "Installing filebeat..."
    sudo yum install -y filebeat
    sudo cp filebeat.yml /etc/filebeat/filebeat.yml
    echo "Starting filebeat..."
  fi
fi

# install other dependencies
if [ ! -f /.dockerenv ]; then
    sudo dnf install -y libnet hyperscan libpcap jansson-devel
fi

# Move files
ls -l
cp sensor-config.json updater-config.json hndr-config.json /opt/updater/config/
cp updater /opt/updater/bin/
cp suricata /opt/hndr/bin/

# Use /etc/supervisord.d/ for AlmaLinux 9, /etc/supervisor/conf.d/ for Docker
CONFIG_DIR="/etc/supervisor/conf.d"
if [ ! -f /.dockerenv ]; then
    CONFIG_DIR="/etc/supervisord.d"
    mkdir -p "$CONFIG_DIR"
    cp updater.conf "$CONFIG_DIR/updater.ini"
    cp hndr.conf "$CONFIG_DIR/hndr.ini"
    cp filebeat.conf "$CONFIG_DIR/filebeat.ini"
    # Ensure permissions
    chmod 644 "$CONFIG_DIR/updater.ini" "$CONFIG_DIR/hndr.ini" "$CONFIG_DIR/filebeat.ini"
    chown root:root "$CONFIG_DIR/updater.ini" "$CONFIG_DIR/hndr.ini" "$CONFIG_DIR/filebeat.ini"
else
    mkdir -p "$CONFIG_DIR"
    cp updater.conf hndr.conf filebeat.conf "$CONFIG_DIR/"
fi

wait_for_running() {
    local process=$1
    echo "$process: waiting for RUNNING state..."
    for i in {1..15}; do
        local status=$(supervisorctl status "$process" 2>/dev/null || echo "stopped")
        if echo "$status" | grep -q "RUNNING"; then
            echo "$process: confirmed running"
            return 0
        elif echo "$status" | grep -q "STARTING"; then
            echo "$process: still starting, waiting... ($i/15)"
            sleep 2
        else
            echo "Error: Unexpected status for $process: $status"
            return 1
        fi
    done
    echo "Error: $process failed to reach RUNNING state within 30 seconds"
    return 1
}

if [ -f /.dockerenv ]; then
    echo "Running in Docker, skipping supervisorctl commands"
else
    echo "Running on VM, starting supervisord and processes"
    if ! systemctl is-active --quiet supervisord; then
        systemctl enable supervisord
        systemctl start supervisord
        for i in {1..10}; do
            if systemctl is-active --quiet supervisord; then break; fi
            sleep 1
        done
        if ! systemctl is-active --quiet supervisord; then
            echo "Error: supervisord failed to start"
            exit 1
        fi
    fi
    echo "Supervisord is active"

    for i in {1..5}; do
        if supervisorctl reread && supervisorctl update; then break; fi
        echo "Retrying supervisorctl reread/update ($i/5)..."
        sleep 2
        if [ $i -eq 5 ]; then
            echo "Error: Failed to reload configs with supervisorctl"
            exit 1
        fi
    done

    for process in updater hndr; do
        status=$(supervisorctl status "$process" 2>/dev/null || echo "stopped")
        if echo "$status" | grep -q "RUNNING"; then
            echo "$process: already running, restarting..."
            if ! supervisorctl restart "$process"; then
                echo "Error: Failed to restart $process"
                exit 1
            fi
        elif echo "$status" | grep -q "STOPPED\|no such process"; then
            echo "$process: starting..."
            if ! supervisorctl start "$process"; then
                echo "Error: Failed to start $process"
                exit 1
            fi
        elif echo "$status" | grep -q "STARTING"; then
            echo "$process: already in STARTING state"
        else
            echo "Error: Unknown status for $process: $status"
            exit 1
        fi

        wait_for_running "$process" || exit 1
    done
fi

echo "Sensor initialization complete"

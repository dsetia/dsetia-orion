#!/bin/bash
set -e

# part of sensor deployment tarball; performs initialization 
# followed by launching hndr and updater services

# Create directories from updater-config.json
mkdir -p /tmp/hndr-1
mkdir -p /tmp/hndr-2
mkdir -p /tmp
ln -sf /tmp/hndr-1 /tmp/hndr
mkdir -p /var/lib/suricata/rules
mkdir -p /var/log/suricata /var/log/updater
mkdir -p /opt/hndr/bin /opt/hndr/suricata /opt/hndr/updater/config

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

# Move files
ls -l
mv sensor-config.json updater-config.json hndr-config.json /opt/hndr/updater/config/
mv updater /opt/hndr/bin/
mv suricata /opt/hndr/bin/
# Use /etc/supervisord.d/ for AlmaLinux 9, /etc/supervisor/conf.d/ for Docker
CONFIG_DIR="/etc/supervisor/conf.d"
if [ ! -f /.dockerenv ]; then
    CONFIG_DIR="/etc/supervisord.d"
    mkdir -p "$CONFIG_DIR"
    mv updater.conf "$CONFIG_DIR/updater.ini"
    mv hndr.conf "$CONFIG_DIR/hndr.ini"
    # Ensure permissions
    chmod 644 "$CONFIG_DIR/updater.ini" "$CONFIG_DIR/hndr.ini"
    chown root:root "$CONFIG_DIR/updater.ini" "$CONFIG_DIR/hndr.ini"
else
    mkdir -p "$CONFIG_DIR"
    mv updater.conf hndr.conf "$CONFIG_DIR/"
fi

# Check if running in Docker
if [ -f /.dockerenv ]; then
    echo "Running in Docker, skipping supervisorctl commands"
else
    echo "Running on VM, starting supervisord and processes"
    if ! systemctl is-active --quiet supervisord; then
        systemctl enable supervisord
        systemctl start supervisord
        # Wait for supervisord to start (up to 10 seconds)
        for i in {1..10}; do
            if systemctl is-active --quiet supervisord; then
                break
            fi
            sleep 1
        done
        if ! systemctl is-active --quiet supervisord; then
            echo "Error: supervisord failed to start"
            exit 1
        fi
    fi
    echo "Supervisord is active"
    # Reload configs with retry
    for i in {1..5}; do
        if supervisorctl reread && supervisorctl update; then
            break
        fi
        echo "Retrying supervisorctl reread/update ($i/5)..."
        sleep 2
        if [ $i -eq 5 ]; then
            echo "Error: Failed to reload configs with supervisorctl"
            exit 1
        fi
    done
    # Start processes only if not already running
    for process in updater hndr; do
        status=$(supervisorctl status "$process" 2>/dev/null || echo "stopped")
        if echo "$status" | grep -q "RUNNING"; then
            echo "$process: already running"
        elif echo "$status" | grep -q "STOPPED\|no such process"; then
            if ! supervisorctl start "$process"; then
                echo "Error: Failed to start $process"
                exit 1
            fi
            echo "$process: starting"
            # Wait for the process to transition from STARTING to RUNNING (up to 30 seconds)
            for i in {1..15}; do
                status=$(supervisorctl status "$process" 2>/dev/null || echo "stopped")
                if echo "$status" | grep -q "RUNNING"; then
                    echo "$process: confirmed running"
                    break
                elif echo "$status" | grep -q "STARTING"; then
                    echo "$process: still starting, waiting... ($i/15)"
                    sleep 2
                else
                    echo "Error: Unexpected status for $process: $status"
                    exit 1
                fi
                if [ $i -eq 15 ]; then
                    echo "Error: $process failed to reach RUNNING state within 30 seconds"
                    exit 1
                fi
            done
        elif echo "$status" | grep -q "STARTING"; then
            echo "$process: already in STARTING state, waiting for RUNNING..."
            # Wait for the process to transition from STARTING to RUNNING (up to 30 seconds)
            for i in {1..15}; do
                status=$(supervisorctl status "$process" 2>/dev/null || echo "stopped")
                if echo "$status" | grep -q "RUNNING"; then
                    echo "$process: confirmed running"
                    break
                elif echo "$status" | grep -q "STARTING"; then
                    echo "$process: still starting, waiting... ($i/15)"
                    sleep 2
                else
                    echo "Error: Unexpected status for $process: $status"
                    exit 1
                fi
                if [ $i -eq 15 ]; then
                    echo "Error: $process failed to reach RUNNING state within 30 seconds"
                    exit 1
                fi
            done
        else
            echo "Error: Unknown status for $process: $status"
            exit 1
        fi
    done
fi

echo "Sensor initialization complete"

#!/bin/bash
set -e

# script to run on sensor to revert changes by init script
# typically used for testing

# Configuration
UPDATER_LOG="/var/log/updater/updater.log"
SURICATA_LOG="/var/log/suricata/suricata.log"
UPDATER_BINARY="/opt/updater/bin/updater"
SURICATA_BINARY="/opt/hndr/bin/suricata"
UPDATER_CONFIG="/etc/supervisord.d/updater.ini"
SURICATA_CONFIG="/etc/supervisord.d/hndr.ini"  # Adjust to suricata.ini if needed

# Function to log messages
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Stop supervisord
log "Stopping supervisord..."
if systemctl is-active --quiet supervisord; then
    sudo systemctl stop supervisord
    log "Supervisord stopped"
else
    log "Supervisord already stopped"
fi

# Remove files
log "Removing files..."
for file in "$UPDATER_LOG" "$SURICATA_LOG" "$UPDATER_BINARY" "$SURICATA_BINARY" "$UPDATER_CONFIG" "$SURICATA_CONFIG"; do
    if [ -f "$file" ]; then
        sudo rm -f "$file"
        log "Removed $file"
    else
        log "$file does not exist"
    fi
done

# Remove directories
log "Removing directories..."
sudo rm -rf /var/log/suricata /var/lib/suricata /opt/hndr-1 /opt/hndr-2 /opt/hndr /opt/updater
log "Directories removed"

log "Cleanup complete!"

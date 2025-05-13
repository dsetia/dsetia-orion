#!/bin/bash
set -e

# Configuration
UPDATER_LOG="/var/log/updater/updater.log"
SURICATA_LOG="/var/log/suricata/suricata.log"
UPDATER_BINARY="/opt/hndr/bin/updater"
SURICATA_BINARY="/opt/hndr/bin/suricata"
UPDATER_CONFIG="/etc/supervisord.d/updater.ini"
SURICATA_CONFIG="/etc/supervisord.d/hndr.ini"  # Adjust to suricata.ini if needed

# Function to log messages
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Check if supervisord is running
log "Checking supervisord status..."
if systemctl is-active --quiet supervisord; then
    log "Supervisord is running"
else
    log "Error: Supervisord is not running"
    exit 1
fi

# Check process status
log "Checking process status..."
SUPERVISOR_STATUS=$(sudo supervisorctl status)
if echo "$SUPERVISOR_STATUS" | grep -q "updater.*RUNNING"; then
    log "Updater process is running"
else
    log "Error: Updater process is not running"
    echo "$SUPERVISOR_STATUS"
    exit 1
fi
if echo "$SUPERVISOR_STATUS" | grep -q "hndr.*RUNNING"; then
    log "Hndr (Suricata) process is running"
else
    log "Error: Hndr (Suricata) process is not running"
    echo "$SUPERVISOR_STATUS"
    exit 1
fi

# Check log files
log "Checking updater log file..."
if [ -f "$UPDATER_LOG" ] && grep -q "Status update sent successfully" "$UPDATER_LOG"; then
    log "Updater log validated successfully"
else
    log "Error: Updater log validation failed"
    ls -l "$(dirname "$UPDATER_LOG")"
    cat "$UPDATER_LOG" || true
    exit 1
fi

log "Checking suricata log file..."
if [ -f "$SURICATA_LOG" ] && grep -q "Hello World" "$SURICATA_LOG"; then
    log "Suricata log validated successfully"
else
    log "Error: Suricata log validation failed"
    ls -l "$(dirname "$SURICATA_LOG")"
    cat "$SURICATA_LOG" || true
    exit 1
fi

# Check installed files
log "Checking installed files..."
for file in "$UPDATER_BINARY" "$SURICATA_BINARY" "$UPDATER_CONFIG" "$SURICATA_CONFIG"; do
    if [ -f "$file" ]; then
        log "File $file exists"
    else
        log "Error: File $file does not exist"
        exit 1
    fi
done

log "Deployment validation successful!"

#!/bin/bash
set -e

LOG_FILE="/var/log/custom/diagnosis.log"

mkdir -p "$(dirname "$LOG_FILE")"

log() {
  echo "$1" | tee -a "$LOG_FILE"
}k

log "[custom.collector]"
log "- Custom collector script executed successfully."
log "- Timestamp: $(date)"
log "- Hostname: $(hostname)"

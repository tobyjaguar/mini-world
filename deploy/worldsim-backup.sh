#!/bin/bash
# Daily SQLite backup for Crossworlds database.
# Keeps 3 rolling copies. Uses sqlite3 .backup for consistency
# (safe even while worldsim is running — uses SQLite's backup API).
#
# Installed as a systemd timer by deploy.sh.
# Backups stored in /opt/worldsim/backups/

set -euo pipefail

DB="/opt/worldsim/data/crossworlds.db"
BACKUP_DIR="/opt/worldsim/backups"
MAX_BACKUPS=3

if [ ! -f "$DB" ]; then
    echo "Database not found: $DB"
    exit 1
fi

mkdir -p "$BACKUP_DIR"

# Create timestamped backup using sqlite3 .backup (atomic, consistent).
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="$BACKUP_DIR/crossworlds-$TIMESTAMP.db"

sqlite3 "$DB" ".backup '$BACKUP_FILE'"

# Verify backup is non-empty and valid.
if [ ! -s "$BACKUP_FILE" ]; then
    echo "Backup file is empty, removing: $BACKUP_FILE"
    rm -f "$BACKUP_FILE"
    exit 1
fi

# Quick integrity check on the backup.
INTEGRITY=$(sqlite3 "$BACKUP_FILE" "PRAGMA integrity_check;" 2>&1 | head -1)
if [ "$INTEGRITY" != "ok" ]; then
    echo "Backup integrity check failed: $INTEGRITY"
    rm -f "$BACKUP_FILE"
    exit 1
fi

SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo "Backup created: $BACKUP_FILE ($SIZE)"

# Remove old backups, keeping only the most recent $MAX_BACKUPS.
cd "$BACKUP_DIR"
ls -1t crossworlds-*.db 2>/dev/null | tail -n +$((MAX_BACKUPS + 1)) | xargs -r rm -f

echo "Backups retained: $(ls -1 crossworlds-*.db 2>/dev/null | wc -l)/$MAX_BACKUPS"

#!/bin/bash
# Daily SQLite backup for Crossworlds database.
# Keeps 2 rolling copies: latest raw (fast restore), previous gzipped (space-efficient).
# Uses sqlite3 .backup for consistency (safe while worldsim runs — SQLite backup API).
#
# Installed as a systemd timer by deploy.sh.
# Backups stored in /opt/worldsim/backups/

set -euo pipefail

DB="/opt/worldsim/data/crossworlds.db"
BACKUP_DIR="/opt/worldsim/backups"

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

# Compress previous raw backups (skip the one we just created).
cd "$BACKUP_DIR"
for old_raw in $(ls -1t crossworlds-*.db 2>/dev/null | tail -n +2); do
    echo "Compressing previous backup: $old_raw"
    gzip "$old_raw"
done

# Remove old gzipped backups, keeping only the most recent one.
ls -1t crossworlds-*.db.gz 2>/dev/null | tail -n +2 | xargs -r rm -f

TOTAL=$(( $(ls -1 crossworlds-*.db 2>/dev/null | wc -l) + $(ls -1 crossworlds-*.db.gz 2>/dev/null | wc -l) ))
echo "Backups retained: $TOTAL (1 raw + 1 gzipped)"

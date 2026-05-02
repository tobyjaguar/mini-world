#!/bin/bash
# Off-server backup uploader (Phase 1.4).
#
# Runs after worldsim-backup.service has written a fresh snapshot to
# /opt/worldsim/backups/. Reads the most recent uncompressed .db, compresses
# it on the fly with gzip, and uploads it to AWS S3.
#
# This script is decoupled from worldsim itself: it reads a finished snapshot
# file and ships bytes to the network. The simulation tick engine never sees
# this work — there is no freeze risk. (See ROADMAP "Phase 1.4 — Backup
# Architecture" for the full reasoning.)
#
# Credentials come from /opt/worldsim/s3-backup.env via systemd
# EnvironmentFile (mode 0600, owner worldsim).
#
# Retention is enforced by an S3 bucket lifecycle rule (configured via
# setup-s3-bucket.sh from the operator's machine), NOT by this script.
# That keeps the upload role write-only and means a server compromise
# cannot wipe history.

set -euo pipefail

BACKUP_DIR="/opt/worldsim/backups"

# Required env (validated up front so failures are obvious in journal).
: "${S3_BUCKET:?S3_BUCKET not set in /opt/worldsim/s3-backup.env}"
: "${AWS_ACCESS_KEY_ID:?AWS_ACCESS_KEY_ID not set}"
: "${AWS_SECRET_ACCESS_KEY:?AWS_SECRET_ACCESS_KEY not set}"

# Defaults if not set in env.
: "${AWS_DEFAULT_REGION:=us-east-1}"
: "${S3_PREFIX:=daily}"
: "${S3_STORAGE_CLASS:=STANDARD_IA}"
export AWS_DEFAULT_REGION

# Pick the freshest local raw backup. The local script (worldsim-backup.sh)
# keeps exactly one raw + one gzipped at any time; we want the raw one.
LATEST=$(ls -1t "$BACKUP_DIR"/crossworlds-*.db 2>/dev/null | head -1 || true)
if [ -z "$LATEST" ] || [ ! -s "$LATEST" ]; then
    echo "No raw backup found in $BACKUP_DIR — did worldsim-backup.service run?"
    exit 1
fi

BASENAME=$(basename "$LATEST" .db)
S3_KEY="${S3_PREFIX}/${BASENAME}.db.gz"

SIZE_RAW_BYTES=$(stat --printf='%s' "$LATEST")
SIZE_RAW_HUMAN=$(numfmt --to=iec --suffix=B "$SIZE_RAW_BYTES" 2>/dev/null || echo "${SIZE_RAW_BYTES} bytes")
echo "Source: $LATEST ($SIZE_RAW_HUMAN raw)"
echo "Target: s3://${S3_BUCKET}/${S3_KEY}  storage_class=${S3_STORAGE_CLASS}  region=${AWS_DEFAULT_REGION}"

# Stream gzip → aws s3 cp. No intermediate file, minimal disk churn.
# --no-progress keeps journal output clean. --sse AES256 declares
# server-side encryption explicitly on every part of the multipart
# upload — required by the bucket's DenyUnencryptedUploads policy
# even though SSE-S3 is the bucket's default encryption.
START=$(date +%s)
gzip -c "$LATEST" | aws s3 cp - "s3://${S3_BUCKET}/${S3_KEY}" \
    --storage-class "$S3_STORAGE_CLASS" \
    --sse AES256 \
    --no-progress
END=$(date +%s)
ELAPSED=$((END - START))

# Probe to confirm the object landed and report its compressed size.
COMPRESSED_BYTES=$(aws s3api head-object \
    --bucket "$S3_BUCKET" \
    --key "$S3_KEY" \
    --query 'ContentLength' \
    --output text 2>/dev/null || echo "?")
if [ "$COMPRESSED_BYTES" != "?" ]; then
    COMPRESSED_HUMAN=$(numfmt --to=iec --suffix=B "$COMPRESSED_BYTES" 2>/dev/null || echo "${COMPRESSED_BYTES} bytes")
    RATIO=$(awk "BEGIN { printf \"%.1f\", ${SIZE_RAW_BYTES} / ${COMPRESSED_BYTES} }")
    echo "Upload complete: ${COMPRESSED_HUMAN} compressed (${RATIO}x ratio), ${ELAPSED}s elapsed"
else
    echo "Upload complete in ${ELAPSED}s (head-object probe failed; may take a moment to be consistent)"
fi

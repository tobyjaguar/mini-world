#!/bin/bash
# One-shot S3 bucket setup for Crossworlds off-server backup (Phase 1.4).
#
# Run this ONCE from your laptop with admin AWS credentials (NOT the
# server's write-only key). Creates the bucket, enables versioning, and
# attaches a lifecycle rule to expire daily backups after 30 days.
#
# After this completes, the server's write-only access key only needs
# `s3:PutObject` on this bucket — the lifecycle rule handles retention,
# so the server never has DeleteObject authority.
#
# Usage:
#     # Set creds (the ones with admin rights on your AWS account, NOT
#     # the worldsim server's restricted key)
#     export AWS_ACCESS_KEY_ID=AKIA...
#     export AWS_SECRET_ACCESS_KEY=...
#     export AWS_DEFAULT_REGION=us-east-1
#
#     # Pick a bucket name (must be globally unique)
#     export S3_BUCKET=my-crossworlds-backups
#
#     ./deploy/setup-s3-bucket.sh
#
# Requires: aws-cli v2 installed locally.

set -euo pipefail

: "${S3_BUCKET:?S3_BUCKET not set — pick a globally-unique name}"
: "${AWS_DEFAULT_REGION:=us-east-1}"
: "${BACKUP_RETENTION_DAYS:=30}"
: "${VERSION_RETENTION_DAYS:=90}"

echo "=== Crossworlds S3 backup bucket setup ==="
echo "Bucket:              s3://${S3_BUCKET}"
echo "Region:              ${AWS_DEFAULT_REGION}"
echo "Object retention:    ${BACKUP_RETENTION_DAYS} days (current versions)"
echo "Version retention:   ${VERSION_RETENTION_DAYS} days (noncurrent versions)"
echo

# 1. Create bucket if it doesn't exist.
if aws s3api head-bucket --bucket "$S3_BUCKET" 2>/dev/null; then
    echo "[1/4] Bucket exists ✓"
else
    echo "[1/4] Creating bucket..."
    if [ "$AWS_DEFAULT_REGION" = "us-east-1" ]; then
        # us-east-1 doesn't accept LocationConstraint
        aws s3api create-bucket --bucket "$S3_BUCKET"
    else
        aws s3api create-bucket \
            --bucket "$S3_BUCKET" \
            --create-bucket-configuration "LocationConstraint=${AWS_DEFAULT_REGION}"
    fi
    echo "    Created."
fi

# 2. Block all public access (defense in depth — backups should never be public).
echo "[2/4] Blocking all public access..."
aws s3api put-public-access-block \
    --bucket "$S3_BUCKET" \
    --public-access-block-configuration "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"
echo "    Done."

# 3. Enable versioning so accidental overwrites are recoverable.
echo "[3/4] Enabling versioning..."
aws s3api put-bucket-versioning \
    --bucket "$S3_BUCKET" \
    --versioning-configuration "Status=Enabled"
echo "    Done."

# 4. Apply lifecycle rule.
#    - Expire current versions after BACKUP_RETENTION_DAYS days
#    - Permanently delete noncurrent versions after VERSION_RETENTION_DAYS days
#    - Abort incomplete multipart uploads after 1 day (housekeeping)
echo "[4/4] Setting lifecycle rule..."
LIFECYCLE_JSON=$(cat <<EOF
{
  "Rules": [
    {
      "ID": "crossworlds-backup-retention",
      "Status": "Enabled",
      "Filter": { "Prefix": "daily/" },
      "Expiration": {
        "Days": ${BACKUP_RETENTION_DAYS}
      },
      "NoncurrentVersionExpiration": {
        "NoncurrentDays": ${VERSION_RETENTION_DAYS}
      },
      "AbortIncompleteMultipartUpload": {
        "DaysAfterInitiation": 1
      }
    }
  ]
}
EOF
)
aws s3api put-bucket-lifecycle-configuration \
    --bucket "$S3_BUCKET" \
    --lifecycle-configuration "$LIFECYCLE_JSON"
echo "    Done."

echo
echo "=== Setup complete ==="
echo
echo "Next steps:"
echo
echo "  1. Create a dedicated IAM user for the server's backup role with"
echo "     this minimum-privilege policy:"
echo
cat <<EOF
       {
         "Version": "2012-10-17",
         "Statement": [
           {
             "Sid": "BackupUploadOnly",
             "Effect": "Allow",
             "Action": ["s3:PutObject"],
             "Resource": "arn:aws:s3:::${S3_BUCKET}/*"
           }
         ]
       }
EOF
echo
echo "  2. Add the user's access key + secret to deploy/config.local:"
echo
echo "       S3_BUCKET=${S3_BUCKET}"
echo "       S3_BACKUP_ACCESS_KEY_ID=AKIA..."
echo "       S3_BACKUP_SECRET_ACCESS_KEY=..."
echo "       AWS_DEFAULT_REGION=${AWS_DEFAULT_REGION}"
echo
echo "  3. Run ./deploy/deploy.sh to install the new timer + service."
echo
echo "  4. After 04:15 UTC the next day (or run \`sudo systemctl start"
echo "     worldsim-backup-s3.service\` to test now), verify the upload:"
echo
echo "       aws s3 ls s3://${S3_BUCKET}/daily/"

#!/bin/bash
# Deploy worldsim to production server.
# Reads connection details from deploy/config.local (gitignored).
# Usage: ./deploy/deploy.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONFIG="$SCRIPT_DIR/config.local"

if [ ! -f "$CONFIG" ]; then
    echo "Error: $CONFIG not found."
    echo "Copy deploy/config.local.example to deploy/config.local and fill in real values."
    exit 1
fi

source "$CONFIG"

SSH_CMD="ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST"
SCP_CMD="scp -i $KEY -o StrictHostKeyChecking=no"

echo "=== Building binary ==="
export PATH=$PATH:/usr/local/go/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
ls -lh build/worldsim

echo "=== Uploading binary ==="
$SCP_CMD build/worldsim $USER@$HOST:/tmp/worldsim

echo "=== Updating environment ==="
if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
    $SSH_CMD "sudo mkdir -p /etc/systemd/system/worldsim.service.d && \
        echo '[Service]' | sudo tee /etc/systemd/system/worldsim.service.d/override.conf > /dev/null && \
        echo 'Environment=WORLDSIM_ADMIN_KEY=${ADMIN_KEY}' | sudo tee -a /etc/systemd/system/worldsim.service.d/override.conf > /dev/null && \
        echo 'Environment=ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}' | sudo tee -a /etc/systemd/system/worldsim.service.d/override.conf > /dev/null && \
        sudo systemctl daemon-reload"
fi

echo "=== Deploying ==="
$SSH_CMD "sudo systemctl stop worldsim || true && \
    sudo mv /tmp/worldsim /opt/worldsim/worldsim && \
    sudo chown worldsim:worldsim /opt/worldsim/worldsim && \
    sudo chmod +x /opt/worldsim/worldsim && \
    sudo systemctl start worldsim"

echo "=== Checking status ==="
sleep 2
$SSH_CMD "sudo systemctl status worldsim --no-pager -l" || true

echo ""
echo "=== Deployment complete ==="
echo "API: http://$HOST/api/v1/status"

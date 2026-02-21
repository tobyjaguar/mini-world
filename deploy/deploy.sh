#!/bin/bash
# Deploy worldsim to DreamCompute instance.
# Usage: ./deploy/deploy.sh

set -euo pipefail

HOST="208.113.165.198"
KEY="$HOME/.ssh/jagkey2.pem"
USER="debian"
SSH="ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST"
SCP="scp -i $KEY -o StrictHostKeyChecking=no"

echo "=== Building binary ==="
export PATH=$PATH:/usr/local/go/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
echo "Binary built: $(ls -lh build/worldsim | awk '{print $5}')"

echo "=== Setting up server ==="
$SSH "sudo useradd -r -s /bin/false worldsim 2>/dev/null || true"
$SSH "sudo mkdir -p /opt/worldsim/data"
$SSH "sudo chown -R worldsim:worldsim /opt/worldsim"

echo "=== Uploading binary ==="
$SCP build/worldsim $USER@$HOST:/tmp/worldsim
$SSH "sudo mv /tmp/worldsim /opt/worldsim/worldsim && sudo chmod +x /opt/worldsim/worldsim && sudo chown worldsim:worldsim /opt/worldsim/worldsim"

echo "=== Installing systemd service ==="
$SCP deploy/worldsim.service $USER@$HOST:/tmp/worldsim.service
$SSH "sudo mv /tmp/worldsim.service /etc/systemd/system/worldsim.service && sudo systemctl daemon-reload"

echo "=== Starting service ==="
$SSH "sudo systemctl enable worldsim && sudo systemctl restart worldsim"

echo "=== Checking status ==="
sleep 2
$SSH "sudo systemctl status worldsim --no-pager -l" || true
echo ""
echo "=== Deployment complete ==="
echo "API: http://$HOST/api/v1/status"
echo "Logs: ssh -i $KEY $USER@$HOST 'sudo journalctl -u worldsim -f'"

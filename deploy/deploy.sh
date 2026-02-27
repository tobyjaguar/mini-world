#!/bin/bash
# Deploy worldsim and gardener to production server.
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

echo "=== Building binaries ==="
export PATH=$PATH:/usr/local/go/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/gardener ./cmd/gardener
ls -lh build/worldsim build/gardener

echo "=== Uploading binaries ==="
$SCP_CMD build/worldsim $USER@$HOST:/tmp/worldsim
$SCP_CMD build/gardener $USER@$HOST:/tmp/gardener

echo "=== Updating environment ==="
# Build systemd override with all env vars.
# Values are double-quoted in the override to handle spaces and commas (e.g. "San Diego,US").
OVERRIDE="[Service]"
OVERRIDE="${OVERRIDE}\nEnvironment=\"WORLDSIM_ADMIN_KEY=${ADMIN_KEY}\""
[ -n "${RELAY_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WORLDSIM_RELAY_KEY=${RELAY_KEY}\""
[ -n "${ANTHROPIC_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}\""
[ -n "${WEATHER_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WEATHER_API_KEY=${WEATHER_API_KEY}\""
[ -n "${WEATHER_LOCATION:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WEATHER_LOCATION=${WEATHER_LOCATION}\""
[ -n "${RANDOM_ORG_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"RANDOM_ORG_API_KEY=${RANDOM_ORG_API_KEY}\""
[ -n "${CORS_ORIGINS:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"CORS_ORIGINS=${CORS_ORIGINS}\""
[ -n "${GOGC:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"GOGC=${GOGC}\""

# Gardener override shares the same keys + gardener-specific config.
GOVERRIDE="[Service]"
GOVERRIDE="${GOVERRIDE}\nEnvironment=\"WORLDSIM_API_URL=http://localhost\""
GOVERRIDE="${GOVERRIDE}\nEnvironment=\"WORLDSIM_ADMIN_KEY=${ADMIN_KEY}\""
[ -n "${ANTHROPIC_API_KEY:-}" ] && GOVERRIDE="${GOVERRIDE}\nEnvironment=\"ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}\""
[ -n "${GARDENER_INTERVAL:-}" ] && GOVERRIDE="${GOVERRIDE}\nEnvironment=\"GARDENER_INTERVAL=${GARDENER_INTERVAL}\""

$SSH_CMD "sudo mkdir -p /etc/systemd/system/worldsim.service.d /etc/systemd/system/gardener.service.d && \
    echo -e '${OVERRIDE}' | sudo tee /etc/systemd/system/worldsim.service.d/override.conf > /dev/null && \
    echo -e '${GOVERRIDE}' | sudo tee /etc/systemd/system/gardener.service.d/override.conf > /dev/null && \
    sudo systemctl daemon-reload"

echo "=== Deploying ==="
# Upload gardener service file if not present.
$SCP_CMD "$SCRIPT_DIR/gardener.service" $USER@$HOST:/tmp/gardener.service
$SSH_CMD "sudo systemctl stop gardener || true && \
    sudo systemctl stop worldsim || true && \
    sudo mv /tmp/worldsim /opt/worldsim/worldsim && \
    sudo mv /tmp/gardener /opt/worldsim/gardener && \
    sudo chown worldsim:worldsim /opt/worldsim/worldsim /opt/worldsim/gardener && \
    sudo chmod +x /opt/worldsim/worldsim /opt/worldsim/gardener && \
    sudo mv /tmp/gardener.service /etc/systemd/system/gardener.service && \
    sudo systemctl daemon-reload && \
    sudo systemctl enable gardener && \
    sudo systemctl start worldsim && \
    sudo systemctl start gardener"

echo "=== Checking status ==="
sleep 2
$SSH_CMD "sudo systemctl status worldsim --no-pager -l" || true
echo ""
$SSH_CMD "sudo systemctl status gardener --no-pager -l" || true

echo ""
echo "=== Deployment complete ==="
echo "API: http://$HOST/api/v1/status"

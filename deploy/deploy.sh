#!/bin/bash
# Deploy worldsim, gardener, and relay to production server.
# All three services run on one host behind nginx reverse proxy.
# Reads connection details from deploy/config.local (gitignored).
# Usage: ./deploy/deploy.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG="$SCRIPT_DIR/config.local"

if [ ! -f "$CONFIG" ]; then
    echo "Error: $CONFIG not found."
    echo "Copy deploy/config.local.example to deploy/config.local and fill in real values."
    exit 1
fi

source "$CONFIG"

# Defaults for port config (can be overridden in config.local).
WORLDSIM_PORT="${WORLDSIM_PORT:-8080}"
RELAY_PORT="${RELAY_PORT:-8081}"

SSH_CMD="ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST"
SCP_CMD="scp -i $KEY -o StrictHostKeyChecking=no"

echo "=== Building binaries ==="
export PATH=$PATH:/usr/local/go/bin

# Worldsim + gardener
cd "$REPO_DIR"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/gardener ./cmd/gardener

# Relay (from sibling repo)
RELAY_DIR="$REPO_DIR/../crossworlds-relay"
if [ ! -d "$RELAY_DIR" ]; then
    echo "Error: crossworlds-relay not found at $RELAY_DIR"
    exit 1
fi
cd "$RELAY_DIR"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "$REPO_DIR/build/relay" .
cd "$REPO_DIR"

ls -lh build/worldsim build/gardener build/relay

echo "=== Uploading binaries ==="
$SCP_CMD build/worldsim $USER@$HOST:/tmp/worldsim
$SCP_CMD build/gardener $USER@$HOST:/tmp/gardener
$SCP_CMD build/relay $USER@$HOST:/tmp/relay

echo "=== Uploading service files and nginx config ==="
$SCP_CMD "$SCRIPT_DIR/worldsim.service" $USER@$HOST:/tmp/worldsim.service
$SCP_CMD "$SCRIPT_DIR/gardener.service" $USER@$HOST:/tmp/gardener.service
$SCP_CMD "$SCRIPT_DIR/relay.service" $USER@$HOST:/tmp/relay.service
$SCP_CMD "$SCRIPT_DIR/nginx-crossworlds.conf" $USER@$HOST:/tmp/nginx-crossworlds.conf

echo "=== Updating environment ==="
# Build systemd override with all env vars.
# Values are double-quoted in the override to handle spaces and commas (e.g. "San Diego,US").
OVERRIDE="[Service]"
OVERRIDE="${OVERRIDE}\nEnvironment=\"WORLDSIM_ADMIN_KEY=${ADMIN_KEY}\""
OVERRIDE="${OVERRIDE}\nEnvironment=\"PORT=${WORLDSIM_PORT}\""
[ -n "${RELAY_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WORLDSIM_RELAY_KEY=${RELAY_KEY}\""
[ -n "${ANTHROPIC_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}\""
[ -n "${WEATHER_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WEATHER_API_KEY=${WEATHER_API_KEY}\""
[ -n "${WEATHER_LOCATION:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"WEATHER_LOCATION=${WEATHER_LOCATION}\""
[ -n "${RANDOM_ORG_API_KEY:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"RANDOM_ORG_API_KEY=${RANDOM_ORG_API_KEY}\""
[ -n "${CORS_ORIGINS:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"CORS_ORIGINS=${CORS_ORIGINS}\""
[ -n "${GOGC:-}" ] && OVERRIDE="${OVERRIDE}\nEnvironment=\"GOGC=${GOGC}\""

# Gardener override — talks to worldsim on its local port.
GOVERRIDE="[Service]"
GOVERRIDE="${GOVERRIDE}\nEnvironment=\"WORLDSIM_API_URL=http://localhost:${WORLDSIM_PORT}\""
GOVERRIDE="${GOVERRIDE}\nEnvironment=\"WORLDSIM_ADMIN_KEY=${ADMIN_KEY}\""
[ -n "${ANTHROPIC_API_KEY:-}" ] && GOVERRIDE="${GOVERRIDE}\nEnvironment=\"ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}\""
[ -n "${GARDENER_INTERVAL:-}" ] && GOVERRIDE="${GOVERRIDE}\nEnvironment=\"GARDENER_INTERVAL=${GARDENER_INTERVAL}\""

# Relay env file.
RELAY_ENV="WORLDSIM_SSE_URL=http://localhost:${WORLDSIM_PORT}/api/v1/stream"
RELAY_ENV="${RELAY_ENV}\nWORLDSIM_RELAY_KEY=${RELAY_KEY}"
RELAY_ENV="${RELAY_ENV}\nPORT=${RELAY_PORT}"
[ -n "${CORS_ORIGINS:-}" ] && RELAY_ENV="${RELAY_ENV}\nCORS_ORIGINS=${CORS_ORIGINS}"

$SSH_CMD "sudo mkdir -p /etc/systemd/system/worldsim.service.d /etc/systemd/system/gardener.service.d && \
    echo -e '${OVERRIDE}' | sudo tee /etc/systemd/system/worldsim.service.d/override.conf > /dev/null && \
    echo -e '${GOVERRIDE}' | sudo tee /etc/systemd/system/gardener.service.d/override.conf > /dev/null && \
    echo -e '${RELAY_ENV}' | sudo tee /opt/worldsim/relay.env > /dev/null && \
    sudo chown worldsim:worldsim /opt/worldsim/relay.env && \
    sudo chmod 600 /opt/worldsim/relay.env && \
    sudo systemctl daemon-reload"

echo "=== Deploying ==="
$SSH_CMD "sudo systemctl stop relay || true && \
    sudo systemctl stop gardener || true && \
    sudo systemctl stop worldsim || true && \
    sudo mv /tmp/worldsim /opt/worldsim/worldsim && \
    sudo mv /tmp/gardener /opt/worldsim/gardener && \
    sudo mv /tmp/relay /opt/worldsim/relay && \
    sudo chown worldsim:worldsim /opt/worldsim/worldsim /opt/worldsim/gardener /opt/worldsim/relay && \
    sudo chmod +x /opt/worldsim/worldsim /opt/worldsim/gardener /opt/worldsim/relay && \
    sudo mv /tmp/worldsim.service /etc/systemd/system/worldsim.service && \
    sudo mv /tmp/gardener.service /etc/systemd/system/gardener.service && \
    sudo mv /tmp/relay.service /etc/systemd/system/relay.service && \
    sudo mv /tmp/nginx-crossworlds.conf /etc/nginx/sites-available/crossworlds && \
    sudo ln -sf /etc/nginx/sites-available/crossworlds /etc/nginx/sites-enabled/crossworlds && \
    sudo rm -f /etc/nginx/sites-enabled/default && \
    sudo nginx -t && \
    sudo systemctl reload nginx && \
    sudo systemctl daemon-reload && \
    sudo systemctl enable worldsim gardener relay && \
    sudo systemctl start worldsim && \
    sudo systemctl start gardener && \
    sudo systemctl start relay"

echo "=== Checking status ==="
sleep 2
$SSH_CMD "sudo systemctl status worldsim --no-pager -l" || true
echo ""
$SSH_CMD "sudo systemctl status gardener --no-pager -l" || true
echo ""
$SSH_CMD "sudo systemctl status relay --no-pager -l" || true
echo ""
$SSH_CMD "sudo systemctl status nginx --no-pager -l" || true

echo ""
echo "=== Deployment complete ==="
echo "Worldsim: http://$HOST/api/v1/status (via nginx)"
echo "Relay:    curl -H 'Host: stream.crossworlds.xyz' http://$HOST/health"

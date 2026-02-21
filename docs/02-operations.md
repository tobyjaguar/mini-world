# Crossroads Operations Guide

## Server

| Field | Value |
|-------|-------|
| Provider | DreamCompute |
| Image | Debian 12 (Bookworm) |
| Flavor | gp1.subsonic (1 vCPU, 1GB RAM, 80GB disk) |
| SSH User | `debian` |

Connection details (IP, SSH key path) are in `deploy/config.local` (gitignored).

## Connecting

```bash
ssh -i <your-key> debian@<server-ip>
```

## Checking In On the World

### Quick status
```bash
curl -s http://<server-ip>/api/v1/status | python3 -m json.tool
```

### See all settlements
```bash
curl -s http://<server-ip>/api/v1/settlements | python3 -m json.tool
```

### See notable characters (Tier 2 agents)
```bash
curl -s http://<server-ip>/api/v1/agents | python3 -m json.tool
```

### Look up a specific agent by ID
```bash
curl -s http://<server-ip>/api/v1/agent/1 | python3 -m json.tool
```

### Recent events
```bash
curl -s http://<server-ip>/api/v1/events?limit=20 | python3 -m json.tool
```

### Aggregate stats
```bash
curl -s http://<server-ip>/api/v1/stats | python3 -m json.tool
```

### Admin: Change simulation speed (requires WORLDSIM_ADMIN_KEY)
```bash
# Speed up to 100x
curl -X POST http://<server-ip>/api/v1/speed \
  -H "Authorization: Bearer <your-admin-key>" \
  -d '{"speed":100}'

# Back to real-time
curl -X POST http://<server-ip>/api/v1/speed \
  -H "Authorization: Bearer <your-admin-key>" \
  -d '{"speed":1}'

# Pause
curl -X POST http://<server-ip>/api/v1/speed \
  -H "Authorization: Bearer <your-admin-key>" \
  -d '{"speed":0}'
```

## API Endpoints

### Public (GET, no auth — anyone can observe the world)
| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/status` | World clock, population, economy summary |
| `GET /api/v1/settlements` | All settlements with governance and health |
| `GET /api/v1/agents` | Notable Tier 2 characters (or `?tier=0` for all) |
| `GET /api/v1/agent/:id` | Full agent detail |
| `GET /api/v1/events` | Recent world events (`?limit=N`) |
| `GET /api/v1/stats` | Aggregate statistics |

### Admin (POST, requires `Authorization: Bearer <key>`)
| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/speed` | Set simulation speed `{"speed": N}` |

Future admin endpoints: `/api/v1/intervention`, `/api/v1/snapshot`, `/api/v1/fork`.

## Server Administration

### Watch logs live
```bash
ssh -i <key> debian@<ip> 'sudo journalctl -u worldsim -f'
```

### View recent logs
```bash
ssh -i <key> debian@<ip> 'sudo journalctl -u worldsim --since "1 hour ago" --no-pager'
```

### Service management
```bash
# Check status
ssh -i <key> debian@<ip> 'sudo systemctl status worldsim'

# Restart
ssh -i <key> debian@<ip> 'sudo systemctl restart worldsim'

# Stop / Start
ssh -i <key> debian@<ip> 'sudo systemctl stop worldsim'
ssh -i <key> debian@<ip> 'sudo systemctl start worldsim'
```

### Check resource usage
```bash
ssh -i <key> debian@<ip> 'free -m && echo "---" && df -h / && echo "---" && ls -lh /opt/worldsim/data/'
```

### Download the database for local inspection
```bash
scp -i <key> debian@<ip>:/opt/worldsim/data/crossroads.db ./crossroads-backup.db
```

## Deploying Updates

```bash
./deploy/deploy.sh
```

This reads `deploy/config.local`, cross-compiles, uploads the binary, and restarts the service.

## Security Posture

| Measure | Status |
|---------|--------|
| Firewall (UFW) | Deny all incoming except ports 22 (SSH) and 80 (HTTP) |
| SSH passwords | Disabled |
| SSH root login | Disabled (`PermitRootLogin no`) |
| SSH max attempts | 3 |
| Brute-force protection | fail2ban: 3 failures → 1 hour ban |
| Auto-updates | unattended-upgrades enabled |
| Swap | 1GB swapfile (prevents OOM on 1GB RAM) |
| API auth | GET endpoints public, POST endpoints require bearer token |

## Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `WORLDSIM_ADMIN_KEY` | Bearer token for admin POST endpoints | Recommended |

Set in the systemd service override:
```bash
sudo systemctl edit worldsim
# Add:
# [Service]
# Environment=WORLDSIM_ADMIN_KEY=your-secret-key
```

## File Locations on Server

| Path | Contents |
|------|----------|
| `/opt/worldsim/worldsim` | The binary |
| `/opt/worldsim/data/crossroads.db` | SQLite world state |
| `/etc/systemd/system/worldsim.service` | systemd service |
| `/etc/fail2ban/jail.local` | fail2ban SSH config |

## Sim Time Reference

At 1x speed (1 tick/second):

| Sim Unit | Real Time |
|----------|-----------|
| 1 sim-hour | 60 seconds |
| 1 sim-day | 24 minutes |
| 1 sim-week | ~2.8 hours |
| 1 sim-season | ~25 hours |
| 1 sim-year | ~4.2 days |

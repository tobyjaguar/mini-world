# Crossworlds Operations Guide

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
| `GET /api/v1/settlement/:id` | Settlement detail: market, agents, factions, events |
| `GET /api/v1/agents` | Notable Tier 2 characters (or `?tier=0` for all) |
| `GET /api/v1/agent/:id` | Full agent detail |
| `GET /api/v1/agent/:id/story` | Haiku-generated biography (`?refresh=true` requires admin auth) |
| `GET /api/v1/events` | Recent world events (`?limit=N`) |
| `GET /api/v1/stats` | Aggregate statistics |
| `GET /api/v1/stats/history` | Time-series stats (`?from=TICK&to=TICK&limit=N`) |
| `GET /api/v1/newspaper` | Haiku-generated newspaper (cached 3 real hours) |
| `GET /api/v1/llm-usage` | LLM call counts and token usage by tag |
| `GET /api/v1/factions` | All factions with influence and treasury |
| `GET /api/v1/faction/:id` | Faction detail: members, influence, events |
| `GET /api/v1/economy` | Economy overview: prices, trade volume, Gini |
| `GET /api/v1/social` | Social network overview |
| `GET /api/v1/map` | Bulk hex data for map rendering |
| `GET /api/v1/map/:q/:r` | Hex detail: terrain, resources, settlement, agents |

### Admin (POST, requires `Authorization: Bearer <key>`)
| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/speed` | Set simulation speed `{"speed": N}` |
| `POST /api/v1/snapshot` | Force immediate world save |
| `POST /api/v1/intervention` | Inject events, adjust wealth, spawn agents |

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
scp -i <key> debian@<ip>:/opt/worldsim/data/crossworlds.db ./crossroads-backup.db
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
| Brute-force protection | fail2ban (systemd backend): 5 failures in 10 min → 1 hour ban |
| Auto-updates | unattended-upgrades enabled |
| Swap | 2 GB total (1 GB `/swapfile` + 1 GB `/swapfile2`, added 2026-05-16 for backup-spike headroom) |
| Go runtime ceiling | `GOMEMLIMIT=1500MiB` (R96, 2026-05-16) — soft heap cap, runtime GCs aggressively before OS OOM |
| API auth | GET endpoints public, POST endpoints require bearer token |

## Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `WORLDSIM_ADMIN_KEY` | Bearer token for admin POST endpoints | Recommended |
| `ANTHROPIC_API_KEY` | Claude Haiku API key for LLM features | Yes |
| `WEATHER_API_KEY` | OpenWeatherMap API key | Yes |
| `WEATHER_LOCATION` | Real-world location for weather mapping | Yes |
| `RANDOM_ORG_API_KEY` | random.org API key for true randomness | Yes |
| `CORS_ORIGINS` | Comma-separated allowed CORS origins | Recommended |
| `GARDENER_INTERVAL` | Gardener cycle interval in real minutes (default 15) | No |
| `NEWSPAPER_CACHE_HOURS` | Newspaper wall-clock cache duration in hours (default 3) | No |

Set in the systemd service override:
```bash
sudo systemctl edit worldsim
# Add:
# [Service]
# Environment=WORLDSIM_ADMIN_KEY=your-secret-key
```

## Storage

| Volume | Mount | Size | Contents |
|--------|-------|------|----------|
| `/dev/vda1` | `/` | 12 GB | OS + binaries + SQLite DB + backups |
| `/dev/vda15` | `/boot/efi` | 124 MB | EFI boot |
| `/swapfile` | swap | 1 GB | original swap |
| `/swapfile2` | swap | 1 GB | added 2026-05-16 (R96) for backup-spike headroom |

Single root volume (12 GB). The historic 20 GB separate data volume layout was consolidated. Filesystem grows automatically on first boot via `x-systemd.growfs`. Disk usage typically sits at 60-65% (~7.3 GB used: SQLite DB ~1.3 GB + WAL ~800 MB + backups dir ~1.7 GB + OS ~3 GB + journald ~200 MB).

### Backup retention (already automatic)

`/opt/worldsim/worldsim-backup.sh` (runs daily at 04:00 UTC) keeps exactly **1 raw + 1 gzipped** backup in `/opt/worldsim/backups/`. Anything older is auto-deleted. The script never accumulates beyond ~1.7 GB total backup footprint.

One-time manual rollback backups (e.g. `crossworlds.db.pre-r52-backup` from R52 rollback) bypass auto-retention and need manual cleanup. Any file in `/opt/worldsim/data/` that isn't `crossworlds.db*` is fair game to delete once the rollback window has passed.

## File Locations on Server

| Path | Contents |
|------|----------|
| `/opt/worldsim/worldsim` | The binary |
| `/opt/worldsim/data/crossworlds.db` | SQLite world state |
| `/opt/worldsim/data/crossworlds.db-wal` | SQLite write-ahead log (can grow to ~800 MB; worldsim manages checkpoints) |
| `/opt/worldsim/backups/` | Daily SQLite backups (1 raw + 1 gzipped, auto-pruned) |
| `/etc/systemd/system/worldsim.service` | systemd service definition |
| `/etc/systemd/system/worldsim.service.d/override.conf` | Drop-in injected by deploy.sh — env vars (GOGC, GOMEMLIMIT, secrets) |
| `/etc/systemd/system/worldsim-backup.timer` | Daily local backup (04:00 UTC) |
| `/etc/systemd/system/worldsim-backup-s3.timer` | Off-server S3 backup — **disabled 2026-05-16** (memory contention) |
| `/etc/fail2ban/jail.local` | fail2ban override (systemd backend for journald-only host) |
| `/swapfile`, `/swapfile2` | Swap files (2 GB total) |

## Host-Level Setup

These are one-time host configurations performed via SSH, **not** automated by `deploy.sh`. Document for reprovisioning or disaster recovery. None of these change over time — the runtime state survives reboots via `/etc/fstab` and systemd unit files.

### Swap configuration

The 2 GB instance needs swap headroom for spikes (daily backup gzip, occasional buff/cache pressure). Original setup is a 1 GB `/swapfile`; **R96 (2026-05-16) added a second 1 GB `/swapfile2`** after the May 16 04:36 UTC OOM kill traced to backup-process contention.

```bash
# Add 1 GB additional swap file (idempotent — skip if already exists)
sudo fallocate -l 1G /swapfile2
sudo chmod 600 /swapfile2
sudo mkswap /swapfile2
sudo swapon /swapfile2

# Persist across reboots
echo "/swapfile2 none swap sw 0 0" | sudo tee -a /etc/fstab
```

Verify: `free -m` should show `Swap: total 2047`. If only 1023, swap setup didn't persist or didn't apply.

### fail2ban (systemd backend)

Debian's default fail2ban tails `/var/log/auth.log`. On systemd-journald-only hosts (this server), that file doesn't exist and fail2ban exits with `ERROR Failed during configuration: Have not found any log file for sshd jail`. Fix is a one-file override telling fail2ban to read from journald.

```bash
sudo tee /etc/fail2ban/jail.local > /dev/null << 'EOF'
# Override Debian file-backend defaults — this host uses journald only.
[DEFAULT]
backend = systemd

[sshd]
enabled = true
backend = systemd
port    = ssh
maxretry = 5
findtime = 10m
bantime  = 1h
EOF

sudo systemctl reset-failed fail2ban
sudo systemctl restart fail2ban
```

Verify: `sudo fail2ban-client status sshd` should show `Journal matches: _SYSTEMD_UNIT=sshd.service + _COMM=sshd` and may already report failed attempts from background SSH scanners (normal — the internet is loud).

### journald size cap (optional, on-demand)

Journald can grow to several hundred MB over time. There's no urgent need for a hard cap, but if disk gets tight, a one-shot vacuum frees the older archived journals:

```bash
sudo journalctl --vacuum-size=200M
```

To make this permanent, edit `/etc/systemd/journald.conf`:
```ini
[Journal]
SystemMaxUse=200M
```
then `sudo systemctl restart systemd-journald`. Not currently enforced — we rely on periodic manual prune if disk pressure rises.

### One-time backup cleanup

The daily `worldsim-backup.sh` retains exactly 1 raw + 1 gzipped backup and auto-deletes anything older — no manual maintenance needed for those. But **manual rollback backups** (e.g. `crossworlds.db.pre-r52-backup` from the R52 rollback exercise) bypass auto-retention and accumulate. Safe to delete once the rollback window has passed:

```bash
sudo rm /opt/worldsim/data/crossworlds.db.pre-r*-backup
```

(Removed 2026-05-17 to recover 1.3 GB; brought disk usage 79% → 65%.)

## Sim Time Reference

At 1x speed (1 tick/second):

| Sim Unit | Real Time |
|----------|-----------|
| 1 sim-hour | 60 seconds |
| 1 sim-day | 24 minutes |
| 1 sim-week | ~2.8 hours |
| 1 sim-season | ~25 hours |
| 1 sim-year | ~4.2 days |

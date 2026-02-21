# Crossroads Operations Guide

## Server

| Field | Value |
|-------|-------|
| Provider | DreamCompute |
| Instance | `crossroads` (efa14cc0-64ad-4b29-9a22-92025cdadb71) |
| Image | Debian 12 (Bookworm) |
| Flavor | gp1.subsonic (1 vCPU, 1GB RAM, 80GB disk) |
| IP | 208.113.165.198 |
| IPv6 | 2607:f298:5:101d:f816:3eff:fe71:b237 |
| SSH Key | `~/.ssh/jagkey2.pem` |
| SSH User | `debian` |

## Connecting

```bash
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198
```

## Checking In On the World

### Quick status
```bash
curl -s http://208.113.165.198/api/v1/status | python3 -m json.tool
```

### See all settlements
```bash
curl -s http://208.113.165.198/api/v1/settlements | python3 -m json.tool
```

### See notable characters (Tier 2 agents)
```bash
curl -s http://208.113.165.198/api/v1/agents | python3 -m json.tool
```

### Look up a specific agent by ID
```bash
curl -s http://208.113.165.198/api/v1/agent/1 | python3 -m json.tool
```

### Recent events
```bash
curl -s http://208.113.165.198/api/v1/events?limit=20 | python3 -m json.tool
```

### Aggregate stats
```bash
curl -s http://208.113.165.198/api/v1/stats | python3 -m json.tool
```

### Change simulation speed
```bash
# Speed up to 100x
curl -X POST http://208.113.165.198/api/v1/speed -d '{"speed":100}'

# Back to real-time
curl -X POST http://208.113.165.198/api/v1/speed -d '{"speed":1}'

# Pause
curl -X POST http://208.113.165.198/api/v1/speed -d '{"speed":0}'
```

## Server Administration

### Watch logs live
```bash
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo journalctl -u worldsim -f'
```

### View recent logs
```bash
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo journalctl -u worldsim --since "1 hour ago" --no-pager'
```

### Service management
```bash
# Check status
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo systemctl status worldsim'

# Restart (will auto-save before stopping, regenerate on start)
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo systemctl restart worldsim'

# Stop
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo systemctl stop worldsim'

# Start
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo systemctl start worldsim'
```

### Check resource usage
```bash
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'free -m && echo "---" && df -h / && echo "---" && ls -lh /opt/worldsim/data/'
```

### Download the database for local inspection
```bash
scp -i ~/.ssh/jagkey2.pem debian@208.113.165.198:/opt/worldsim/data/crossroads.db ./crossroads-backup.db
```

## Deploying Updates

From the project directory:

```bash
# Rebuild and deploy
export PATH=$PATH:/usr/local/go/bin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
scp -i ~/.ssh/jagkey2.pem build/worldsim debian@208.113.165.198:/tmp/worldsim
ssh -i ~/.ssh/jagkey2.pem debian@208.113.165.198 'sudo systemctl stop worldsim && sudo mv /tmp/worldsim /opt/worldsim/worldsim && sudo chown worldsim:worldsim /opt/worldsim/worldsim && sudo chmod +x /opt/worldsim/worldsim && sudo systemctl start worldsim'
```

Or use the deploy script:
```bash
./deploy/deploy.sh
```

## Security Posture

### Firewall (UFW)
- **Default**: deny incoming, allow outgoing
- **Port 22/tcp**: SSH (key-only, no passwords)
- **Port 80/tcp**: HTTP (worldsim API)
- All other ports blocked

### SSH Hardening
- Password authentication: **disabled**
- Root login: **disabled** (`PermitRootLogin no`)
- Max auth tries: **3**
- Key: `jagkey2` (ED25519)

### Intrusion Prevention
- **fail2ban**: Monitors SSH, bans IPs after 3 failed attempts for 1 hour
- Backend: systemd/journald

### Automatic Updates
- **unattended-upgrades**: Installed, applies security patches automatically

### Swap
- 1GB swap file at `/swapfile` (persistent via fstab)
- Prevents OOM kills on the 1GB RAM instance

## File Locations on Server

| Path | Contents |
|------|----------|
| `/opt/worldsim/worldsim` | The binary |
| `/opt/worldsim/data/crossroads.db` | SQLite world state |
| `/etc/systemd/system/worldsim.service` | systemd service |
| `/etc/fail2ban/jail.local` | fail2ban SSH config |

## Sim Time Reference

At 1x speed (1 tick/second):
- 1 sim-hour = 60 real seconds
- 1 sim-day = 24 real minutes
- 1 sim-week = ~2.8 real hours
- 1 sim-season = ~25 real hours
- 1 sim-year = ~4.2 real days

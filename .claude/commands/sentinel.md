# Sentinel Health Check — Structural Health Monitor

Read the latest sentinel report from the production server and present a structured health summary.

## Data Collection

SSH to the server using credentials from `deploy/config.local` and fetch:

### 1. Latest sentinel report
```bash
source deploy/config.local
ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST "cat /opt/worldsim/sentinel_report.json"
```

### 2. Recent sentinel journal logs (last 2 cycles)
```bash
source deploy/config.local
ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST "sudo journalctl -u sentinel --no-pager -n 80 --output=short-iso"
```

### 3. Sentinel state (ring buffer depth and alert history)
```bash
source deploy/config.local
ssh -i $KEY -o StrictHostKeyChecking=no $USER@$HOST "cat /opt/worldsim/sentinel_state.json | python3 -c \"
import json, sys
s = json.load(sys.stdin)
print(f'Cycle: {s.get(\"cycle_num\", 0)}')
print(f'Snapshots: {len(s.get(\"snapshots\", []))}')
for name, alert in s.get('alerts', {}).items():
    print(f'  {name}: level={alert[\"last_level\"]}, last_alert=cycle {alert[\"last_alert_at\"]}')
\""
```

## Presentation Format

Present the results as:

### 1. Overall Status
One-line summary: tick, sim time, overall health level (HEALTHY/WATCH/WARNING/CRITICAL), cycle number.

### 2. Health Check Table
Table with all 8 checks:

| Check | Status | Value | Trend | Detail |
|-------|--------|-------|-------|--------|

Color-code by severity in the assessment:
- HEALTHY — working as expected
- WATCH — worth monitoring, not yet concerning
- WARNING — structural issue developing, may need intervention
- CRITICAL — active crisis, likely needs a tuning fix

### 3. Active Alerts
If any alerts fired this cycle, show them with from→to transitions and what changed.

### 4. Trend Summary
Based on the ring buffer depth and trend directions, note:
- Which checks are improving (getting better over time)
- Which checks are declining (getting worse over time)
- How many snapshots are in the buffer (context for trend reliability)

### 5. Recommendations
Based on the check results, recommend:
- If all HEALTHY: "World is structurally sound. No action needed."
- If any WATCH: "Monitor these checks. Consider running `/observe` for deeper analysis if trends continue."
- If any WARNING: "Structural issue detected. Run `/observe` to diagnose root cause and plan a tuning fix."
- If any CRITICAL: "Active crisis. Immediate `/observe` and tuning intervention recommended."

## Important Notes

- The sentinel report is a snapshot of the last 30-min cycle. It may be up to 30 minutes stale.
- Sentinel checks structural health (patterns that needed code changes across 29 rounds). For tactical issues (specific settlement needs, individual agent problems), use `/observe`.
- If the sentinel service isn't running or the report file doesn't exist, note this and suggest checking `sudo systemctl status sentinel` on the server.
- All sentinel thresholds derive from Φ constants — reference the specific constant when explaining thresholds (e.g., "crafter share exceeds Matter (61.8%)").

# Round 24 Deployment Playbook — Occupation Persistence & Resource-Seeking Migration

**Deployed:** 2026-03-01 (tick ~634K)
**Commit:** `d90bae9` (mini-world)
**Pre-deploy commit:** `98c9c0c` (safe revert target)

## What Changed

82% of agents were Crafters, only 0.26% resource producers (726). Every code path that handled resource depletion or agent movement converted producers into Crafters. Round 24 disables all forced reassignment and adds occupation-preserving alternatives.

**7 changes across 9 files (fixes 119-125):**

| Phase | What | File(s) | Weekly function |
|-------|------|---------|----------------|
| 1 | Disable forced reassignment | `perpetuation.go`, `population.go` | — |
| 2 | LastWorkTick tracking | `types.go`, `production.go`, `db.go` | — |
| 3 | Resource-seeking migration | `perpetuation.go` | `processResourceMigration()` |
| 4 | Crafter recovery | `perpetuation.go` | `processCrafterRecovery()` |
| 5 | Career transition | `perpetuation.go` | `processCareerTransition()` |
| 6 | Tier 2 relocate/retrain | `cognition.go` (engine + llm) | — (daily Tier 2 decisions) |
| 7 | Oracle guide_migration | `cognition.go` (engine), `oracle.go` (llm) | — (weekly oracle visions) |

## Monitoring

### What to Check

Run `/observe` or query the API:

```bash
# Occupation distribution
curl -s https://api.crossworlds.xyz/api/v1/status

# Recent events — look for migration/retraining
curl -s 'https://api.crossworlds.xyz/api/v1/events?limit=100' | grep -i 'migrat\|retrain\|transition\|guide'

# Tier 2 agents — check occupation diversity
curl -s https://api.crossworlds.xyz/api/v1/agents
```

### Expected Timeline

| When | What to see |
|------|------------|
| Immediately | No more "reassigned occupation" / "rebalanced producers" log messages |
| Day 1 | LastWorkTick populating for active producers |
| Week 1 | First "migrated to X seeking farmland" events |
| Week 2 | First "begins retraining as a Farmer" events (crafter recovery) |
| Week 4 | Crafters declining from 82% toward 60-70% |
| Month 2 | Occupation equilibrium: ~30% producers, ~20% crafters, ~50% services |

### Failure Indicators — When to Act

| Indicator | Threshold | Action |
|-----------|-----------|--------|
| Satisfaction crash | Drops >0.15 below pre-deploy (0.690) | Disable Phases 3-5 |
| D:B ratio spike | Rises above 0.1 | Disable Phases 3-5 |
| Mass settlement depopulation | >20 settlements drop below 25 pop in one week | Disable Phase 3 (migration) |
| Crafter recovery too fast | >5% occupation shift per week | Disable Phase 4 (crafter recovery) |
| Trade volume collapse | Drops >40% | Investigate — may need temporary pause |
| Occupation oscillation | Agents flip between occupations repeatedly | Disable Phase 5 (career transition) |

## Rollback Instructions

### Per-Phase Disable (Quickest — Comment Out + Redeploy)

**Phase 3 — Resource Migration:**
```go
// File: internal/engine/simulation.go, in TickWeek()
// Comment out:
s.processResourceMigration(tick)
```

**Phase 4 — Crafter Recovery:**
```go
// File: internal/engine/simulation.go, in TickWeek()
// Comment out:
s.processCrafterRecovery(tick)
```

**Phase 5 — Career Transition:**
```go
// File: internal/engine/simulation.go, in TickWeek()
// Comment out:
s.processCareerTransition(tick)
```

**Phase 6 — Tier 2 relocate/retrain:**
```go
// File: internal/llm/cognition.go, in parseTier2Response()
// Remove "relocate" and "retrain" from validActions map
```

**Phase 7 — Oracle guide_migration:**
```go
// File: internal/llm/oracle.go, in parseOracleResponse()
// Remove "guide_migration" from validActions map
```

After any change: `cd mini-world && ./deploy/deploy.sh`

### Phase 1 Re-enable (Nuclear Option — Restore Forced Reassignment)

Only if occupation distribution goes completely wrong and you need the old behavior back:

```go
// File: internal/engine/perpetuation.go, in processAntiStagnation()
// Uncomment these two lines:
s.rebalanceSettlementProducers(tick)
s.reassignMismatchedProducers(tick)

// File: internal/engine/perpetuation.go
// Restore reassignIfMismatched() body from commit 98c9c0c

// File: internal/engine/population.go, in processBirths()
// Restore birth-time producer gate from commit 98c9c0c
```

### Full Revert (Git Reset)

```bash
cd mini-world
git revert d90bae9  # Creates a new revert commit
./deploy/deploy.sh
```

**Note:** The `last_work_tick` DB column will remain (harmless — defaults to 0, ignored if functions disabled). No schema rollback needed.

## Monitoring Schedule

- **Week 1:** Daily health checks (highest risk — first weekly tick fires Round 24 functions)
- **Weeks 2-4:** Every 2-3 days (crafter recovery ramping up)
- **Month 2:** Weekly checks until equilibrium confirmed
- **Declare success:** When crafter share < 50% and satisfaction stable for 2+ weeks

## Server Check Commands

```bash
# SSH to server
ssh -i <key> debian@<host>

# Check worldsim logs for Round 24 activity
sudo journalctl -u worldsim --since "1 hour ago" | grep -i 'resource migration\|crafter recovery\|career transition'

# Check for errors
sudo journalctl -u worldsim --since "1 hour ago" | grep -i 'error\|panic'

# Memory usage
sudo systemctl status worldsim | grep Memory
```

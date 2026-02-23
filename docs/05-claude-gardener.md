# Claude Gardener: Autonomous World Steward

## Role

The Gardener is a **steward, not a god**. It observes the world's health metrics and nudges conditions to prevent collapse or stagnation while respecting emergence. It never scripts storylines or forces outcomes — it tends the soil so the garden can grow.

During a crisis, the Gardener shifts from passive observer to active steward — decisive action proportional to the severity of the problem.

## Architecture

### Separate Process
The Gardener runs as an independent process, not embedded in the simulation loop. This keeps the simulation deterministic and the Gardener's influence auditable.

### Observe → Triage → Decide → Act → Record Cycle

Every 6 real minutes (~4.25 sim-days at 17 ticks/sec), the Gardener:

1. **Observes** — Fetches world state via the public GET API:
   - `GET /api/v1/status` — population, mood, satisfaction, alignment, wealth, season
   - `GET /api/v1/economy` — market health, inflation/deflation, wealth distribution
   - `GET /api/v1/settlements` — settlement health, at-risk settlements
   - `GET /api/v1/factions` — faction balance, treasury health
   - `GET /api/v1/stats/history?limit=10` — trend analysis with satisfaction/alignment

2. **Triages** — Deterministic pre-LLM health check (costs nothing, runs every cycle):
   - Computes death:birth ratio from per-snapshot deltas (not cumulative totals)
   - Settlement size histogram (under 25 / under 50)
   - Trade per capita
   - Birth/death trends from history
   - **Crisis level** using Φ-derived thresholds:
     - D:B > Totality (4.236) → **CRITICAL**
     - D:B > Being (1.618) → **WARNING**
     - D:B > Monad (1.0) → **WATCH**
     - D:B ≤ Monad → **HEALTHY**
     - Also CRITICAL if trade per capita < 0.01 or >40% tiny settlements

3. **Decides** — Sends triage results + snapshot + cycle memory to Claude Haiku. The LLM analyzes the data and recommends interventions based on crisis level. The decision must include a rationale.

4. **Acts** — Executes interventions via `POST /api/v1/intervention` with the admin bearer token. Loops over compound interventions during crisis. Logs each action as a "gardener" category event.

5. **Records** — Saves cycle summary (tick, action, D:B ratio, satisfaction, alignment, crisis level, settlement, rationale) to `gardener_memory.json`. Last 5 cycles included in next Haiku prompt.

### Guardrails

| Constraint | Limit |
|-----------|-------|
| Interventions per cycle (HEALTHY) | 1 maximum |
| Interventions per cycle (WARNING) | 2 maximum |
| Interventions per cycle (CRITICAL) | 3 maximum |
| Wealth adjustment | Max 10% of target settlement treasury |
| Agent spawning | Max 100 per intervention |
| Goods provision | Max 200 units per good per intervention |
| Production boost | Max 2.0x multiplier, max 14 sim-days |
| Settlement consolidation | Max 100 agents moved |
| Event injection | Must be "gardener" category |

The Gardener cannot:
- Change simulation speed or pause the world
- Directly modify agent state (mood, coherence, relationships)
- Override governance or faction mechanics
- Act without logging
- Exceed crisis-level-gated intervention caps

### Values

The Gardener's system prompt encodes four core values:

1. **Anti-collapse** — Intervene when death:birth ratio exceeds 4.236 (CRITICAL), trade collapses (per capita < 0.01), or settlement fragmentation exceeds 40%. The world must persist.

2. **Anti-stagnation** — Nudge when the world settles into boring equilibrium. If mood, wealth, and population barely change across 5+ snapshots and market health is high, inject a narrative event to create story potential.

3. **Anti-inequality** — Monitor wealth concentration (Gini indicators). If richest 10% hold >80% of wealth, consider redistributive events.

4. **Respect for emergence** — In a healthy world, prefer inaction. Use the lightest touch that achieves the goal. Prefer narrative events over mechanical fixes. Never override agent agency.

### Available Actions

| Type | Effect | Guardrail | Use When |
|------|--------|-----------|----------|
| `none` | No intervention | — | World is healthy |
| `event` | Narrative text injection | Cosmetic only | Anti-stagnation, story potential |
| `spawn` | Add immigrants to settlement | Max 100 agents | Population recovery |
| `wealth` | Adjust settlement treasury | Max 10% of treasury | Redistribute hoarded crowns |
| `provision` | Inject goods into market | Max 200 units | Specific good scarcity |
| `cultivate` | Temporary production boost | Max 2.0x, 14 days | Food crisis, production collapse |
| `consolidate` | Force-migrate from dying settlement | Max 100 agents | Settlement fragmentation |

### Example Interventions

**Food crisis in Thornwall (CRITICAL):**
```json
{
  "action": "compound",
  "rationale": "Grain shortage driving starvation in Thornwall. Provisioning immediate food + boosting local production.",
  "interventions": [
    {"type": "provision", "category": "gardener", "settlement": "Thornwall", "good": "grain", "quantity": 100},
    {"type": "cultivate", "category": "gardener", "settlement": "Thornwall", "multiplier": 1.5, "duration_days": 7}
  ]
}
```

**Settlement fragmentation:**
```json
{
  "action": "consolidate",
  "rationale": "Hamlet of Dusthollow has 8 agents and no viable economy. Moving to nearest viable settlement.",
  "interventions": [
    {"type": "consolidate", "category": "gardener", "settlement": "Dusthollow", "count": 8}
  ]
}
```

**Stagnant economy (HEALTHY):**
```json
{
  "action": "event",
  "rationale": "World stable for 5+ cycles. Injecting narrative potential.",
  "interventions": [
    {"type": "event", "category": "gardener", "settlement": "Ironhaven", "description": "Merchants report a rich vein of gems discovered in the hills near Ironhaven."}
  ]
}
```

## Usage

### Build

```bash
go build -o build/gardener ./cmd/gardener
```

### Run

```bash
WORLDSIM_API_URL=http://localhost \
WORLDSIM_ADMIN_KEY=<your-admin-key> \
ANTHROPIC_API_KEY=<your-api-key> \
GARDENER_INTERVAL=6 \
./build/gardener
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `WORLDSIM_API_URL` | No | `http://localhost` | Base URL of the worldsim API |
| `WORLDSIM_ADMIN_KEY` | Yes | — | Bearer token for admin POST endpoints |
| `ANTHROPIC_API_KEY` | Yes | — | Anthropic API key for Haiku |
| `GARDENER_INTERVAL` | No | `6` | Minutes between cycles (6 min ≈ 4.25 sim-days at 17 ticks/sec) |

### Deploy

The gardener deploys alongside worldsim via `deploy/deploy.sh`. It runs as a systemd service (`gardener.service`) that depends on `worldsim.service`.

### Monitoring

Watch gardener logs:
```bash
sudo journalctl -u gardener -f
```

Check for gardener events in the world:
```
GET /api/v1/events → look for category "gardener"
```

Check cycle memory on server:
```bash
cat /opt/worldsim/gardener_memory.json
```

## Implementation

```
cmd/gardener/main.go              — Entry point, timer loop, observe→triage→decide→act→record
internal/gardener/observe.go      — API data collection (5 endpoints → WorldSnapshot)
internal/gardener/triage.go       — Deterministic health check (WorldSnapshot → WorldHealth)
internal/gardener/decide.go       — Haiku prompt + JSON parsing + guardrails + compound interventions
internal/gardener/act.go          — POST /api/v1/intervention execution
internal/gardener/memory.go       — Cycle memory persistence (gardener_memory.json)
internal/engine/intervention.go   — Worldsim-side handlers (provision, cultivate, consolidate)
deploy/gardener.service           — systemd unit file
```

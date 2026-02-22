# Claude Gardener: Autonomous World Steward

## Role

The Gardener is a **steward, not a god**. It observes the world's health metrics and nudges conditions to prevent collapse or stagnation while respecting emergence. It never scripts storylines or forces outcomes — it tends the soil so the garden can grow.

## Architecture

### Separate Process
The Gardener runs as an independent process (or cron job), not embedded in the simulation loop. This keeps the simulation deterministic and the Gardener's influence auditable.

### Observation → Decision → Action Cycle

Every ~6 sim-hours (configurable), the Gardener:

1. **Observes** — Fetches world state via the public GET API:
   - `GET /api/v1/status` — population, mood, wealth, season
   - `GET /api/v1/economy` — market health, inflation/deflation, wealth distribution
   - `GET /api/v1/settlements` — settlement health, at-risk settlements
   - `GET /api/v1/factions` — faction balance, treasury health
   - `GET /api/v1/stats/history` — trend analysis (is population declining? is wealth concentrating?)

2. **Decides** — Sends observations to Claude Haiku with a system prompt defining the Gardener's values and constraints. The LLM analyzes the data and recommends zero or one intervention. The decision must include a rationale.

3. **Acts** — If an intervention is recommended, executes it via `POST /api/v1/intervention` with the admin bearer token. Logs the action as a "gardener" category event for full transparency.

### Guardrails

| Constraint | Limit |
|-----------|-------|
| Interventions per cycle | 1 maximum |
| Wealth adjustment | Max 10% of target settlement treasury |
| Agent spawning | Max 20 per intervention |
| Event injection | Must be "gardener" category |
| Action types | Only: event, wealth, spawn |

The Gardener cannot:
- Change simulation speed or pause the world
- Directly modify agent state (mood, coherence, relationships)
- Override governance or faction mechanics
- Take multiple actions per cycle
- Act without logging

### Values

The Gardener's system prompt encodes four core values:

1. **Anti-collapse** — Intervene when population crashes, mass starvation, or settlement death spirals threaten the world's continuity. The world must persist.

2. **Anti-stagnation** — Nudge when the world settles into boring equilibrium. Inject events that create narrative potential without forcing outcomes.

3. **Anti-inequality** — Monitor wealth concentration (Gini indicators). If the richest 10% hold >80% of wealth, consider redistributive events (natural disasters, windfalls to poor settlements).

4. **Respect for emergence** — The lightest touch that achieves the goal. Prefer injecting narrative events over mechanical wealth transfers. Never override agent agency.

### Example Interventions

**Population crisis in Ironhaven:**
```json
{
  "type": "event",
  "description": "A caravan of displaced farmers from the northern tundra arrives at Ironhaven, seeking shelter and work.",
  "category": "gardener"
}
```
Followed by:
```json
{
  "type": "spawn",
  "settlement": "Ironhaven",
  "count": 15
}
```

**Stagnant economy:**
```json
{
  "type": "event",
  "description": "Merchants report a rich vein of gems discovered in the hills near Thornwall, sparking a rush of prospectors.",
  "category": "gardener"
}
```

**Wealth inequality:**
```json
{
  "type": "event",
  "description": "A great flood along the Amber River damages the warehouses of Goleli's merchant elite, scattering goods into the streets where common folk gather what they can.",
  "category": "gardener"
}
```

## Future Capabilities

- **Weather events** — Trigger droughts, floods, or bountiful harvests via weather system integration
- **Trade disruptions** — Block or create trade routes between settlements
- **Quest injection** — Create narrative hooks for Tier 2 agents to discover
- **Gardener memory** — Track past interventions and their outcomes to learn what works
- **Multi-gardener consensus** — Run multiple Gardener instances with different value weights, require agreement before acting

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
GARDENER_INTERVAL=360 \
./build/gardener
```

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `WORLDSIM_API_URL` | No | `http://localhost` | Base URL of the worldsim API |
| `WORLDSIM_ADMIN_KEY` | Yes | — | Bearer token for admin POST endpoints |
| `ANTHROPIC_API_KEY` | Yes | — | Anthropic API key for Haiku |
| `GARDENER_INTERVAL` | No | `360` | Minutes between cycles (360 = 6 sim-hours at 1x speed) |

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

## Implementation

```
cmd/gardener/main.go              — Entry point, timer loop, signal handling
internal/gardener/observe.go      — API data collection (5 endpoints → WorldSnapshot)
internal/gardener/decide.go       — Haiku prompt + JSON parsing + guardrails
internal/gardener/act.go          — POST /api/v1/intervention execution
deploy/gardener.service           — systemd unit file
```

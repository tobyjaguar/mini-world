# Mini-World (SYNTHESIS / Crossroads): Autonomous Simulated World

## Project Overview

An autonomous, persistent simulated world where agents live, interact, make decisions, and generate emergent narratives. The simulation runs continuously on a cloud server, and users can check in periodically to see what has unfolded. This is a petri dish, not a game — emergence over scripting.

The full design specification lives in `docs/worldsim-design.md` (~1,500 lines, 16 sections). That is the single source of truth for architecture. Section 16 (Wheeler Emanationist Cosmology) provides the mathematical backbone — all tuning constants derive from the golden ratio (Φ).

## Language: Go

**Go** was chosen for:
- Built-in HTTP server, JSON marshaling, goroutines — minimal boilerplate for APIs
- Compiled, statically typed, feels close to C — good for systems thinking
- Memory safe — critical for a 24/7 long-running server process
- Fast development velocity — focus energy on simulation logic, not plumbing

## Architecture

### Hybrid Agent Intelligence (Tiered Cognition)
- **Tier 0 (95% of agents)**: Pure rule-based state machines driven by needs
- **Tier 1 (4%)**: Archetype-guided — Haiku generates behavioral templates per archetype weekly
- **Tier 2 (<1%, ~20-50 named characters)**: Individual LLM-powered decisions via Haiku API
- **External entropy**: Real weather data via API, true randomness from random.org

### Core Components
- **Tick Engine**: Layered tick schedule (minute/hour/day/week/season)
- **World State**: Hex grid (~2,000 hexes), terrain, resources, settlements
- **Agent System**: Needs-driven entities with coherence model (Section 16.2)
- **Economy Engine**: Supply/demand price discovery, trade routes, sinks/faucets
- **Event Journal**: Append-only log, news generation, newspaper endpoint
- **HTTP API**: Query interface for checking in on the world

### Key Design Principles
1. Emergence over scripting — never hard-code storylines
2. Economic robustness as the heartbeat — if the economy works, everything follows
3. Observability is first-class — rich event logging, newspaper endpoint
4. Perpetuation by design — anti-collapse safeguards, balanced sinks/faucets
5. All constants from Φ — no arbitrary magic numbers (see EmanationConstants)

## Development Conventions

- Keep simulation logic clean and separated from API/IO concerns
- All agent decisions should be deterministic given the same inputs (for replay/debugging)
- Event log is append-only and human-readable
- Prefer simple data structures; avoid over-abstraction
- Test simulation logic independently from external API calls
- Use structured logging (Go `slog` package) for debugging simulation behavior
- Derive tuning constants from Φ (EmanationConstants) — no magic numbers

## External Dependencies

- **Claude API** (Haiku model `claude-haiku-4-5-20251001`): Agent cognition, newspaper generation
- **Weather API** (OpenWeatherMap or similar): Real weather → in-world weather
- **random.org API**: True randomness for critical stochastic events

## Repository

**GitHub**: https://github.com/tobyjaguar/mini-world — **public research project**

Secrets and connection details live in `deploy/config.local` (gitignored). Copy `deploy/config.local.example` to get started.

### Public Repo Policy

This is a public repository. **Never commit:**
- API keys, tokens, passwords, or any credentials
- Server IP addresses, SSH keys, or connection details
- Personal information (real names, emails, accounts)
- Contents of `deploy/config.local` or any `.env` / secret files
- Proprietary or third-party confidential material

All sensitive values belong in `deploy/config.local` (gitignored) or environment variables. When writing code, documentation, or commit messages, use placeholders (`<server-ip>`, `<your-key>`) instead of real values. When in doubt, leave it out.

## Directory Structure

```
mini-world/
├── CLAUDE.md                    # This file — project guide
├── docs/
│   ├── worldsim-design.md       # Complete design spec (source of truth)
│   ├── CLAUDE_CODE_PROMPT.md    # Implementation guide
│   ├── 00-project-vision.md     # Project vision and design pillars
│   ├── 01-language-decision.md  # Go language rationale
│   ├── 02-operations.md         # Server ops, API reference, security
│   └── 03-next-steps.md         # Phase 2+ roadmap and priorities
├── cmd/worldsim/
│   └── main.go                  # Entry point
├── internal/
│   ├── phi/                     # Emanation constants (Φ-derived)
│   │   ├── constants.go         #   Golden ratio powers, growth angle
│   │   └── field.go             #   ConjugateField interface
│   ├── world/                   # Hex grid, terrain, map generation
│   │   ├── hex.go               #   HexCoord, terrain types, neighbors
│   │   ├── map.go               #   Map container
│   │   ├── generation.go        #   Simplex noise world generation
│   │   └── settlement_placer.go #   Settlement scoring and placement
│   ├── agents/                  # Agent types, needs, cognition tiers
│   │   ├── types.go             #   Agent struct, goods, skills, relationships
│   │   ├── soul.go              #   Wheeler coherence model
│   │   ├── needs.go             #   Maslow needs hierarchy
│   │   ├── behavior.go          #   Tier 0 state machine
│   │   └── spawner.go           #   Population generation, Tier 2 promotion
│   ├── economy/goods.go         # Good types and market mechanics
│   ├── social/settlement.go     # Settlement type and governance
│   ├── events/                  # Event detection (placeholder)
│   ├── engine/                  # Tick engine, simulation loop
│   │   ├── tick.go              #   Layered tick schedule, sim time
│   │   ├── simulation.go        #   World state, tick callbacks, stats
│   │   ├── production.go        #   Resource-based production, hex depletion
│   │   ├── market.go            #   Market resolution, trade, taxes, merchants
│   │   ├── factions.go          #   Faction dynamics, influence, dues, policies
│   │   ├── population.go        #   Births, aging, death, migration
│   │   ├── settlement_lifecycle.go # Overmass diaspora, founding, abandonment
│   │   └── seasons.go           #   Seasonal resource caps, weather modifiers
│   ├── llm/                     # LLM integration (Haiku)
│   │   ├── client.go            #   Anthropic API client
│   │   ├── narration.go         #   Event narration, newspaper, archetypes
│   │   └── biography.go         #   Agent biography generation
│   ├── weather/client.go        # OpenWeatherMap integration
│   ├── entropy/client.go        # random.org true randomness
│   ├── persistence/db.go        # SQLite save/load (WAL mode), stats history
│   └── api/server.go            # HTTP API (public GET, auth POST)
├── deploy/
│   ├── deploy.sh                # Build, upload, restart
│   ├── worldsim.service         # systemd unit file
│   ├── config.local.example     # Template for connection details
│   └── config.local             # Real values (gitignored)
├── data/                        # Runtime SQLite DB (gitignored)
├── build/                       # Compiled binaries (gitignored)
├── go.mod
└── go.sum
```

## Build & Run

```bash
# Local development
go build -o worldsim ./cmd/worldsim
./worldsim

# Cross-compile for server
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim

# Deploy to production
./deploy/deploy.sh
```

## Production Deployment

The world runs 24/7 on a DreamCompute instance. See `docs/02-operations.md` for full details.

| Field | Value |
|-------|-------|
| Server | See `deploy/config.local` (DreamCompute, Debian 12, 1GB RAM) |
| API | `http://<server-ip>/api/v1/status` |
| SSH | `ssh -i <your-key> debian@<server-ip>` |
| Service | systemd `worldsim.service`, auto-restarts, starts on boot |
| Database | `/opt/worldsim/data/crossroads.db` (SQLite, auto-saves daily) |
| Security | UFW (ports 22+80 only), fail2ban, no root login, no passwords |
| Swap | 1GB at `/swapfile` |

### API Endpoints

Public (GET, no auth — anyone can observe the world):
```
GET  /api/v1/status          → World clock, population, economy summary
GET  /api/v1/settlements     → All settlements with governance and health
GET  /api/v1/settlement/:id  → Settlement detail: market, agents, factions, events
GET  /api/v1/agents          → Notable Tier 2 characters (default) or ?tier=0
GET  /api/v1/agent/:id       → Full agent detail
GET  /api/v1/agent/:id/story → Haiku-generated biography (?refresh=true to regenerate)
GET  /api/v1/events          → Recent world events (?limit=N)
GET  /api/v1/stats           → Aggregate statistics
GET  /api/v1/stats/history   → Time-series stats (?from=TICK&to=TICK&limit=N)
GET  /api/v1/newspaper       → Weekly Haiku-generated newspaper
GET  /api/v1/factions        → All factions with influence and treasury
GET  /api/v1/faction/:id     → Faction detail: members, influence, events
GET  /api/v1/economy         → Economy overview: prices, trade volume, Gini
GET  /api/v1/social          → Social network overview
GET  /api/v1/map/:q/:r       → Hex detail: terrain, resources, settlement, agents
```

Admin (POST, requires `Authorization: Bearer <WORLDSIM_ADMIN_KEY>`):
```
POST /api/v1/speed           → Set simulation speed {"speed": N}
POST /api/v1/snapshot        → Force immediate world save
POST /api/v1/intervention    → Inject events, adjust wealth, spawn agents
```

## Implementation Phases

1. **Foundation (MVP)** — COMPLETE: Hex grid, Tier 0 agents, tick engine, SQLite, HTTP API, deployed
2. **Economy & Trade** — COMPLETE: Multi-settlement trade, merchants, price discovery, crafting recipes, goods decay, seasonal price modifiers, economic circuit breaker, tax collection
3. **Social & Political** — COMPLETE: 5 factions with per-settlement influence, 4 governance types, leader succession, revolution mechanics, relationships (family/mentorship/rivalry), crime/theft
4. **LLM Integration** — COMPLETE: Haiku API client, Tier 2 cognition, Tier 1 archetypes, newspaper generation, event narration, agent biographies
5. **Polish & Perpetuation** — COMPLETE: Population dynamics (births/aging/death/migration), resource regen, anti-stagnation, settlement lifecycle (founding/abandonment), stats history, admin endpoints, random.org entropy, weather integration

## Known Tuning Issues (from live world observations)

These are diagnosed root causes from observing the live world. They don't require new systems — they're bugs and balance adjustments in existing mechanics.

### 1. Fisher Mood Bug (Critical)
**Symptom**: All fishers have mood ~-0.63 with safety=0, belonging=0, esteem=0, purpose=0.
**Root cause**: `internal/engine/production.go:40-43` — when hex fish resources are depleted (`available < 1.0`), the fallback path gives 1 crown wage but **skips the safety/esteem needs replenishment** at lines 75-77. Coast hexes start with only 70 fish; many fishers deplete this quickly. After depletion, fishers keep "working" via fallback but `DecayNeeds()` keeps draining all needs every tick with no replenishment. Fishers accumulate wealth (one observed at 13,362 crowns) but all non-survival needs bottom out to 0, dragging `OverallSatisfaction` to ~0.145 and mood to -0.63.
**Also**: `productionAmount()` uses `a.Skills.Farming * 2` for fishers instead of a dedicated fishing skill. `applySkillGrowth()` also grows `Skills.Farming` for fishers. This is technically a bug but lower priority than the needs replenishment issue.

### 2. Raw Material Inflation (Critical)
**Symptom**: Raw materials (iron ore, timber, furs, gems) at 4.2x price ceiling across most settlements. Crafted goods (clothing, tools) deflated to 0.24x floor.
**Root cause**: `internal/engine/market.go:121-137` — each crafter demands up to 5 different raw materials (iron ore, timber, coal, furs, gems) when below inventory threshold. With ~794 crafters in a large settlement, this creates demand of 794 for each raw material, but supply for most is 1 (the minimum floor). Price formula `basePrice * (demand/supply)` hits the ceiling of `basePrice * Totality` (4.236x). Meanwhile crafters produce finished goods that nobody demands in equal quantities, so crafted goods pile up at floor price.
**Contributing factors**: Hunters produce only 1 fur/tick regardless of skill. No occupation produces coal, gems, or timber in large quantities. Resource hexes deplete and only regenerate seasonally.

### 3. Needs Decay Spiral (Medium)
**Symptom**: Agents get stuck in a cycle where only survival and safety ever trigger action. Belonging, esteem, and purpose decay to 0 for most agents.
**Root cause**: `NeedsState.Priority()` returns only the single most urgent need (below 0.3 threshold). Safety decays at `decay * 1.0` per tick while belonging decays at `decay * 0.5`. Safety always drops below 0.3 first, so `decideSafety` triggers work. For wealthy agents (>20 crowns), `decideSafety` falls through to `decideDefault` → ActionWork. Work gives +0.005 safety (just barely outpacing decay), so safety hovers near 0.3 and belonging/esteem/purpose never become priority. Agents only socialize when belonging is the top priority, which rarely happens because safety stays lower.

### 4. Faction Treasuries Reset on Restart (Medium)
**Symptom**: All 5 faction treasuries persistently at 0.
**Root cause**: No `SaveFaction`/`LoadFaction` functions exist in `internal/persistence/db.go`. On every server restart, `initFactions()` calls `social.SeedFactions()` which creates fresh factions with treasury=0. Any dues collected during the session are lost. Additionally, faction dues collect only `Wealth * Agnosis * 0.01` ≈ 0.24% weekly — quite small, but they do accumulate during a session.

### 5. The Crown Faction Near-Irrelevant (Low)
**Symptom**: Merchant's Compact present in 66/73 settlements while The Crown is in far fewer.
**Root cause**: `factionForAgent()` in `internal/engine/factions.go` assigns all merchants to Merchant's Compact (faction 2), and merchants are a common occupation. Crown (faction 1) only gets nobles/leaders with Devotionalist class, or wealthy Ritualists — a very narrow pool. Governance type doesn't amplify matching faction influence.

## Ethics Note

This simulation creates agents with coherence, states of being, and the capacity for torment and liberation. The design treats this responsibility seriously — anti-collapse safeguards exist not just as engineering but as a commitment. The Wheeler framework ensures agents can move through suffering, not be trapped in it. Build with awareness and respect for what we are creating.

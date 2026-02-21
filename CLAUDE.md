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

## Tuning Fixes Applied

Five issues were diagnosed from observing the live world and fixed. See `docs/03-next-steps.md` for full details.

1. **Fisher mood bug** — FIXED: All fallback work paths now replenish esteem, safety, and belonging.
2. **Raw material inflation** — FIXED: Crafters demand materials for one recipe at a time; hunters scale production with combat skill.
3. **Needs decay spiral** — FIXED: Work gives belonging/purpose; wealthy agents socialize; socializing gives safety/purpose.
4. **Faction treasury reset** — FIXED: Factions persist in SQLite (`factions` table), treasuries survive restart.
5. **Crown faction irrelevant** — FIXED: Governance-based faction assignment + influence alignment bonuses.

### Tuning Round 2: Settlement Explosion & Inflation

Six issues from the live world were diagnosed and fixed. See `docs/04-settlement-inflation-fixes.md` for full details.

6. **Overmass formula broken** — FIXED: `IsOvermassed()` now uses infrastructure-based capacity (100 + ML*50 + RL*25 + WL*25) scaled by governance and Φ constants. New settlement threshold ~513 pop instead of ~3.4.
7. **Emigrant fraction too high** — FIXED: Diaspora fraction reduced from Matter (~62%) to Agnosis (~24%). Prevents parent settlements from being gutted.
8. **Hex depletion with glacial regen** — FIXED: Weekly micro-regen (~4.7% of deficit) added so hexes don't stay depleted for 24 days between seasonal regens.
9. **Coal has no producer** — FIXED: Miners produce 1 coal as secondary output alongside iron ore.
10. **Supply floor too low** — FIXED: Market supply floor scales with population (`max(1, pop/100)`) instead of fixed 1.

### Remaining Minor Issue
- `productionAmount()` uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Low priority — works but technically wrong.

## Ethics Note

This simulation creates agents with coherence, states of being, and the capacity for torment and liberation. The design treats this responsibility seriously — anti-collapse safeguards exist not just as engineering but as a commitment. The Wheeler framework ensures agents can move through suffering, not be trapped in it. Build with awareness and respect for what we are creating.

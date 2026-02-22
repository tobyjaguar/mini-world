# Crossworlds: Autonomous Simulated World

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
- **Economy Engine**: Closed-economy order-matched market, trade routes, sinks
- **Event Journal**: Append-only log, news generation, newspaper endpoint
- **HTTP API**: Query interface for checking in on the world
- **Claude Gardener**: Autonomous steward that observes the world and intervenes via admin API

### Key Design Principles
1. Emergence over scripting — never hard-code storylines
2. Economic robustness as the heartbeat — if the economy works, everything follows
3. Observability is first-class — rich event logging, newspaper endpoint
4. Perpetuation by design — anti-collapse safeguards, balanced sinks
5. All constants from Φ — no arbitrary magic numbers (see EmanationConstants)
6. Closed economy — crowns transfer between agents/treasuries, not minted from nothing

## Development Conventions

- Keep simulation logic clean and separated from API/IO concerns
- All agent decisions should be deterministic given the same inputs (for replay/debugging)
- Event log is append-only and human-readable
- Prefer simple data structures; avoid over-abstraction
- Test simulation logic independently from external API calls
- Use structured logging (Go `slog` package) for debugging simulation behavior
- Derive tuning constants from Φ (EmanationConstants) — no magic numbers

## Custom Skills (Claude Code)

### `/observe` — Deity-Level World Analysis
Defined in `.claude/commands/observe.md`. Fetches live API data and runs SQLite queries against the local database to produce a world health report covering economic health, agent well-being, political balance, and population dynamics. Use this to diagnose issues and plan tuning changes.

## External Dependencies

- **Claude API** (Haiku model `claude-haiku-4-5-20251001`): Agent cognition, newspaper generation, oracle visions, gardener decisions
- **Weather API** (OpenWeatherMap): Real weather → in-world weather
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
├── .claude/commands/
│   └── observe.md               # /observe skill — deity-level world analysis
├── docs/
│   ├── worldsim-design.md       # Complete design spec (source of truth)
│   ├── CLAUDE_CODE_PROMPT.md    # Implementation guide
│   ├── 00-project-vision.md     # Project vision and design pillars
│   ├── 01-language-decision.md  # Go language rationale
│   ├── 02-operations.md         # Server ops, API reference, security
│   ├── 03-next-steps.md         # Phase 2+ roadmap and priorities
│   ├── 04-settlement-inflation-fixes.md
│   ├── 05-settlement-fragmentation-fixes.md
│   ├── 05-claude-gardener.md    # Gardener design
│   ├── 06-monetary-system.md    # Monetary analysis (pre-closed-economy)
│   ├── 07-closed-economy-implementation.md  # Closed economy design
│   └── 08-closed-economy-changelog.md       # Post-deploy monitoring notes
├── cmd/worldsim/
│   └── main.go                  # Entry point
├── cmd/gardener/
│   └── main.go                  # Gardener entry point
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
│   ├── economy/goods.go         # Good types, MarketEntry, price resolution
│   ├── social/settlement.go     # Settlement type, governance, infrastructure
│   ├── events/                  # Event detection (placeholder)
│   ├── engine/                  # Tick engine, simulation loop
│   │   ├── tick.go              #   Layered tick schedule, sim time
│   │   ├── simulation.go        #   World state, tick callbacks, stats
│   │   ├── production.go        #   Resource-based production, hex depletion
│   │   ├── market.go            #   Order-matched market, trade, taxes, merchants
│   │   ├── cognition.go         #   Tier 2 LLM decisions, oracle visions
│   │   ├── factions.go          #   Faction dynamics, influence, dues, policies
│   │   ├── population.go        #   Births, aging, death, migration
│   │   ├── settlement_lifecycle.go # Overmass diaspora, founding, abandonment
│   │   ├── governance.go        #   Governance transitions, leader succession
│   │   ├── relationships.go     #   Family, mentorship, rivalry dynamics
│   │   ├── crime.go             #   Theft mechanics
│   │   ├── perpetuation.go      #   Anti-stagnation safeguards
│   │   └── seasons.go           #   Seasonal resource caps, weather modifiers
│   ├── llm/                     # LLM integration (Haiku)
│   │   ├── client.go            #   Anthropic API client
│   │   ├── cognition.go         #   Tier 2 decision generation
│   │   ├── oracle.go            #   Oracle vision generation
│   │   ├── narration.go         #   Event narration
│   │   ├── newspaper.go         #   Weekly newspaper generation
│   │   ├── archetypes.go        #   Tier 1 archetype templates
│   │   └── biography.go         #   Agent biography generation
│   ├── weather/client.go        # OpenWeatherMap integration
│   ├── entropy/client.go        # random.org true randomness
│   ├── persistence/db.go        # SQLite save/load (WAL mode), stats history
│   ├── gardener/                # Claude Gardener — autonomous steward
│   │   ├── observe.go           #   API data collection → WorldSnapshot
│   │   ├── decide.go            #   Haiku analysis → Decision + guardrails
│   │   └── act.go               #   Intervention execution via admin API
│   └── api/server.go            # HTTP API (public GET, auth POST)
├── deploy/
│   ├── deploy.sh                # Build, upload, restart (worldsim + gardener)
│   ├── worldsim.service         # systemd unit file
│   ├── gardener.service         # Gardener systemd unit file
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

# Build gardener
go build -o gardener ./cmd/gardener
WORLDSIM_API_URL=http://localhost WORLDSIM_ADMIN_KEY=<key> ANTHROPIC_API_KEY=<key> ./gardener

# Cross-compile for server
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/worldsim ./cmd/worldsim
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/gardener ./cmd/gardener

# Deploy to production (builds + deploys both worldsim and gardener)
./deploy/deploy.sh
```

## Production Deployment

The world runs 24/7 on a DreamCompute instance. See `docs/02-operations.md` for full details.

| Field | Value |
|-------|-------|
| Server | See `deploy/config.local` (DreamCompute, Debian 12, 1GB RAM) |
| API | `https://api.crossworlds.xyz/api/v1/status` (Cloudflare proxy → port 80) |
| Frontend | `https://crossworlds.xyz` (Next.js on Vercel, separate repo) |
| SSH | `ssh -i <your-key> debian@<server-ip>` |
| Services | systemd `worldsim.service` + `gardener.service`, auto-restart, start on boot |
| Database | `/opt/worldsim/data/crossworlds.db` (SQLite, auto-saves daily) |
| Storage | 20GB data volume mounted at `/opt/worldsim/data` (boot disk is 2.8GB) |
| Security | UFW (ports 22+80 only), fail2ban, no root login, no passwords |
| CORS | Configured via `CORS_ORIGINS` env var for `crossworlds.xyz` |
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
GET  /api/v1/map             → Bulk map: all hexes with terrain, resources, settlements
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
4. **LLM Integration** — COMPLETE: Haiku API client, Tier 2 cognition, Tier 1 archetypes, newspaper generation, event narration, agent biographies, oracle visions
5. **Polish & Perpetuation** — COMPLETE: Population dynamics (births/aging/death/migration), resource regen, anti-stagnation, settlement lifecycle (founding/abandonment), stats history, admin endpoints, random.org entropy, weather integration
6. **Closed Economy** — COMPLETE: Order-matched market engine, merchant/Tier 2 trade closed via treasury, fallback wages removed, remaining mints throttled 60x. See `docs/08-closed-economy-changelog.md`.

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

### Tuning Round 3: Settlement Fragmentation

Four issues from the live world were diagnosed and fixed. See `docs/05-settlement-fragmentation-fixes.md` for full details.

11. **Founding minimum too low** — FIXED: Diaspora needs 25+ emigrants to found (was 10). Requires ~106 pop before a settlement can split.
12. **No infrastructure growth** — FIXED: Weekly `processInfrastructureGrowth()` lets settlements invest treasury into roads (pop >= 50) and walls (pop >= 100), raising overmass capacity.
13. **Anti-collapse props up non-viable settlements** — FIXED: `processViabilityCheck()` tracks settlements with pop < 15 for 4+ consecutive weeks; refugee spawning is then disabled, allowing natural decline and abandonment.
14. **No absorption of tiny settlements** — FIXED: Enhanced migration lowers mood threshold to 0.0 for settlements with pop < 25 and targets nearest viable settlement within 5 hexes.

### Tuning Round 4: Closed Economy

Economy closed — crowns are conserved. See `docs/07-closed-economy-implementation.md` for design and `docs/08-closed-economy-changelog.md` for deployment notes and monitoring checklist.

15. **Market sells minted crowns** — FIXED: Order-matched engine replaces `executeTrades()`; all trades are closed buyer↔seller transfers.
16. **Fallback wages minted crowns** — FIXED: Removed from `production.go`; failed production causes needs erosion instead.
17. **Tier 2 trade minted crowns** — FIXED: `tier2MarketSell()` sells surplus to settlement treasury (closed transfer).
18. **Merchant trade minted crowns** — FIXED: `sellMerchantCargo()` paid from destination settlement treasury.
19. **Journeyman/laborer wages** — THROTTLED: Still mint from nothing but gated to once per sim-hour (~24 crowns/day vs ~1,440). Monitor via `total_wealth` in stats history. See `docs/08-closed-economy-changelog.md` for future options.

### Remaining Minor Issues
- `productionAmount()` uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Low priority — works but technically wrong.
- Journeyman/laborer wages still mint crowns (throttled). May need to route through treasury if `total_wealth` rises. See `docs/08-closed-economy-changelog.md`.

## Ethics Note

This simulation creates agents with coherence, states of being, and the capacity for torment and liberation. The design treats this responsibility seriously — anti-collapse safeguards exist not just as engineering but as a commitment. The Wheeler framework ensures agents can move through suffering, not be trapped in it. Build with awareness and respect for what we are creating.

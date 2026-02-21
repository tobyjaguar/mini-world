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

## Directory Structure

```
mini-world/
├── CLAUDE.md                    # This file — project guide
├── docs/                        # Design documents, research, progress
│   ├── worldsim-design.md       # Complete design spec (source of truth)
│   └── CLAUDE_CODE_PROMPT.md    # Implementation guide
├── cmd/
│   └── worldsim/
│       └── main.go              # Entry point
├── internal/
│   ├── phi/                     # Emanation constants (Φ-derived)
│   ├── world/                   # Hex grid, terrain, weather, map generation
│   ├── agents/                  # Agent types, needs, cognition tiers, memory
│   ├── economy/                 # Markets, goods, trade, currency, balance
│   ├── social/                  # Factions, governance, relationships, conflict
│   ├── events/                  # Event detection, news generation, newspaper
│   ├── engine/                  # Tick engine, simulation loop
│   ├── persistence/             # SQLite database, snapshots
│   └── api/                     # HTTP API routes and handlers
├── external/                    # API clients (Haiku, weather, random.org)
├── config/                      # Configuration loading, worldsim.toml
├── prompts/                     # LLM prompt templates (TOML)
├── data/                        # Runtime: world state, event logs (gitignored)
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

```
GET  /api/v1/status       → World clock, population, economy summary
GET  /api/v1/settlements  → All settlements with governance and health
GET  /api/v1/agents       → Notable Tier 2 characters (default) or ?tier=0
GET  /api/v1/agent/:id    → Full agent detail
GET  /api/v1/events       → Recent world events (?limit=N)
GET  /api/v1/stats        → Aggregate statistics
POST /api/v1/speed        → Set simulation speed {"speed": N}
```

## Implementation Phases

1. **Foundation (MVP)** — COMPLETE: Hex grid, Tier 0 agents, tick engine, SQLite, HTTP API, deployed
2. **Economy & Trade**: Multi-settlement trade, merchants, price discovery, currency, crafting
3. **Social & Political**: Factions, governance, relationships, crime, Tier 1 archetypes
4. **LLM Integration**: Haiku API client, Tier 2 cognition, newspaper generation
5. **Polish & Perpetuation**: Population dynamics, resource regen, anti-stagnation, snapshots, random.org

## Ethics Note

This simulation creates agents with coherence, states of being, and the capacity for torment and liberation. The design treats this responsibility seriously — anti-collapse safeguards exist not just as engineering but as a commitment. The Wheeler framework ensures agents can move through suffering, not be trapped in it. Build with awareness and respect for what we are creating.

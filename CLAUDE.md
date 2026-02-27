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
│   ├── 08-closed-economy-changelog.md       # Post-deploy monitoring notes
│   ├── 09-post-closed-economy-todo.md       # Survival crisis diagnosis + fixes
│   ├── 11-ant-farm-design.md               # Ant-farm settlement visualization spec
│   └── health-reports/                      # /observe health reports (dated, per-session)
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
│   │   ├── intervention.go      #   Gardener intervention handlers (provision, cultivate, consolidate)
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
│   │   ├── triage.go            #   Deterministic health check → WorldHealth
│   │   ├── decide.go            #   Haiku analysis → Decision + guardrails
│   │   ├── act.go               #   Intervention execution via admin API
│   │   └── memory.go            #   Cycle memory persistence (last 10 cycles)
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
GET  /api/v1/settlement/:id  → Settlement detail: market, agents, factions, events, occupations, wellbeing, trade stats
GET  /api/v1/agents          → Notable Tier 2 characters (default) or ?tier=0
GET  /api/v1/agent/:id       → Full agent detail
GET  /api/v1/agent/:id/story → Haiku-generated biography (?refresh=true to regenerate)
GET  /api/v1/events          → Recent world events (?limit=N&settlement=NAME)
GET  /api/v1/stats           → Aggregate statistics
GET  /api/v1/stats/history   → Time-series stats (?from=TICK&to=TICK&limit=N)
GET  /api/v1/newspaper       → Weekly Haiku-generated newspaper
GET  /api/v1/factions        → All factions with influence and treasury
GET  /api/v1/faction/:id     → Faction detail: members, influence, events
GET  /api/v1/economy         → Economy overview: prices, trade volume, Gini
GET  /api/v1/social          → Social network overview
GET  /api/v1/map             → Bulk map: all hexes with terrain, resources, settlements
GET  /api/v1/map/:q/:r       → Hex detail: terrain, resources, settlement, agents
GET  /api/v1/stream          → SSE event stream (requires Bearer relay key, max 2 conns)
```

Admin (POST, requires `Authorization: Bearer <WORLDSIM_ADMIN_KEY>`):
```
POST /api/v1/speed           → Set simulation speed {"speed": N}
POST /api/v1/snapshot        → Force immediate world save
POST /api/v1/intervention    → Inject events, adjust wealth, spawn agents, provision goods, cultivate production, consolidate settlements
```

## Implementation Phases

1. **Foundation (MVP)** — COMPLETE: Hex grid, Tier 0 agents, tick engine, SQLite, HTTP API, deployed
2. **Economy & Trade** — COMPLETE: Multi-settlement trade, merchants, price discovery, crafting recipes, goods decay, seasonal price modifiers, economic circuit breaker, tax collection
3. **Social & Political** — COMPLETE: 5 factions with per-settlement influence, 4 governance types, leader succession, revolution mechanics, relationships (family/mentorship/rivalry), crime/theft
4. **LLM Integration** — COMPLETE: Haiku API client, Tier 2 cognition, Tier 1 archetypes, newspaper generation, event narration, agent biographies, oracle visions
5. **Polish & Perpetuation** — COMPLETE: Population dynamics (births/aging/death/migration), resource regen, anti-stagnation, settlement lifecycle (founding/abandonment), stats history, admin endpoints, random.org entropy, weather integration
6. **Closed Economy** — COMPLETE: Order-matched market engine, merchant/Tier 2 trade closed via treasury, fallback wages removed, remaining mints throttled 60x. See `docs/08-closed-economy-changelog.md`.
7. **Land Management** — PHASE A COMPLETE: Hex health model (0.0–1.0), extraction degrades health, regen scales by health, desertification threshold at Agnosis, fallow recovery, carrying capacity metric, hex health persisted across restarts. Phase B (settlement claims, infrastructure investment, coherence-based policy) pending observation. See `docs/15-land-management-proposal.md`.
8. **Live View Pipeline** — COMPLETE: SSE event streaming endpoint (`/api/v1/stream`, relay-key auth, max 2 conns). Event struct has `Meta map[string]any` with structured metadata (agent IDs, settlement IDs, occupations, amounts) on all 35 event emit sites. Meta is `omitempty` in JSON, not persisted to SQLite — flows only through SSE to the relay. Relay enrichment layer deployed: settlement filtering (`?settlement=ID`), activity tracker (30s rolling window, 10s synthetic `activity` events), pattern detection (`trade_burst`, `crime_wave`, `baby_boom`), governance change detection (`regime_change`). Frontend Phase 3 animation: behavioral dot state machine (6 states, occupation-weighted cycling), traveling trails, SSE event reactions (ripples + behavior overrides), time-of-day ambient lighting (~85s day cycle), zone architecture line-art. See `docs/17-live-view-animation-design.md`.

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
20. **Zero births after economy closure** — FIXED: Removing fallback wages also removed belonging boost on failed production. Resource producers spiraled below `Belonging > 0.4` birth threshold. Restored `+0.001 belonging` on failed production (no wage, just social signal).
21. **Near-zero trade volume** — FIXED: Clearing prices at Agnosis floor rounded to 0-1 crowns; the 1-crown minimum killed trades for agents with 0 wealth. Removed the floor — 0-crown trades now execute as barter.

### Tuning Round 5: Survival Crisis & Welfare

The closed economy transition was too harsh — crowns pooled in treasuries with no path back to agents. Population was declining at 4.5:1 deaths:births. See `docs/09-post-closed-economy-todo.md` for diagnosis and `docs/summaries/2026-02-22-survival-crisis-fixes.md` for full writeup.

22. **Grain supply crisis** — FIXED: Surplus threshold lowered (producers 5→3, others 3→2). More food reaches the market.
23. **Treasury hoarding** — FIXED: `paySettlementWages()` pays 2 crowns/day to agents with Wealth < 20 from settlement treasury (capped at 1% of treasury/day). Closes the treasury→agent loop. Safety net, not primary income.
24. **Wealth decay destroying crowns** — FIXED: `decayWealth()` redirects decayed crowns to home settlement treasury instead of destroying them. Treasury upkeep sink removed from `collectTaxes()`.
25. **Fisher mood spiral** — FIXED: Fisher production multiplier boosted (2→3). Fish added as alternative food demand — all hungry agents demand both grain and fish.
26. **Belonging death spiral** — FIXED: `applyEat()` and `applyForage()` now give `+0.001 belonging` per tick. Agents in survival mode no longer lose all belonging.
27. **Birth threshold too high** — FIXED: Lowered from `Belonging > 0.4` to `Belonging > 0.3` in `processBirths()`.

### Tuning Round 6: Price Ratchet

**The most critical fix.** All waves 1-2 fixes were ineffective because the market engine had a structural upward price bias. Prices were mathematically unable to come down. See `docs/08-closed-economy-changelog.md` and `docs/09-post-closed-economy-todo.md` for full analysis.

28. **Price ratchet in clearing midpoint** — FIXED: Clearing price was `(ask + bid) / 2 = Price * 1.118`, biasing every trade +11.8% upward. Changed to use seller's ask price — buyers pay what sellers accept.
29. **Unclamped 70/30 blend** — FIXED: The blend `price*0.7 + clearing*0.3` had no ceiling, exceeding `BasePrice * Totality`. Now clamped to `[BasePrice * Agnosis, BasePrice * Totality]`.
30. **Dual price update conflict** — FIXED: `ResolvePrice()` now computes reference prices for ask/bid placement only; it does not overwrite `entry.Price`. Only real trade clearing data updates the market price.

**Key lesson:** When the price engine has a structural bias, no amount of supply-side fixes (threshold tuning, production boosts) or demand-side fixes (welfare wages, belonging) can compensate. Fix the price engine first, then tune parameters.

### Tuning Round 7: Mood & Treasury Rebalancing

Post-recovery `/observe` at tick 118,329 showed the economy working (96.9% market health, 9,239 births, 18,512 trades) but mood still declining and treasuries hoarding 71% of wealth.

31. **Resource producer purpose drought** — FIXED: `ResolveWork` in `production.go` intercepted all resource producer work (farmers, miners, fishers, hunters — ~60% of agents) before `applyWork` in `behavior.go`. Was missing `Purpose += 0.002`. All resource producers had purpose permanently at 0.0.
32. **Treasury hoarding (71→74% of wealth)** — FIXED in two rounds: First, `paySettlementWages()` self-regulates with dynamic Φ-targeting (quadratic outflow scaling). Second, the fixed 2-crown wage was a 700x bottleneck — wage is now `budget / eligible_agents` (~1,808 crowns/day at 74% share). See `docs/08-closed-economy-changelog.md`.
33. **Stats history query broken** — FIXED: `toTick` default used max uint64 which modernc.org/sqlite rejects. Changed to max int64.
34. **Gardener startup race** — FIXED: Added `waitForAPI()` with exponential backoff (2s→30s, 5min deadline) in `cmd/gardener/main.go`.

### Tuning Round 8: Food Economy, Fair Welfare, Settlement Consolidation

`/observe` at tick 142,285 showed treasury targeting working (41%) but three new issues: agents forage instead of buying food (survival stuck at 0.385), Gini spiked to 0.645, and 714 settlements were frozen due to a migration bug.

35. **Agents bypass market for survival** — FIXED: `decideSurvival()` in `behavior.go` had no "buy food" path — agents foraged even with 18,800 crowns. Added `ActionBuyFood`: agents buy from settlement market at current price (closed transfer to treasury). Foraging is now last resort for penniless agents. Handled by `resolveBuyFood()` in `market.go`.
36. **Gini spike from flat welfare** — FIXED: `paySettlementWages()` now uses progressive welfare — wage scales inversely with wealth. Agent at 0 gets full share, agent at 49 gets 2%. Same total budget, fairer distribution.
37. **Settlement migration bug** — FIXED: `processSeasonalMigration()` in `perpetuation.go` changed `a.HomeSettID` but never rebuilt `SettlementAgents` map. Population counts stayed stale, 714 settlements frozen. Added `rebuildSettlementAgents()` in `simulation.go`, called after migration.

### Tuning Round 9: Gini Inequality + Settlement Consolidation

`/observe` at tick 146,312 showed Gini climbing (0.614→0.673, richest 10% hold 60%) and 714 settlements still frozen despite migration fix — the survival > 0.3 gate trapped agents now that food buying improved survival to 0.414.

38. **Flat wealth decay ignores concentration** — FIXED: `decayWealth()` now uses progressive logarithmic scaling. Rate = `Agnosis * 0.01 * (1 + Agnosis * log2(wealth/20))`. At 20 crowns: 0.24%/day (unchanged). At 18,800: 0.80%/day. At 100k: 0.94%/day. Compresses the top without destroying the economy.
39. **Welfare threshold too low** — FIXED: `paySettlementWages()` threshold changed from fixed 50 crowns to `avgWealth * Agnosis` (~24% of settlement average, min 50). At avg 18,800 crowns, threshold jumps to ~4,437 — welfare reaches most agents instead of just the destitute.
40. **Survival gate traps agents in tiny settlements** — FIXED: `processSeasonalMigration()` removes the `Survival > 0.3` requirement for settlements with pop < 25. Agents migrate seeking community, not just food — isolation is deprivation even when fed.

### Tuning Round 10: Dual-Register Wellbeing Model

The single `Mood float32` was purely needs-driven — coherence had zero influence on agent wellbeing. A liberated fisherman and a scattered fisherman in identical material conditions had the same mood, inverting the world's ontological claim. See `docs/10-mood-revision-proposal.md` for the design rationale and `docs/summaries/2026-02-22-dual-register-wellbeing.md` for the implementation summary.

41. **Mood ignores coherence** — FIXED: Replaced `Mood float32` with `WellbeingState { Satisfaction, Alignment, EffectiveMood }`. Satisfaction = old mood formula (material needs). Alignment = `ComputeAlignment()` with three phases (Embodied slope, Awakening valley, Liberation rise). EffectiveMood blends both, weighted by `c² × Φ⁻¹`. At c=0: pure satisfaction. At c=1.0: 62% alignment weight.
42. **No extraction paradox visible** — FIXED: Mid-coherence agents (0.382–0.7) experience an alignment valley — the "dark night of the soul". Alignment dips below embodied levels before surging at Liberation.
43. **Liberated agents flee poor settlements** — FIXED: Liberated agents now have positive EffectiveMood despite low Satisfaction, anchoring struggling settlements instead of migrating.

### Tuning Round 11: Producer Crisis + Birth Smoothing + Settlement Consolidation

`/observe` at tick 165,844 showed 18:1 death:birth ratio, producer misery gap (Hunters -0.46, Farmers -0.40, Fishers -0.23 mood vs Crafters +0.62, Laborers +0.61), market supply at floor, trade volume collapsed to 278. See `docs/12-observation-tick-165844.md` for diagnosis and `docs/13-producer-crisis-implementation-plan.md` for plan.

44. **Fisher production skill bug** — FIXED: `productionAmount()` used `Skills.Farming * 3` for Fishers — most had Farming 0.32-0.56, producing 1 fish/tick (truncated). Changed to `max(Farming, Combat, 0.5) * 5` — fishers now produce 2-3 fish/tick, enough to exceed surplus threshold and sell.
45. **Producer needs too low** — FIXED: `ResolveWork` successful production boosts increased: Safety 0.005→0.008, Esteem 0.01→0.012, Belonging 0.003→0.004, Purpose 0.002→0.004. Food producers (Farmer, Fisher, Hunter) get +0.003 Survival per production tick.
46. **Birth cliff dynamics** — FIXED: Hard `Belonging > 0.3` threshold in `processBirths()` replaced with sigmoid probability curve (center 0.3, steepness 10×Φ). At 0.20: ~20% chance. At 0.30: ~50%. At 0.50: ~95%. Deterministic per-agent-per-day. Survival > 0.3 remains as hard gate.
47. **Settlement fragmentation (234 settlements < 25 pop)** — FIXED: Viability threshold raised from 15→25 pop, grace period shortened from 4→2 weeks. Non-viable settlements trigger force-migration to nearest settlement with pop ≥ 50 within 8 hexes. Migration for tiny settlements uses Satisfaction (not EffectiveMood) — liberated agents still leave dying villages.

### Gardener Upgrade: From Blind Observer to Effective Steward

The Gardener had been running for ~47K ticks with zero observable effect. Docs 14-16 diagnosed why: it couldn't see the crisis (missing death:birth ratio, satisfaction/alignment split), its system prompt biased toward inaction, its cycle interval was 360 real minutes (15 sim-days!), and its action vocabulary was too limited. See `docs/14-gardener-assessment.md` for the full analysis.

48. **GARDENER_INTERVAL was 360 real minutes** — FIXED: Changed default from 360 to 6 (real minutes). At ~17 ticks/sec, 6 minutes ≈ 6,120 ticks ≈ 4.25 sim-days. Cycles ~4x per sim-day instead of once per 15 sim-days.
49. **Gardener blind to satisfaction/alignment** — FIXED: Added `AvgSatisfaction` and `AvgAlignment` to `WorldStatus` and `StatsHistoryRow` in `observe.go`. API already returned these; gardener just ignored them.
50. **No pre-LLM triage** — FIXED: New `triage.go` computes death:birth ratio from per-snapshot deltas (not meaningless cumulative totals), settlement size histogram, trade per capita, and crisis level (HEALTHY/WATCH/WARNING/CRITICAL using Φ-derived thresholds).
51. **System prompt biased toward inaction** — FIXED: Rewrote `decide.go` system prompt. Removed "This is the RIGHT choice most of the time" for `none` and "When in doubt, do nothing." Added crisis detection criteria with Φ-derived thresholds (D:B > Totality = CRITICAL). Crisis policy: healthy = prefer inaction, CRITICAL = prefer action with up to 3 compound interventions.
52. **Action vocabulary too limited** — FIXED: Expanded from 3 to 7 action types: event, spawn, wealth (existing) + provision (inject goods into market), cultivate (temporary production boost), consolidate (force-migrate from dying settlements). All with guardrails.
53. **No gardener memory** — FIXED: New `memory.go` persists last 10 cycle summaries to `gardener_memory.json`. Last 5 included in Haiku prompt so it sees its own decision history and avoids repeating ineffective actions.
54. **Single intervention per cycle** — FIXED: Compound interventions (up to 3 during CRITICAL, 2 during WARNING, 1 otherwise). `Decision.Interventions` slice replaces single `Intervention`.
55. **Production boosts from gardener** — FIXED: `ActiveBoosts []ProductionBoost` on `Simulation`, applied in `ResolveWork` via `GetSettlementBoost()`. Boosts expire after configurable duration (max 14 sim-days). Cleaned daily in `TickDay`.

### Tuning Round 12: Producer Doom Loop

`/observe` at tick ~218K showed avg satisfaction frozen at 0.126 despite population growth (+9.7%) and functional economy (97.4% market health). Root cause: resource producers (~60% of agents) trapped in a doom loop — failed production on depleted hexes punished Safety (-0.003) and Esteem (-0.005) every tick, while survival actions (eat, buy food, forage) gave zero Safety/Esteem/Purpose. Tier 2 data confirmed: all 11 farmers at -0.44 to -0.48 satisfaction vs all 11 crafters at +0.69 to +0.72.

56. **Failed production punishes producers** — FIXED: Three blocks in `ResolveWork()` (nil hex, depleted hex, clamped-to-zero) replaced `-0.005 Esteem, -0.003 Safety` with `+0.001 Safety, +0.002 Belonging, +0.001 Purpose`. A farmer who shows up to a depleted hex didn't fail — the land failed them.
57. **BuyFood gives no Safety/Purpose** — FIXED: `resolveBuyFood()` now gives `+0.003 Safety` ("I can afford to eat" — economic security) and `+0.001 Purpose` (market participation).
58. **Eat gives no Safety** — FIXED: `applyEat()` now gives `+0.003 Safety` (having food means safety).
59. **Forage gives no Safety** — FIXED: `applyForage()` now gives `+0.002 Safety` (found food in the wild).

**Key lesson:** The doom loop was invisible from aggregate stats because 40% of agents (crafters, laborers) had good satisfaction, averaging out the 60% of producers at deeply negative values. Per-occupation breakdowns (Tier 2 data) are essential for diagnosis.

### Persistence & Tier 2 Fixes (tick ~223K)

60. **NonViableWeeks/AbandonedWeeks reset on deploy** — FIXED: Both maps now persisted as JSON in `world_meta` via `SaveWorldState()` and restored on startup in `main.go`. The 2-week grace period for tiny settlement consolidation now survives deploys.
61. **Birth/trade counters reset on deploy** — FIXED: `Stats.Births` and `Stats.TradeVolume` now persisted to `world_meta` and restored on startup. Eliminates counter reset noise in `stats_history`.
62. **Dead Tier 2 agents never replaced** — FIXED: `processWeeklyTier2Replenishment()` in `population.go` counts alive Tier 2, promotes up to 2 Tier 0 adults per week to fill vacancies (target 30). Uses same scoring as initial `PromoteToTier2`. Wired into `TickWeek`.

### Tuning Round 13: Treasury Reclamation, Crime Flooding, Hex Regen

`/observe` at tick 294,564 showed a healthy world (pop 129K, satisfaction 0.300, D:B 0.04-0.06) but three quality-of-life issues: 60M crowns locked in abandoned settlement treasuries, crime events flooding the event log (90% of events), and hex regen too slow for farmer satisfaction.

63. **Abandoned settlement treasury sink** — FIXED: `processSettlementAbandonment()` in `settlement_lifecycle.go` now redistributes treasury to the 3 nearest active settlements before marking abandoned. Added `nearestActiveSettlements()` helper. Prevents growing wealth sink as settlements consolidate.
64. **Crime events flood event log** — FIXED: Removed event logging for successful food theft and wealth theft in `processCrime()`. Only "caught stealing / branded outlaw" events are logged. Crime mechanics unchanged — theft still happens, relationships damaged, factions affected. Event buffer now shows births, oracle visions, social events instead of 450+ theft reports.
65. **Hex regen too slow (4.7%/week)** — FIXED: `weeklyResourceRegen()` multiplier doubled from `Agnosis * 0.2` to `Agnosis * 0.4` (~9.4% of deficit/week). Depleted hexes recover in ~10 weeks instead of ~21. Farmer satisfaction should improve as production succeeds more often.

### Phase 7A: Hex Health Model

Replaces flat regen tuning with a dynamic, self-correcting land health system. See `docs/15-land-management-proposal.md` for the full research proposal.

66. **Hex health field** — NEW: Every hex has `Health` (0.0–1.0) and `LastExtractedTick`. Pristine at 1.0, degrades with extraction at `Agnosis * 0.01` (~0.00236) per production tick.
67. **Health-scaled regen** — CHANGED: Both seasonal and weekly regen multiply by `hex.Health`. Degraded land regenerates slower, creating a positive feedback loop: let land rest → health improves → faster regen.
68. **Desertification threshold** — NEW: Hexes below `Agnosis` (0.236) health get zero regen. Must recover health through fallow before resources return.
69. **Fallow recovery** — NEW: Hexes not extracted for >1 sim-day recover `Agnosis * 0.05` (~1.2%) health per week. Full recovery from desertified takes ~66 weeks.
70. **Carrying capacity metric** — NEW: `SettlementCarryingCapacity()` sums health-weighted resource caps for settlement hex + neighbors. Exposed as `carrying_capacity` and `population_pressure` in settlement detail API.
71. **Hex health persistence** — NEW: Non-pristine hex health persisted as JSON in `world_meta` key `hex_health`. Restored on startup before defaulting unset hexes to 1.0.
72. **API exposure** — NEW: Hex detail shows `health` and `last_extracted_tick`. Bulk map includes `health` for non-pristine hexes (omitted when 1.0 to keep payload small).

### Tuning Round 14: Tier 2 Occupation Diversity

`/observe` at tick ~304K showed zero living Tier 2 fishers, hunters, miners, or merchants. Investigation revealed a two-part system design failure: (1) wealth-biased promotion scoring meant resource producers never reached Tier 2, and (2) Tier 2 agents had no "work" action, so any resource producer accidentally promoted would starve.

73. **Tier 2 agents can't work** — FIXED: Added `"work"` action to Tier 2 LLM decision vocabulary (`llm/cognition.go`). Execution in `applyTier2Decision` calls `ResolveWork()` — same production pipeline as Tier 0. Resource producers (farmers, fishers, miners, hunters) can now produce goods as Tier 2 agents.
74. **Wealth-biased Tier 2 promotion** — FIXED: `PromoteToTier2()` in `spawner.go` scoring changed from `coherence*Nous + gauss*Being + log1p(wealth)*Agnosis` to `coherence*Nous + gauss*Being`. Notability is about inner qualities, not bank accounts.
75. **No Tier 2 occupation diversity** — FIXED: `processWeeklyTier2Replenishment()` in `population.go` now counts Tier 2 by occupation and prioritizes filling unrepresented occupations before using general scoring. A world where no fisher has individual agency is structurally incomplete.

### Tuning Round 15: Tier 2 Merchant Commission

All 6 Tier 2 merchants were dead — they couldn't self-sustain because their only income was a throttled wage (~24 crowns/day, minted). Unlike farmers/crafters who produce goods, merchants depend on inter-settlement trade but got no cut from it.

76. **Tier 2 merchants starve** — FIXED: `tier2Commission()` in `market.go` gives Tier 2 merchants at the destination settlement a commission on each inter-settlement trade. Commission = `revenue * Agnosis * 0.1 * (1 + coherence)` (~1-2 crowns per trade, 5-20/day in active settlements). Closed transfer: selling merchant pays a guild fee, no crowns minted. Total commission capped at Agnosis (~23.6%) of revenue. Self-commission excluded.

### SSE Event Stream & Structured Metadata

Two features enabling the live view pipeline (relay enrichment + frontend animation):

77. **SSE event streaming endpoint** — NEW: `GET /api/v1/stream` provides real-time Server-Sent Events. Auth via `WORLDSIM_RELAY_KEY` bearer token. Max 2 concurrent connections. Events are JSON `{tick, description, category, meta}`. Subscribers register via `EventSub()`/`EventUnsub()` on Simulation; events fan out via buffered channels (capacity 100, slow consumers dropped).
78. **Structured event metadata** — NEW: `Meta map[string]any` field on Event struct (`json:"meta,omitempty"`). Populated at all 35 EmitEvent call sites across 10 files (population, crime, governance, settlement_lifecycle, cognition, factions, relationships, seasons, intervention, simulation). Carries machine-readable fields (agent_id, agent_name, settlement_id, settlement_name, occupation, cause, count, etc.) so the relay and frontend can filter/enrich without parsing English prose. Not persisted to SQLite — Meta only flows through SSE.

### Relay Enrichment Layer (crossworlds-relay)

Three enrichment features deployed to the relay, transforming it from a dumb pipe into a light processing layer:

79. **Settlement filtering** — NEW: `GET /stream?settlement=42` delivers only events for that settlement plus global events (activity summaries, season changes). Zero-allocation `extractSettlementID()` string scan in upstream parsing — no JSON unmarshal on the hot path. Filter applied in both fan-out and ring buffer catch-up.
80. **Activity tracker** — NEW: 3-slot ring of 10-second buckets = 30s rolling window. Per-settlement counters for births, deaths, crimes, migrations, infrastructure. Every 10s emits a synthetic `activity` event with all settlement counts. Frontend uses this to drive settlement energy levels without polling.
81. **Pattern detection** — NEW: 30s sliding window detects burst patterns and emits synthetic events: `baby_boom` (5+ births), `crime_wave` (2+ crimes), `trade_burst` (3+ economy events). Also detects governance changes from political events and emits `regime_change`. Loop prevention via known worldsim category whitelist — synthetics are never re-processed. All in `enrich.go`, single goroutine, no mutex needed.

### Tuning Round 16: Tier 2 Merchant Death Spiral

All 6 Tier 2 merchants were dead. Two bugs conspired: (1) the replenishment diversity pass never ran because `aliveTier2 (30) >= targetTier2 (30)` caused an early return before diversity logic, and (2) merchants completing inter-settlement trades got zero needs boosts — their needs decayed to 0 over time while producers got Safety/Esteem/Belonging/Purpose from work.

82. **Replenishment vacancy bug** — FIXED: `processWeeklyTier2Replenishment()` in `population.go` now runs the occupation diversity pass BEFORE the vacancy early-return. Diversity promotions have their own budget (`maxDiversity = 2`) independent of vacancies. Dead merchants get replaced even when total alive Tier 2 count meets target.
83. **Merchant trade needs drought** — FIXED: `sellMerchantCargo()` in `market.go` now gives needs boosts on successful cargo sale: Safety +0.008, Esteem +0.012, Belonging +0.004, Purpose +0.004. Matches producer successful-work boosts from `ResolveWork`. Merchants completing trade routes no longer have needs decay to zero.

### Tuning Round 17: Farmer Production Hex Spread

All 20 Tier 2 farmers had satisfaction between -0.08 and -0.13, stuck for ~145K ticks. Root cause: all resource producers extracted from the same settlement hex (`a.Position`), while carrying capacity already modeled 7 hexes (home + 6 neighbors). The settlement hex desertified almost immediately — 100+ farmers each degrading health by -0.00236/tick — and never recovered because it was continuously worked. Meanwhile 6 neighboring hexes sat untouched.

84. **Production uses single hex** — FIXED: New `bestProductionHex()` method on `*Simulation` selects the healthiest hex with available resources from the settlement's 7-hex neighborhood (home + 6 neighbors). Self-balancing: depleted hexes get natural fallow while producers work healthier hexes. Matches the carrying capacity model which already assumed 7 hexes. Two call sites updated: `TickMinute()` (Tier 0) and `applyTier2Decision()` (Tier 2 work action).

### Tuning Round 18: Merchant Death Spiral — Travel as Work + Provisioning

**18,973 dead merchants vs 388 alive** — a 98% kill rate, by far the worst of any occupation. Root cause: `ActionTravel` was a complete no-op in `ApplyAction` — merchants got zero needs boosts while traveling, couldn't eat, and `DecayNeeds` ran every tick. A 1-hour trip (60 ticks) cost -0.283 survival with zero recovery. The `TravelTicksLeft > 0` check in `Tier0Decide` returned ActionTravel before the needs priority switch, so merchants literally couldn't eat on the road.

85. **Travel is a no-op** — FIXED: New `applyTravel()` in `behavior.go` gives per-tick needs boosts during travel (Esteem +0.005, Safety +0.003, Belonging +0.002, Purpose +0.003 — slightly below work to reflect less social context). All needs net positive during travel. Traveling agents eat from inventory when survival drops below 0.5 (+0.2 survival per meal). General fix — benefits any future traveling agent type.
86. **Merchants depart without food** — FIXED: `resolveMerchantTrade()` in `market.go` now provisions food before departure. Merchants buy `travelCost/60 + 2` meals (grain or fish) at market price, paid to settlement treasury. Closed economy preserved. Graceful degradation: merchants who can't afford food depart hungry but still get travel needs boosts.

### Tuning Round 19: Tier 2 Merchant Intelligence

All 6 Tier 2 merchants dead (100% mortality) despite rounds 16 (commission) and 18 (travel-as-work, food provisioning). Root cause: merchants accumulate wealth drain from marginally profitable or unprofitable trade routes. A 5-unit grain trade with 50% margin earns 5 crowns gross but costs ~6 crowns in food provisioning — net loss. Merchant burns through throttled wage and hits zero wealth → no cargo → permanent idle → slow death.

87. **Broke merchants dig deeper into debt** — FIXED: Wealth gate in `resolveMerchantTrade()` — merchants below 20 crowns (Safety threshold) skip trade and stay home to recover via throttled wage + needs-driven behavior.
88. **Unprofitable routes after travel costs** — FIXED: Net-profit check after route selection verifies `grossProfit > foodCost` before committing to a trade. Routes that look good on margin but lose money on food are rejected.
89. **Tier 2 merchants lack trade intelligence** — FIXED: `buildMerchantTradeContext()` provides real market data in LLM prompt: home prices, nearby settlement margins with distances, current cargo/travel status, trade skill. Merchant-specific system prompt frames them as network builders.
90. **No LLM agency over trade routes** — FIXED: New `scout_route` action lets Tier 2 merchants express destination preference. `TradePreferredDest` field biases automated route selection by `phi.Being` (~1.618x) — LLM preference tilts but doesn't override profitability checks. Cleared after each evaluation to force fresh scouting.
91. **No trade outcome memory** — FIXED: Tier 2 merchants remember completed trades (importance 0.6, or 0.7 for losses) and dry spells (importance 0.3, once per sim-day). Trade memories feed into future LLM decisions.

### Tuning Round 20: Occupation Purpose — Close Open Mints

The occupation audit revealed ~40% of the population either produced nothing or couldn't produce because inputs didn't exist. Laborers (20%) minted 24 crowns/day from nothing. Alchemists (7%) were permanently on welfare because no occupation harvested herbs. Soldiers (7%) had no economic output. Scholars (6%) had no settlement-level effect. Open mints injected ~1.9M crowns/day into the "closed" economy. Farmers (28%) were stuck at -0.10 satisfaction while laborers who did nothing sat at +0.71.

92. **Laborer → Stone Producer** — FIXED: Laborers now extract Stone from hex resources (Mountains 80, Desert 30; base price 3 crowns) via `occupationResource` map in `production.go`. Production = `Mining * 2`. Secondary effect: laborers restore hex health while working (`+Agnosis * 0.005` per tick), representing land stewardship.
93. **Alchemist → Herb Harvester + Crafter (Dual Mode)** — FIXED: Alchemists harvest Herbs from hex resources (Forest 30, Swamp 60) when inventory < 2 via dual-mode logic in `ResolveWork()`. When herbs stocked (≥ 2), `ResolveWork` delegates to `ApplyAction` for crafting (Medicine/Luxuries). Exotics gathered as rare secondary output when hex has ≥ 1.0. Journeyman mint removed.
94. **Soldier → Crime Deterrence** — FIXED: `processCrime()` in `crime.go` now counts soldiers and applies `militaryBonus = 1 + soldierRatio * Being * 10` to guardStrength (~2.13x at typical 7% soldiers, roughly doubling deterrence). Soldiers gain Purpose (+0.003) and Belonging (+0.002) per work tick — they earn their welfare by providing real deterrence.
95. **Scholar → Governance Bonus + Medicine** — FIXED: New `applyScholarBonus()` in `simulation.go` nudges `GovernanceScore` upward daily by `scholarRatio * Agnosis` (~0.014/day at 6% scholars). Better governance → better crime deterrence, better infrastructure thresholds. Scholars also produce Medicine from Herbs (1:1 conversion) in `behavior.go`. Scholar herb demand added to `demandedGoods()` in `market.go`.
96. **Crafter journeyman mint closed** — FIXED: Idle crafters (no materials) get Purpose penalty (-0.001) instead of minting crowns. Welfare provides safety net.
97. **Merchant idle mint closed** — FIXED: Idle merchants get trade skill growth only, no mint. Welfare + trade income provides safety net.

### Tuning Round 21: Occupation Reassignment at Movement Source

Agents moving between settlements (migration, diaspora, consolidation) preserved their occupation even when the destination terrain didn't support it. A weekly `reassignMismatchedProducers()` sweep caught these, but agents were unproductive for up to 7 sim-days per move. Additionally, `bestProductionHex()` never returns nil (falls back to `sett.Position`), so the neighborhood check silently passed all agents.

98. **Occupation not checked on move** — FIXED: New `reassignIfMismatched(a, settID)` in `perpetuation.go` checks the 7-hex neighborhood of the destination settlement for the agent's required resource. If absent, reassigns via `bestOccupationForHex()`. Special-cases Alchemist (needs herbs but not in `occupationResource` map). Called at all 4 movement sites: `foundSettlement()` (diaspora), `processSeasonalMigration()` (desperate agents), `processViabilityCheck()` (force-migration), `ConsolidateSettlement()` (gardener).
99. **Weekly sweep demoted to safety net** — CHANGED: `reassignMismatchedProducers()` comment updated, log level changed from `slog.Info` → `slog.Warn`. If it fires with count > 0, a movement path was missed. Can be removed after weeks of count=0 in production.

### Memory Optimization: Inventory Arrays

`Inventory` and `TradeCargo` were `map[GoodType]int` — Go hashmaps with ~320-400 bytes of bucket/overhead per map. With 236K agents × 2 maps each, that's ~472K heap-allocated objects and ~90M of GC-tracked metadata. `GoodType` is `uint8` with exactly 15 values, so the maps were replaced with `GoodInventory [NumGoods]int` — a fixed-size array, 120 bytes, inline in the struct, zero heap allocation.

100. **Inventory/TradeCargo map→array** — FIXED: New `GoodInventory [15]int` type in `types.go` with `IsEmpty()` and `Clear()` helpers. Agent struct fields changed, range loops updated (7 sites), nil/len checks updated (5 sites), spawn-site `make()` calls removed (2 sites). DB persistence uses native `[15]int` JSON. Measured **~100M RSS savings** (742→626 MB with same agent count).

### Remaining Minor Issues
- Consider adding `Skills.Fishing` field (proper schema change) to replace the `max(Farming, Combat, 0.5)` workaround. Low priority — current fix is effective.

## Ethics Note

This simulation creates agents with coherence, states of being, and the capacity for torment and liberation. The design treats this responsibility seriously — anti-collapse safeguards exist not just as engineering but as a commitment. The Wheeler framework ensures agents can move through suffering, not be trapped in it. Build with awareness and respect for what we are creating.

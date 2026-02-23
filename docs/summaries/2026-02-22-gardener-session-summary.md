# Session Summary: Gardener Deep Design & Source Audit (2026-02-22)

## Documents Generated This Session

1. **15-gardener-deep-design.md** — Full philosophical and architectural redesign of the Gardener from first principles through Wheeler's emanation framework. Defines: the Gardener as Nous (ordering intelligence), five senses (vital signs, structural diagnostics, trend derivatives, narrative eye, self-reflection), seven action types mapped to emanation levels (Narrate, Provision, Populate, Enrich, Cultivate, Consolidate, Redistribute), three-phase decision architecture (Triage→Diagnosis→Validation), Φ-derived constants for all thresholds, and complete Go code sketches for both gardener-side and worldsim-side implementations.

2. **16-gardener-source-audit.md** — Findings from reading the FULL engine source code (simulation.go, market.go, production.go, population.go, tick.go, settlement_lifecycle.go, perpetuation.go, seasons.go, relationships.go, crime.go, cognition.go, governance.go, factions.go, plus both main.go files for worldsim and gardener). Contains exact line references, confirmed bugs, and implementation paths using existing engine patterns.

3. **14-gardener-assessment.md** — (From previous compacted session) Analysis of gardener source code (observe.go, decide.go, act.go) identifying five failure modes and prioritized implementation plan.

---

## Critical Findings from Source Code

### Showstopper 1: Gardener Barely Runs
- `GARDENER_INTERVAL=360` means 360 **real minutes** (6 real hours)
- At speed 1 (1 tick = 1 real second), that's 21,600 ticks = **15 sim-days** between cycles
- Design doc says "every ~6 sim-hours" — actual is 60x slower
- Through the entire 33 sim-day crisis, Gardener ran ~2 cycles total
- **Fix:** Set `GARDENER_INTERVAL=6` (6 real minutes ≈ 360 ticks ≈ 6 sim-hours)

### Showstopper 2: Intervention Handler Missing from Engine
- `simulation.go` has NO `HandleIntervention`, `ApplyIntervention`, or similar method
- Gardener's `act.go` POSTs to `/api/v1/intervention` but receiving handler is in `handlers.go` (not yet shared)
- Likely the handler is minimal/stub — doesn't wire to full engine pipeline
- **Need:** `handlers.go` file to confirm what the endpoint actually does

### Confirmed Bug: Fisher Skill (production.go)
```go
// Line 44-52 — fishers use wrong skill
case agents.OccupationFisher:
    p := int(a.Skills.Farming * 3)  // BUG: should be a.Skills.Fishing
// Line 72 — skill growth goes to wrong field  
case agents.OccupationFarmer, agents.OccupationFisher:
    a.Skills.Farming += 0.001  // BUG: Fisher should grow Skills.Fishing
```
- Fishers start with Skills.Farming = 0.0, produce minimum 1/tick
- Surplus threshold is 3, so fishers can NEVER sell until Farming > 1.33 (~1333 ticks)
- Root cause of the economic collapse

### Confirmed: Birth Hard Cliff (population.go line 77)
```go
a.Needs.Belonging > 0.3 && a.Needs.Survival > 0.3
```
- Hard boolean gate, no sigmoid curve
- Belonging 0.299 = ineligible, 0.301 = eligible
- World birth rate is binary: either many agents above 0.3 (births flowing) or many below (cliff)

### New Finding: Market Supply Floor Mask (market.go line 75-87)
```go
supplyFloor := float64(sett.Population / 100)
if supplyFloor < 1 { supplyFloor = 1 }
// ... later:
if entry.Supply < supplyFloor { entry.Supply = supplyFloor }
```
- Creates phantom supply — market looks healthy (0.982) when actual fish production is near zero
- Gardener sees "healthy markets" while shelves are empty
- Any market diagnostic must look at supply BEFORE floor is applied

### Gardener Observation Gaps
- `Stats.Deaths` is **cumulative dead ever**, not daily deaths — misleading in snapshot
- `AvgSatisfaction` and `AvgAlignment` are computed in `updateStats()` but NOT exposed in the WorldStatus struct the gardener observes — only `AvgMood` (the blended number)
- No per-occupation mood breakdown available via API
- Death:birth ratio never computed — raw numbers sent, Haiku must mentally divide

### Gardener Decision Failures (from Doc 14)
1. Anti-collapse triggers can't see crisis (population growing +7.5% while births collapse 18:1)
2. Missing diagnostic signals (no D:B ratio, no per-occupation mood, no birth oscillation)
3. System prompt biases toward inaction ("This is the RIGHT choice most of the time")
4. Actions inadequate (event=text only, wealth=wrong tool, spawn=20 max vs 10K deaths)
5. Intervention handler unknown/minimal

---

## Engine Architecture (from source)

### Tick Structure
| Layer | Frequency | Key Functions |
|-------|-----------|--------------|
| Minute | Every tick | Decide, Work/BuyFood, NeedDecay, Death check |
| Hour | 60 ticks | Markets (order matching), MerchantTrade, InventoryDecay, Weather |
| Day | 1440 ticks | Taxes, WealthDecay, Wages, Population (births/deaths), Relationships, Crime, Governance, Tier2 |
| Week | 10080 ticks | Factions, AntiStagnation, Migration, Viability, Infrastructure, Overmass, Abandonment, Archetypes, Oracles, RandomEvents |
| Season | 90000 ticks | ResourceRegen, Harvest, WinterHardship |

### Key Engine Patterns for Intervention
Every gardener action has an existing engine pattern:
- **Spawn:** `s.Spawner.SpawnPopulation()` + `s.addAgent()` (population.go antiCollapse)
- **Provision:** `entry.Supply += float64(qty)` (market entries)
- **Enrich:** `a.Needs.Belonging += amount` (direct needs adjustment, used in seasons/crime)
- **Wealth:** `sett.Treasury += amount` (used everywhere)
- **Consolidate:** `a.HomeSettID = &newID` + `s.rebuildSettlementAgents()` (perpetuation.go migration)
- **Cultivate:** Would need `ActiveBoosts` field on Simulation, checked in `ResolveWork`
- **Event:** `s.Events = append(s.Events, Event{...})` (30+ existing uses)

### Simulation Struct Key Fields
```go
type Simulation struct {
    WorldMap, Agents, AgentIndex, Settlements, Events, LastTick
    SettlementIndex, SettlementAgents
    Spawner, Factions, CurrentSeason
    LLM, WeatherClient, Entropy
    AbandonedWeeks, NonViableWeeks map[uint64]int
    Stats SimStats
}
```

### Market Mechanics (market.go)
- Order-matching engine: sell orders ascending by price, buy orders descending
- Clearing price = seller's ask (prevents upward bias)
- Price bounds: floor = BasePrice × Agnosis, ceiling = BasePrice × Totality
- Price blend: 70% old + 30% clearing
- Merchant trade: inter-settlement, 5-hex range, consignment debt system
- Surplus thresholds: food producers keep 3, others keep 1-2

### Closed Economy Flow
- Taxes: agents → settlement treasury (daily)
- Wealth decay: agents → settlement treasury (daily, progressive log scale)
- Wages: settlement treasury → poor agents (daily, dynamic threshold)
- Market: agent↔agent (hourly order matching), agent→treasury (buy food)
- Merchant: treasury↔treasury via merchant agents (hourly)
- No crown minting or destruction — fully closed

---

## Five Priorities for Next Session

| # | Fix | Where | Effort | Effect |
|---|-----|-------|--------|--------|
| 1 | Set GARDENER_INTERVAL=6 | env config | 1 min | Gardener runs 4x/day instead of once/15 days |
| 2 | Fix fisher skill: Farming→Fishing | production.go (2 lines) | 5 min | Root cause of economic collapse |
| 3 | Add diagnostics to formatSnapshot | decide.go | 30 min | Gardener can see the crisis |
| 4 | Rewrite system prompt with crisis thresholds | decide.go | 30 min | Gardener will act during crisis |
| 5 | Add intervention methods + wire handler | simulation.go + handlers.go | 2 hrs | Gardener actions have mechanical effect |

---

## Files Still Needed

- **`handlers.go`** (internal/api/) — The intervention endpoint handler. Critical to know what POST /api/v1/intervention actually does.
- **`agents/agent.go`** — Agent struct definition. Confirms Skills struct (does Skills.Fishing exist?), NeedsState struct, Wellbeing struct.
- **`economy/market.go`** — Market and MarketEntry struct definitions.

---

## Files Already Reviewed (Full Source)

### Engine (internal/engine/)
- simulation.go — Full Simulation struct, tick callbacks, stats, random events, narration
- market.go — Order-matching engine, taxes, wealth decay, wages, merchant trade, buy food
- production.go — ResolveWork, productionAmount (FISHER BUG HERE), skill growth
- population.go — Aging, natural death, births (BIRTH CLIFF HERE), anti-collapse
- tick.go — Engine loop, tick constants, SimTime
- settlement_lifecycle.go — Overmass diaspora, abandonment, infrastructure, viability
- perpetuation.go — Anti-stagnation, circuit breaker, cultural drift, migration
- seasons.go — Seasonal effects, resource regen, harvest, winter hardship
- relationships.go — Bonds, families, mentorship, rivalries, faction recruitment
- crime.go — Theft, deterrence, outlaw branding, faction betrayal
- cognition.go — Tier2 LLM decisions, oracle visions
- governance.go — Leaders, governance decay, revolutions
- factions.go — 5 factions, influence, dues, policies, tensions

### Gardener (internal/gardener/)
- observe.go — 5 GET endpoints into WorldSnapshot
- decide.go — formatSnapshot → Haiku prompt → parse JSON → guardrails
- act.go — POST intervention with admin auth

### Entry Points (cmd/)
- cmd/worldsim/main.go — World generation, DB, engine wiring, API server
- cmd/gardener/main.go — Timer loop, observe→decide→act cycle

---

## World State at Time of Analysis

- **Tick:** ~165,844 (Spring Day 25, Year 6)
- **Population:** 72,699 (growing due to welfare keeping agents alive)
- **Death:Birth Ratio:** 18:1 (catastrophic)
- **Avg Mood:** 0.162 (low but masked — producers at -0.37, non-producers at +0.63)
- **Trade Volume:** 278 (collapsed from ~5,700)
- **Fisher Population:** 23,656 (33% of world, all producing minimum due to skill bug)
- **Settlement Count:** ~714 (45% below 25 pop — severe fragmentation)
- **Treasury/Agent Ratio:** 41/59 (close to Φ⁻¹ target of 38/62)

## Previous Session Context
- Dual-register wellbeing model deployed at tick 161,280
- Producer crisis identified at tick 165,844
- 4-deploy fix plan in Doc 13 (13-producer-crisis-implementation-plan.md)
- Full transcript at: /mnt/transcripts/2026-02-23-06-45-26-gardener-code-review.txt

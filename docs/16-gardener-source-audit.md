# 16 — Gardener Source Audit: What the Code Actually Shows

**Date:** 2026-02-22  
**Source:** Full engine codebase reviewed — simulation.go, market.go, production.go, population.go, tick.go, settlement_lifecycle.go, perpetuation.go, plus gardener observe.go, decide.go, act.go, and both main.go files.

---

## Critical Finding: The Intervention Handler Is Missing

`simulation.go` contains no `HandleIntervention`, `ApplyIntervention`, or any method that receives and processes gardener actions. The `Simulation` struct has:

- `addAgent()` — registers a new agent in all indexes
- `rebuildSettlementAgents()` — reconstructs settlement→agent map
- `Spawner` — full agent creation pipeline
- `SettlementIndex`, `AgentIndex` — all the lookup structures

But no method wires the gardener's `POST /api/v1/intervention` to any of these. That endpoint lives in `handlers.go` (API layer, not provided). Based on the architecture pattern — API handlers call `sim.SomeMethod()` — the handler must be calling *something*, but we don't see it.

**Three possibilities:**
1. The handler is in `handlers.go` and directly manipulates `sim` fields (event insertion, treasury adjustment, simple spawn)
2. The handler exists but is a stub that returns `{"success": true}` without doing anything
3. The handler doesn't exist and the endpoint returns 404

We need to see `handlers.go` to confirm. But regardless, the handler is NOT using the full engine pipeline — if it were, there would be methods on `Simulation` like `ApplyProvision()`, `ApplySpawn()`, etc.

**Recommendation:** Share `handlers.go` so we can see exactly what happens when the gardener POSTs an intervention. This is the single most important missing piece.

---

## The Supply Floor Mask — Why Market Health Lies

`market.go` line 75:
```go
supplyFloor := float64(sett.Population / 100)
if supplyFloor < 1 {
    supplyFloor = 1
}
```

Then line 86:
```go
if entry.Supply < supplyFloor {
    entry.Supply = supplyFloor
}
```

This is why market health reads 0.982 during a crisis where actual fish production is near zero. The floor creates **phantom supply** — the market looks healthy because the numbers say "supply exists" when it doesn't. Real fish supply from fishers might be 0-1 units, but the floor bumps it to 1+ for every good in every settlement.

The Gardener sees `AvgMarketHealth: 0.982` and concludes markets are fine. They're not — they're empty shelves with a "fully stocked" sign.

**Impact on Gardener:** Any market health diagnostic must look at *actual supply vs demand* before the floor is applied, not after. The current observation pipeline inherits this mask.

---

## The Fisher Skill Bug — Confirmed in Code

`production.go` lines 44-52:
```go
case agents.OccupationFisher:
    p := int(a.Skills.Farming * 3)  // BUG: should be a.Skills.Fishing
    if p < 1 {
        p = 1
    }
    return p
```

And skill growth (line 72):
```go
case agents.OccupationFarmer, agents.OccupationFisher:
    a.Skills.Farming += 0.001
```

So fishers:
1. Start with coherence-seeded `Skills.Fishing` (e.g. 0.3-0.7)
2. Produce based on `Skills.Farming` which starts at 0.0
3. Get `int(0.0 * 3) = 0`, clamped to minimum 1 unit/tick
4. Slowly grow Farming at 0.001/tick
5. After 100 ticks (~1.7 hours), Farming = 0.1, production = `int(0.1 * 3) = 0`, still 1/tick
6. After 1000 ticks (~17 hours), Farming = 1.0, production = `int(1.0 * 3) = 3`

Surplus threshold for fishers is 3 (market.go line 258). So a fisher can **never sell** until their Farming skill exceeds 1.0 (after ~1000 ticks of working). And even then, only 0 surplus (3 produced - 3 kept = 0). They need Farming > 1.33 to sell 1 fish, which takes ~1333 ticks.

Meanwhile, Farmers produce `int(a.Skills.Farming * 3)` — same formula, same skill, starts correctly seeded. A farmer with Farming=0.5 produces 1/tick; Farming=2.0 produces 6/tick, surplus 3.

**The fix is a one-line change** in `production.go`:
```go
case agents.OccupationFisher:
    p := int(a.Skills.Fishing * 3)  // FIXED
```

And in skill growth:
```go
case agents.OccupationFarmer:
    a.Skills.Farming += 0.001
case agents.OccupationFisher:
    a.Skills.Fishing += 0.001  // FIXED: was grouped with Farmer
```

But the Gardener can't make this fix. Only a code deploy can.

---

## The Birth Cliff — Confirmed in Code

`population.go` line 77:
```go
if a.Alive && a.Age >= 18 && a.Age <= 45 && a.Health > 0.5 &&
    a.Needs.Belonging > 0.3 && a.Needs.Survival > 0.3 {
    eligibleParents = append(eligibleParents, a)
}
```

This is a hard boolean gate. At Belonging = 0.299, an agent is ineligible. At 0.301, eligible. No gradual curve. The entire birth rate of the world hinges on how many agents sit above 0.3 Belonging.

The Gardener's `enrich` action (proposed in Doc 15) directly addresses this — a +0.1 Belonging boost to a settlement where avg Belonging is 0.25 would push many agents above the threshold. But the current Gardener has no such action.

---

## What the Gardener Can Actually See

The gardener's `observe.go` calls 5 endpoints. From `simulation.go`, the `updateStats()` method computes:

```go
s.Stats.TotalPopulation = alive
s.Stats.TotalWealth = totalWealth
s.Stats.Deaths = deaths  // NOTE: cumulative dead, not daily deaths
s.Stats.AvgMood = totalMood / float32(alive)
s.Stats.AvgSatisfaction = totalSatisfaction / float32(alive)
s.Stats.AvgAlignment = totalAlignment / float32(alive)
s.Stats.AvgSurvival = totalSurvival / float32(alive)
```

Important: `Deaths` is **total dead agents ever**, not daily deaths. The gardener sees `Deaths: 89234` and `Births: 12583` — these are cumulative. The trend data (`stats/history`) has per-snapshot values, but the system prompt uses the status endpoint values which are cumulative.

Also: `AvgSatisfaction` and `AvgAlignment` are computed and stored but NOT included in the `WorldStatus` struct that the gardener observes (see observe.go `WorldStatus`). The gardener only gets `AvgMood` — the blended number that hides the satisfaction/alignment split.

---

## The Gardener's Timer Is in Real Minutes, Not Sim-Hours

Gardener `main.go`:
```go
intervalMin := envIntOrDefault("GARDENER_INTERVAL", 360)
interval := time.Duration(intervalMin) * time.Minute
```

Default: 360 real minutes = 6 real hours. But the worldsim runs at speed 1 (1 tick = 1 real second = 1 sim minute). So 6 real hours = 21,600 ticks = 15 sim-days.

The gardener runs once every **15 sim-days**, not every 6 sim-hours as designed. The design doc says "every ~6 sim-hours" but the interval is in real-world minutes. At speed 1, 6 sim-hours = 360 sim-minutes = 360 real seconds = 6 real minutes.

`GARDENER_INTERVAL=6` would give ~6 sim-hours between cycles. The default of 360 means the gardener barely runs.

**This might be the single biggest reason the gardener has had no effect** — it's cycling once every 15 sim-days instead of 4 times per sim-day. Through the 47K-tick crisis (~33 sim-days), the gardener has run approximately **2 cycles total**, not dozens.

---

## The Full Engine Tick Structure

From `tick.go` and `simulation.go`:

| Layer | Frequency | Functions | Gardener Relevance |
|-------|-----------|-----------|-------------------|
| Minute | Every tick | Decide, Work, Buy, Die, NeedDecay | Where production happens |
| Hour | 60 ticks | Markets, MerchantTrade, InventoryDecay, Weather | Where trade happens |
| Day | 1440 ticks | Taxes, WealthDecay, Wages, Population, Relationships, Crime, Governance, Tier2 | Where births/deaths happen |
| Week | 10080 ticks | Factions, AntiStagnation, Migration, Viability, Infrastructure, Overmass, Abandonment, Archetypes, Oracles, RandomEvents | Where structural changes happen |
| Season | 90000 ticks | ResourceRegen, Harvest, WinterHardship | Where seasonal effects happen |

The Gardener should observe and intervene at the **daily** timescale — that's where population dynamics play out. Weekly is too slow for crisis response. The ideal cycle is 1440 ticks (1 sim-day) or even 360 ticks (6 sim-hours).

---

## Existing Engine Patterns for Intervention

The engine already has patterns for every intervention type the Gardener needs:

### Pattern 1: Spawning Agents (population.go → `processAntiCollapse`)
```go
refugees := s.Spawner.SpawnPopulation(uint32(needed), sett.Position, sett.ID, terrain)
for _, r := range refugees {
    r.BornTick = tick
    s.addAgent(r)
}
sett.Population = uint32(aliveCount + needed)
```
This is the exact code the intervention handler should use for `spawn`. `SpawnPopulation` creates fully-initialized agents with soul, coherence, skills, occupation, inventory.

### Pattern 2: Injecting Goods (population.go → `processAntiCollapse`)
```go
for _, a := range settAgents {
    if a.Alive && a.Needs.Survival < 0.1 {
        a.Inventory[agents.GoodGrain] += 3
    }
}
```
Famine relief already injects food directly. A `provision` action would inject into the market `entry.Supply` instead of agent inventory.

### Pattern 3: Inserting Events (everywhere)
```go
s.Events = append(s.Events, Event{
    Tick:        tick,
    Description: desc,
    Category:    "gardener",
})
```
This pattern is used in 30+ places. Trivial.

### Pattern 4: Adjusting Treasury (governance.go, market.go)
```go
sett.Treasury += amount
```
Direct treasury manipulation is used by tax collection, wealth decay, wages, and revolution seizure.

### Pattern 5: Adjusting Agent Needs (seasons.go → `winterHardship`)
```go
a.Wellbeing.Satisfaction -= 0.1
```
Direct needs adjustment is used by winter hardship, crime effects, work production. An `enrich` action would do `a.Needs.Belonging += amount`.

### Pattern 6: Migration (perpetuation.go → `processSeasonalMigration`)
```go
a.HomeSettID = &newID
a.Position = target.Position
// ... then:
s.rebuildSettlementAgents()
```
Migration with settlement map rebuild. A `consolidate` action uses exactly this pattern.

### Pattern 7: Production Boost (seasons.go → `autumnHarvest`)
```go
bonus := int(a.Skills.Farming * 10)
a.Inventory[agents.GoodGrain] += bonus
```
Seasonal harvest gives farmers a production bonus. A `cultivate` action would store a temporary multiplier checked in `ResolveWork`.

**Every single gardener action has an existing engine pattern.** There's nothing novel required — just wiring the intervention endpoint to call these patterns.

---

## Implementation: The Minimum Viable Gardener Upgrade

### Step 1: Fix `GARDENER_INTERVAL` (1 minute change)

Set `GARDENER_INTERVAL=6` in the gardener's environment. This gives ~6 sim-minute cycles (close to 6 sim-hours at speed 1, since 6 real minutes = 360 ticks = 6 sim-hours). Or better: make it sim-tick-aware by having the gardener check the worldsim's current tick and only run when TicksPerSimDay/4 have elapsed.

### Step 2: Fix the Fisher Skill Bug (2 lines in production.go)

This is the root cause of the crisis. The gardener shouldn't need to defend the world against a code bug — fix the bug. Change:
```go
case agents.OccupationFisher:
    p := int(a.Skills.Fishing * 3)  // was: a.Skills.Farming
```
```go
case agents.OccupationFisher:
    a.Skills.Fishing += 0.001  // was: grouped with Farmer → a.Skills.Farming
```

### Step 3: Add Computed Diagnostics to `formatSnapshot()` (decide.go, 30 min)

Using existing snapshot data — no API changes:
```go
// Death:birth ratio from history
if len(snap.History) > 0 {
    last := snap.History[len(snap.History)-1]
    dbr := float64(last.Deaths) / math.Max(float64(last.Births), 1)
    fmt.Fprintf(&b, "\n## Diagnostics\n")
    fmt.Fprintf(&b, "Death:Birth Ratio: %.1f:1", dbr)
    if dbr > 4.236 { b.WriteString(" ⚠ CRITICAL") }
    b.WriteString("\n")
    
    // Birth trend
    fmt.Fprintf(&b, "Birth trend: ")
    for _, h := range snap.History {
        fmt.Fprintf(&b, "%d ", h.Births)
    }
    b.WriteString("\n")
}

// Settlement fragmentation
small := 0
for _, st := range snap.Settlements {
    if st.Population < 25 { small++ }
}
fmt.Fprintf(&b, "Settlements below 25 pop: %d/%d (%.0f%%)\n",
    small, len(snap.Settlements),
    float64(small)/float64(len(snap.Settlements))*100)

// Trade per capita
tpc := float64(snap.Economy.TradeVolume) / math.Max(float64(snap.Status.Population), 1)
fmt.Fprintf(&b, "Trade per capita: %.4f\n", tpc)
```

### Step 4: Rewrite System Prompt (decide.go, 30 min)

Replace the "do nothing most of the time" bias with crisis detection logic. See Doc 15 Section IV for the full proposed prompt.

### Step 5: Raise Spawn Cap (decide.go, 1 line)

```go
if d.Intervention.Count > 100 { // was 20
    d.Intervention.Count = 100
}
```

### Step 6: Add Intervention Methods to Simulation (simulation.go or new intervention.go)

```go
// ApplyGardenerSpawn creates agents using the full engine pipeline.
func (s *Simulation) ApplyGardenerSpawn(settID uint64, count int) error {
    sett, ok := s.SettlementIndex[settID]
    if !ok { return fmt.Errorf("settlement %d not found", settID) }
    
    hex := s.WorldMap.Get(sett.Position)
    terrain := world.TerrainPlains
    if hex != nil { terrain = hex.Terrain }
    
    agents := s.Spawner.SpawnPopulation(uint32(count), sett.Position, settID, terrain)
    for _, a := range agents {
        a.BornTick = s.LastTick
        s.addAgent(a)
    }
    sett.Population += uint32(count)
    
    s.Events = append(s.Events, Event{
        Tick:        s.LastTick,
        Description: fmt.Sprintf("%d immigrants arrive in %s", count, sett.Name),
        Category:    "gardener",
    })
    return nil
}

// ApplyGardenerProvision injects goods into a settlement market.
func (s *Simulation) ApplyGardenerProvision(settID uint64, good agents.GoodType, qty int) error {
    sett, ok := s.SettlementIndex[settID]
    if !ok { return fmt.Errorf("settlement %d not found", settID) }
    if sett.Market == nil { return fmt.Errorf("settlement has no market") }
    
    entry, ok := sett.Market.Entries[good]
    if !ok { return fmt.Errorf("good not found in market") }
    
    entry.Supply += float64(qty)
    
    s.Events = append(s.Events, Event{
        Tick:        s.LastTick,
        Description: fmt.Sprintf("A merchant caravan brings %d goods to %s", qty, sett.Name),
        Category:    "gardener",
    })
    return nil
}

// ApplyGardenerEnrich gives a one-time needs boost to all agents in a settlement.
func (s *Simulation) ApplyGardenerEnrich(settID uint64, need string, amount float32) error {
    agents := s.SettlementAgents[settID]
    for _, a := range agents {
        if !a.Alive { continue }
        switch need {
        case "Belonging": a.Needs.Belonging += amount
        case "Survival":  a.Needs.Survival += amount
        case "Safety":    a.Needs.Safety += amount
        case "Esteem":    a.Needs.Esteem += amount
        case "Purpose":   a.Needs.Purpose += amount
        }
        clampAgentNeeds(&a.Needs)
    }
    return nil
}

// ApplyGardenerWealth adjusts a settlement treasury.
func (s *Simulation) ApplyGardenerWealth(settID uint64, amount int64) error {
    sett, ok := s.SettlementIndex[settID]
    if !ok { return fmt.Errorf("settlement %d not found", settID) }
    
    if amount > 0 {
        sett.Treasury += uint64(amount)
    } else {
        reduce := uint64(-amount)
        if reduce > sett.Treasury { reduce = sett.Treasury }
        sett.Treasury -= reduce
    }
    return nil
}
```

### Step 7: Wire Intervention Endpoint in handlers.go

In the API handler for `POST /api/v1/intervention`:
```go
case "spawn":
    settID := findSettlementIDByName(sim, intervention.Settlement)
    err = sim.ApplyGardenerSpawn(settID, intervention.Count)
case "wealth":
    settID := findSettlementIDByName(sim, intervention.Settlement)
    err = sim.ApplyGardenerWealth(settID, intervention.Amount)
case "event":
    sim.Events = append(sim.Events, engine.Event{
        Tick: sim.LastTick,
        Description: intervention.Description,
        Category: "gardener",
    })
case "provision":
    // NEW
case "enrich":
    // NEW
```

---

## What I Need from You to Finish This

1. **`handlers.go`** — The API handler file. This tells us exactly what the current intervention endpoint does (or doesn't do). Without it, we're guessing about the last mile.

2. **`agents/agent.go`** or equivalent — The Agent struct definition. I can see how agents are used but need to confirm the exact `Skills` struct (does `Skills.Fishing` exist?) and the `NeedsState` struct.

3. **`economy/market.go`** — The `Market` and `MarketEntry` structs. I can see they have `Supply`, `Demand`, `Price`, `BasePrice`, `Entries`, `TradeCount`, `MostTradedGood`, but the full struct definition confirms what fields exist.

These three files complete the picture and let me write exact, deployable code for the full Gardener upgrade.

---

## Summary: The Five Things That Would Save the World

In priority order:

| # | Fix | Where | Effort | Effect |
|---|-----|-------|--------|--------|
| 1 | Set GARDENER_INTERVAL=6 | env config | 1 min | Gardener runs 4x/day instead of once/15 days |
| 2 | Fix fisher skill: Farming→Fishing | production.go | 2 lines | Root cause of economic collapse |
| 3 | Add diagnostics to formatSnapshot | decide.go | 30 min | Gardener can see the crisis |
| 4 | Rewrite system prompt with crisis thresholds | decide.go | 30 min | Gardener will act during crisis |
| 5 | Add intervention methods to Simulation | simulation.go + handlers.go | 2 hrs | Gardener actions have mechanical effect |

Items 1-2 are **code fixes** that address the root cause. Items 3-5 are **gardener upgrades** that give it the ability to defend the world against future crises that aren't code bugs.

The deeper philosophical point: the Gardener shouldn't be defending the world against code bugs — that's the developer's job. The Gardener should be defending the world against *emergent crises* that arise from the interaction of correct code. The fisher bug needs to be fixed by deploying code. The Gardener needs to be upgraded so that *after* the bug fix, it can handle the next crisis — the one we can't predict because it emerges from complexity.

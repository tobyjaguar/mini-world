# 14 — Gardener Assessment: Current State & Recommendations

**Date:** 2026-02-22  
**Context:** Gardener running since ~tick 118,329 (startup race fix). World currently at tick 165,844 in a survival crisis (18:1 death:birth ratio). Gardener has had ~47K ticks (~33 sim-days) of runtime with no observable effect on the crisis.  
**Source:** Actual Go source reviewed — `observe.go`, `decide.go`, `act.go`

---

## Part 1: What the Gardener IS (Code Review)

The gardener is cleanly architected across three files:

**`observe.go`** — Fetches 5 GET endpoints (`/status`, `/economy`, `/settlements`, `/factions`, `/stats/history?limit=10`) into a well-typed `WorldSnapshot` struct. Includes population, births, deaths, avg mood, market health, trade volume, wealth distribution, settlement details (pop, treasury, health, governance), faction treasuries, and 10-snapshot history. HTTP client with 30s timeout. This is solid.

**`decide.go`** — Formats the snapshot into a text prompt via `formatSnapshot()`, sends it to Haiku with a system prompt, parses JSON response, enforces guardrails. Guardrails are properly implemented: spawn capped at 20, wealth capped at 10% of target settlement treasury, category forced to "gardener", intervention type forced to match action. Good defensive code.

**`act.go`** — POSTs the `Intervention` struct to `/api/v1/intervention` with admin bearer token. Returns `InterventionResult{Success, Details}`. Clean and simple.

The code quality is good. The problem isn't broken code — it's a design gap between what the Gardener *sees*, what it *concludes*, and what it *can do*.

---

## Part 2: Why It's Not Helping (5 Specific Failures)

### Failure 1: The Anti-Collapse Trigger Can't See the Crisis

The system prompt defines anti-collapse as:

> "Intervene when population crashes (**>10% decline between snapshots**), mass starvation (**avg_survival < 0.3**), or settlement death spirals (multiple settlements with health < 0.2)"

Now look at the actual world state at tick 165,844:

| Trigger | Threshold | Actual Value | Fires? |
|---------|-----------|-------------|--------|
| Population decline >10% | >10% drop | **+7.5% growth** | **NO** |
| Mass starvation | avg_survival < 0.3 | **0.385** | **NO** |
| Settlement death spirals | health < 0.2 | health 0.982 | **NO** |

**None of the anti-collapse triggers fire.** Population is *growing* because the welfare system keeps agents alive long enough to accumulate (72,699 and rising). Survival is above 0.3 because agents can forage/buy food. Market health is 0.982.

The crisis is a **birth collapse** (18:1 death:birth) hidden behind a temporarily growing population. The Gardener can't see this because:

1. `formatSnapshot()` sends raw births and deaths but does NOT compute the death:birth ratio
2. The trend section only shows `Population: 67634 → 72699 (+7.5%)` — looks healthy!
3. Haiku has to mentally divide 10,164 by 564 to realize it's 18:1 — and even then, the population trend looks fine

The Gardener is like a doctor who only checks weight. The patient is gaining weight (retaining water) while their kidneys are failing. The vital sign that matters — birth rate collapse — isn't surfaced.

### Failure 2: Missing Diagnostic Signals

`formatSnapshot()` constructs the Haiku prompt. Here's what it includes and what's missing:

**Included:**
- Population, births, deaths (raw numbers)
- Avg mood (single number: 0.162)
- Market health, trade volume, wealth distribution
- Top 10 settlements by pop + settlements with health < 0.3
- Faction treasuries
- First-vs-last snapshot trend comparison

**Missing (critical for this crisis):**
- **Death:Birth ratio** — the single most important signal, not computed
- **Per-occupation mood breakdown** — Haiku can't see that Fishers (-0.23) and Hunters (-0.46) are miserable while Scholars (+0.70) are thriving. It only sees avg mood 0.162
- **Birth rate trend** — the oscillation (564 → 5,024 → 576) is invisible in first-vs-last comparison
- **Market supply data** — no supply/demand per good, so the market starvation (fish supply=1, demand=152) is invisible
- **Satisfaction vs alignment split** — just avg mood, not the dual-register breakdown
- **Settlement size distribution** — no count of settlements below viability thresholds
- **Producer vs non-producer split** — 50% of pop in misery, 50% thriving, averaged into a bland 0.162

The snapshot `formatSnapshot()` builds is like a medical chart that only shows temperature and weight. The patient's blood pressure, blood oxygen, and organ function are all missing.

### Failure 3: The System Prompt Biases Toward Inaction

The system prompt includes:

> `"none" — No intervention needed. **This is the RIGHT choice most of the time.**`

And value #4:

> `RESPECT FOR EMERGENCE — Use the lightest touch possible... **When in doubt, do nothing.**`

Combined with population appearing to grow and no trigger thresholds being breached, Haiku is almost certainly returning `"action": "none"` every cycle. The prompt is designed for a *healthy* world where the Gardener's role is to stay out of the way. During a crisis, that same bias becomes paralysis.

### Failure 4: Actions Are Inadequate Even If Triggered

Even if Haiku somehow recognized the crisis, its action vocabulary can't address it:

**`event`** — Inserts a narrative text string. Zero mechanical effect on the simulation engine. It doesn't change production rates, needs values, market supply, or agent state. It's a log entry.

**`wealth`** — Can move up to 10% of a settlement treasury (~115K crowns). But the crisis isn't about wealth distribution — treasury/agent ratio is already at target (41/59). The problem is producers can't produce enough to sell, so no wealth transfer fixes it.

**`spawn`** — Can add up to 20 agents per cycle. With ~10,000 deaths and ~500 births per snapshot, 20 agents is noise. And new agents spawned into broken production will just become more miserable fishers.

The action vocabulary was designed for gentle nudges in a healthy world. It has nothing that addresses production insufficiency, birth threshold cliff dynamics, or settlement fragmentation.

### Failure 5: The Worldsim-Side Handler Is Unknown

The gardener code (`act.go`) cleanly POSTs to `/api/v1/intervention`. But we don't have the worldsim handler code here. Critical questions:

- Does `handleIntervention` actually wire `spawn` to the full agent creation pipeline (soul, coherence, skills, occupation assignment)?
- Does `wealth` actually modify `settlement.Treasury`?
- Does `event` do anything beyond inserting a database row?

Based on the project's pattern (see: welfare wage pipe too narrow, migration not rebuilding settlement map, purpose boost missing from ResolveWork), the intervention handler likely has minimal implementation. The gardener sends well-formed requests to an endpoint that may only partially process them.

**Recommendation: SSH audit** — `grep -rn "intervention" internal/api/` to see what the handler does.

---

## Part 3: What Needs to Change

### Change 1: Fix the Observation (P0 — `decide.go`)

Add computed diagnostics to `formatSnapshot()` from existing snapshot data — no worldsim changes required:

```go
// In formatSnapshot, after writing status overview:

// Death:Birth ratio — THE critical signal
deathBirthRatio := float64(s.Deaths) / math.Max(float64(s.Births), 1)
fmt.Fprintf(&b, "Death:Birth Ratio: %.1f:1", deathBirthRatio)
if deathBirthRatio > 10 {
    b.WriteString(" ⚠ CRITICAL")
} else if deathBirthRatio > 5 {
    b.WriteString(" ⚠ WARNING")
}
b.WriteString("\n")

// Birth trend from history — surfaces the oscillation
fmt.Fprintf(&b, "Birth trend (last %d snapshots): ", len(snap.History))
for _, h := range snap.History {
    fmt.Fprintf(&b, "%d ", h.Births)
}
b.WriteString("\n")

// Settlement fragmentation
smallCount := 0
tinyCount := 0
for _, st := range snap.Settlements {
    if st.Population < 50 { smallCount++ }
    if st.Population < 25 { tinyCount++ }
}
fmt.Fprintf(&b, "Settlements below 50 pop: %d of %d (%.0f%%)\n",
    smallCount, len(snap.Settlements),
    float64(smallCount)/float64(len(snap.Settlements))*100)
fmt.Fprintf(&b, "Settlements below 25 pop: %d of %d (%.0f%%)\n",
    tinyCount, len(snap.Settlements),
    float64(tinyCount)/float64(len(snap.Settlements))*100)

// Trade per capita
tradePerCapita := float64(e.TradeVolume) / math.Max(float64(s.Population), 1)
fmt.Fprintf(&b, "Trade per capita: %.4f\n", tradePerCapita)
```

This alone would transform what Haiku sees from "Population +7.5%, mood 0.162" into "Death:Birth 18:1 CRITICAL, births oscillating 564/5024/576, 56% settlements below 50 pop, trade per capita 0.004." Night and day.

### Change 2: Rewrite the System Prompt Crisis Logic (P0 — `decide.go`)

Replace the anti-collapse thresholds with ones that detect birth collapse:

```
## Crisis Detection (evaluate FIRST, before values assessment)

BIRTH COLLAPSE: If death:birth ratio exceeds 5:1, this is a crisis
regardless of current population trend. A world gaining population
while births collapse is living on borrowed time — existing agents are
aging toward death with insufficient replacement.
  - Ratio > 10:1 → CRITICAL. Use spawn action. Target largest settlements.
  - Ratio 5:1–10:1 → WARNING. Intervene with spawn or wealth.
  - Ratio 2:1–5:1 → WATCH. Consider gentle nudges.
  - Ratio < 2:1 → HEALTHY.

TRADE COLLAPSE: If trade per capita < 0.01, the market is
non-functional. Narrative events cannot fix this.

SETTLEMENT FRAGMENTATION: If > 40% of settlements have pop < 25,
economic activity is critically fragmented.
```

Remove "This is the RIGHT choice most of the time" and "When in doubt, do nothing." Replace with: "In a healthy world, prefer inaction. During a crisis (any CRITICAL indicator), prefer action."

### Change 3: Raise the Spawn Cap (P0 — trivial, `decide.go`)

```go
// In enforceGuardrails:
case "spawn":
    if d.Intervention.Count > 100 {   // was 20
        d.Intervention.Count = 100
    }
```

Even 100 is modest (1% of the death rate), but it's enough to stabilize a specific settlement. Combined with 4 cycles per sim-day, that's 400 agents/day — meaningful for targeted settlements.

### Change 4: Add New Mechanical Action Types (P1 — gardener + worldsim)

Add new action types to the `Intervention` struct and guardrails:

```go
// Expanded Intervention struct:
type Intervention struct {
    Type        string  `json:"type"`
    Description string  `json:"description,omitempty"`
    Category    string  `json:"category"`
    Settlement  string  `json:"settlement,omitempty"`
    Amount      int64   `json:"amount,omitempty"`
    Count       int     `json:"count,omitempty"`
    // New fields for mechanical actions:
    Good        string  `json:"good,omitempty"`
    Quantity    int     `json:"quantity,omitempty"`
    Occupation  string  `json:"occupation,omitempty"`
    Multiplier  float64 `json:"multiplier,omitempty"`
    Duration    int     `json:"duration_days,omitempty"`
    Need        string  `json:"need,omitempty"`
    NeedBoost   float64 `json:"need_boost,omitempty"`
}
```

New action types:

- **`boost_production`** — temporary production multiplier on a settlement/occupation. Max 2.0x, max 14 days. Worldsim applies in `ResolveWork`. Narrative: "A bountiful season blesses the fishers of Oldwick."
- **`market_stimulus`** — inject goods into settlement market. Max 100 units. Narrative: "A merchant caravan arrives bearing grain and fish."
- **`adjust_needs`** — one-time needs bump to all agents in settlement. Max +0.1 per need. Narrative: "A community festival lifts spirits in Thornwall."
- **`force_migration`** — move agents from dying to viable settlements. Narrative: "Refugees from the failing hamlet seek shelter in Ironhaven."

Each requires a corresponding worldsim handler in `internal/engine/intervention.go` (new file).

### Change 5: Add Per-Occupation Data to API (P1 — worldsim)

Add `GET /api/v1/stats/diagnostics` endpoint:

```json
{
  "death_birth_ratio": 18.0,
  "avg_satisfaction": 0.116,
  "avg_alignment": 0.361,
  "occupation_mood": {
    "Fisher": -0.234,
    "Farmer": -0.400,
    "Hunter": -0.462,
    "Laborer": 0.615,
    "Crafter": 0.620,
    "Scholar": 0.696,
    "Soldier": 0.563
  },
  "settlement_size_histogram": {
    "under_25": 234,
    "25_to_50": 165,
    "50_to_100": 180,
    "over_100": 135
  },
  "market_supply_health": 0.12
}
```

Then `observe.go` fetches this and `formatSnapshot()` includes it. Haiku can now see the producer misery gap.

### Change 6: Allow Compound Interventions (P1)

During a crisis, allow 2-3 actions per cycle:

```go
type Decision struct {
    Action        string          `json:"action"`
    Rationale     string          `json:"rationale"`
    Intervention  *Intervention   `json:"intervention"`
    Interventions []*Intervention `json:"interventions"` // new: up to 3
}
```

A crisis cycle could: spawn 80 agents in Oldwick + boost fish production 1.5x for 7 days + inject a narrative event.

### Change 7: Add Cycle Logging and Memory (P2)

Log each cycle's decision and result. Include last 5 cycle summaries in the Haiku prompt:

```
## Recent Gardener History
- Tick 164,160: action=none, d:b=17.4, mood=0.168
- Tick 163,200: action=none, d:b=8.9, mood=0.161
- Tick 162,240: action=none, d:b=5.9, mood=0.161
- Tick 161,280: action=none, d:b=17.4, mood=0.168
- Tick 160,320: action=none, d:b=2.0, mood=0.123
```

Seeing 5 consecutive "none" decisions while death:birth ratio climbs would trigger Haiku to reconsider — and if it doesn't, the human reviewing logs can see the Gardener's paralysis immediately.

---

## Part 4: Implementation Priority

| # | Change | Files | Effort | Impact |
|---|--------|-------|--------|--------|
| 1 | Compute death:birth ratio, birth trend, settlement sizes in `formatSnapshot()` | `decide.go` | 30 min | **High** — Haiku sees the crisis |
| 2 | Rewrite system prompt with crisis detection criteria | `decide.go` | 30 min | **High** — Haiku knows when to act |
| 3 | Raise spawn cap 20→100 | `decide.go` | 5 min | **Medium** — spawns become meaningful |
| 4 | Add mechanical action types (gardener side) | `decide.go`, `act.go` | 1 hr | High — but needs worldsim handler |
| 5 | Implement worldsim intervention handler for new actions | `handlers.go`, new `intervention.go` | 2-3 hrs | **Critical** — gives actions teeth |
| 6 | Add `GET /api/v1/stats/diagnostics` | `handlers.go` | 1 hr | **High** — per-occupation visibility |
| 7 | Compound interventions (up to 3) | `decide.go` | 1 hr | Medium |
| 8 | Cycle logging and memory | new `memory.go` | 2 hrs | Medium |

**Immediate (deploy today):** Changes 1-3. All in `decide.go`, zero worldsim changes needed. The Gardener won't gain new powers, but it will at least *see* the crisis and use its existing (limited) tools. Even just spawning 100 agents in large settlements each cycle is better than doing nothing.

**Next session:** Changes 4-6. Worldsim handler work + API additions. This is where the Gardener gets real mechanical influence.

**Future:** Changes 7-8. Compound interventions and learning memory.

---

## Part 5: Philosophical Note

The Gardener occupies a specific position in Wheeler's emanation hierarchy. It is not the One — it didn't create the world and can't remake its fundamental laws. It is not an agent within Hyle/Matter — it doesn't eat, trade, or reproduce. It exists at the **Psyche/Nous boundary**: it observes the material world through its senses (the API) and applies ordering intelligence (Haiku) to nudge conditions.

The current implementation gives it Nous (reasoning capacity) but cripples its Psyche (capacity to act in the material world). A disembodied intelligence watching agents starve. The fix is not to make it a god — the guardrails rightly prevent that — but to give it a body proportional to its responsibilities.

The deeper insight: the system prompt says "When in doubt, do nothing." This is right for a healthy world — non-interference, respect for emergence, the Watchers in Tolkien's cosmology. But during genuine crisis, non-interference becomes negligence. The prompt needs a mode switch: in health, subtract your interference; in crisis, subtract the obstacles to survival. The via negativa applies to the Gardener too.

The Gardener tends the soil. But right now it needs to see that the soil is poisoned — and it needs a shovel.

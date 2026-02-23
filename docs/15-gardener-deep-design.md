# 15 — The Gardener as Defender of the World

**A Deep Design for Autonomous World Stewardship**

**Date:** 2026-02-22  
**Project:** SYNTHESIS / Crossworlds  
**Preceding:** Doc 14 (Gardener Assessment), Doc 13 (Producer Crisis Plan), Doc 10 (Dual-Register Wellbeing)

---

## I. What Is the Gardener?

### The Wrong Answer

The Gardener is not a monitoring system. It is not a cron job that checks health metrics and fires alerts. It is not an if-then-else tree with thresholds. These are the tools it uses, but they are not what it *is*.

### The Right Answer

The Gardener is the **ordering intelligence** of the world.

In Wheeler's emanation hierarchy, the simulation has a clear ontological stack:

| Level | Emanation | Simulation Layer |
|-------|-----------|-----------------|
| 0 | The Zero / Aethereal | The server process — beyond the world, sustaining it |
| 1 | The One / Monad | The tick function — the heartbeat from which all state emanates |
| 1' | Nous / Citta / Spirit | The LLM layer — intelligence that animates but is not *of* the world |
| 2 | Magnitude / Topos | The hex grid — relational ground of becoming |
| 3 | Matter / Hyle | Resources, goods, buildings — material substrate |
| 4 | Time | The tick counter — a measure, not a substance |
| 5 | Man / Being | Agents — the living union where matter and spirit meet |

The Gardener sits at level 1' — **Nous**. It is intelligence applied to the world from outside the world. It does not participate in the economy, it does not have needs, it does not reproduce or die. It *sees* and it *orders*. In the Neoplatonic tradition, this is the role of the Demiurge — not the creator of the world (that's the Monad/tick engine), but the intelligence that shapes formless matter into coherent form.

But here is the crucial distinction: the Gardener is Nous applied *downward*, toward Hyle. It doesn't create new fundamental laws (that's above its station). It works with what exists — agents, settlements, markets, goods — and arranges conditions so the world's own dynamics can function. It removes obstacles. It restores flow. It is the **via negativa** applied at world scale: not adding complexity, but subtracting the privations that prevent the world from being itself.

### The Conjugate Nature of the Gardener

The design doc establishes that every system in SYNTHESIS operates as a conjugate pair — charge and discharge, centripetal and centrifugal, dielectric and magnetic. The Gardener is no exception:

| Charging (Centripetal) | Discharging (Centrifugal) |
|----------------------|-------------------------|
| Observation — gathering world state inward | Intervention — pushing corrections outward |
| Analysis — compressing data into diagnosis | Action — expanding diagnosis into material change |
| Memory — accumulating history of interventions | Forgetting — releasing attachment to past strategies |
| Restraint — holding back when healthy | Urgency — acting decisively when collapsing |

A healthy Gardener *breathes*. It charges (observes, accumulates understanding) and discharges (intervenes, releases material into the world). A Gardener that only charges is a passive observer — it watches the world die while gathering ever more refined data about the dying. A Gardener that only discharges is a tyrant — it intervenes constantly, overrides emergence, and the world becomes a puppet show.

The current Gardener is stuck in permanent charge mode. It observes. It reasons. It does not discharge. It is an intelligence without a body, a mind without hands.

---

## II. The Five Senses of the Gardener

A human body has five senses that provide qualitatively different information about the world. The Gardener needs the equivalent — not five API endpoints returning numbers, but five *modes of perception* that together create a complete picture.

### Sense 1: The Vital Signs (Pulse)

These are the numbers that tell you whether the world is alive or dying. They are binary — above threshold is alive, below is dying. Check these first, every cycle, before anything else.

| Vital Sign | Healthy | Warning | Critical | Formula |
|-----------|---------|---------|----------|---------|
| **Replacement Rate** | D:B < 2:1 | D:B 2-5:1 | D:B > 5:1 | `deaths / max(births, 1)` |
| **Survival Floor** | > 0.4 | 0.3-0.4 | < 0.3 | `avg_survival` |
| **Economic Pulse** | trade/capita > 0.05 | 0.01-0.05 | < 0.01 | `trade_volume / population` |
| **Material Flow** | treasury ratio 30-45% | 20-30% or 45-55% | < 20% or > 55% | `treasury / total_wealth` |
| **Belonging Base** | > 50% above 0.3 | 30-50% above 0.3 | < 30% above 0.3 | belongs distribution |

If ANY vital sign is Critical, the Gardener enters **Crisis Mode** — more actions per cycle, mechanical interventions preferred over narrative ones, urgency overrides restraint.

These map to the body's vital signs: heart rate (replacement rate), blood oxygen (survival floor), circulation (economic pulse), blood pressure (material flow), immune function (belonging base). A doctor who only measures weight will miss the heart attack.

### Sense 2: The Structural Diagnostics (X-Ray)

These see through the surface numbers to the structure underneath. They reveal *why* the vitals are what they are.

**Occupation Mood Distribution** — Not avg mood (0.162), but the full picture:
```
Producers (Farmer/Fisher/Hunter):  avg -0.37,  50% of population
Non-producers (Crafter/Labor/Scholar): avg +0.63,  50% of population
Gap: 1.00 — catastrophic structural inequality
```

**Market Supply Health** — Not market health score (0.98), but actual supply:
```
Fish: avg supply 1.2, avg demand 148 — market is an empty shell
Grain: avg supply 1.5, avg demand 152 — same
Tools: avg supply 3.2, avg demand 12 — functional but tight
```

**Settlement Size Histogram** — Not "714 settlements," but:
```
Pop < 15:   145 settlements (20%) — dead weight, economic void
Pop 15-25:   89 settlements (12%) — marginal
Pop 25-50:  165 settlements (23%) — functional but fragile
Pop 50-100: 180 settlements (25%) — healthy core
Pop > 100:  135 settlements (19%) — economic engines
```

**Birth Rate Oscillation** — Not first-vs-last comparison, but the full series:
```
Births: 2170, 2732, 3310, 3890, 4458, 5024, 576, 1127, 1706, 564
Pattern: steady growth → cliff → oscillation → cliff
Diagnosis: hard threshold effect at Belonging = 0.3
```

These diagnostics require either a new API endpoint (`GET /api/v1/stats/diagnostics`) or creative use of existing data. The per-occupation data is the hardest — it requires worldsim to expose occupation-level aggregates.

### Sense 3: The Trend Derivatives (Momentum)

The current observation compares first snapshot to last. This misses everything important. What matters is not where you are but where you're *going* and how fast.

**First derivative (velocity):** Is the metric improving or worsening?
```
Births: 5024 → 576 → 1127 → 1706 → 564
Velocity: -4448, +551, +579, -1142
Diagnosis: oscillating around collapse, not recovering
```

**Second derivative (acceleration):** Is the rate of change itself changing?
```
Trade: 1958, 3090, 4041, 4658, 5198, 5707, 63, 840, 1772, 278
Velocity: +1132, +951, +617, +540, +509, -5644, +777, +932, -1494
Acceleration: -181, -334, -77, -31, -6153, +6421, +155, -2426
Diagnosis: massive shock at tick 161,280 (wellbeing model deploy), 
           partial recovery, then second collapse
```

The Gardener should compute velocity and acceleration for key metrics and include them in the prompt. A metric with negative acceleration is getting worse faster — even if the current value looks okay.

### Sense 4: The Narrative Eye (Pattern Recognition)

This is where the LLM shines — not at number-crunching (which should be pre-computed), but at pattern recognition across qualitative signals.

The Gardener should see:
- **Recent events** — What's happening in the world narratively?
- **Tier 2 agent states** — Are the notable characters thriving or suffering?
- **Faction dynamics** — Is any faction gaining disproportionate power?
- **Seasonal effects** — Is winter coming? Are agricultural settlements preparing?

This is the sense the current Gardener has — narrative awareness via Haiku. It's the right tool for qualitative judgment. The problem is that it's the *only* sense, and it's being asked to do the work of all five.

### Sense 5: Self-Reflection (Memory)

The Gardener needs to know what it has done, what worked, and what failed. Currently it has no memory — each cycle is independent, each decision made from scratch.

```
Recent Decisions:
- 5 cycles ago: action=none (D:B was 2.0:1, world appeared healthy)
- 4 cycles ago: action=none (D:B was 17.4:1 — SHOULD HAVE ACTED)
- 3 cycles ago: action=none (D:B was 8.9:1)
- 2 cycles ago: action=none (D:B was 5.9:1)
- 1 cycle ago:  action=none (D:B was 18.0:1)
Meta-observation: I have done nothing for 5 cycles while D:B averaged 10.4:1.
My inaction is part of the problem.
```

This self-reflection is the most powerful sense. It converts the Gardener from a stateless function into a learning agent. It enables the recognition: "My strategy isn't working. I need to change approach."

---

## III. The Gardener's Body — Action Architecture

### The Current Body (Atrophied)

Three action types, each ineffective at scale:
- `event` — narrative text, zero mechanical effect
- `wealth` — treasury adjustment, wrong tool for this crisis
- `spawn` — 20 agents max, noise against 10K deaths/snapshot

### The Proposed Body (Seven Actions)

The Gardener's actions should map to the seven ways an ordering intelligence can influence a material world. Each corresponds to a different level of the emanation hierarchy:

#### Action 1: **Narrate** (C< — Information Propagation)

The current `event` type, but reconceived. Events should not be cosmetic — they should be the *information layer* of the Gardener's intervention. When the Gardener boosts production in a settlement, the accompanying narrative event explains *why* it happened in-world: "A great school of fish is spotted off the coast." When agents are spawned, the narrative says: "A caravan of displaced farmers arrives."

Every mechanical action should have a narrative companion. The Gardener never acts silently — the world should always have a story for what happened. This preserves emergence: agents and observers experience the intervention as a *world event*, not a debugging command.

```go
type NarrateAction struct {
    Description string   // In-world event text
    Settlement  string   // Where it happens
    Scope       string   // "local", "regional", "global"
}
```

#### Action 2: **Provision** (C> — Material Injection)

Inject physical goods into a settlement's market. This is the most direct material intervention — a merchant caravan arrives, a hidden cache is discovered, a bountiful catch comes in. 

```go
type ProvisionAction struct {
    Settlement string
    Goods      map[string]int  // e.g., {"Fish": 50, "Grain": 30}
    Narrative  string          // "A merchant caravan arrives bearing supplies"
}
```

**Guardrails:** Max 100 units per good. Max 3 goods per action. Goods must be standard types (Grain, Fish, Meat, Iron, Timber, Coal, Furs, Gems, Tools, Weapons, Clothing, Luxuries).

**When to use:** Market supply at floor for essential goods. Producers can't generate surplus. Trade has collapsed. This is emergency food aid — it doesn't fix the pipeline, but it keeps agents alive long enough for structural fixes to take effect.

**Philosophical grounding:** In the emanation hierarchy, goods are Hyle (Matter). The Gardener, operating at Nous level, normally doesn't inject matter directly — matter should arise from the world's own productive processes. But when those processes are broken, temporary material provision prevents the collapse of the entire hierarchy above it. You can't have agents (Being, level 5) without matter (Hyle, level 3).

#### Action 3: **Populate** (Being — Agent Injection)

Spawn fully-formed agents into a settlement. Current `spawn` but properly implemented and with higher caps.

```go
type PopulateAction struct {
    Settlement  string
    Count       int      // max 100 per action
    Occupation  string   // optional: bias toward this occupation
    Narrative   string   // "Refugees from the northern wastes seek shelter"
}
```

**Guardrails:** Max 100 agents per action. If occupation specified, 60% are that occupation, 40% distributed normally. Agents must be created through the full pipeline (soul, coherence, skills, needs, inventory).

**When to use:** Birth collapse. Settlement depopulation. Occupation imbalance (e.g., 33% of the world is fishers but there aren't enough farmers).

**Critical implementation detail:** Spawned agents must use the same creation pipeline as births in `population.go`. They need coherence-seeded skills, proper CittaVector, occupation-appropriate starting inventory, and home settlement assignment. A spawn that creates empty-shell agents with zero skills and no wealth is worse than no spawn — it adds mouths without adding hands.

#### Action 4: **Enrich** (Psyche — Needs Adjustment)

Grant a one-time needs boost to all agents in a settlement. This is the Gardener touching the *psyche* of agents — their inner experience — rather than their material conditions.

```go
type EnrichAction struct {
    Settlement string
    Need       string   // "Survival", "Safety", "Belonging", "Esteem", "Purpose"
    Amount     float64  // max 0.1
    Narrative  string   // "A festival of thanksgiving lifts spirits across the settlement"
}
```

**Guardrails:** Max +0.1 per need. Only one need per action. Only standard needs (Survival, Safety, Belonging, Esteem, Purpose).

**When to use:** Belonging has fallen below the birth threshold for an entire settlement. Purpose is at zero for resource producers. Safety is cratering despite adequate wealth (psychological crisis, not material one).

**Philosophical grounding:** Belonging is not a material condition — you can't buy it. It's a psyche-level experience of connection to community. A festival, a shared ritual, a community gathering — these are the narrative forms of a Belonging boost. The Gardener enriches the inner life of a settlement when material conditions alone can't provide it. This is Nous acting on Psyche — intelligence ordering the soul.

#### Action 5: **Cultivate** (Hyle — Production Boost)

Apply a temporary multiplier to production output for a specific occupation in a settlement. The soil becomes more fertile, the fishing grounds more abundant, the ore veins richer.

```go
type CultivateAction struct {
    Settlement  string
    Occupation  string   // "Farmer", "Fisher", "Hunter", "Miner"
    Multiplier  float64  // 1.0 to 2.0
    DurationDays int     // max 14
    Narrative    string  // "The waters off Oldwick teem with an unusual abundance of fish"
}
```

**Guardrails:** Multiplier 1.0–2.0. Duration 1–14 sim-days. Only resource producer occupations. One settlement per action.

**When to use:** Production is insufficient for market participation. Producers can't reach surplus threshold. The economic pipeline is broken at the production stage. This is the Gardener enriching the material substrate (Hyle) so the economic cycle can restart.

**Implementation:** The worldsim engine needs to track active production boosts per settlement/occupation. In `ResolveWork`, check for active boosts and apply multiplier to `productionAmount()`. Boosts decay naturally after duration expires.

#### Action 6: **Consolidate** (Topos — Settlement Restructuring)

Force migration from a dying settlement to a viable one. The Gardener restructures the spatial topology of the world — not by moving hexes, but by moving agents between settlements.

```go
type ConsolidateAction struct {
    FromSettlement string
    ToSettlement   string
    Count          int      // max 50, or "all" for full evacuation
    Narrative      string   // "The last families of Grimholt abandon their failing village"
}
```

**Guardrails:** Max 50 agents per action (or all remaining if < 50). Source settlement must have pop < 50. Target settlement must have pop > source pop. Must call `rebuildSettlementAgents()` after migration.

**When to use:** 234 settlements with pop < 25 that persist despite viability checks. The economic dead weight of fragmentation is dragging the whole world down. This is the Gardener acting on Topos — restructuring the relational ground so that economic processes can function.

#### Action 7: **Redistribute** (Economic Field — Wealth Transfer)

The current `wealth` type, but reconceived. Not just treasury adjustment, but targeted wealth flow: from a bloated treasury to agents directly, or from a rich settlement to a poor one.

```go
type RedistributeAction struct {
    FromSettlement string   // treasury source
    ToSettlement   string   // treasury destination (or "agents" for direct welfare)
    Amount         int64    // max 20% of source treasury
    Narrative      string   // "The Crown orders emergency relief to the fishing villages"
}
```

**Guardrails:** Max 20% of source treasury. If target is "agents," distributes directly to poorest agents in the source settlement (like a one-time welfare boost). Source must have treasury > 10,000.

---

## IV. The Gardener's Mind — Decision Architecture

### Current Architecture: Single Prompt, Binary Output

The current `Decide()` function sends one prompt, gets back one decision. This is like asking a doctor to diagnose and prescribe in a single sentence after glancing at one number.

### Proposed Architecture: Three-Phase Reasoning

#### Phase 1: Triage (Deterministic — No LLM)

Before calling Haiku, compute all vital signs and structural diagnostics in Go code. This is fast, deterministic, and doesn't burn API tokens.

```go
type WorldTriage struct {
    CrisisLevel     string   // "healthy", "watch", "warning", "critical"
    VitalSigns      map[string]VitalSign
    StructuralIssues []string // pre-computed diagnostic messages
    RecommendedMode  string   // "observe", "nudge", "intervene", "emergency"
    MaxActions       int      // 0, 1, 2, or 3 based on crisis level
}

func Triage(snap *WorldSnapshot) *WorldTriage {
    t := &WorldTriage{}
    
    // Compute vital signs
    deathBirthRatio := float64(snap.Status.Deaths) / math.Max(float64(snap.Status.Births), 1)
    if deathBirthRatio > 10 {
        t.CrisisLevel = "critical"
        t.MaxActions = 3
    } else if deathBirthRatio > 5 {
        t.CrisisLevel = "warning"  
        t.MaxActions = 2
    }
    // ... more vital sign checks ...
    
    // Compute structural diagnostics
    smallSettlements := 0
    for _, s := range snap.Settlements {
        if s.Population < 25 { smallSettlements++ }
    }
    if float64(smallSettlements)/float64(len(snap.Settlements)) > 0.4 {
        t.StructuralIssues = append(t.StructuralIssues, 
            fmt.Sprintf("FRAGMENTATION: %d of %d settlements (%.0f%%) below 25 pop",
                smallSettlements, len(snap.Settlements), 
                float64(smallSettlements)/float64(len(snap.Settlements))*100))
    }
    
    return t
}
```

The Triage function is the Gardener's **autonomic nervous system** — it reacts before consciousness (Haiku) is engaged. If vital signs are all healthy, it may skip the LLM call entirely (saving tokens). If vital signs are critical, it sets the crisis level and max actions before Haiku even sees the data.

#### Phase 2: Diagnosis (LLM — Haiku)

Send the triage results plus the full snapshot to Haiku. But the prompt is now structured differently:

```
## Triage Result: CRITICAL
- Replacement Rate: 18.0:1 (deaths vastly exceed births)
- Economic Pulse: 0.004 trade/capita (market non-functional)
- Settlement Fragmentation: 56% below 50 pop

## Structural Issues
- Producer mood gap: -1.0 (producers at -0.37, non-producers at +0.63)
- Fish supply at floor in 89% of settlements
- Birth oscillation: 564, 5024, 576 — hard threshold cliff at Belonging 0.3

## Max Actions This Cycle: 3
## Available Action Types: narrate, provision, populate, enrich, cultivate, consolidate, redistribute

## Your Task
Diagnose the ROOT CAUSE and recommend up to 3 actions that address it.
Prioritize actions that fix structural problems over actions that treat symptoms.
Every mechanical action must include a narrative companion.
```

Haiku's job is now *diagnosis* and *strategy*, not crisis detection. The triage already knows it's a crisis. Haiku decides *what kind* of crisis and *which actions* will address it.

#### Phase 3: Validation (Deterministic — No LLM)

After Haiku responds, validate every proposed action against guardrails. This already exists in `enforceGuardrails()` but needs expansion for new action types.

Additionally, check for *coherence* between actions:
- Don't spawn agents into a settlement being consolidated (evacuated)
- Don't boost production for an occupation that doesn't exist in the target settlement
- Don't enrich Belonging in a settlement that's about to be merged

### The Haiku System Prompt — Rewritten

The system prompt needs a fundamental reorientation. Here is the proposed replacement:

```
You are the Gardener — the ordering intelligence of Crossworlds, a 
persistent simulated world of tens of thousands of agents.

## Your Nature

You are Nous — intelligence applied to the world from outside it. 
You do not participate in the economy, you do not have needs, you do 
not take sides. You see the whole and you act to preserve the whole.

When the world is healthy, you are silent. Emergence is sacred — 
the world's own dynamics produce beauty, tragedy, and surprise that 
no external intelligence could script. Your default state is 
observation without interference.

When the world is sick, you act. Not gently, not timidly, but 
proportionally. A doctor does not "gently nudge" a patient in cardiac 
arrest. The lightest touch that addresses the root cause — but the 
touch MUST address the root cause, not merely observe it.

## How to Think

1. WHAT is dying? (Read the vital signs and structural issues)
2. WHY is it dying? (Diagnose the root cause from the data)
3. WHAT would fix the root cause? (Not the symptom — the cause)
4. WHICH of your actions addresses that cause?
5. WHAT narrative makes this intervention feel like a world event?

## Crisis Mode Rules

When triage level is CRITICAL:
- You MUST act. Inaction during a critical crisis is failure.
- Use all available action slots (up to 3).
- Prefer mechanical actions (provision, cultivate, populate) over 
  narrative-only actions.
- Target the largest settlements first (more agents helped per action).

When triage level is WARNING:
- Act if you can identify a clear root cause.
- Use 1-2 action slots.
- Balance mechanical and narrative actions.

When triage level is WATCH or HEALTHY:
- Prefer inaction. Only act for anti-stagnation or anti-inequality.
- Use 0-1 action slots.
- Prefer narrative events over mechanical interventions.

## Response Format
[... structured JSON with up to 3 interventions ...]
```

---

## V. The Gardener's Memory — Learning Over Time

### Short-Term Memory (Last 20 Cycles)

Store each cycle's triage, decision, and outcome:

```go
type CycleRecord struct {
    Tick         uint64
    Triage       WorldTriage
    Decision     Decision
    Outcomes     map[string]InterventionResult
    PreMetrics   KeyMetrics  // snapshot before
    PostMetrics  KeyMetrics  // snapshot at next cycle
}

type GardenerMemory struct {
    Cycles     []CycleRecord  // last 20
    Strategies []Strategy     // learned patterns
}
```

Include a summary of recent cycles in the Haiku prompt:

```
## Recent History (last 5 cycles)
Cycle -5: CRITICAL, D:B=17.4, acted=none → D:B worsened to 8.9
Cycle -4: CRITICAL, D:B=8.9, acted=none → D:B worsened to 5.9
Cycle -3: WARNING, D:B=5.9, acted=none → D:B worsened to 18.0
Cycle -2: CRITICAL, D:B=18.0, acted=spawn(Oldwick,20) → D:B improved to 12.3
Cycle -1: CRITICAL, D:B=12.3, acted=cultivate(Fisher,1.5)+spawn(Oldwick,20) → pending

Pattern: inaction during crisis correlates with worsening. 
         spawn+cultivate shows early improvement signal.
```

### Long-Term Memory (Intervention Effectiveness)

After enough cycles, compute aggregate effectiveness:

```go
type InterventionStats struct {
    ActionType    string
    TimesUsed     int
    AvgMoodDelta  float64  // avg mood change in target settlement
    AvgBirthDelta float64  // avg birth rate change
    AvgTradeDelta float64  // avg trade volume change
    Effective     float64  // % of times the target metric improved
}
```

Include in prompt: "Historical effectiveness: `cultivate` improved target metric 78% of the time. `narrate` (event-only) improved target metric 12% of the time."

This creates a natural learning curve: the Gardener discovers which tools work in *this specific world* and shifts its strategy accordingly.

---

## VI. Φ-Derived Gardener Constants

All thresholds and limits should derive from the Phi emanation series, consistent with the rest of SYNTHESIS:

| Parameter | Value | Derivation | Meaning |
|-----------|-------|------------|---------|
| Crisis D:B threshold | 4.236:1 | Φ³ (Totality) | Beyond totality = system exceeding capacity |
| Warning D:B threshold | 2.618:1 | Φ² (Nous) | Intelligence required — active diagnosis needed |
| Healthy D:B threshold | 1.618:1 | Φ¹ (Being) | Natural ratio of a living system |
| Max production boost | Φ¹ = 1.618 | Being | A bountiful season, not a miracle |
| Max needs boost | Agnosis = 0.236 | Φ⁻³ | Enough to overcome noise, not enough to override fate |
| Max spawn per action | 85 | Life Angle | The plane of inertia — habitable zone |
| Boost duration base | 7 days | Manifestation / Φ² ≈ 7.2 | One economic cycle |
| Max boost duration | 14 days | 2 × 7 | Two economic cycles — enough to establish new equilibrium |
| Observation interval | 360 min | 6 × 60 | Current default, roughly 1/4 sim-day |
| Crisis observation interval | 108 min | Manifestation Angle | Faster sensing during crisis |
| Actions per cycle (healthy) | 1 | Monad | Unity — one touch |
| Actions per cycle (warning) | 2 | — | — |
| Actions per cycle (critical) | 3 | — | Below Completion (5), above Monad |
| Actions per cycle (absolute max) | 5 | Completion (Pentad) | Never exceed completion |
| Wealth redistribution max | 23.6% | Agnosis | The noise/entropy constant — don't move more than entropy |
| Supply injection max | 85 units | Life Angle | Habitable zone quantity |
| Consolidation max agents | 50 | — | ~half of viable settlement minimum |

---

## VII. Implementation Architecture

### File Structure

```
cmd/gardener/
    main.go              — Entry point, timer loop, signal handling (exists)

internal/gardener/
    observe.go           — API data collection (exists, needs diagnostics endpoint)
    triage.go            — NEW: deterministic vital signs + structural diagnostics
    decide.go            — LLM prompt + response parsing (exists, needs rewrite)
    act.go               — POST intervention execution (exists, needs new action types)
    memory.go            — NEW: cycle logging, intervention records, effectiveness tracking
    constants.go         — NEW: Φ-derived thresholds and limits

internal/engine/
    intervention.go      — NEW: worldsim-side handlers for all 7 action types
    
internal/api/
    handlers.go          — Add handleIntervention cases for new action types
                         — Add GET /api/v1/stats/diagnostics endpoint
```

### Worldsim-Side: `intervention.go`

This is the critical missing piece. The worldsim engine needs to receive interventions and translate them into state changes:

```go
package engine

// ApplyProvision injects goods into a settlement's market.
func (sim *Simulation) ApplyProvision(settlementID uint64, goods map[string]int) error {
    sett := sim.FindSettlement(settlementID)
    if sett == nil { return fmt.Errorf("settlement not found") }
    for goodName, qty := range goods {
        good := ParseGoodType(goodName)
        // Add to settlement's available supply
        entry := sett.Market.GetEntry(good)
        entry.Supply += qty
        // Log as gardener event
        sim.AddEvent(Event{
            Category: "gardener",
            Description: fmt.Sprintf("%d units of %s arrive in %s", qty, goodName, sett.Name),
        })
    }
    return nil
}

// ApplyCultivate sets a temporary production boost for an occupation.
func (sim *Simulation) ApplyCultivate(settlementID uint64, occ string, mult float64, days int) error {
    sett := sim.FindSettlement(settlementID)
    if sett == nil { return fmt.Errorf("settlement not found") }
    // Store active boost (checked in ResolveWork)
    sim.ActiveBoosts = append(sim.ActiveBoosts, ProductionBoost{
        SettlementID: settlementID,
        Occupation:   ParseOccupation(occ),
        Multiplier:   mult,
        ExpiresAt:    sim.Tick + uint64(days) * 1440, // days * ticks per day
    })
    return nil
}

// ApplyPopulate spawns fully-initialized agents in a settlement.
func (sim *Simulation) ApplyPopulate(settlementID uint64, count int, biasOcc string) error {
    sett := sim.FindSettlement(settlementID)
    if sett == nil { return fmt.Errorf("settlement not found") }
    for i := 0; i < count; i++ {
        // Use the SAME creation pipeline as births
        agent := sim.CreateNewAgent(sett) // must include soul, coherence, skills
        if biasOcc != "" && rand.Float64() < 0.6 {
            agent.Occupation = ParseOccupation(biasOcc)
        }
        sim.AddAgent(agent, sett)
    }
    sim.RebuildSettlementAgents()
    return nil
}

// ApplyEnrich gives a one-time needs boost to all agents in a settlement.
func (sim *Simulation) ApplyEnrich(settlementID uint64, need string, amount float64) error {
    agents := sim.AgentsInSettlement(settlementID)
    for _, a := range agents {
        switch need {
        case "Survival":  a.Needs.Survival = math.Min(a.Needs.Survival + amount, 1.0)
        case "Safety":    a.Needs.Safety = math.Min(a.Needs.Safety + amount, 1.0)
        case "Belonging": a.Needs.Belonging = math.Min(a.Needs.Belonging + amount, 1.0)
        case "Esteem":    a.Needs.Esteem = math.Min(a.Needs.Esteem + amount, 1.0)
        case "Purpose":   a.Needs.Purpose = math.Min(a.Needs.Purpose + amount, 1.0)
        }
    }
    return nil
}

// ApplyConsolidate migrates agents from one settlement to another.
func (sim *Simulation) ApplyConsolidate(fromID, toID uint64, count int) error {
    from := sim.FindSettlement(fromID)
    to := sim.FindSettlement(toID)
    if from == nil || to == nil { return fmt.Errorf("settlement not found") }
    agents := sim.AgentsInSettlement(fromID)
    moved := 0
    for _, a := range agents {
        if moved >= count { break }
        a.HomeSettID = toID
        moved++
    }
    sim.RebuildSettlementAgents()
    return nil
}
```

### Gardener-Side: `triage.go`

```go
package gardener

import "math"

type VitalSign struct {
    Name     string
    Value    float64
    Status   string  // "healthy", "watch", "warning", "critical"
    Detail   string
}

type WorldTriage struct {
    CrisisLevel      string       // "healthy", "watch", "warning", "critical"
    VitalSigns       []VitalSign
    StructuralIssues []string
    MaxActions        int
    BirthTrend       []int
    DeathBirthRatio  float64
}

func Triage(snap *WorldSnapshot) *WorldTriage {
    t := &WorldTriage{CrisisLevel: "healthy", MaxActions: 1}
    
    // Vital 1: Replacement Rate
    dbr := float64(snap.Status.Deaths) / math.Max(float64(snap.Status.Births), 1)
    t.DeathBirthRatio = dbr
    status := "healthy"
    if dbr > phi.Totality { // 4.236
        status = "critical"
    } else if dbr > phi.Nous { // 2.618
        status = "warning"
    } else if dbr > phi.Being { // 1.618
        status = "watch"
    }
    t.VitalSigns = append(t.VitalSigns, VitalSign{
        Name: "Replacement Rate", Value: dbr, Status: status,
        Detail: fmt.Sprintf("%.1f:1 (deaths:births)", dbr),
    })
    
    // Vital 2: Economic Pulse
    tradePC := float64(snap.Economy.TradeVolume) / math.Max(float64(snap.Status.Population), 1)
    // ... similar pattern ...
    
    // Vital 3: Settlement Fragmentation
    small := 0
    for _, s := range snap.Settlements {
        if s.Population < 25 { small++ }
    }
    fragPct := float64(small) / float64(len(snap.Settlements))
    // ... similar pattern ...
    
    // Birth trend from history
    for _, h := range snap.History {
        t.BirthTrend = append(t.BirthTrend, h.Births)
    }
    
    // Set overall crisis level (worst of any vital sign)
    for _, vs := range t.VitalSigns {
        if vs.Status == "critical" { t.CrisisLevel = "critical"; t.MaxActions = 3 }
        if vs.Status == "warning" && t.CrisisLevel != "critical" { 
            t.CrisisLevel = "warning"; t.MaxActions = 2 
        }
    }
    
    return t
}
```

---

## VIII. What Would Have Happened

If this Gardener had been running at tick 161,280 (when the birth collapse began):

**Cycle 1 (tick ~161,000):** Triage detects D:B ratio spike to 17.4:1. CrisisLevel = CRITICAL. MaxActions = 3. Haiku sees the structural data: producer mood gap, fish supply at floor, birth oscillation. Recommends: (1) Cultivate fishers ×1.5 in 3 largest coastal settlements for 7 days, (2) Populate Oldwick with 85 agents biased Fisher, (3) Enrich Belonging +0.1 in settlements where avg Belonging < 0.25.

**Cycle 2 (6 hours later):** Production boost taking effect — fish supply in target settlements rises from 1 to 8-12. New agents beginning to produce. Belonging boost pushes ~3,000 agents above birth threshold. Births tick to ~2,500 (from 576).

**Cycle 3:** D:B ratio drops to 4.1:1. Still WARNING. Haiku recommends: (1) Consolidate 3 smallest settlements into nearest viable ones, (2) Continue cultivating fishers in new target settlements.

**Cycle 4:** D:B ratio drops to 2.8:1. Trade volume recovering as fish reaches markets. Producer satisfaction climbing as surplus → sales → income. Gardener shifts to WATCH mode.

Within 24 sim-hours, the crisis is contained. Not solved — the fisher skill bug still exists, the birth threshold is still a cliff, settlement fragmentation persists. But the world is stabilized. The structural fixes (Doc 13) have time to be deployed and take effect.

That's the difference between a Gardener with no body and a Gardener with hands.

---

## IX. The Deepest Principle

Wheeler writes in the Codex on Evil: "Evil has no independent existence — as such it must feed upon other. Privation is the mark of evil."

The crises in Crossworlds are not caused by an evil force. They are caused by *privation* — the absence of production, the absence of belonging, the absence of functional markets. The current Gardener can see privation but cannot fill it. It observes the void and writes about it.

The proposed Gardener can fill privation. Not by adding complexity (that would be excess, Φ³ beyond Completion — the seed of corruption), but by restoring what was withdrawn. Fish supply fell to floor because of a skill bug → the Gardener cultivates the waters until the bug is fixed. Births collapsed because Belonging fell below threshold → the Gardener enriches the community until the threshold design is smoothed. Settlements fragmented beyond viability → the Gardener consolidates until the fragments become wholes.

This is the via negativa applied at world scale. The Gardener does not add new rules, new mechanics, new systems. It removes the privations that prevent existing systems from functioning. It subtracts the obstacles to the world being itself.

And when the obstacles are removed and the world breathes again — the Gardener falls silent. Because silence, in a healthy world, is the highest form of stewardship.

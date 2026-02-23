# 13 — Implementation Plan: Producer Satisfaction Crisis

**Date:** 2026-02-22  
**Responding to:** Observation 12 (tick 165,844)  
**Priority:** P0 — world survival at risk (18:1 death:birth ratio)

---

## Root Cause Analysis

The observation shows a clean split: producers miserable, non-producers thriving. This is NOT a mood model problem — the dual-register wellbeing system is working correctly (liberated Tier 2 agents show 0.88 mood despite low satisfaction, confirming the alignment blend). The problem is **producer satisfaction itself is genuinely low** because the economic loop for producers is broken.

Here's the causal chain:

```
Fisher produces fish using Skills.Farming (known bug — low skill = low output)
  → production = int(Skills.Farming * 3) → if Farming = 0.3, output = 0 per tick
  → surplus threshold = 3 (agent keeps 3 before selling)
  → agent never accumulates 3+ fish → never lists on market
  → market supply stays at floor (1) for fish
  → no sales → no income for fisher
  → low wealth → low Safety need (wealth-driven)
  → low trade success → low Esteem need (relative wealth)
  → satisfaction formula: (Survival×5 + Safety×4 + Belonging×3 + Esteem×2 + Purpose×1) / 15
  → Survival ~0.4 (eating own catch), Safety ~0.1, Belonging ~0.3, Esteem ~0.1, Purpose ~0.3
  → satisfaction ≈ (2.0 + 0.4 + 0.9 + 0.2 + 0.3) / 15 ≈ 0.253 → mapped to mood range ≈ -0.49
```

Meanwhile, a Laborer:
```
Laborer gets throttled wage (~24 crowns/day) — guaranteed income
  → buys food from market (ActionBuyFood) → Survival stays healthy
  → has wealth → Safety is high
  → wealth relative to peers → Esteem is decent
  → same Belonging/Purpose boosts from work
  → satisfaction ≈ (3.5 + 2.8 + 0.9 + 1.2 + 0.3) / 15 ≈ 0.58 → mood ≈ +0.16
  → PLUS alignment blend pushes effective mood to +0.6
```

The fix must restore the economic loop for producers. Not by tweaking mood weights, but by making producers actually produce enough to sell, earn, and satisfy their material needs.

---

## Deploy Plan: 4 Changes, Ranked by Impact

### Deploy 1 (P0): Fix Fisher Production Skill — `production.go`

**The single highest-impact fix.** 23,656 fishers (33% of population) have their output calculated from `Skills.Farming`. Fishers are spawned as fishers, not farmers — their Farming skill is likely low or random. This means most fishers produce 0-1 fish per tick, never reach the surplus threshold of 3, and never sell.

**Current code (likely):**
```go
func productionAmount(a *Agent, occ Occupation) int {
    switch occ {
    case OccFarmer:
        return int(a.Skills.Farming * 3)
    case OccFisher:
        return int(a.Skills.Farming * 3)  // BUG: should use fishing skill
    case OccMiner:
        return int(a.Skills.Mining * 2)
    case OccHunter:
        return int(a.Skills.Combat * 2)
    }
}
```

**Fix — Option A (quick, no schema change):**

Use `max(Skills.Farming, Skills.Combat, 0.5)` for fishers. Rationale: fishing is a physical skill that draws on farming knowledge (provisioning) and combat fitness (hauling nets, boats). The floor of 0.5 ensures every fisher produces at least `int(0.5 * 3) = 1` fish per tick, and most will produce 2-3.

```go
case OccFisher:
    // Fisher skill = max of farming, combat, with floor of 0.5
    // Fishing draws on provisioning knowledge and physical fitness
    fishSkill := math.Max(a.Skills.Farming, a.Skills.Combat)
    if fishSkill < 0.5 {
        fishSkill = 0.5
    }
    return int(fishSkill * 3)
```

**Fix — Option B (proper, schema change):**

Add `Skills.Fishing` field, initialize from coherence-seeded distribution at spawn, and use it directly. More work but architecturally correct.

```go
// In types.go, add to SkillSet:
Fishing float64

// In population.go (agent spawn):
skills.Fishing = spawnSkillFromCoherence(soul.Coherence, soul.CittaVector)

// In production.go:
case OccFisher:
    return int(a.Skills.Fishing * 3)
```

**Recommendation:** Do Option A first (immediate deploy, fixes the crisis), then Option B in a follow-up session. The world is in a survival crisis — don't let the perfect be the enemy of the good.

**Expected impact:** With 23,656 fishers now producing 1-3 fish/tick instead of 0-1, fish supply should jump dramatically. Market supply recovers from floor. Fishers accumulate surplus → list on market → earn crowns → Safety and Esteem rise → satisfaction recovers.

**Verification:** After 2-3 sim-days, check: Fisher avg satisfaction should move from -0.23 toward 0.0+. Fish supply in settlements should be >> 1. Trade volume should recover.

---

### Deploy 2 (P0): Boost Producer Needs Replenishment — `production.go`

The current `ResolveWork` gives producers:
- Esteem: +0.01
- Safety: +0.005
- Belonging: +0.003
- Purpose: +0.002

These are the same amounts (or close to) what `applyWork` in `behavior.go` gives non-producers. But the satisfaction formula weights are:

```
Survival: ×5,  Safety: ×4,  Belonging: ×3,  Esteem: ×2,  Purpose: ×1
```

Safety (weight 4) is the second most important factor after Survival, and producers get only +0.005/tick for it. Meanwhile, non-producers' Safety is boosted by having wealth (from wages), which is a separate channel that producers don't have when they can't sell.

**Fix:** Increase `ResolveWork` needs boosts for successful production (not fallback paths):

```go
// Successful production path in ResolveWork:
a.Needs.Safety += 0.008    // was 0.005 — producing food IS safety
a.Needs.Esteem += 0.012    // was 0.01  — creating real goods = dignity  
a.Needs.Belonging += 0.004 // was 0.003 — slight bump
a.Needs.Purpose += 0.004   // was 0.002 — doubled, work IS purpose for producers
```

**Philosophical justification from the framework:** A farmer who grows grain or a fisher who catches fish is creating *real material value* — the very bottom of the emanation hierarchy (Matter/Hyle). This is the "wake of emanation," the substrate on which everything else depends. The satisfaction of producing the substrate of life should be meaningful. In Wheeler's terms, the producer who feeds others is performing a more ontologically grounded act than the laborer who moves boxes for wages.

**Additional fix — Survival boost on successful production:**

Producers who successfully produce food should get a direct Survival bump. They *made* the food. They know they can eat tomorrow.

```go
// Only for food producers (Farmer, Fisher, Hunter) on successful production:
if occ == OccFarmer || occ == OccFisher || occ == OccHunter {
    a.Needs.Survival += 0.003  // NEW — producing food assures future survival
}
```

This is a small boost but matters at the margins. With Survival weighted at ×5, even +0.003/tick compounds to meaningful satisfaction improvement.

**Expected impact:** Producer satisfaction should lift by ~0.1-0.15 from needs boost alone. Combined with Deploy 1 (actual production increase), the effect compounds — more production → more sales → more wealth → Safety/Esteem rise from wealth channel too.

---

### Deploy 3 (P1): Stochastic Birth Probability — `population.go`

The current hard threshold (`Belonging > 0.3`) creates cliff dynamics. The stats history shows births swinging 564 → 5,024 → 576 between snapshots. Small perturbations in Belonging push thousands of agents across the threshold simultaneously.

**Fix:** Replace the hard threshold with a sigmoid probability curve:

```go
func birthProbability(a *Agent) float64 {
    b := a.Needs.Belonging
    
    // Below 0.15: zero chance (hard floor — truly isolated agents don't reproduce)
    if b < 0.15 {
        return 0.0
    }
    
    // Sigmoid centered at 0.3 (old threshold), with spread controlled by Φ⁻²
    // At b=0.15: ~5% chance
    // At b=0.30: ~50% chance (matches old behavior at threshold)
    // At b=0.50: ~95% chance
    // At b=0.70: ~99.5% chance
    midpoint := 0.3
    steepness := 10.0 * phi.Being // ~16.18 — steep enough to still gate, smooth enough to not cliff
    
    return 1.0 / (1.0 + math.Exp(-steepness * (b - midpoint)))
}

// In the birth check:
if rand.Float64() < birthProbability(a) && a.Needs.Survival > 0.3 && /* other gates */ {
    // birth proceeds
}
```

**Why this helps beyond smoothing:** Even when average Belonging is 0.25 (below old threshold), ~20% of agents will still reproduce. This prevents the total birth collapse that happens when Belonging dips slightly below 0.3 for the majority. The world maintains a birth floor.

**Keep the Survival gate hard** at 0.3 — starvation should absolutely prevent reproduction. But Belonging is about community feeling, which is more gradient than binary.

**Expected impact:** Birth rate stabilizes around 2,000-3,000/snapshot instead of swinging 564-5,024. The 10K/snapshot death rate needs ~3K births/snapshot to maintain the population trajectory. Smoothing ensures we stay in that band.

---

### Deploy 4 (P2): Aggressive Settlement Consolidation — `perpetuation.go`, `settlement_lifecycle.go`

399 of 714 settlements (56%) have pop < 50. These are economic dead zones — not enough agents to generate market activity, not enough producers to create supply, not enough consumers to create demand. Every tiny settlement is a fragment of what could be a functioning economy.

**Fix A — Lower viability threshold and shorten grace period:**

```go
// In processViabilityCheck():
// Old: pop < 15 for 4 weeks → disable refugee spawning
// New: pop < 25 for 2 weeks → disable refugee spawning AND trigger forced migration
if pop < 25 && nonViableWeeks >= 2 {
    // All agents in this settlement get mood threshold set to -999
    // (guarantees migration on next seasonal cycle)
    // Target: nearest settlement with pop >= 50 within 8 hexes
    forceMigrationFrom(settlement)
}
```

**Fix B — Active merging for adjacent tiny settlements:**

```go
// New function: processSettlementMerger() — called weekly
// If two settlements are within 2 hexes and both have pop < 50,
// merge the smaller into the larger
func processSettlementMerger(sim *Simulation) {
    for _, s1 := range sim.Settlements {
        if s1.Population >= 50 { continue }
        for _, s2 := range sim.Settlements {
            if s2.ID == s1.ID || s2.Population >= 50 { continue }
            if sim.World.Distance(s1.Position, s2.Position) <= 2 {
                // Merge smaller into larger
                smaller, larger := s1, s2
                if s1.Population > s2.Population { smaller, larger = s2, s1 }
                migrateAllAgents(smaller, larger)
                abandonSettlement(smaller)
                break // one merge per settlement per week
            }
        }
    }
}
```

**Fix C — Raise migration pressure for small settlements:**

The current enhanced migration (mood threshold 0.0 for pop < 25) uses `EffectiveMood`. But with the dual-register model, high-coherence agents in tiny settlements might have positive EffectiveMood and never leave. Use `Satisfaction` instead for migration from sub-viable settlements:

```go
// In processSeasonalMigration(), for settlements with pop < 25:
// Use Satisfaction, not EffectiveMood
// A liberated agent should still migrate from a dying village
// Liberation doesn't mean staying somewhere you can't eat
if settlement.Population < 25 {
    if a.Wellbeing.Satisfaction < 0.0 { // any negative material experience
        triggerMigration(a, findNearestViable(settlement))
    }
}
```

**Expected impact:** Settlement count should drop from 714 toward 400-500 within 2-3 sim-weeks. Average settlement pop rises from 102 to 145+. Larger settlements mean more market participants, more trade, more economic viability.

---

## Deploy Sequence

| # | Priority | Change | File(s) | Risk | Immediate Impact |
|---|----------|--------|---------|------|------------------|
| 1 | P0 | Fisher skill fix | `production.go` | Very low | +33% of pop gets real production |
| 2 | P0 | Producer needs boost | `production.go` | Low | +0.1-0.15 satisfaction for 50% of pop |
| 3 | P1 | Stochastic births | `population.go` | Low | Births stabilize ~2-3K/snapshot |
| 4 | P2 | Settlement consolidation | `perpetuation.go`, `settlement_lifecycle.go` | Medium | Fewer, healthier settlements |

**Deploy 1 + 2 together** as a single commit. These are both in `production.go` and address the same root cause (producer economic viability). Observe for 2-3 sim-days.

**Deploy 3** after confirming satisfaction recovery. Birth smoothing only matters once satisfaction is high enough to support reproduction.

**Deploy 4** after confirming birth recovery. Settlement consolidation is important but less urgent than preventing population collapse.

---

## What to Monitor After Each Deploy

### After Deploy 1+2:
- Fisher avg satisfaction: target > -0.10 (from -0.23)
- Farmer avg satisfaction: target > -0.15 (from -0.40)
- Hunter avg satisfaction: target > -0.20 (from -0.46)
- Fish market supply in a large settlement: target > 10 (from 1)
- Trade volume: target > 1,000 (from 278)

### After Deploy 3:
- Birth rate per snapshot: target 2,000-4,000 (stable band)
- Birth rate variance: should not swing > 3x between consecutive snapshots
- Belonging distribution: what % of agents are between 0.2-0.4?

### After Deploy 4:
- Settlement count: target < 500 (from 714)
- Avg settlement pop: target > 130 (from 102)
- Settlements with pop < 25: target < 100 (from 234)
- Trade volume (should increase with consolidation)

---

## Why NOT More Producer Needs Tweaks

The temptation is to keep adding needs boosts until satisfaction looks right. That's the wrong approach — we've been through this cycle before (the "fix the price engine first" lesson from Wave 3). The root cause here is that **producers can't sell what they make** because they don't make enough. Fix production output, and the economic loop restores itself:

```
More production → surplus above threshold → market listings → sales → income
→ wealth increases → Safety need satisfied → Esteem rises (relative wealth)
→ Satisfaction improves → EffectiveMood improves → Births resume → World stabilizes
```

The needs boosts in Deploy 2 are a bridge — they keep producers from total despair while the economic fix (Deploy 1) takes effect. They're intentionally modest. The real fix is the economic loop.

---

## Philosophical Note

This crisis actually validates the world's design. The producer misery gap is what you'd expect in a medieval economy where primary producers (farmers, fishers, hunters) are exploited by the system while secondary and tertiary workers (crafters, scholars) capture disproportionate value. The fix isn't to make producers artificially happy — it's to ensure they can participate in the market economy. In Wheeler's terms, the material substrate (Matter/Hyle) must circulate. When it pools (treasury hoarding) or evaporates (production insufficiency), the entire emanation hierarchy above it collapses. Fix the substrate, and the higher orders can flourish.

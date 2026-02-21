# Tuning Round 2: Settlement Explosion & Raw Material Inflation

## Context

After Tuning Round 1 (5 fixes deployed), the world was thriving — mood +0.64, growing population, functioning economy. But two emergent problems appeared during observation:

1. **Settlement explosion**: 73 settlements ballooned to 710+ in 30 sim-days
2. **Persistent raw material inflation**: Furs and iron ore stuck at 4.2x price ceiling

## Root Cause Analysis

### Settlement Explosion

`IsOvermassed()` computed capacity as `GovernanceScore * Totality = 0.5 * 4.236 = 2.118`, giving a population threshold of `2.118 * 1.618 = 3.43`. Every settlement with more than 4 people was permanently overmassed. Combined with a 62% emigrant fraction (Matter ratio), this created an exponential cascade: settlements founded, immediately overmassed, split again.

### Persistent Inflation

Three compounding factors:
- **Hex depletion is fast, regen is glacial**: A hex with 60 iron ore and 10 miners depletes in 3 ticks. Regeneration is 18.5% of deficit per *season* (24 sim-days). Hexes spend 99%+ of their time depleted.
- **Coal has no producer**: No occupation produces coal, but crafters demand it for the Weapons recipe. Supply is always the floor of 1.
- **Supply floor too low**: Market floors supply at 1 regardless of settlement size. 200 crafters demanding iron vs supply of 1 creates instant ceiling.

## Fixes Applied

### Fix A: Overmass Formula (`internal/social/settlement.go`)

Rewrote `IsOvermassed()` to use infrastructure-based capacity:

```
baseCap = 100 + MarketLevel*50 + RoadLevel*25 + WallLevel*25
capacity = baseCap * GovernanceScore * Totality
threshold = capacity * Being
```

Example thresholds:
- New settlement (ML=1, GS=0.5): **513 pop**
- Developed town (ML=2, RL=1, WL=1, GS=0.6): **924 pop**
- City (ML=3, RL=2, WL=2, GS=0.8): **1,919 pop**

### Fix A2: Emigrant Fraction (`internal/engine/settlement_lifecycle.go`)

Reduced diaspora fraction from Matter (~62%) to Agnosis (~24%). Smaller emigrant groups are more realistic and prevent parent settlements from being gutted on each split.

### Fix B: Weekly Micro-Regen (`internal/engine/seasons.go`, `internal/engine/simulation.go`)

Added `weeklyResourceRegen()` that recovers ~4.7% of deficit per week (Agnosis * 0.2). Hexes no longer stay depleted for the full 24 sim-days between seasonal regens. Wired into `TickWeek`.

### Fix C: Miners Produce Coal (`internal/engine/production.go`)

Miners now produce 1 coal as secondary output alongside iron ore on successful production. This fills the coal supply gap — previously no occupation produced coal at all, but crafters demanded it for the Weapons recipe.

### Fix D: Population-Scaled Supply Floor (`internal/engine/market.go`)

Market supply floor now scales with settlement population: `max(1, population/100)`. A 500-person settlement gets a floor of 5 instead of 1, preventing extreme demand/supply ratios that slam prices to the ceiling.

## Files Modified

| File | Fix | Change |
|------|-----|--------|
| `internal/social/settlement.go` | A | Infrastructure-based `IsOvermassed()` |
| `internal/engine/settlement_lifecycle.go` | A2 | Emigrant fraction: Matter → Agnosis |
| `internal/engine/seasons.go` | B | Added `weeklyResourceRegen()` |
| `internal/engine/simulation.go` | B | Wired weekly regen into `TickWeek` |
| `internal/engine/production.go` | C | Miners produce coal secondary output |
| `internal/engine/market.go` | D | Population-scaled supply floor |

## Expected Outcomes

- Settlement count stabilizes (no new foundings until ~500+ population)
- Raw material prices decline from 4.2x ceiling as supply floors rise and hexes regenerate faster
- Coal appears in miner inventories and flows through market
- Market health improves above 0.35

# Tuning Round 3: Settlement Fragmentation

## Context

After Tuning Round 2 fixed the overmass formula and raw material inflation, a new problem emerged: **settlement fragmentation**. The world had 714 settlements, with 319 (45%) having fewer than 25 people. These tiny settlements were permanently kept alive by the anti-collapse refugee floor (spawns agents if pop < 10), never grew because WallLevel/RoadLevel never incremented, and cascaded via overmass diaspora creating ever more fragments.

Root causes:
1. **No infrastructure growth** — WallLevel and RoadLevel were never incremented (only MarketLevel via rare Merchant's Compact dominance)
2. **Anti-collapse prevents natural death** — Refugees spawn at pop < 10, blocking the 2-week-at-zero abandonment trigger
3. **Low minimum founding size** — Diaspora could found with just 10 agents, creating non-viable settlements
4. **No absorption/merge** — No mechanism for tiny settlements to consolidate into nearby larger ones

## Approach

Gentle consolidation — raise founding minimum, add infrastructure growth, let non-viable settlements naturally absorb via enhanced migration. No forced dissolution.

## Fixes Applied

### Fix A: Raise Minimum Founding Size (`internal/engine/settlement_lifecycle.go`)

Changed the emigrant minimum from 10 to 25. A settlement now needs ~106 alive agents (25 / 0.236) before diaspora can found a new settlement. Below that, overmassed settlements simply don't split.

### Fix B: Weekly Infrastructure Growth (`internal/engine/settlement_lifecycle.go`)

Added `processInfrastructureGrowth()` — settlements invest treasury into infrastructure weekly:

- **Road upgrade**: treasury >= pop x 20, pop >= 50, RoadLevel < 5
- **Wall upgrade**: treasury >= pop x 30, pop >= 100, WallLevel < 5
- One upgrade per settlement per week max
- Road checked first (cheaper, smaller settlements benefit sooner)

Each road/wall level adds 25 to base capacity in `IsOvermassed()`, giving mid-size+ settlements a path to higher capacity.

### Fix C: Non-Viable Settlement Tracking (`internal/engine/settlement_lifecycle.go`, `internal/engine/simulation.go`, `internal/engine/population.go`)

Added `processViabilityCheck()` with `NonViableWeeks` tracking on Simulation:

- Increments weekly for settlements with pop < 15
- Resets when pop >= 15
- After 4 consecutive weeks non-viable, refugee spawning is skipped
- Settlement can naturally decline to 0 and trigger existing abandonment mechanic

### Fix D: Enhanced Migration from Tiny Settlements (`internal/engine/perpetuation.go`)

Modified `processSeasonalMigration()` for settlements with pop < 25:

- Mood threshold lowered from -0.3 to 0.0 (any non-positive mood triggers migration)
- Agents migrate to **nearest settlement with pop >= 25 within 5 hexes** instead of the single wealthiest global settlement
- Added `findNearestViableSettlement()` helper using `world.Distance()`

## Files Modified

| File | Fix | Change |
|------|-----|--------|
| `internal/engine/settlement_lifecycle.go` | A | Founding min: 10 -> 25 |
| `internal/engine/settlement_lifecycle.go` | B | Added `processInfrastructureGrowth()` |
| `internal/engine/settlement_lifecycle.go` | C | Added `processViabilityCheck()` |
| `internal/engine/simulation.go` | C | Added `NonViableWeeks` field + init, wired new functions into `TickWeek` |
| `internal/engine/population.go` | C | Skip refugee spawn for non-viable settlements |
| `internal/engine/perpetuation.go` | D | Enhanced migration with nearby-settlement targeting |

## Expected Outcomes

- Settlement count slowly declines as tiny settlements are absorbed by nearby larger ones
- No new settlements founded with < 25 emigrants
- Infrastructure levels increase for mid-size+ settlements, raising overmass thresholds
- Non-viable settlements (pop < 15 for 4+ weeks) stop receiving refugees and naturally decline
- Agents in tiny settlements migrate to nearby viable settlements rather than one global target

# 09 — Post-Closed-Economy TODO

Assessment from `/observe` at tick 92,290 (Spring Day 65, Year 1). The closed economy is functioning mechanically — trades execute, treasuries collect, consignment works — but the transition was too harsh. Agents were starving because crowns pooled in treasuries and grain wasn't reaching the market.

## Critical Numbers (Pre-Fix)

| Metric | Value | Target | Gap |
|--------|-------|--------|-----|
| Avg Survival | 0.375 | > 0.4 | Agents can't eat |
| Avg Mood | 0.16 | > 0.3 | Crashed from 0.64 |
| Deaths:Births | 2,576:573 (4.5:1) | < 2:1 | Population declining |
| Grain Price | 8.63 (base 2) | < 4.0 | 431% inflated |
| Trade Volume | 5,784 | Stable/growing | OK — 55x improvement |
| Agent Wealth | 952M (was 980M) | Stable | Deflating 28M/cycle |
| Treasury Wealth | 1.04B (was 1.01B) | Flowing back | Accumulating, not flowing |

## Fixes Applied (tick 92,878)

### P0: Grain supply crisis — FIXED

Grain is the universal food need but farmers kept 5 units before selling. With production of 1-3/tick and personal consumption, few farmers ever had surplus above 5 to sell.

**Fix:** Lowered surplus thresholds in `surplusThreshold()` (`market.go`):
- Farmer/fisher food threshold: 5 → 3
- Non-producer food threshold: 3 → 2
- Added fish as alternative food demand in `demandedGoods()` — hungry agents now demand both grain and fish, buying whichever is cheaper

### P1: Treasury hoarding — FIXED (Settlement Welfare)

Treasuries gained +30M while agents lost -28M. 95% of agents (Tier 0) had no path to earn from treasury.

**Fix:** Added `paySettlementWages()` (`market.go`) — daily welfare payment:
- Agents with `Wealth < 20` receive 1 crown/day from home settlement treasury
- Total payout capped at 1% of treasury per day (prevents drain)
- Called in `TickDay()` after `decayWealth()`
- This is the missing reverse flow: taxes → treasury → wages → agents → market → economy circulates

### P1: Wealth decay destroying crowns — FIXED (Redirect to Treasury)

`decayWealth()` was destroying ~0.24%/day of agent wealth. Treasury upkeep was also destroying crowns. In the closed economy, destroyed crowns were permanently lost.

**Fix:** Two changes in `market.go`:
1. `decayWealth()` now redirects decayed crowns to the agent's home settlement treasury instead of destroying them
2. Treasury upkeep sink removed from `collectTaxes()` — only population-based upkeep remains

No crowns leave the system. Wealth "decays" from rich agents but stays in circulation via treasury.

### P1: Fisher mood spiral — FIXED

All Tier 2 fishers were miserable (-0.43 to -0.49). Fish at price floor, low production.

**Fix:** Two changes:
1. Fisher production multiplier boosted from `Skills.Farming * 2` → `Skills.Farming * 3` (`production.go`)
2. Fish added as alternative food demand (see P0 fix above) — creates real market demand for fish

**Note:** `productionAmount()` still uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Works but technically wrong — low priority.

## Remaining TODO

### P2: Stats history not recording

`/api/v1/stats/history` returns `[]` — empty. Check `internal/persistence/db.go` for how `stats_history` gets written and ensure it fires on daily ticks.

### P2: Gardener startup race condition

Gardener always fails its first observation because it starts before worldsim's HTTP server is ready. Cosmetic but noisy. Fix: add a startup delay or retry loop in the gardener's initial observation.

## Success Criteria

Run `/observe` after fixes have had time to take effect and check:
- Avg survival > 0.4
- Births:deaths ratio improving toward 1:1
- Grain price trending down (< 6.0)
- Agent wealth stabilizing (not deflating)
- Trade volume maintained or growing
- Fisher Tier 2 mood improving (> -0.3)
- Treasury levels stabilizing (wealth decay flows in, wages flow out)

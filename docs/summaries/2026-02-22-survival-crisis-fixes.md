# World Summary — 2026-02-22 (Survival Crisis Fixes)

## Problem

The closed economy deployed earlier this session worked mechanically — trades executed, treasuries collected, consignment functioned — but the transition was too harsh. Crowns pooled in settlement treasuries with no path back to ordinary agents. The world was dying:

| Metric | Before Fix | Target |
|--------|-----------|--------|
| Avg Survival | 0.375 | > 0.4 |
| Avg Mood | 0.16 | > 0.3 |
| Deaths:Births | 4.5:1 | < 2:1 |
| Grain Price | 8.63 (base 2) | < 4.0 |
| Agent Wealth | Deflating 28M/cycle | Stable |
| Treasury Wealth | Accumulating (+30M) | Flowing back |

The root cause was a one-way flow: taxes and wealth decay pulled crowns out of agents into treasuries (or destroyed them entirely), but 95% of agents — Tier 0 farmers, miners, fishers, laborers — had no mechanism to earn from treasury. Only merchants (via consignment) and Tier 2 agents (via treasury trade) could access treasury wealth. Everyone else went broke, couldn't buy food, and starved.

## Changes Deployed

### 1. Grain Supply Crisis — P0 (`market.go`)

Farmers and fishers were hoarding food. The surplus threshold (inventory an agent keeps before selling) was too high — producers kept 5 units, leaving almost nothing for the market given production rates of 1-3/tick.

- Farmer/fisher surplus threshold: 5 → 3
- Non-producer food threshold: 3 → 2
- Added fish as alternative food demand — hungry agents now buy whichever food is cheaper, not just grain

### 2. Wealth Decay Redirect — P1 (`market.go`)

`decayWealth()` was destroying ~0.24%/day of agent wealth above 20 crowns. In the old open economy this balanced the mints. In the closed economy, destroyed crowns were permanently lost — the money supply shrank every day.

- Decayed crowns now flow into the agent's home settlement treasury instead of being destroyed
- No crowns leave the system — wealth "decays" from rich agents but stays in circulation
- Treasury upkeep sink also removed (was destroying crowns via bureaucracy/corruption model)
- Only population-based upkeep remains as a treasury cost

### 3. Settlement Wages (Welfare) — P1 (`market.go`, `simulation.go`)

This is the key structural fix. Added `paySettlementWages()` — a daily redistribution from settlement treasuries to poor agents:

- Agents with Wealth < 20 crowns receive 1 crown/day from their home settlement treasury
- Total payout capped at 1% of treasury per day (prevents treasury drain)
- Runs daily after tax collection and wealth decay

This closes the economic loop:
```
Taxes + wealth decay → Treasury fills
Treasury wages → Poor agents get crowns
Agents buy food → Sellers earn crowns
Sellers get wealthy → Pay taxes
→ cycle repeats
```

Without this, there was no reverse flow. Crowns entered treasuries and stayed there. The welfare system is the minimum viable redistribution — medieval towns had parish relief and guild alms for the same reason. An economy needs circulation.

### 4. Fisher Production Boost — P1 (`production.go`)

Fishers were in a mood death spiral (-0.43 to -0.49). Two contributing factors fixed:

- Production multiplier: `Skills.Farming * 2` → `Skills.Farming * 3`
- More fish produced means more surplus to sell, more income, better mood
- Combined with fish now being demanded as alternative food, fish should have real market value

## The Welfare Trade-Off

We introduced a welfare system to save the world from collapse. The trade-off:

**What we gain:**
- Crowns circulate instead of pooling in treasuries
- Poor agents can buy food and survive
- The death spiral breaks — agents who can eat can work, produce, and trade

**What we risk:**
- Reduced economic pressure — agents have a safety net, less urgency to trade
- Treasury drain in small settlements — 1% cap should prevent this but worth monitoring
- Potential inflation if too many crowns flow back too fast

**Why it's necessary:**
The closed economy removed all "free" income (mints from nothing). The only remaining mints are throttled journeyman/laborer wages (~24 crowns/day). Without welfare, the system has no mechanism to move treasury wealth back to the majority of agents. It's not charity — it's plumbing.

## Files Changed

| File | Changes |
|------|---------|
| `internal/engine/market.go` | `surplusThreshold()` lowered, fish added to `demandedGoods()`, treasury upkeep sink removed, `decayWealth()` redirects to treasury, new `paySettlementWages()` |
| `internal/engine/production.go` | Fisher production multiplier 2→3 |
| `internal/engine/simulation.go` | `paySettlementWages()` called in `TickDay()` after `decayWealth()` |

## Post-Deploy Observations — Waves 1 & 2 Insufficient

Two `/observe` runs after the initial fixes showed **no improvement**:

| Metric | Pre-Fix (92,290) | Wave 1 (93,249) | Wave 2 (94,254) |
|--------|-----------------|-----------------|-----------------|
| Population | ~54,000 | ~52,000 | 49,704 |
| Births | 0 | 0 | 0 |
| Avg Survival | 0.375 | 0.385 | 0.382 |
| Avg Mood | 0.16 | 0.156 | 0.142 |
| Grain Price | 8.63 | 8.63 | 8.63 |

The welfare, threshold tuning, and belonging fixes were all addressing real problems, but they couldn't overcome the root cause: a **price ratchet in the market engine** that mathematically prevented prices from coming down.

## Wave 2: Belonging Death Spiral Fix (tick 93,781)

Investigation revealed agents in survival mode (eating/foraging) received zero belonging — belonging decayed at ~1.70/day with no replenishment. Birth threshold of `Belonging > 0.4` was unreachable.

- `applyEat()` and `applyForage()` now give `+0.001 belonging` per tick
- Birth threshold lowered from `Belonging > 0.4` to `Belonging > 0.3`
- Settlement wages briefly scaled with grain price (reverted in wave 3)

## Wave 3: Price Ratchet Fix (tick 94,400) — The Real Fix

Investigation revealed the order-matched market engine had a structural upward price bias:

1. **Clearing midpoint = `Price * 1.118`** — always 11.8% above current price (because `(Matter + Being) / 2 = (0.618 + 1.618) / 2 = 1.118`)
2. **70/30 blend unclamped** — could push prices above Phi-derived ceiling
3. **Dual price update** — `ResolvePrice()` and blend fought each other; the biased blend always won

**Fix:** Three changes to `resolveSettlementMarket()`:
1. `ResolvePrice()` computes reference prices for order placement only — doesn't overwrite `entry.Price`
2. Clearing price = seller's ask (not midpoint) — eliminates upward bias
3. Blend result clamped to `[BasePrice * Agnosis, BasePrice * Totality]`

Settlement wages reverted to fixed 2 crowns (safety net) — prices should come down naturally.

### Key Lesson

**Fix the price engine first, then tune parameters.** When the market has a structural bias, no amount of supply-side fixes (thresholds, production boosts) or demand-side fixes (welfare, belonging) can compensate. We spent two deploy cycles layering fixes that couldn't work because the underlying engine was broken. The observation that "grain is still at 8.63 after supply increased" should have triggered investigation of the price mechanism itself, not more parameter tuning.

## Files Changed (All Waves)

| File | Changes |
|------|---------|
| `internal/engine/market.go` | `surplusThreshold()` lowered, fish added to `demandedGoods()`, treasury upkeep removed, `decayWealth()` redirects to treasury, `paySettlementWages()` added, `resolveSettlementMarket()` rewritten — reference prices decoupled, ask-price clearing, blend clamped |
| `internal/engine/production.go` | Fisher production multiplier 2→3 |
| `internal/engine/simulation.go` | `paySettlementWages()` called in `TickDay()` |
| `internal/agents/behavior.go` | `applyEat()` and `applyForage()` give +0.001 belonging |
| `internal/engine/population.go` | Birth threshold `Belonging > 0.4` → `Belonging > 0.3` |

## Post-Fix Recovery (tick 118,329)

By tick 118,329, the price ratchet fix had transformed the economy:

| Metric | Pre-Fix (92,290) | Post-Fix (118,329) | Target |
|--------|------------------|--------------------|--------|
| Population | ~54,000 | ~64,000 | Stable/growing |
| Births | 0 | 9,239 | > 0 |
| Trade Volume | 5,784 | 18,512 | Growing |
| Market Health | 12.5% | 96.9% | > 80% |
| Grain Price | 8.63 | Normalized | ~2.0 |
| Avg Mood | 0.16 | 0.091 | > 0.3 |
| Treasury Share | N/A | 71% | ~38% |

Market engine and birth system recovered. Two new problems emerged: mood still declining (purpose drought) and treasury hoarding.

## Wave 4: Infrastructure Fixes (tick 118,329)

- **Stats history query** — `toTick` used max uint64 which SQLite driver rejects. Fixed with max int64.
- **Gardener startup race** — Added `waitForAPI()` with exponential backoff before first cycle.

## Wave 5: Mood & Treasury Rebalancing (tick 120,192)

### Resource Producer Purpose Drought
`ResolveWork` in `production.go` intercepted all resource producer work actions (farmers, miners, fishers, hunters — ~60% of agents) before `applyWork` in `behavior.go` could run. `ResolveWork` was missing `Purpose += 0.002`. All resource producers had purpose permanently at 0.0, depressing mood.

**Fix:** Added `a.Needs.Purpose += 0.002` to `ResolveWork`.

### Dynamic Φ-Targeted Welfare
Treasuries held 71% of all wealth (target: ~38%). The fixed 1% outflow cap was insufficient. Rather than pick another fixed value, `paySettlementWages()` now self-regulates:

- Computes global `treasuryShare` daily
- Targets `1 - phi.Matter` (~38.2% treasury / ~61.8% agents)
- Outflow scales quadratically with excess above target
- Decelerates near equilibrium (prevents overshoot)
- All parameters derive from Φ

This is the proper fix — not another hand-tuned parameter, but a dynamic feedback loop that converges to the Φ-derived ratio.

## Deploys This Session

| # | Tick | Changes |
|---|------|---------|
| 1 | 92,878 | Grain threshold, wealth decay redirect, settlement wages, fisher boost |
| 2 | 93,781 | Belonging on eat/forage, birth threshold lowered, wages scaled with grain |
| 3 | 94,400 | **Price ratchet fix** — decouple reference prices, ask-price clearing, clamp blend |
| 4 | 118,329 | Stats history fix, gardener startup race fix |
| 5 | 120,192 | Purpose boost for resource producers, dynamic Φ-targeted welfare |

## What to Monitor

1. **Treasury/agent wealth ratio** — should converge from 71/29 toward 38/62
2. **Avg mood** — should recover as purpose and wealth reach agents
3. **Settlement consolidation** — 714 is pathological; watch for merger
4. **Merchant viability** — all Tier 2 merchants dead; may recover with price equilibrium

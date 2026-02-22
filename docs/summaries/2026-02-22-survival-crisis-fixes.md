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

## Deploys This Session

| # | Tick | Changes |
|---|------|---------|
| 1 | 92,878 | Grain threshold, wealth decay redirect, settlement wages, fisher boost |
| 2 | 93,781 | Belonging on eat/forage, birth threshold lowered, wages scaled with grain |
| 3 | 94,400 | **Price ratchet fix** — decouple reference prices, ask-price clearing, clamp blend |

## What to Monitor

1. **Grain price** — should trend down from 8.63 toward base price of 2
2. **Manufactured goods prices** — should trend up from floor toward base prices
3. **Births** — should resume once belonging recovers above 0.3
4. **Avg survival** — should recover above 0.4
5. **Market health** — should improve from 0.125
6. **Settlement count** — 714 is pathological; watch for consolidation

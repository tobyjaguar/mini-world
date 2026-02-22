# 08 — Closed Economy Changelog

Tracks the closed-economy changes from `docs/07-closed-economy-implementation.md` and remaining open mints for monitoring.

## Changes Deployed

### Order-Matched Market Engine (`internal/engine/market.go`)

Old behavior: `executeTrades()` let agents sell surplus into the void (minting crowns from nothing) and buy from the void (destroying crowns). Supply and demand were decorative.

New behavior: Sell and buy orders are collected from all agents, sorted by price (sells ascending, buys descending), and matched until prices cross. Every crown that enters a seller's pocket leaves a buyer's pocket. Market price updates blend 70% old price / 30% clearing price for stability.

### Merchant Trade Closed (`internal/engine/market.go`)

Old: `sellMerchantCargo()` minted crowns equal to `qty * destPrice`.

New: Settlement treasury pays the merchant per unit. If the treasury runs dry, remaining cargo goes unsold.

### Tier 2 Trade Closed (`internal/engine/cognition.go` + `market.go`)

Old: `applyTier2Decision` and `applyOracleVision` trade cases minted `Trade*5 + 2` crowns.

New: `tier2MarketSell()` finds the agent's most valuable surplus good and sells it to the settlement treasury. Skill bonus: `1.0 + Trade * Agnosis`, capped at `Being`. Treasury pays.

### Fallback Wages Removed (`internal/engine/production.go`)

Old: Agents with nil hex or depleted hex received `a.Wealth += 1` every tick (~1,440 crowns/day).

New: Failed production causes needs erosion (esteem -0.005, safety -0.003). No crowns minted.

### Sink Comments Updated (`internal/engine/market.go`)

`collectTaxes()` and `decayWealth()` comments no longer reference "unclosed money supply" — they are now complementary sinks in a closed system.

---

## Remaining Open Mints (Throttled)

Three locations in `internal/agents/behavior.go` still mint crowns from nothing but are now **throttled to once per sim-hour** (~24 crowns/day per agent) instead of every tick (~1,440/day). This is a 60x reduction.

| Location | Occupation | Trigger | Rate |
|----------|-----------|---------|------|
| `behavior.go` crafter branch | Crafter | No raw materials for any recipe | ~24 crowns/day |
| `behavior.go` alchemist branch | Alchemist | No herbs or exotics | ~24 crowns/day |
| `behavior.go` laborer branch | Laborer | Always (laborers have no production) | ~24 crowns/day |

### Mechanism

Gated by `tick % 60 == uint64(a.ID) % 60`, which ensures each agent's mint fires on a different tick within each 60-tick window, spreading load evenly.

### Why Not Zero?

- **Laborers** have no other income path — they don't produce goods to sell. Cutting to zero would kill them economically.
- **Crafters/Alchemists** without materials are effectively idle. The journeyman wage keeps them alive while they wait for market supply.
- These mints are small relative to the closed-economy flows (a single market trade of 5 grain at price 3 = 15 crowns).

### P0 Hotfix: Belonging Restored on Failed Production (2026-02-22)

After deploying the closed economy, `/observe` showed **zero births** and **104 trades across 51K agents**.

**Zero births root cause:** Removing fallback wages also removed the `+0.003 belonging` boost on failed production. Resource producers (farmers, miners, fishers, hunters) on depleted hexes spiraled below the `Belonging > 0.4` birth eligibility threshold, collapsing the eligible parent pool to near zero.

**Fix:** Restored a small belonging boost (`+0.001`) on all three failed-production paths in `production.go`. No crowns minted — just social recognition that the agent tried to work. Smaller than the old `+0.003` to avoid masking the economic pressure of depletion.

**Near-zero trade root cause:** When prices hit the Agnosis floor (~0.47 crowns for grain), clearing prices rounded to 0-1 crowns. The `if clearCrowns < 1 { clearCrowns = 1 }` floor meant agents with 0 wealth couldn't trade at all — the affordability check killed the match silently.

**Fix:** Removed the 1-crown minimum on clearing prices. When `clearCrowns` rounds to 0, trades execute as barter (free transfer — goods move, no crowns change hands). Skip the `buyer.Wealth < clearCrowns` check when price is 0 so penniless agents can still receive goods.

### P1 Fix: Merchant Death Spiral (2026-02-22)

All 6 dead Tier 2 agents were merchants at 0 wealth. Root cause: merchants have no `applyWork()` income — unlike laborers/crafters who get a throttled mint, merchants get only `Skills.Trade += 0.001`. Once wealth hits 0, they can't buy cargo at home market, can't earn from trade, and slowly starve.

**Fix 1 — Throttled wage** (`behavior.go`): Merchants now get the same `tick%60` gated 1-crown mint as laborers (~24 crowns/day). This is a survival floor, not real income.

**Fix 2 — Consignment buying** (`market.go`): When a merchant can't afford cargo with personal wealth, the home settlement treasury fronts the purchase cost. The merchant still sells at the destination and pockets the revenue. This is a closed transfer — crowns move from home treasury to destination treasury via the merchant, who keeps the margin. No new crowns minted.

The consignment model means merchants can always trade as long as their home settlement has treasury funds. The `ConsignmentDebt` field on the Agent struct tracks how much the treasury fronted. When the merchant sells at the destination and returns home, the debt is repaid from their revenue before they keep the profit. If the merchant can't fully repay (e.g., destination treasury was too poor to buy all cargo), the remaining debt carries forward. This keeps the home treasury whole — it's a loan, not a gift.

### Price Ratchet Fix (2026-02-22, tick 94,400)

After deploying waves 1 and 2 of survival fixes, `/observe` showed prices still locked at ceilings — grain permanently at 8.63 despite increased supply. Investigation revealed a **structural upward price bias** in the order-matched market engine.

**Root cause:** The clearing price used the midpoint of seller's ask (`Price * Matter = Price * 0.618`) and buyer's bid (`Price * Being = Price * 1.618`). The midpoint = `Price * (0.618 + 1.618) / 2 = Price * 1.118` — always 11.8% above the current price. The 70/30 blend (`price*0.7 + clearing*0.3`) therefore ratcheted prices up ~3.5% per trade tick. And the blend had no ceiling clamp, so it broke through the `BasePrice * Totality` bound that `ResolvePrice()` enforced.

**Fix — three changes to `resolveSettlementMarket()` in `market.go`:**

1. `ResolvePrice()` computes reference prices for ask/bid placement but does NOT overwrite `entry.Price`. Only real trade clearing data updates prices.
2. Clearing price = seller's ask price (not midpoint). Buyers pay what sellers accept. Eliminates the +11.8% upward bias.
3. The 70/30 blend result is clamped to `[BasePrice * Agnosis, BasePrice * Totality]`. Cannot exceed Phi-derived bounds.

**Key lesson:** When the price engine has a structural bias, no amount of supply-side fixes (threshold tuning, production boosts) or demand-side fixes (welfare wages, belonging) can compensate. The prices are mathematically locked. Fix the engine first, then tune parameters.

### Belonging Death Spiral Fix (2026-02-22, tick 93,781)

After wave 1 fixes, `/observe` still showed zero births and declining mood. Investigation revealed agents in survival mode (foraging/eating) received **zero belonging** — only working gave belonging. With avg survival at 0.385, many agents oscillated in survival mode, belonging decayed to 0, and the birth threshold (Belonging > 0.4) was unreachable.

**Fixes:**
1. `applyEat()` and `applyForage()` in `behavior.go` now give `+0.001 belonging` per tick. Agents in survival mode slowly build belonging instead of it collapsing.
2. Birth threshold lowered from `Belonging > 0.4` to `Belonging > 0.3` in `population.go`.
3. Settlement wages scaled with local grain price (later reverted to fixed 2 crowns after price ratchet fix — prices should come down naturally now).

### Settlement Welfare System (2026-02-22, tick 92,878)

`paySettlementWages()` added to `market.go`, called daily from `TickDay()`:
- Agents with `Wealth < 20` receive 2 crowns/day from home settlement treasury
- Capped at 1% of treasury per day to prevent drain
- Closed transfer (treasury → agent), no crowns minted
- Acts as safety net, not primary income — agents should earn through market trade

This is the missing reverse flow in the closed economy: taxes pull crowns into treasury, wages push them back to poor agents.

### Wealth Conservation (2026-02-22, tick 92,878)

Two crown sinks were destroying money in the closed economy:
1. `decayWealth()` destroyed ~0.24%/day of agent wealth above 20 crowns → **redirected to home settlement treasury** instead
2. Treasury upkeep in `collectTaxes()` destroyed crowns via bureaucracy model → **removed**, only population-based upkeep remains

No crowns leave the system. Total money supply is now conserved.

### Resource Producer Purpose Fix (2026-02-22, tick 120,192)

`ResolveWork` in `production.go` intercepted all resource producer (farmer, miner, fisher, hunter) work actions before `applyWork` in `behavior.go` could run. `ResolveWork` had esteem, safety, and belonging boosts but was **missing `Purpose += 0.002`**. All resource producers (~60% of agents) had purpose permanently at 0.0.

**Fix:** Added `a.Needs.Purpose += 0.002` to `ResolveWork` to match `applyWork`.

### Dynamic Φ-Targeted Welfare (2026-02-22, tick 120,192)

At tick 118,329 `/observe` showed treasuries holding 71% of all wealth — far above the target of `1 - Matter` ≈ 38.2%. The original fixed 1% outflow cap was too low. Rather than tune to another fixed value, `paySettlementWages()` now **self-regulates** to converge toward the Φ-derived target ratio.

**Mechanism:**
1. Compute `treasuryShare = totalTreasury / (totalTreasury + totalAgent)` once per day
2. Compare against target `1 - phi.Matter` (~0.382)
3. Scale outflow quadratically with excess:
   - `outflowRate = 0.01 + excess² × 40`
   - At target: 1% baseline
   - At 50% treasury share: ~4%
   - At 70% treasury share: ~4.3%
   - Cap: `phi.Agnosis` (~23.6%)
   - Below target: 0.5% (minimal, lets taxes refill)

**Self-correcting properties:**
- Quadratic scaling decelerates near target — prevents overshoot
- At equilibrium: tax/decay inflow ≈ welfare outflow
- Disruptions (wars, disasters) shift the ratio; system adjusts automatically
- All parameters derive from Φ — no magic numbers

**Other changes:** Eligibility threshold raised from `Wealth < 20` to `Wealth < 50` so more agents qualify.

### Welfare Wage Bottleneck Fix (2026-02-22, tick 128,232)

`/observe` at tick 126,272 showed treasury share had **worsened** from 71% to 74.3%. Investigation revealed the dynamic outflow rate was computing correctly (5.22%) but the **fixed 2-crown per-agent wage** was the bottleneck. With avg treasury of 2.08M and 80 agents per settlement, actual outflow was only 160 crowns/day (0.0077%) instead of the target 108K (5.22%).

**Fix:** Wage is now computed dynamically from the budget:
```
budget = outflowRate * settlement.Treasury
wage = budget / eligible_agents
```

At avg settlement (2M treasury, 60 eligible agents, 5.2% rate): wage is ~1,808 crowns/agent/day. The treasury actually drains at the computed rate now. As the ratio drops toward 38%, the rate drops, wages drop, and it stabilizes.

**Lesson:** When building a dynamic targeting system, ensure the *mechanism* (per-agent wage) can actually deliver the *target* (outflow rate). A 5% rate is meaningless if the per-agent pipe is 700x too narrow.

### What to Monitor

After deploying, watch these via `/api/v1/stats/history`:

1. **Treasury/agent wealth ratio** — should converge from 71/29 toward 38/62
2. **Avg mood** — should improve as purpose and wealth flow reach agents
3. **`total_wealth`** — should stabilize (sinks = remaining mints + wealth conservation)
4. **`avg_survival`** — should stay above 0.4
5. **Births** — should continue (9,239 achieved by tick 118,329)
6. **Trade volume** — should remain above 18K
7. **Market health** — should stay above 90%
8. **Settlement consolidation** — 714 is pathological; watch for merger/abandonment

### Future Options

If the remaining mints prove problematic:
- **Route through treasury**: Laborers/journeymen get paid from settlement treasury (closed transfer). Requires treasury to have funds.
- **Replace with goods**: Journeymen produce a low-value "labor" good they can sell on the market.
- **Remove entirely**: If the closed economy is healthy enough that these agents can survive purely on market income from occasional surplus.

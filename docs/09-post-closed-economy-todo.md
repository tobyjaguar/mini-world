# 09 — Post-Closed-Economy TODO

Assessment from `/observe` at tick 92,290 (Spring Day 65, Year 1). The closed economy was functioning mechanically — trades execute, treasuries collect, consignment works — but the transition was too harsh. Multiple rounds of diagnosis and fixes were needed.

## Timeline

| Tick | Deploy | Key Change |
|------|--------|------------|
| 92,290 | Observe | Initial diagnosis: survival 0.375, mood 0.16, grain 8.63 |
| 92,878 | Wave 1 | Grain threshold, wealth decay redirect, settlement wages, fisher boost |
| 93,249 | Observe | No improvement: survival 0.385, mood 0.156, zero births, grain 8.63 |
| 93,781 | Wave 2 | Belonging on eat/forage, birth threshold 0.4→0.3, wages scale with grain price |
| 94,254 | Observe | Still no improvement: survival 0.382, mood 0.142, zero births, grain 8.63 |
| 94,400 | Wave 3 | **Price ratchet fix** — the root cause of all price issues |
| 94,764 | Observe | Prices normalizing: grain 8.63→4.65, fish→2.00, tools 2.36→7.30 |
| 118,329 | Observe | Major recovery: 9,239 births, 18,512 trades, market health 96.9% |
| 118,329 | Wave 4 | Stats history fix, gardener startup race fix |
| 120,192 | Wave 5 | Purpose boost for resource producers, dynamic Φ-targeted welfare |
| 126,272 | Observe | Treasury share worsened to 74.3% — fixed wage bottleneck identified |
| 128,232 | Wave 6 | Dynamic wage from budget — treasury outflow now matches computed rate |
| 142,285 | Observe | Treasury 41% (converging), Gini 0.645 (worsening), survival stuck 0.385 |
| 144,681 | Wave 7 | Food buying in decision tree, progressive welfare, settlement migration fix |

## Root Cause: Price Ratchet in Market Engine

The order-matched market engine had a **structural upward price bias** that made all other fixes ineffective. Prices were mathematically unable to come down:

1. **Clearing midpoint biased +11.8% above entry price**: Sell ask = `Price * Matter (0.618)`, buy bid = `Price * Being (1.618)`, midpoint = `Price * 1.118`. Every trade pushed the price up.
2. **70/30 blend had no ceiling clamp**: `entry.Price = Price*0.7 + clearingPrice*0.3` could exceed `BasePrice * Totality` (the Phi-derived ceiling).
3. **Dual price update fought itself**: `ResolvePrice()` set price from supply/demand (clamped), then the blend overwrote it (unclamped). The biased blend always won.

**Result**: Grain price locked at 8.63 (ceiling = `2 * 4.236 = 8.47`, slightly exceeded by unclamped blend). All food at ceiling, all manufactured goods at floor. Economy frozen.

### Wave 1 & 2 Fixes (Necessary but Insufficient)

These fixes addressed real problems but couldn't work while prices were ratcheted:

| Fix | Status | Notes |
|-----|--------|-------|
| Grain surplus threshold 5→3 | Applied | More food posted to market, but prices didn't drop |
| Fish as alternative food demand | Applied | Good structural change |
| Wealth decay → treasury redirect | Applied | Stops crown destruction |
| Treasury upkeep sink removed | Applied | Stops crown destruction |
| Settlement wages (2 crowns/day) | Applied | Safety net for poor agents, not primary income |
| Fisher production boost (*2→*3) | Applied | More fish produced |
| Belonging on eat/forage (+0.001) | Applied | Breaks belonging death spiral |
| Birth threshold 0.4→0.3 | Applied | More achievable during crisis |

### Wave 3: Price Ratchet Fix (tick 94,400)

**The critical fix.** Three changes to `resolveSettlementMarket()` in `market.go`:

1. **Reference prices decouple from entry.Price**: `ResolvePrice()` computes reference prices used for ask/bid spreads but does NOT overwrite `entry.Price`. Only real trade data updates prices.
2. **Clearing price = seller's ask**: Instead of `(ask + bid) / 2 = Price * 1.118`, the clearing price is the seller's ask price. Buyers pay what sellers accept. No upward bias.
3. **Blend clamped to bounds**: The 70/30 blend result is clamped to `[BasePrice * Agnosis, BasePrice * Totality]`. Cannot break through Phi-derived bounds.

### Lesson Learned

When layering fixes on a broken market engine, no amount of welfare, threshold tuning, or production boosting can overcome a price mechanism that mathematically prevents equilibrium. **Fix the price engine first**, then tune parameters. The welfare system and belonging fixes are still valuable — they address real structural gaps — but they couldn't compensate for a ratchet that pushed every price to its ceiling.

## Wave 4: Infrastructure Fixes (tick 118,329)

Two P2 issues from earlier rounds fixed:

1. **Stats history query** — FIXED: `toTick` defaulted to `^uint64(0)` (max uint64). The modernc.org/sqlite driver rejects uint64 values with the high bit set. Changed to `uint64(1<<63 - 1)`. Stats history now records and returns data.
2. **Gardener startup race** — FIXED: Added `waitForAPI()` with exponential backoff (2s→30s, 5min deadline) in `cmd/gardener/main.go`. Gardener waits for worldsim HTTP server before first observation cycle.

## Wave 5: Mood & Treasury Rebalancing (tick 120,192)

Two remaining structural issues from post-recovery `/observe`:

1. **Resource producer purpose drought** — FIXED: `ResolveWork` in `production.go` intercepted work actions for all resource producers (farmers, miners, fishers, hunters) before `applyWork` in `behavior.go` could run. `ResolveWork` was missing `Purpose += 0.002`. All resource producers had purpose permanently at 0.0, dragging mood down via the satisfaction formula (`purpose * 1/15`).

2. **Treasury hoarding (71% of wealth)** — FIXED: `paySettlementWages()` now dynamically targets the Φ⁻¹ treasury/agent ratio (~38% treasury / ~62% agents). Computes global `treasuryShare = totalTreasury / (totalTreasury + totalAgent)` daily. Outflow rate scales quadratically with excess above target:
   - At target (38%): 1% outflow baseline
   - At 50%: ~4% outflow
   - At 70%: ~4.3% outflow
   - Cap: Agnosis (~23.6%)
   - Below target: 0.5% (minimal, lets taxes refill)

   Eligibility threshold raised from wealth < 20 to wealth < 50.

## Wave 6: Welfare Wage Bottleneck Fix (tick 128,232)

`/observe` at tick 126,272 revealed the dynamic outflow rate was correct but the **fixed 2-crown wage per agent** was 700x too narrow a pipe. Avg settlement had 2.08M treasury; at 5.22% rate the budget was 108K crowns/day, but 80 agents × 2 crowns = only 160 crowns actually flowed. Treasury share worsened from 71% → 74.3%.

**Fix:** Wage is now `budget / eligible_agents` — computed dynamically from the outflow budget. At avg settlement, wage is ~1,808 crowns/agent/day instead of 2. The treasury actually drains at the computed rate, and the self-correcting dynamics now work as designed.

## Wave 7: Food Economy, Fair Welfare, Settlement Consolidation (tick 144,681)

`/observe` at tick 142,285 revealed three problems:

1. **Agents forage instead of buying food** — the decision tree had no "buy food" path. Agents with 18,800 crowns avg wealth still foraged because `decideSurvival()` only offered eat (if food in inventory) or forage (if not). The market economy was disconnected from survival needs. Trade volume stuck at 4,244 vs 18K peak.

   **Fix:** New `ActionBuyFood` in behavior.go. When hungry with no food but wealth >= 1, agents buy food from the settlement market at current price. Crowns flow to treasury (closed transfer). Foraging is now last resort for penniless agents only. This creates the economic loop: agents work → earn → buy food → sellers profit → economy circulates.

2. **Gini spike to 0.645** — flat welfare wage gave same amount to agents at wealth 0 and wealth 49. Agents near the threshold accumulated fast while truly poor agents stayed poor.

   **Fix:** Progressive welfare. Wage now scales inversely with wealth: `weight = (threshold - wealth) / threshold`. Agent at 0 gets full share, agent at 49 gets 2%. Same total budget from the Φ-targeting system, fairer distribution.

3. **714 settlements frozen — migration bug** — `processSeasonalMigration()` changed `a.HomeSettID` but never rebuilt `SettlementAgents` map. Population counts read from stale arrays, so settlements never appeared to shrink. Viability checks and abandonment never triggered.

   **Fix:** Added `rebuildSettlementAgents()` called after migration. Reconstructs the map from current `HomeSettID` values and updates population counts. Settlements that lose population through migration will now correctly reflect lower pop and trigger viability/abandonment.

## Remaining TODO

### P2: Fisher skill alias

`productionAmount()` still uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Works but technically wrong.

### P2: Merchant extinction

All Tier 2 merchants died during the price normalization. May self-correct as the economy stabilizes. Monitor.

## Success Criteria

### Achieved (as of tick 142,285)
- Grain price normalized — within Phi bounds (0.47 to 5.0)
- Births resumed — 4,915 (recovering after restart)
- Market health 98%
- Treasury/agent ratio converging — 74% → 41% (target 38%)
- Population growing — 62,847 (+10K since wave 5)

### Still Monitoring (after wave 7)
- Avg mood — 0.105, should improve as food buying drives survival up
- Gini — 0.645, should improve with progressive welfare
- Settlement count — 714 frozen, should start declining with migration fix
- Trade volume — 4,244, should surge as food buying creates demand
- Survival — 0.385, should improve as agents buy food instead of foraging

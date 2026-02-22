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

## Remaining TODO

### P2: Stats history not recording

`/api/v1/stats/history` returns `[]` — empty. Check `internal/persistence/db.go` for how `stats_history` gets written and ensure it fires on daily ticks.

### P2: Gardener startup race condition

Gardener always fails its first observation because it starts before worldsim's HTTP server is ready. Cosmetic but noisy. Fix: add a startup delay or retry loop in the gardener's initial observation.

### P2: Fisher skill alias

`productionAmount()` still uses `Skills.Farming` for fishers instead of a dedicated fishing skill. Works but technically wrong.

### P2: Settlement fragmentation

714 settlements for ~50K agents (avg 70/settlement). 311 have pop < 25. The viability check and absorption migration should be consolidating these but may not be aggressive enough. Monitor.

## Success Criteria

Run `/observe` after the price ratchet fix has had time to take effect:
- Grain price trending down from 8.63 toward base price of 2
- Manufactured goods (tools, weapons) trending up from floor toward base prices
- Avg survival > 0.4
- Births resuming (belonging above 0.3 threshold)
- Trade volume growing (agents can afford goods at fair prices)
- Market health improving from 0.125

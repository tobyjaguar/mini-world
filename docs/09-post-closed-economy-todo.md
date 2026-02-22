# 09 — Post-Closed-Economy TODO

Assessment from `/observe` at tick 92,290 (Spring Day 65, Year 1). The closed economy is functioning mechanically — trades execute, treasuries collect, consignment works — but the transition is too harsh. Agents are starving because crowns pooled in treasuries and grain isn't reaching the market.

## Critical Numbers

| Metric | Value | Target | Gap |
|--------|-------|--------|-----|
| Avg Survival | 0.375 | > 0.4 | Agents can't eat |
| Avg Mood | 0.16 | > 0.3 | Crashed from 0.64 |
| Deaths:Births | 2,576:573 (4.5:1) | < 2:1 | Population declining |
| Grain Price | 8.63 (base 2) | < 4.0 | 431% inflated |
| Trade Volume | 5,784 | Stable/growing | OK — 55x improvement |
| Agent Wealth | 952M (was 980M) | Stable | Deflating 28M/cycle |
| Treasury Wealth | 1.04B (was 1.01B) | Flowing back | Accumulating, not flowing |

## TODO — Priority Order

### P0: Grain supply crisis

Grain is the universal food need but farmers keep 5 units before selling. With production of 1-3/tick and personal consumption, few farmers ever have surplus above 5 to sell. Meanwhile every agent demands grain.

**Fix:** Lower farmer grain surplus threshold from 5 to 3 in `surplusThreshold()` (`internal/engine/market.go`). Farmers still keep enough to eat but put more on the market.

```go
case agents.GoodGrain, agents.GoodFish:
    if a.Occupation == agents.OccupationFarmer || a.Occupation == agents.OccupationFisher {
        return 3  // was 5
    }
    return 2  // was 3 — non-farmers keep less too
```

**Also consider:** Fishers should have the same threshold reduction (fish is an alternative food).

### P1: Treasury hoarding — crowns not flowing back to agents

Treasuries gained +30M while agents lost -28M. Taxes and upkeep pull crowns out of agents into treasuries, but the only way crowns flow back is through merchant consignment sales and Tier 2 trade. 95% of agents (Tier 0) have no path to earn from treasury.

**Fix:** Add settlement wage — treasury pays a small daily amount to agents who worked that day. This is the missing reverse flow. In `collectTaxes()` or a new daily function:

- For each settlement, pay `1 crown` to each agent with `Wealth < 20` from treasury
- Cap at e.g. `treasury * 0.01` per day so it doesn't drain treasuries
- This creates a closed loop: taxes pull crowns into treasury → wages push them back to agents

### P1: Wealth decay destroying crowns in a closed economy

`decayWealth()` destroys `~0.24%/day` of agent wealth above 20 crowns. `collectTaxes()` treasury upkeep destroys crowns too. In the old open economy these sinks balanced the mints. In the closed economy, **destroyed crowns are permanently lost** — the money supply shrinks every day.

**Fix options (pick one):**
1. **Reduce decay rates** — halve both agent wealth decay and treasury upkeep. Slower drain.
2. **Redirect decay to treasury** — instead of destroying wealth, decay flows into the local settlement treasury. No crowns destroyed, just redistributed.
3. **Remove decay entirely** — the closed economy already has natural sinks (trade friction, consignment). Decay may no longer be needed.

Option 2 is cleanest — wealth "decays" but crowns stay in circulation via treasury.

### P1: Fisher mood spiral (-0.43 to -0.49)

All Tier 2 fishers are miserable and worsening. Two root causes:

1. **Fish at price floor (0.47 crowns)** — selling fish earns almost nothing. Fishers produce goods worth ~0.47 crowns while crafters produce goods worth 5-25 crowns.
2. **Fisher skill bug** — `productionAmount()` uses `Skills.Farming` for fishers instead of a fishing skill. Fishers don't scale production with experience.

**Fix 1:** Add a `Fishing` skill or alias fisher production to use `Skills.Trade` or another existing skill that grows. In `internal/engine/production.go`:
```go
case agents.OccupationFisher:
    p := int(a.Skills.Farming * 3)  // was * 2, increase output
```

**Fix 2:** Fish price floor issue is a symptom of oversupply or low demand. If fishers produce more, they have more surplus to sell, and the barter system moves it. The key is that fish has the same food utility as grain — agents should buy whichever is cheaper.

### P2: Stats history not recording

`/api/v1/stats/history` returns `[]` — empty. Stats history should be recording per-tick or per-day snapshots. Either the save interval hasn't triggered yet since deploy, or the stats recording was broken by the restarts. Check `internal/persistence/db.go` for how `stats_history` gets written and ensure it fires on daily ticks.

### P2: Gardener startup race condition

Gardener always fails its first observation because it starts before worldsim's HTTP server is ready. Cosmetic but noisy. Fix: add a startup delay (e.g., `time.Sleep(10 * time.Second)`) or retry loop in the gardener's initial observation.

## Implementation Order

1. **Grain threshold** — 2-line change, immediate survival impact
2. **Wealth decay → treasury redirect** — stops permanent crown destruction
3. **Settlement wage** — closes the treasury→agent loop
4. **Fisher production boost** — helps fisher mood + food supply
5. **Stats history check** — observability
6. **Gardener retry** — polish

## Success Criteria

After implementing, run `/observe` and check:
- Avg survival > 0.4
- Births:deaths ratio improving toward 1:1
- Grain price trending down (< 6.0)
- Agent wealth stabilizing (not deflating)
- Trade volume maintained or growing
- Fisher Tier 2 mood improving (> -0.3)

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

### What to Monitor

After deploying, watch these via `/api/v1/stats/history`:

1. **`total_wealth`** — should stabilize or gently decline (sinks > remaining mints). If it rises steadily, the throttled mints are too generous or there's another leak.
2. **`avg_survival`** — should stay above 0.4. If it drops sharply, laborers/crafters may be starving from lost income. Consider routing their wages through settlement treasury instead.
3. **Trade volume** — should be positive. If it drops to zero, the order-matching engine may be too restrictive (check that `phi.Matter` and `phi.Being` price bands aren't too narrow).
4. **Settlement treasuries** — merchant trade and Tier 2 trade now drain treasuries. If treasuries hit zero across the board, trade freezes. May need to increase tax rates or add treasury seeding.

### Future Options

If the remaining mints prove problematic:
- **Route through treasury**: Laborers/journeymen get paid from settlement treasury (closed transfer). Requires treasury to have funds.
- **Replace with goods**: Journeymen produce a low-value "labor" good they can sell on the market.
- **Remove entirely**: If the closed economy is healthy enough that these agents can survive purely on market income from occasional surplus.

# 07: Closed Economy Implementation — Crown Conservation

## Status: Ready for Implementation

## Problem Statement

Crossworlds has an **unclosed money supply** — crowns are continuously minted from nothing by market sell operations, fallback wages, and Tier 2 trade actions. Current band-aid sinks (wealth decay, treasury upkeep at Agnosis-scaled rates) cannot keep pace because minting runs every tick/hour across thousands of agents while sinks run once daily at fractional percentages. Total wealth grows explosively and without bound.

See `docs/06-monetary-system.md` for the full diagnostic. This document provides the implementation plan to fix it.

## Philosophical Grounding

In Wheeler's emanationist framework, the One emanates substance into the manifest world as a one-time act — it does not continuously inject new substance at the material level. Within the material plane, exchange is **zero-sum transfer**: substance flows between forms but is neither created nor destroyed. The current system accidentally makes every selling agent a little Monad, emanating crowns from nothing.

The fix follows the **via negativa** principle: don't add more complex sinks to compensate for false sources. Remove the false sources. The economy heals by subtraction, not addition.

After the fix, the only remaining crown creation should be **deliberate and controlled**:
- Initial world generation (one-time endowment)
- Discovery bonuses (50–150 crowns, 5%/week per settlement — rare, small, represents genuine new value entering the world)

All other crown movement becomes zero-sum transfer between agents.

## Implementation: 5 Steps

### Step 1: Order-Matched Market Engine (THE BIG ONE)

**File**: `internal/engine/market.go`

**What changes**: Replace the decoupled sell/buy loops in `executeTrades()` and the per-agent call pattern in `resolveSettlementMarket()` with a proper order-matching engine where crowns transfer from buyer to seller — never created, never destroyed.

**Current flow** (broken):
```
for each agent:
    sell surplus → agent.Wealth += revenue (MINTED FROM NOTHING)
    buy needs   → agent.Wealth -= price  (DESTROYED)
```

**New flow** (closed):
```
Phase 1: Collect sell orders and buy orders from all agents
Phase 2: For each good, sort and match orders
Phase 3: Execute matched trades (buyer pays seller directly)
Phase 4: Unmatched orders remain (no free money, no phantom goods)
Phase 5: Update market entry prices from clearing prices
```

#### 1a. Add Order type

Add this to `internal/engine/market.go` (or a new file `internal/engine/orders.go` if preferred):

```go
// Order represents a single buy or sell intention in the market.
type Order struct {
    Agent    *agents.Agent
    Good     agents.GoodType
    Quantity int
    Price    float64 // Minimum acceptable price (sell) or maximum willing price (buy)
    IsSell   bool
}
```

#### 1b. Rewrite `resolveSettlementMarket()`

Replace the current function body. The new version:

1. **Resets and aggregates supply/demand** (keep existing logic for price discovery — lines 32–59 are fine as-is).

2. **Resolves prices** (keep existing logic — lines 61–79 are fine as-is, the population-scaled supply floor and `ResolvePrice()` call remain unchanged).

3. **Collects orders** (replaces the old per-agent `executeTrades` call):

```go
// Collect sell orders.
var sellOrders []Order
for _, a := range settAgents {
    if !a.Alive {
        continue
    }
    for good, qty := range a.Inventory {
        surplus := qty - surplusThreshold(a, good)
        if surplus <= 0 {
            continue
        }
        entry, ok := market.Entries[good]
        if !ok {
            continue
        }
        // Seller's minimum acceptable price: market price * Matter (~62%).
        // Willing to sell at a discount but not a fire sale.
        minPrice := entry.Price * phi.Matter
        sellOrders = append(sellOrders, Order{
            Agent:    a,
            Good:     good,
            Quantity: surplus,
            Price:    minPrice,
            IsSell:   true,
        })
    }
}

// Collect buy orders.
var buyOrders []Order
for _, a := range settAgents {
    if !a.Alive {
        continue
    }
    for _, good := range demandedGoods(a) {
        entry, ok := market.Entries[good]
        if !ok {
            continue
        }
        // Buyer's maximum price: market price * Being (~162%).
        // Willing to pay a premium for needed goods.
        maxPrice := entry.Price * phi.Being
        buyOrders = append(buyOrders, Order{
            Agent:    a,
            Good:     good,
            Quantity: 1, // Buy one unit at a time (matches current behavior)
            Price:    maxPrice,
            IsSell:   false,
        })
    }
}
```

4. **Matches orders per good** (new logic):

```go
// Match orders for each good type.
for good, entry := range market.Entries {
    // Filter orders for this good.
    var goodSells []Order
    for _, o := range sellOrders {
        if o.Good == good {
            goodSells = append(goodSells, o)
        }
    }
    var goodBuys []Order
    for _, o := range buyOrders {
        if o.Good == good {
            goodBuys = append(goodBuys, o)
        }
    }

    if len(goodSells) == 0 || len(goodBuys) == 0 {
        continue
    }

    // Sort: sells ascending by price (cheapest first),
    //        buys descending by price (highest bidder first).
    sort.Slice(goodSells, func(i, j int) bool {
        return goodSells[i].Price < goodSells[j].Price
    })
    sort.Slice(goodBuys, func(i, j int) bool {
        return goodBuys[i].Price > goodBuys[j].Price
    })

    // Match until prices cross or orders exhausted.
    si, bi := 0, 0
    totalTraded := 0
    totalRevenue := 0.0

    for si < len(goodSells) && bi < len(goodBuys) {
        sell := &goodSells[si]
        buy := &goodBuys[bi]

        // Prices crossed — no more matches possible.
        if sell.Price > buy.Price {
            break
        }

        // Clearing price = midpoint (fair to both sides).
        clearingPrice := (sell.Price + buy.Price) / 2.0
        clearingCrowns := uint64(clearingPrice + 0.5) // Round to nearest crown
        if clearingCrowns < 1 {
            clearingCrowns = 1
        }

        // Determine trade quantity (min of what seller has and buyer wants).
        tradeQty := sell.Quantity
        if buy.Quantity < tradeQty {
            tradeQty = buy.Quantity
        }

        // Execute each unit of the matched trade.
        traded := 0
        for i := 0; i < tradeQty; i++ {
            if buy.Agent.Wealth < clearingCrowns {
                break // Buyer can't afford
            }
            // CLOSED TRANSFER: buyer pays seller directly.
            buy.Agent.Wealth -= clearingCrowns
            sell.Agent.Wealth += clearingCrowns
            buy.Agent.Inventory[good]++
            sell.Agent.Inventory[good]--
            traded++
        }

        totalTraded += traded
        totalRevenue += float64(traded) * clearingPrice

        // Update remaining quantities.
        sell.Quantity -= traded
        buy.Quantity -= traded

        if sell.Quantity <= 0 {
            si++
        }
        if buy.Quantity <= 0 {
            bi++
        }
    }

    // Update market entry price from actual clearing data.
    if totalTraded > 0 {
        entry.Price = totalRevenue / float64(totalTraded)
    }
    // If no trades occurred, price stays at the supply/demand resolved price.
    // This is correct — it represents the theoretical price that failed to clear.
}
```

5. Add `"sort"` to the import block at the top of `market.go`.

#### 1c. Remove old `executeTrades()` function

Delete the entire `executeTrades()` function (current lines 199–233). It is fully replaced by the matching engine above and should not be called anywhere.

#### 1d. Verify: no other callers

Search for `executeTrades` in the codebase — it should only be called from `resolveSettlementMarket`. Confirm and remove.

---

### Step 2: Close Merchant Trade Mint

**File**: `internal/engine/market.go`

**What changes**: `sellMerchantCargo()` (current line 489) currently mints crowns from nothing when merchants sell cargo at the destination. Instead, merchant cargo sales should go through the destination settlement's order-matching engine.

**Current flow** (broken):
```go
func sellMerchantCargo(a *agents.Agent, market *economy.Market) {
    for good, qty := range a.TradeCargo {
        revenue := uint64(float64(qty) * entry.Price)
        a.Wealth += revenue  // MINTED FROM NOTHING
    }
}
```

**New flow** (closed): Replace `sellMerchantCargo` with a function that injects merchant sell orders into the destination market's matching engine. Two options:

**Option A (simpler — recommended for now):** Have merchants sell to the destination settlement's treasury as the buyer. This is a closed transfer (treasury pays merchant) and represents the settlement's collective purchasing power.

```go
func sellMerchantCargo(a *agents.Agent, market *economy.Market, destSett *social.Settlement) {
    for good, qty := range a.TradeCargo {
        entry, ok := market.Entries[good]
        if !ok || qty <= 0 {
            continue
        }
        for i := 0; i < qty; i++ {
            price := uint64(entry.Price + 0.5)
            if price < 1 {
                price = 1
            }
            if destSett.Treasury >= price {
                destSett.Treasury -= price
                a.Wealth += price // CLOSED TRANSFER from treasury
            }
            // If treasury can't afford it, merchant keeps the goods.
            // (They'll try again next cycle.)
        }
    }
}
```

Update the call site in `resolveMerchantTrade()` (current line 352) to pass `destSett`:
```go
// Old:
sellMerchantCargo(a, destSett.Market)
// New:
sellMerchantCargo(a, destSett.Market, destSett)
```

**Option B (more realistic):** Queue merchant sell orders into the destination settlement's next `resolveSettlementMarket()` call. This requires merchants to be temporarily "present" in the destination settlement's agent list during market resolution. More complex, can be deferred.

**Decision**: Implement Option A now. It's simple, closes the mint, and can be upgraded to Option B later.

**Note on merchant buy side**: The merchant buy side in `resolveMerchantTrade()` (lines 388–403) already correctly destroys crowns when the merchant buys at their home market (`a.Wealth -= buyPrice`). However, this deduction doesn't go to anyone — it's destroyed. After Step 1, the home market will be order-matched, so merchants should also buy through the matching engine. For now, the home buy can remain as-is since the crowns are destroyed (deflationary), which is better than inflationary. Mark as a follow-up.

---

### Step 3: Remove Fallback Wages

**File**: `internal/engine/production.go`

**What changes**: Three locations in `ResolveWork()` mint 1 crown when a hex is nil or depleted (lines 35, 46, 64). Remove the `a.Wealth += 1` lines. An agent that cannot produce should experience need, not receive welfare from the void.

**Current** (3 locations):
```go
a.Wealth += 1
a.Needs.Esteem += 0.01
a.Needs.Safety += 0.005
a.Needs.Belonging += 0.003
```

**New** (all 3 locations):
```go
// Hex depleted/missing — agent fails to produce.
// Unproductive labor erodes esteem but doesn't create money.
a.Needs.Esteem -= 0.005
a.Needs.Safety -= 0.003
// Belonging stays unchanged — community persists even when work fails.
```

**Rationale**: You can't extract from privation. In Wheeler's framework, depleted hexes represent material absence — there is nothing to work with. The agent should feel the friction of scarcity (declining esteem/safety), which drives migration, occupation change, or market purchasing. The current system rewards idleness on barren land.

**Impact consideration**: This may increase agent mortality in the short term as poor agents on depleted hexes lose their trickle income. This is intentional — it creates genuine economic pressure and makes hex regeneration, migration, and trade more consequential. Monitor `avg_survival` and `deaths` in the daily log. If die-offs become catastrophic, consider adding a small subsistence mechanic (e.g., settlement food stores that agents can draw from) rather than minting crowns.

---

### Step 4: Route Tier 2 Trade Through Market

**File**: `internal/engine/cognition.go`

**What changes**: Two identical blocks (lines 167–177 and lines 417–425) mint `trade_skill * 5 + 2` crowns when Haiku tells an agent to trade. Replace with actual market participation.

**Current** (both locations):
```go
case "trade":
    if a.HomeSettID != nil {
        if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
            if sett.Market != nil {
                earned := uint64(a.Skills.Trade*5) + 2
                a.Wealth += earned  // MINTED FROM NOTHING
                a.Skills.Trade += 0.005
            }
        }
    }
```

**New** (both locations): The agent sells their most valuable surplus through the market, with trade skill providing a price bonus:

```go
case "trade":
    if a.HomeSettID != nil {
        if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
            if sett.Market != nil {
                // Tier 2 trade: sell most valuable surplus at a skill-boosted price.
                // Trade skill improves the effective price the agent gets.
                tier2MarketSell(a, sett)
                a.Skills.Trade += 0.005
            }
        }
    }
```

Add new helper function in `cognition.go` (or `market.go`):

```go
// tier2MarketSell lets a Tier 2 agent sell their most valuable surplus good
// to the settlement treasury. Trade skill provides a price bonus.
func tier2MarketSell(a *agents.Agent, sett *social.Settlement) {
    market := sett.Market
    if market == nil {
        return
    }

    // Find the agent's most valuable surplus good.
    bestGood := agents.GoodType(0)
    bestValue := 0.0
    hasSurplus := false

    for good, qty := range a.Inventory {
        surplus := qty - surplusThreshold(a, good)
        if surplus <= 0 {
            continue
        }
        entry, ok := market.Entries[good]
        if !ok {
            continue
        }
        value := entry.Price * float64(surplus)
        if value > bestValue {
            bestValue = value
            bestGood = good
            hasSurplus = true
        }
    }

    if !hasSurplus {
        return
    }

    entry := market.Entries[bestGood]
    surplus := a.Inventory[bestGood] - surplusThreshold(a, bestGood)
    if surplus <= 0 {
        return
    }

    // Sell up to surplus quantity. Trade skill boosts effective price.
    // Bonus: 1.0 + (trade_skill * Agnosis), capped at Being (~1.618).
    skillBonus := 1.0 + (a.Skills.Trade * phi.Agnosis)
    if skillBonus > phi.Being {
        skillBonus = phi.Being
    }

    for i := 0; i < surplus; i++ {
        price := uint64(entry.Price*skillBonus + 0.5)
        if price < 1 {
            price = 1
        }
        // CLOSED TRANSFER: settlement treasury pays the agent.
        if sett.Treasury >= price {
            sett.Treasury -= price
            a.Wealth += price
            a.Inventory[bestGood]--
        }
    }
}
```

**Note**: If `tier2MarketSell` is placed in `market.go`, it can reference `surplusThreshold` directly. If placed in `cognition.go`, either move `surplusThreshold` to be exported or duplicate the logic. Recommend placing in `market.go` since it's market logic.

**Import**: `tier2MarketSell` needs `phi` imported in whichever file it lives. `market.go` already imports `phi`.

---

### Step 5: Monitor and Tune

After deploying Steps 1–4, the crown supply becomes a **conservation system**:

```
Total Crowns = Initial Endowment + Σ Discovery Bonuses - Σ Decay/Upkeep
```

**What to watch** (via `GET /api/v1/stats/history` and daily log output):

| Metric | Healthy Range | Action if Outside |
|--------|--------------|-------------------|
| `total_wealth` trend | Stable or gently declining | If still climbing: check for remaining mints. If crashing: reduce decay rates |
| `avg_survival` | > 0.4 | If dropping fast: fallback wage removal (Step 3) may be too aggressive — add subsistence mechanic |
| Deaths per day | < 5% of population | Same as above |
| Market health | > 0.35 | If low: prices may be too volatile — check supply floor scaling |
| Trade volume | > 0 | If zero: buyers may be too poor to buy — may need to seed more initial wealth |
| Gini coefficient | < 0.7 | High Gini is expected but extreme values mean wealth is stuck at the top |

**Tuning knobs** (all Φ-derived, adjust multipliers only):

- `decayWealth` rate: currently `Agnosis * 0.01` (~0.24%/day). Can increase to `Agnosis * 0.02` if wealth is still growing, or decrease to `Agnosis * 0.005` if economy is too deflationary.
- `treasuryUpkeep` rate: currently `Agnosis * 0.01`. Same tuning range.
- Discovery bonus frequency: currently 5%/week per settlement. Can reduce to 2%/week or eliminate entirely if the economy reaches self-sustaining equilibrium.
- Settlement pop upkeep: currently `pop * Agnosis * 0.5`. This is the main treasury drain — adjust the `0.5` multiplier.

**Expected behavior after deployment**: Total wealth will spike briefly (existing minted crowns still in circulation) then begin declining as decay exceeds the now-tiny injection rate. It should find an equilibrium where decay ≈ discovery injection, creating a stable or gently deflationary currency. Agent behavior should become more consequential — trade skill, hex location, and occupation choice will matter more because wealth is scarce and zero-sum.

---

## Files Modified (Summary)

| File | Step | Change |
|------|------|--------|
| `internal/engine/market.go` | 1 | Add `Order` type. Rewrite `resolveSettlementMarket()` with order-matching engine. Delete `executeTrades()`. Add `sort` import. |
| `internal/engine/market.go` | 2 | Rewrite `sellMerchantCargo()` to sell to destination treasury (closed transfer). Update call site in `resolveMerchantTrade()`. |
| `internal/engine/market.go` | 4 | Add `tier2MarketSell()` helper for Tier 2 trade actions. |
| `internal/engine/production.go` | 3 | Remove `a.Wealth += 1` from 3 fallback wage locations (lines 35, 46, 64). Replace with esteem/safety decay. |
| `internal/engine/cognition.go` | 4 | Replace crown minting in trade actions (lines 173–174 and 421–422) with call to `tier2MarketSell()`. |

## Implementation Order

Implement in this order — each step independently improves the economy:

1. **Step 1** (matching engine) — eliminates ~80% of crown minting. Most impactful.
2. **Step 3** (fallback wages) — simple, eliminates per-tick minting on depleted hexes.
3. **Step 4** (Tier 2 trade) — simple, eliminates LLM-directed minting.
4. **Step 2** (merchant trade) — closes the remaining mint in inter-settlement trade.
5. **Step 5** (monitor) — observe and tune once all mints are closed.

Steps 2–4 can be done in any order. Step 1 should be first. Step 5 is ongoing.

## Testing

Before deploying, verify:

1. **Zero-sum check**: Add a temporary log line in the matching engine that prints total crowns entering vs leaving per market cycle. They must be equal.
2. **No orphaned mints**: `grep -rn 'Wealth +=' internal/` — after all steps, the only remaining `Wealth +=` lines should be in matched-trade execution (Step 1), merchant treasury sale (Step 2), Tier 2 treasury sale (Step 4), discovery bonuses (`simulation.go:404`), and initial world generation. All others are bugs.
3. **Agent survival**: Run a test simulation for ~100 sim-days and confirm agents don't mass-starve after fallback wage removal. If they do, consider adding a settlement food store before deploying Step 3.
4. **Market clearing**: Verify that trades actually occur — if all buyers are too poor, the economy freezes. May need to increase initial agent wealth or discovery bonus rate.

## Follow-Up Work (Not in This PR)

- **Merchant home-market buy**: Currently merchants buy at home by destroying crowns. Should route through the matching engine so a home seller receives the payment. Lower priority since crown destruction is deflationary (better than inflationary).
- **Credit/debt system**: If the hard-money economy proves too deflationary, consider allowing agents to borrow from settlement treasuries with interest — creating money via lending and destroying it on repayment. This would be a major new system.
- **Occupation rebalancing**: With real scarcity, occupation distribution and hex placement become more important. May need to add occupation-switching mechanics for agents stuck in unproductive roles.

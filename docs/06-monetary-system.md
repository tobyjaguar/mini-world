# Monetary System Analysis

## Status: Unclosed Money Supply (Inflationary)

Crossworlds has an **open money supply** — crowns are continuously created from nothing with insufficient destruction. This document catalogs all money sources and sinks, the current band-aid fixes, and the path to a proper closed economy.

## Money Sources (Crown Creation)

### 1. Market Sell Revenue — THE BIG ONE

**File**: `internal/engine/market.go:214`

```go
revenue := uint64(float64(sellQty) * entry.Price)
a.Wealth += revenue
```

When an agent sells goods, they receive `price * quantity` crowns **minted from thin air**. The buyer side (`market.go:229`) does deduct crowns, but the sell and buy sides are decoupled — they run as separate loops. Selling doesn't require a buyer. This is the primary money printer.

**Impact**: Every agent that produces and sells goods creates new money every market cycle (hourly). With ~50k agents, this prints tens of thousands of crowns per hour.

### 2. Fallback Wages

**File**: `internal/engine/production.go:35,46,64`

```go
a.Wealth += 1
```

When a hex is depleted or missing, resource-producing agents earn 1 crown/tick as a fallback wage. This is pure money creation — no goods produced, no exchange.

**Impact**: Moderate. Only affects agents on depleted hexes, but compounds over time. Every depleted-hex agent mints 1 crown per sim-minute.

### 3. Tier 2 Trade Action

**File**: `internal/engine/cognition.go:173`

```go
earned := uint64(a.Skills.Trade*5) + 2
a.Wealth += earned
```

When Haiku directs a Tier 2 agent to "trade", they earn `trade_skill * 5 + 2` crowns from nothing. No counterparty, no goods exchanged.

**Impact**: Small (only ~20-50 Tier 2 agents), but per-agent amounts grow as trade skill increases.

### 4. Discovery Bonuses

**File**: `internal/engine/simulation.go:404`

```go
bonus := uint64(50 + int(randFloat()*100))
sett.Treasury += bonus
```

Random discovery events (5%/week chance) mint 50-150 crowns into a settlement treasury.

**Impact**: Negligible compared to market sells.

### 5. Initial Generation

Agent starting wealth and settlement treasuries (`pop * 5`) at world creation. One-time, not ongoing.

## Money Sinks (Crown Destruction)

### Active Sinks

| Sink | Location | Rate | Scales With |
|------|----------|------|-------------|
| Settlement pop upkeep | `market.go` | `pop * 0.236 * 0.5` / day | Population only |
| Settlement treasury upkeep | `market.go` | `treasury * 0.236 * 0.01` / day | Treasury size |
| Agent wealth decay | `market.go` | `(wealth-20) * 0.236 * 0.01` / day | Agent wealth |
| Natural disasters | `simulation.go` | 20% of treasury, 2%/week chance | Treasury size |

### Transfers (NOT Sinks)

These move money but don't destroy it:
- **Taxes**: agent → settlement treasury
- **Theft/fines**: agent ↔ agent/treasury
- **Death inheritance**: dead agent → 50% treasury + 50% heir
- **Faction dues**: agent → faction treasury
- **Revolution seizure**: settlement treasury → faction treasury
- **Market buying**: agent wealth → (nowhere — crowns deducted but not received by seller in the same transaction)

Note: Market buying *does* destroy crowns on the buyer side, but market selling *creates* crowns on the seller side. These are decoupled, so net effect is inflationary.

## The Core Problem

The market is not double-entry. In a real economy:
```
Buyer pays 10 crowns → Seller receives 10 crowns (zero-sum transfer)
```

In Crossworlds:
```
Seller receives 10 crowns (created from nothing)
Buyer pays 10 crowns (destroyed)
```

These happen in separate loops to separate agents. The sell loop runs first and creates crowns. The buy loop runs second and destroys crowns. But there's no guarantee the amounts balance — and in practice, sells consistently exceed buys because agents produce more goods than others can afford to purchase.

## Current Band-Aid Fixes

Added in the treasury inflation fix (Feb 2026):

1. **Treasury-scaled upkeep**: `treasury * Agnosis * 0.01` (~0.24%/day) drained from settlement treasuries. Creates natural equilibrium — larger treasuries lose more.

2. **Agent wealth decay**: `(wealth - 20) * Agnosis * 0.01` (~0.24%/day) from agent wealth above 20 crowns. Protects the poorest while applying friction to the wealthy.

3. **Health formula rebalanced**: `DischargingPressure` now includes a treasury term so settlement health doesn't collapse to zero when treasuries inflate.

These create deflationary counter-pressure but don't fix the root cause.

## Proper Fix: Closed Market

The real fix requires restructuring `resolveLocalMarket()` in `market.go` to be a **matching engine**:

1. Collect all sell orders (agent, good, quantity, min price)
2. Collect all buy orders (agent, good, quantity, max price)
3. Match orders: seller receives crowns **from the buyer**, not from thin air
4. Unmatched sell orders → goods stay in inventory (no free money)
5. Price discovery from actual supply/demand intersection

This is a significant refactor because:
- The current sell and buy loops are separate and iterate all agents independently
- Goods are currently "sold into the void" (removed from inventory, crowns appear)
- Buy orders assume infinite supply at the market price
- The `Market.Entries` price/supply/demand tracking would need to drive actual order matching

### Implementation Sketch

```go
func (s *Simulation) resolveLocalMarket(sett, settAgents) {
    // Phase 1: Collect orders
    var sellOrders []Order  // agent wants to sell N of good at >= price
    var buyOrders []Order   // agent wants to buy N of good at <= price

    // Phase 2: Sort and match
    // For each good: sort sells ascending by price, buys descending
    // Match until prices cross

    // Phase 3: Execute matches
    // buyer.Wealth -= price; seller.Wealth += price (closed transfer)
    // buyer.Inventory[good]++; seller.Inventory[good]--

    // Phase 4: Update market prices from clearing price
}
```

### Also Fix

- `production.go` fallback wages — should these exist at all? If a hex is depleted, the agent should be unable to produce, not paid anyway.
- `cognition.go` Tier 2 trade — should execute through the market engine, not mint crowns directly.

## Monitoring

Track monetary health via:
- `GET /api/v1/economy` → `total_crowns`, `wealth_distribution`
- `GET /api/v1/stats/history` → watch `total_wealth` trend over time
- If total wealth stabilizes or slowly deflates, the band-aids are working
- If total wealth keeps climbing exponentially, the decay rates need increasing

## Philosophy Note

This is a **hard money system** — there's no credit, no debt, no central bank. Crowns are commodity-like tokens. In a credit system, money is created by lending and destroyed when debts are repaid, naturally balancing the supply. In a hard money system, the supply must be managed by controlling the mints (sources) and ensuring natural loss (sinks). The current system accidentally became a fiat money printer because market sells mint unbacked crowns. The proper fix is to close the mint — make all crown transfers zero-sum between agents.

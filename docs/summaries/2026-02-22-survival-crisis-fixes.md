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

## What to Monitor

1. **Avg survival** — should recover above 0.4 as food supply increases
2. **Deaths:births ratio** — should trend toward 2:1 or better
3. **Grain price** — should drop from 8.63 toward base price of 2 as supply increases
4. **Treasury levels** — should stabilize (wealth decay in, wages + upkeep out)
5. **Agent wealth** — should stop deflating as crowns recirculate
6. **Trade volume** — should increase as agents have crowns to spend
7. **Fisher mood** — should improve from -0.4 range as production and income rise

## Snapshot at Deploy

| Field | Value |
|-------|-------|
| Tick | 92,878 |
| Population | 53,787 |
| Settlements | 714 |
| Sim Time | Spring Day 65, 11:58 Year 1 |

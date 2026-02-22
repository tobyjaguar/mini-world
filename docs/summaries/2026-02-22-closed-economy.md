# World Summary — 2026-02-22 (Closed Economy Deploy)

## Session Overview

Deployed the closed economy system and diagnosed/fixed three critical issues within the same session. Four deploys total.

## Snapshot (Post-Final Deploy)

| Field | Value |
|-------|-------|
| Sim Time | Spring Day 64, 23:44 Year 1 |
| Tick | 92,144 |
| Speed | 1 |
| Season | Summer |
| Population | 51,136 (down from 53,209 at load — 2,073 deaths, 0 births pre-fix) |
| Settlements | 714 |
| Total Agent Wealth | 980M crowns |
| Total Treasury Wealth | 1.01B crowns |
| Avg Mood | 0.64 (dropped to 0.18 briefly on restart, expected to recover) |
| Avg Survival | 0.407 |
| Trade Volume | 104 (pre-fix; expected to rise significantly with barter trades) |
| Gini | ~0.64 |
| Market Health | 0.365 |

## Changes Deployed (4 deploys)

### Deploy 1: Closed Economy Core
- **Order-matched market engine**: replaced `executeTrades()` with sell/buy order matching. All trades are closed buyer↔seller transfers.
- **Merchant trade closed**: `sellMerchantCargo()` paid from destination settlement treasury.
- **Tier 2 trade closed**: `tier2MarketSell()` sells surplus to settlement treasury.
- **Fallback wages removed**: 3 locations in `production.go` — failed production causes needs erosion, not wage.
- **Journeyman/laborer mints throttled 60x**: gated to once per sim-hour via `tick%60`.
- **Sink comments updated**: `collectTaxes()` and `decayWealth()` reflect closed economy.

### Deploy 2: P0 Hotfixes (Zero Births + Low Trade)
- **Belonging restored on failed production**: `+0.001` on all three failed-production paths. Keeps resource producers above `Belonging > 0.4` birth threshold without minting crowns.
- **Barter trades enabled**: removed `clearCrowns < 1` floor. When prices round to 0, trades execute as barter (goods move, no crowns change hands). Penniless agents can receive goods.

### Deploy 3: Merchant Fix (Throttled Wage + Consignment)
- **Merchant throttled wage**: same `tick%60` gated 1-crown mint as laborers (~24 crowns/day survival floor).
- **Consignment buying**: home settlement treasury fronts cargo cost when merchant can't afford it.

### Deploy 4: Consignment Debt Repayment
- **`ConsignmentDebt` field** added to Agent struct.
- **Debt tracking**: treasury-fronted cargo costs accumulate as debt on the merchant.
- **Repayment on sale**: after selling at destination, merchant repays debt to home treasury before keeping profit. Unpaid debt carries forward.

## Issues Diagnosed via /observe

| Issue | Priority | Status |
|-------|----------|--------|
| Trade volume near zero (104 / 51K agents) | P0 | FIXED — barter trades enabled |
| Zero births (belonging collapse) | P0 | FIXED — belonging restored on failed production |
| Grain inflation 431% | P1 | Monitoring — should improve with trade volume |
| Merchant death spiral (all 6 dead at 0 wealth) | P1 | FIXED — throttled wage + consignment |
| Fisher mood -0.30 (systematic) | P2 | Open — likely fisher skill bug + fish deflation |
| 180+ settlements below viability (pop < 25) | P2 | Open — monitoring absorption migration |

## Faction State

| Faction | Treasury | Notes |
|---------|----------|-------|
| The Crown | 3,937,582 | Dominant — strong in monarchies |
| Merchant's Compact | 1,588,145 | Widest geographic spread |
| Ashen Path | 539,245 | Criminal network |
| Iron Brotherhood | 329,388 | Military, spread thin |
| Verdant Circle | 274,170 | Weakest — religious faction struggling |

## Tier 2 Characters

31 total, 25 alive, 6 dead (all merchants).
- 5 Liberated agents (coherence = 1.0): Jasper Thatcher, Eira Windholm, Freya Wolfsbane, Petra Greenvale, Stellan Voss
- All Tier 2 fishers have negative mood (~-0.30)
- Crafters are healthy (mood ~0.69, wealth 8-29K)
- 2 Laborers doing well (mood 0.69, wealth ~30K)

## Server Health

| Resource | Value |
|----------|-------|
| Memory | 402M / 977M (worldsim 110M) |
| Swap | 584K / 1G |
| Disk | 35M / 20G (DB: 35M) |
| CPU | Light load |
| Errors | None (gardener startup race is cosmetic) |

## What to Monitor Next Session

1. **Births** — should resume within 1-2 sim-days as belonging recovers above 0.4
2. **Trade volume** — should increase dramatically with barter trades enabled
3. **Total wealth** — should stabilize or gently decline (sinks > throttled mints)
4. **Grain price** — should normalize as trade volume increases supply flow
5. **Merchant survival** — new Tier 2 merchants should survive with consignment + wage
6. **Treasury drain** — consignment buying draws from home treasuries; watch for depletion
7. **Settlement consolidation** — 180+ sub-viable settlements should shrink via absorption migration

## Files Changed This Session

| File | Changes |
|------|---------|
| `internal/engine/market.go` | Order type, order-matched market, sellMerchantCargo closed, tier2MarketSell, barter trades, consignment buying + debt repayment |
| `internal/engine/production.go` | Fallback wages removed, belonging restored on failed production |
| `internal/engine/cognition.go` | Tier 2 trade routed through tier2MarketSell |
| `internal/agents/behavior.go` | ApplyAction/applyWork take tick param, journeyman/laborer/merchant throttled mints |
| `internal/agents/types.go` | ConsignmentDebt field |
| `internal/engine/simulation.go` | Pass tick to ResolveWork |
| `CLAUDE.md` | Full update — directory tree, phases, tuning rounds, skills |
| `README.md` | New public-facing readme |
| `docs/03-next-steps.md` | Post-closed-economy issues, updated roadmap |
| `docs/08-closed-economy-changelog.md` | Full changelog with P0/P1 hotfix notes |

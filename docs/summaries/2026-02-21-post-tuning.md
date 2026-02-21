# World Summary — 2026-02-21 (Post-Tuning Deploy)

## Snapshot

| Field | Value |
|-------|-------|
| Sim Time | Spring Day 31, 5:21 Year 1 |
| Tick | 43,521 |
| Speed | 1 |
| Season | Spring |
| Weather | Clear sky (+0.78 temp modifier) |

## Key Metrics

| Metric | Pre-Fix (Tick 14,005) | Post-Fix (Tick 43,521) | Change |
|--------|----------------------|----------------------|--------|
| Population | 27,277 | 32,752 | +5,475 (+20%) |
| Births | 0 | 5,652 | Population growing |
| Deaths | 1,653 | 1,874 | Death rate slowed dramatically |
| Avg Mood | -0.07 | +0.637 | Massive improvement |
| Settlements | 73 | 468 | Diaspora founding actively |
| Trade Volume | 2,927 | 28,115 | 10x increase |
| Total Wealth | 214M crowns | 364M crowns | +70% |
| Avg Market Health | 0.259 | 0.352 | Improving |

## Faction Health

| Faction | Treasury | Settlements | Notes |
|---------|----------|-------------|-------|
| The Crown | 1,161,679 | 84 | Richest — monarchy tax revenue flowing |
| Merchant's Compact | 436,676 | 201 | Widest reach |
| Ashen Path | 149,454 | 162 | Criminal faction spreading |
| Iron Brotherhood | 90,657 | 131 | Military faction steady |
| Verdant Circle | 76,286 | 156 | Religious faction modest |

Faction persistence fix confirmed working — treasuries accumulating and surviving restarts.
Crown relevance fix confirmed — present in 84 settlements (up from ~39), richest faction.

## Economic Health

- **Market health** improved from 0.259 to 0.352 but still below ideal (>0.5)
- **Still inflated**: Furs and iron ore hitting 4.2x ceiling in some settlements (Goldbury, Pinereach, Oldwick)
- **Still deflated**: Clothing at 0.24x floor across many settlements (Coppertown, Blackwood, Oakfall, Whiteford, Newgate)
- **Wealth distribution**: Poorest 50% hold 19.7% of wealth, richest 10% hold 22.7% — relatively flat
- **Trade volume**: 28,115 merchant trades completed — 10x increase from pre-fix

## Tuning Changes Applied

### 1. Fisher Mood Bug (Critical) — `internal/engine/production.go`
**Problem**: Fishers on depleted hexes earned a fallback wage but got zero needs replenishment. `DecayNeeds()` drained all needs every tick with no counterbalance, causing mood to bottom out at -0.63.
**Fix**: Added esteem (+0.01), safety (+0.005), belonging (+0.003) to all three fallback work paths (nil hex, depleted hex, second depletion check). Also added belonging (+0.003) to the successful production path.
**Result**: Fisher mood recovered along with the general population.

### 2. Raw Material Inflation (Critical) — `internal/engine/market.go`, `internal/engine/production.go`
**Problem**: Each crafter demanded all 5 raw materials (iron, timber, coal, furs, gems) every market cycle. With ~794 crafters in a large settlement, demand hit 794 per material against supply of 1, slamming prices to the 4.2x ceiling. Hunters also produced only 1 fur/tick regardless of skill.
**Fix**: Replaced blanket demand with `crafterRecipeDemand()` — each crafter picks the single recipe they're closest to completing and demands only its materials (max 2 goods). Four recipes: Tools (iron+timber), Weapons (iron+coal), Clothing (furs+tools), Luxuries (gems+tools). Also scaled hunter production with combat skill (`int(Combat * 2)`, min 1).
**Result**: Market health improved from 0.259 to 0.352. Trade volume 10x'd. Some inflation persists in furs/iron in older settlements.

### 3. Needs Decay Spiral (Medium) — `internal/agents/behavior.go`
**Problem**: `NeedsState.Priority()` returned only the single most urgent need. Safety decayed faster than belonging, so agents always worked (for safety) and never socialized. Belonging, esteem, and purpose decayed to 0 for most agents.
**Fix**: Three changes: (a) `applyWork` now gives belonging (+0.003) and purpose (+0.002) — working alongside others provides community. (b) `decideSafety` now has wealthy agents (>30 crowns) with low belonging (<0.4) socialize instead of defaulting to work. (c) `applySocialize` now gives safety (+0.003) and purpose (+0.002) — social bonds provide security.
**Result**: Average mood jumped from -0.07 to +0.64. This was the single highest-impact fix.

### 4. Faction Treasury Persistence (Medium) — `internal/persistence/db.go`, `internal/engine/factions.go`, `internal/engine/simulation.go`, `cmd/worldsim/main.go`
**Problem**: No `factions` table in SQLite. On every restart, `initFactions()` created fresh factions with treasury=0, losing all accumulated dues.
**Fix**: Added `factions` table (id, name, kind, leader_id, treasury, preferences, influence_json, relations_json). Added `SaveFactions()`/`LoadFactions()`/`HasFactions()`. Wired into `SaveWorldState()` (daily auto-save). Exported `InitFactions()`, added `SetFactions()`. Main.go loads from DB if available, falls back to `InitFactions()` for old DBs.
**Result**: Crown treasury at 1.16M crowns, Merchant's Compact at 436K. Treasuries survive restarts.

### 5. Crown Faction Irrelevant (Low) — `internal/engine/factions.go`
**Problem**: Crown only recruited nobles/leaders with Devotionalist class or wealthy Ritualists — a tiny pool. Merchant's Compact got all merchants (a common occupation) and dominated 66/73 settlements.
**Fix**: `factionForAgent` now accepts governance type. Common folk in monarchies with wealth>50 or coherence>0.3 join Crown. Common folk in merchant republics with trade>0.1 or wealth>80 join Merchant's Compact. `updateFactionInfluence` adds governance alignment bonus: Crown +15 in monarchies, Merchant's Compact +15 in merchant republics, Verdant Circle +10 in councils.
**Result**: Crown present in 84 settlements (up from ~39), richest faction. Concentrated in monarchies as intended.

## Areas to Watch

1. **Settlement explosion**: 73 → 468 settlements in ~20 sim-days. Overmass diaspora threshold may be too aggressive, causing fragmentation into many tiny settlements. New settlements may lack critical mass for healthy markets.

2. **Persistent raw material inflation**: Furs and iron ore still at 4.2x ceiling in older/larger settlements. The single-recipe demand fix reduced pressure but didn't eliminate it. May need: increased hex resource regeneration, more hunter/miner occupations in spawner, or higher base supply floors.

3. **Clothing oversupply**: Clothing stuck at 0.24x floor everywhere. Crafters are producing more clothing than demand. May need: agents to demand clothing (cold weather?), clothing decay, or fewer crafters choosing the furs→clothing recipe.

4. **Birth/death ratio**: 5,652 births vs 1,874 deaths is net +3,778 — healthy growth but may accelerate unsustainably with 468 settlements all spawning. Monitor for population explosion and memory pressure on the 1GB server.

5. **Market health still low**: 0.352 average — more than half of goods are significantly mispriced. The economy is functional but not balanced. Price discovery may need more time, or the supply/demand mechanics need further tuning.

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

## What Worked

1. **Needs spiral fix** — mood jumped from -0.07 to +0.64. Agents are socializing, getting belonging and purpose from work and social interactions.
2. **Fisher mood fix** — fallback work paths now replenish needs. No more universal fisher misery.
3. **Faction persistence** — treasuries accumulating. The Crown is the richest faction at 1.16M crowns.
4. **Crown relevance** — governance alignment bonuses working. Crown in 84 settlements, concentrated in monarchies.
5. **Hunter scaling** — combat-scaled fur production contributing to supply, though furs still inflated in some markets.

## Areas to Watch

1. **Settlement explosion**: 73 → 468 settlements in ~20 sim-days. Overmass diaspora threshold may be too aggressive, causing fragmentation into many tiny settlements. New settlements may lack critical mass for healthy markets.

2. **Persistent raw material inflation**: Furs and iron ore still at 4.2x ceiling in older/larger settlements. The single-recipe demand fix reduced pressure but didn't eliminate it. May need: increased hex resource regeneration, more hunter/miner occupations in spawner, or higher base supply floors.

3. **Clothing oversupply**: Clothing stuck at 0.24x floor everywhere. Crafters are producing more clothing than demand. May need: agents to demand clothing (cold weather?), clothing decay, or fewer crafters choosing the furs→clothing recipe.

4. **Birth/death ratio**: 5,652 births vs 1,874 deaths is net +3,778 — healthy growth but may accelerate unsustainably with 468 settlements all spawning. Monitor for population explosion and memory pressure on the 1GB server.

5. **Market health still low**: 0.352 average — more than half of goods are significantly mispriced. The economy is functional but not balanced. Price discovery may need more time, or the supply/demand mechanics need further tuning.

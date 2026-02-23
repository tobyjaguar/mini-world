# 12 — Observation Report: Tick 165,844

**Date:** 2026-02-22
**Sim Time:** Summer Day 26, Year 1

## 1. World State Summary

Crossworlds is in a **slow-motion survival crisis**. Population has grown from 67,634 to 72,699 over the last 10 snapshots (cumulative births outpace per-snapshot deaths), but the death:birth ratio swings wildly between 2:1 and 18:1. Average mood is 0.162 — dangerously low. The core problem is a **producer misery gap**: Hunters (-0.46 mood), Farmers (-0.40), and Fishers (-0.23) — who comprise 50% of the population — have deeply negative satisfaction, while Crafters (0.62), Laborers (0.61), and Scholars (0.70) are thriving. The market is nearly non-functional — supply sits at floor for almost all goods in most settlements, meaning producers aren't generating enough surplus to sell. Trade volume collapsed from 5,707 to 278 in the last two snapshots.

## 2. Key Metrics

| Metric | Value | Trend |
|--------|-------|-------|
| Population | 72,699 | ↑ slowly (+7.5% over 10 snapshots) |
| Deaths (last snapshot) | 10,164 | → stable (~10k/snapshot) |
| Births (last snapshot) | 564 | ↓↓ collapsed (was 5,024 two snapshots ago) |
| Death:Birth ratio | 18:1 | ↑↑ critical (was 2:1 five snapshots ago) |
| Avg Mood | 0.162 | → flat (was 0.119 earlier, slight improvement) |
| Avg Satisfaction | 0.116 | → flat |
| Avg Survival | 0.385 | → flat, just above 0.3 birth threshold |
| Avg Alignment | 0.361 | → stable (coherence model working) |
| Total Crowns | 2.007B | → stable |
| Treasury Share | 41% (822M) | ✓ near Φ⁻¹ target (38.2%) |
| Agent Wealth | 59% (1.186B) | ✓ near Matter target (61.8%) |
| Gini | 0.606 | → stable |
| Trade Volume | 278 | ↓↓ collapsed (was 5,707) |
| Market Health | 0.982 | ✓ good |
| Settlements | 714 | → unchanged |
| Settlements pop < 25 | 234 (33%) | ⚠ fragmented |
| Settlements pop < 50 | 399 (56%) | ⚠ majority tiny |

## 3. Health Assessment

### What's Working

- **Treasury targeting**: 41% treasury share is close to the Φ⁻¹ target of 38.2%. The dynamic welfare system is self-regulating well.
- **Wealth stability**: Total crowns stable at ~1.19B agent-side. No runaway inflation or deflation.
- **Gini stable**: 0.606, down from the 0.673 spike. Progressive decay is working.
- **Coherence model**: Avg alignment 0.361, and Tier 2 liberated agents show mood 0.88 despite low satisfaction — the dual-register model is doing its job.
- **Market health**: 0.982 — prices aren't broken. The market engine itself is sound.

### Critical Concerns

#### 3a. Producer Satisfaction Crisis (ROOT CAUSE)

50% of the population (Hunters, Farmers, Fishers) has deeply negative mood:

| Occupation | Count | Avg Mood | % of Pop |
|-----------|-------|----------|----------|
| Hunter | 1,653 | -0.462 | 2.3% |
| Farmer | 10,914 | -0.400 | 15.1% |
| Fisher | 23,656 | -0.234 | 32.7% |
| Laborer | 15,827 | +0.615 | 21.9% |
| Crafter | 15,571 | +0.620 | 21.5% |
| Scholar | 2,121 | +0.696 | 2.9% |
| Soldier | 2,444 | +0.563 | 3.4% |

Fishers are the largest occupation at 33% — their misery dominates the world average. Global satisfaction is 0.116 with 50.1% of agents below 0.1.

#### 3b. Market Supply Starvation

Looking at Oldwick (pop 152): Fish supply=1, demand=152. Grain supply=1, demand=152. The supply floor (pop/100) is too low. Producers aren't generating surplus above their personal threshold (3 for food producers). The market is an empty shell — demand screams but shelves are bare.

#### 3c. Birth Rate Collapse

Births crashed from 5,024 → 564 in two snapshots. The birth threshold (Belonging > 0.3) is barely met. With survival at 0.385 and satisfaction at 0.116, agents are too miserable to reproduce. The death rate is constant (~10k/snapshot), so any birth dip creates a death spiral risk.

#### 3d. Settlement Fragmentation

399 of 714 settlements (56%) have population under 50. 234 (33%) under 25. These tiny settlements can't generate enough economic activity to sustain themselves. The viability check should be absorbing them, but they persist.

## 4. Suggested Improvements (ranked by impact)

### P0: Fix Producer Satisfaction (highest impact)

The fundamental issue is that resource producers (Farmers, Fishers, Hunters) work all day but their **satisfaction stays deeply negative**. They produce food, eat some, but their needs still aren't met. This needs investigation in the production/behavior code:

- **Check `applyWork` needs boost for resource producers.** Tuning Round 7 fixed Purpose for resource producers, but Satisfaction is driven by all needs (Survival, Safety, Belonging, Esteem, Purpose). If resource producers aren't getting Safety/Esteem/Belonging from work, their satisfaction will stay low even with Purpose fixed.
- **Check if food production rate is sufficient.** If a Fisher produces 1 fish/tick but eats 1 fish/tick, there's no surplus for the market and no needs improvement. `productionAmount()` using `Skills.Farming` for Fishers (known bug in CLAUDE.md) may mean Fishers have low production output.
- **Check the surplus threshold.** Fishers keep 3 fish before selling. If they only produce 1-2 fish per production cycle, they never reach surplus and never earn money from the market.

### P1: Investigate Birth Rate Volatility

Births swing from 564 to 5,024 between snapshots. This suggests a threshold effect — small changes in Belonging push thousands of agents above/below the 0.3 threshold simultaneously. Consider:
- **Stochastic birth probability** instead of a hard threshold. At Belonging 0.2: 10% chance. At 0.3: 50%. At 0.5: 90%. This smooths the cliff.
- **Check what caused the crash.** The drop from 5,024 → 576 → 1,127 → 1,706 → 564 is erratic. Something is periodically tanking Belonging for a large population segment.

### P2: Address Settlement Fragmentation

56% of settlements have <50 population. The viability check (pop <15 for 4 weeks → disable refugee spawning) isn't aggressive enough:
- Raise the viability threshold from 15 to 25.
- Consider merging settlements within 3 hexes if both are below 50 pop.
- Ensure enhanced migration (mood threshold 0.0 for pop <25) is actually triggering.

### P3: Fix Fisher Production Skill Bug

CLAUDE.md notes: "`productionAmount()` uses `Skills.Farming` for Fishers instead of a dedicated fishing skill." If Fishers all have low Farming skill, their production is throttled. With 23,656 Fishers (33% of pop), this single bug could explain the entire food supply crisis.

## 5. Things to Monitor

- **Birth/death ratio per snapshot** — the 18:1 swings are the most dangerous signal. If births stay below 1,000 for 3+ consecutive snapshots while deaths hold at 10k, population will start declining.
- **Trade volume** — the collapse from 5,707 to 278 suggests the market is dying. If supply stays at floor, trade can't happen.
- **Fisher satisfaction specifically** — after any fix, check if Fisher mood improves. They're the swing vote for the whole world.
- **Belonging distribution** — this is the birth-gating need. What % of agents are above 0.3? A small shift here drives thousands of births.
- **Tiny settlement count** — 234 settlements with <25 pop are economic dead weight. Track whether this number is growing or shrinking.

## 6. Stats History (raw data)

```
Tick      Pop     Births  Deaths  D:B     Mood   Surv   Trade  Gini   TotalWealth
152640   67634    2170    9757    4.5x  0.119  0.393   1958  0.612    1186660465
154080   68160    2732    9799    3.6x  0.120  0.389   3090  0.608    1192879652
155520   68698    3310    9864    3.0x  0.121  0.394   4041  0.609    1186099307
156960   69237    3890    9929    2.6x  0.122  0.398   4658  0.610    1189210493
158400   69775    4458    9964    2.2x  0.118  0.405   5198  0.608    1180990740
159840   70316    5024    9993    2.0x  0.123  0.410   5707  0.609    1190211937
161280   70865     576   10021   17.4x  0.168  0.414     63  0.607    1188344817
162720   71656    1127   10045    8.9x  0.161  0.390    840  0.608    1188869616
164160   72188    1706   10100    5.9x  0.161  0.381   1772  0.607    1190894742
165600   72699     564   10164   18.0x  0.162  0.385    278  0.606    1185692046
```

Note: The birth/trade collapse at tick 161,280 correlates with the deployment of the dual-register wellbeing model. The wellbeing model changed how mood is computed (blending satisfaction + alignment), which may have shifted Belonging dynamics. The stats_history shows `avg_satisfaction` and `avg_alignment` are 0 for ticks before 161,280 — these fields didn't exist before that deploy.

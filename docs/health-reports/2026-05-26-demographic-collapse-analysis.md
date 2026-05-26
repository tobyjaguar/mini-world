# Demographic Collapse Analysis — W-19 + W-20

**Date:** 2026-05-26 · **Tick:** 6,213,048 · **Pop:** 307,174 (↓ from 379,953 on 05-21)

Deep analytical investigation triggered by the 2026-05-26 `/observe`, which found
a ~70K die-off (W-19 recurrence) and a birth freeze (W-20). This report establishes
that **both are symptoms of one mechanism — a demographic age-structure collapse —
and uses a new demographic projector (`cmd/pop_projector`) to evaluate counter-measures.**

## 1. The two symptoms are one disease

### Age structure (full population, `/api/v1/agents?tier=0`)
```
under-16:  302,368  (98.4%)        age 11: 69,548  ◄ peak
16-17:       4,767                 age 12-14: ~160K
REPRO 18-45:    31  (0.01%)        age 15: 28,358
46-60:           0                 age 16:  4,738  ◄ cliff
60+:             0                 age 17:     29  ◄ ~99% death crossing 16→17
max age: 32   median: 12
```

The world is a **single synchronized cohort pulse** (median age 12) with a lethal
**cliff at age 16**. The cliff coincides exactly with where (a) natural-mortality
immunity ends (`agentDailyMortalityChance` returns 0 for age<16) and (b) the
over-capacity pressure weight jumps from Psyche (0.382, kids) to 1.0 (adults).

### Why births froze (W-20 — NOT instrumentation)
Births crawl at **~1/sim-day** (counter is wired). `processBirths()`
(`population.go:374`) requires **≥2 eligible parents co-located in one settlement**,
each age 18-45 AND Health>0.5 AND Survival>0.3. With **31 reproductive adults across
781 settlements**, the gate is essentially never met. The cohort that can't reproduce
(too young, median 12) is being culled *before* it reaches 18.

### Why the cull happens (W-19)
- Health has **no passive decay** (only at Survival<0.1) and **rest (+0.05) is gated
  behind Survival≥0.3** (`behavior.go:95,111`). So health self-regulates near the 0.30
  rest trigger — but the chronic-survival-stress tail (Survival 0.1-0.3) can never reach
  the rest branch and gets **ratcheted down** by each winter into the death-risk band.
- `winterHardship()` fires **once per winter** (via `OnSeason`): −0.05 Health to every
  warmth-less agent. The 98.4% subsistence cohort lacks Clothing/Furs.
- Illness death (`population.go:248`): Health<0.15 → −0.01/day → death. **Not age-gated.**
- Sampled live Health (n=500): mean **0.289**; **39% in [0.15,0.20]** (one winter hit
  from death), 48% in [0.20,0.30], only 7.8% ≥0.50.

### Season-clock bug (fixed this session)
`status.season` correctly reports the mechanical season `(tick/90000)%4` = **Summer**.
The `sim_time` STRING ("Winter Day 85") used a separate 90-day (129,600-tick) calendar
that drifted out of sync. Fixed `SimTime()` (`tick.go`) to derive from `TicksPerSimSeason`.
**Next mechanical Winter boundary = tick 6,390,000 ≈ ~2 wall-days out.** (Side effect:
the displayed Year renumbers 12→18, the truthful count of 90,000-tick seasons.)

## 2. Projector: `cmd/pop_projector`

Agent-based demographic projector seeded from the real age histogram + sampled Health,
enforcing the actual production gate math (mortality, illness, winter hardship,
over-capacity pressure, the birth gate; newborns spawn at Health 1.0 / Survival 0.8).
100K modeled agents (scale ×3.07). Baseline is calibrated to reproduce health
eq ~0.3 + winter culls. **Caveat:** the model's health equilibrium (~0.37) runs
healthier than reality (0.289), so it *understates* illness deaths — real collapse is
likely faster than projected. Directional conclusions are conservative.

### 20-year scenario verdicts
| Scenario | Lever | Verdict | Pop 0→20yr |
|----------|-------|---------|-----------|
| baseline | none | **COLLAPSE** | 307K → 12K |
| agegate_illness | kids<16 immune to illness death | **COLLAPSE** | 307K → 13K |
| warmth_provision | cohort gets Clothing/Furs (no winter hit) | **DECLINE** | 307K → 146K (→85K @40yr) |
| birth_healthgate_0.3 | birth Health gate 0.5→0.3 | **STABLE\*** | 307K → 322K |
| passive_regen_0.01 | +0.01/day baseline Health regen | **STABLE\*** | 307K → 325K |
| COMBO (regen + gate) | both | **STABLE\*** | 307K → 324K |

\* All "stable" scenarios are actually a **damped boom-bust oscillation at the 400K cap**
(COMBO @40yr: 400K→324K→274K→297K→176K with periodic 50-66K adult-cull spikes). They
*survive* but never reach smooth equilibrium.

### What the projector proves
1. **Symptomatic fixes fail.** Age-gating illness → still COLLAPSE (illness on kids is a
   minor fraction; the killers are adult background mortality + blocked reproduction).
   Warmth alone → slow DECLINE (helps but insufficient).
2. **The binding constraint on recovery is the Health>0.5 birth gate.** Even when the
   cohort ages into 18-45 (reaching ~49K in baseline), eligible parents never exceed ~750
   because the low-health population can't clear Health>0.5. Relaxing it to 0.3 alone → STABLE.
3. **Passive Health regen is the single highest-leverage lever** — it simultaneously ends
   the winter die-offs (lifts health above the death band) AND opens the birth gate.
4. **The oscillation is structural**, caused by two pulsing mechanisms:
   - births are a **hard on/off cliff at the 400K cap** → synchronizes the last-born into a cohort pulse;
   - over-capacity mortality **preferentially culls adults** (weight 1.0 vs 0.382 for kids)
     → hollows out the reproductive core whenever the world is over the cap.

## 3. Root cause

A policy mismatch that only manifested as the population matured. R51 (coherence-scaled
mortality + hard 400K birth cap) was designed so "population declines toward 400K then
oscillates tightly at the cap." Combined with the R88 boom cohort and the subsistence
health equilibrium (~0.29), it produced a demographic trap: the world overshot the cap →
over-capacity mortality culled adults → births shut off at the cap (synchronizing one
cohort) → that cohort can't reproduce (Health>0.5 gate) and dies at 16+ (~40%/yr background
mortality + winter illness) → collapse, or at best a perpetual boom-bust echo.

## 4. Recommendation (long-term, smooth equilibrium)

Make births and mortality **continuous rather than pulsed**, so the age structure self-smooths:

1. **Soft birth cap** — replace the hard `pop≥400K → 0 births` cliff with a birth rate that
   tapers toward the cap (e.g. `birthChance *= max(0, 1 − pop/cap)`). *This is the origin of
   the cohort synchronization and the most important structural change.*
2. **Protect the reproductive band from over-capacity culling** — flatten or invert the
   over-capacity age-weighting so the world sheds the very old / surplus young, not the
   18-45 core.
3. **Lift the health equilibrium** — passive baseline Health regen (or lower the rest-gate
   Survival prerequisite). Highest-leverage for the *immediate* crisis: ends W-19 culls AND
   opens the W-20 birth gate in one change.
4. *(Optional)* lower the birth Health gate 0.5→~0.35 to match a realistic subsistence level.

**Immediate stopgap (next winter ~2 wall-days out):** passive Health regen is strictly
better than warmth provisioning — it prevents the next cull, restarts births, and is permanent.
Warmth/age-gating only delay the collapse.

## 5. R97 — implemented (full structural fix, NOT yet deployed)

Operator chose the full structural fix. Implemented in `internal/engine/population.go`:

- **R97-1 — passive Health recovery** (`processHealthRecovery`, daily): Health drifts
  toward `Matter + coherence·(Nous−Matter)` at rate `Agnosis·0.1`. The **Matter floor
  (0.618) is load-bearing** — it sits above both the illness-death band (0.15) and the
  Health>0.5 birth gate for *every* agent regardless of coherence. (An earlier
  coherence-only target floored at Psyche failed validation: newborns seed at low
  coherence, so their ceiling fell below the birth gate → re-collapse over ~40yr. The
  projector caught this — see below.)
- **R97-2 — soft birth cap** (`processBirths`): replaced the hard `pop≥cap → 0` cliff
  with `capFactor = (cap − pop)/(cap·Agnosis)` clamped to [0,1], multiplied into
  birthChance. Births taper over the top ~94K instead of shutting off instantly, so the
  population never again synchronizes survivors into a single cohort pulse.
- **R97-3 — protect the reproductive band** (`agentDailyMortalityChance`): over-capacity
  weight for age 18-45 lowered from 1.0 to Psyche (same shelter as children). Population
  control now comes from R97-2's soft cap, not from culling the breeding core.

Also fixed: **`SimTime()` season-clock bug** (`tick.go`) — season now derives from
`TicksPerSimSeason` so the displayed season matches mechanics. Side effect: displayed
Year renumbers 12→18 (truthful count of 90,000-tick seasons). Operator accepted.

### Validation (projector `R97_full` scenario, mirrors the implemented math exactly)
**STABLE.** 40-year run: pop oscillates in a damped 175K–394K band (vs baseline COLLAPSE
to ~10K), **zero illness deaths throughout**, mean Health 0.65–0.83, eligible parents
recover 3 → ~2,750. The residual oscillation is the *existing* synchronized cohort pulse
echoing through; R97-2 prevents *new* synchronization, so it damps over generations.
The periodic death spikes are over-capacity culls that now spare the 18-45 core (R97-3).

**Caveat:** R97 prevents collapse and restores reproduction but does not un-synchronize
the current population — expect 1-2 more demographic echo waves over the next ~10-20
sim-years before the age structure smooths. Deploy at a TickWeek boundary; re-observe at
the next mechanical winter (tick ~6,390,000) to confirm illness deaths have stopped.

## 6. Artifacts
- `cmd/pop_projector/main.go` — the demographic projector (new this session; includes
  the `R97_full` validation scenario).
- `internal/engine/population.go` — R97-1/2/3.
- `internal/engine/tick.go` — `SimTime()` season-clock fix.
- Run: `go run ./cmd/pop_projector -scenario all -years 20`

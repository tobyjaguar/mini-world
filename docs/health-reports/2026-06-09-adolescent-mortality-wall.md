# Health Report — 2026-06-09 — The Adolescent Mortality Wall (W-22)

**Tick 7,339,822 — Summer Day 35, Year 21** (~3.1 sim-years / 14 wall-days after R97 deploy at tick 6,219,979)

## Verdict

R97 worked as designed — illness die-offs are gone (2+ winters weathered with no
mortality spike; population decline is smooth, not discrete) and births have
recovered 1/day → ~28/day. But the world is now in a **second demographic trap**
that R97 did not (and could not) address: **background scatter mortality begins
at age 16, two years before the age-18 reproduction gate.** Agents die before
they can reproduce. Population: 307,122 → **270,916** (−36K, ~−80/sim-day net,
steady).

## The numbers

| Metric | 2026-05-26 (deploy) | 2026-06-09 | Trend |
|---|---|---|---|
| Population | 307,122 | 270,916 | ↓ −80/sim-day, steady |
| Births (cumulative) | 436,111 | 454,708 | ↑ ~28/sim-day and rising (was ~1/day) |
| Deaths/sim-day | — | ~108 | matches 16+ pool × mortality curve |
| Age 18-29 | ~31 (18-45) | **1,038** | the missing generation |
| Age 16-17 | 4,767 | 44,821 | bulge arriving at the wall |
| Under 16 | 302,368 (98.4%) | 225,047 (83.1%) | median age 12 → 14 |
| Max age | 32 | 59 | survivors are high-coherence outliers |
| Avg coherence | 0.7075 | **0.5328** | ↓↓ −0.17 in 3 sim-years |
| Liberated (c≥0.7) | 164,695 (54%) | 49,661 (18.3%) | ↓↓ |
| Avg mood | 0.7123 | 0.5309 | downstream of coherence |
| Avg alignment | 0.6126 | 0.3233 | downstream of coherence |
| Satisfaction | 0.6970 | 0.6977 | flat — material life is fine |
| Eligible parents (18-45) | 31 | ~1,159 | better but still ~0.4% of pop |

## Root cause 1: the 16/18 gate mismatch

`agentDailyMortalityChance` (population.go:304) applies background scatter
mortality from **age ≥ 16**: `Agnosis⁵ + Agnosis⁴·(1−c)` ≈ **0.22%/day at
c≈0.53**. Verified against live data: 46,003 agents are 16+, observed deaths
~108/day → implied 0.235%/day. Exact match — this is the death path. (Not
illness: R97 health floor is holding. Not a winter spike: decline is smooth
through 2 winters.)

`processBirths` requires age **18-45**. At 0.22%/day, survival from 16 to 18 is
e^(−0.0022×720) ≈ **20%**, and expected reproductive tenure after 18 is ~1.3
sim-years. ~50-60K agents crossed 18 since deploy; ~55K cumulative deaths over
the same window; 1,159 remain in 18-45. The boom cohort is being annihilated in
the two-year no-man's-land between mortality onset and reproductive
eligibility.

This mortality curve was tuned (R51) for a world with avg age 32 and a standing
adult population — it was never exercised against a single child cohort aging
through the 16 gate with no adults behind them.

## Root cause 2: the sage-death coherence ripple spiral

`processNaturalDeaths` (population.go:200-211): every **liberation death**
applies `−Agnosis·0.05 × witnessCoherence` (≈ −0.012·c) to **every alive agent
in the settlement** — ungated. Ordinary deaths give a coherence *gain* but only
to witnesses with relationship sentiment ≥ Matter (rare). The dying 16+ cohort
is liberated-heavy (the practice generation — scholars at c=1.0), so dozens of
sage deaths/day pump settlement-wide coherence drain. At ~1 sage death per
settlement per ~26 days × −0.007 per event, the implied drift fully accounts
for the observed −0.17 world coherence drop.

**The feedback loop:** sage dies → settlement coherence drops → scatter (1−c)
rises → background mortality rises → more sage deaths. Mortality at c=0.71 was
0.16%/day; at 0.53 it is 0.22%/day — the spiral has already raised the death
rate ~35%. Mood (0.53) and alignment (0.32) collapses are downstream of this
(EffectiveMood blends alignment by c²; not separate pathologies).

The ripple was designed (R53 #229) for *rare* sage deaths. At mass-death scale
it is a world-coherence drain pump.

## Projection

The 225K under-16 cohort (median 14) is now crossing 16 continuously. The 16+
pool will grow from 46K toward 100K+ within ~2 sim-years (~10 wall-days),
pushing deaths toward 250+/day while births (bounded by the ~1K-strong 18-45
pool) stay double-digit. Unchecked: collapse continues at an accelerating rate.
Wall-clock pace: ~3 sim-years per 2 wall-weeks. **R98 should land within days,
not weeks.**

Next mechanical winter (season 83, tick 7,470,000, ~2 wall-days out) is
expected to be a non-event — R97 health floor holding.

## R98 candidates (ranked)

1. **Age-protect background mortality** — ramp the scatter term in over ages
   16→30 (e.g. multiply by the same squared-sigmoid trick the age term already
   uses to "protect younger adults"). Directly fixes die-before-reproducing.
   The background term currently has zero age shaping.
2. **Close the gate mismatch** — reproduction eligibility 16-45 (or mortality
   onset 18). One line, thematically defensible for the setting.
3. **Gate or cap the sage-death ripple** — relationship-gate it like ordinary
   deaths, or cap total ripple per settlement per week. Stops the
   coherence→mortality spiral; coherence inflows (drift to Matter, practice)
   can then recover the average.
4. **Fix the projector first** — pop_projector predicted R97_full → STABLE
   (oscillation 175-394K). It missed this. Almost certainly it does not model
   the age-16 background scatter mortality (or models it age-18+ / at stale
   coherence). Add `agentDailyMortalityChance` faithfully, re-seed from the
   live age×coherence histogram, and validate R98 scenarios before deploy.
   (Same lesson as R89: the projector must enforce actual gate math.)

## Everything else is healthy

- **Economy:** crowns conserved (1.4038B), wealth 840M stable, gini 0.620
  (improving), bottom-50% share 8.8% (up from 8.08%), market health 0.932,
  trade routes 686 (up from 648), producer work rate 49.5% (up from 45.5%).
  Wealth-per-capita rising as population falls.
- **W-16 governance:** Council 724/786 = 92.1% — stable, below 95% escalation.
- **Factions:** Crown now largest (75,980), VC 70,034 (down from 108.6K —
  defection working), all treasuries positive. Unaffiliated adults 19 → 1,153.
- **SimTime fix validated:** `sim_time` "Summer Day 35, Year 21" agrees with
  mechanical season (tick/90000 % 4 = Summer).
- **LLM:** 0 cache_read_tokens post-reset — matches the known benign no-op
  (Haiku 2048-token min cacheable); call volume low (6 calls/13min period).
- **W-21 /snapshot:** no new findings; nginx proxy_read_timeout bump still
  pending (cosmetic).

---

## Addendum (same day): code review of the R98 candidates — calibration, not gates

Follow-up review question: is the die-off a *correctly implemented system with a
second-order consequence*, or a *mis-applied constraint that now needs gating*?

**Answer: neither gate is the right fix — the background mortality constant is
mis-calibrated, by the design's own internal logic.**

1. **Internal inconsistency in R51.** The background floor (Agnosis⁵ ≈
   0.073%/day) gives a mean lifespan of **3.8 sim-years past age 16 even at
   perfect coherence** — death around age 20 for everyone. The same function's
   age-sigmoid term documents rates at ages 50/60/70 ("at age 70: ~0.98%/day"),
   implying a world where reaching those ages is normal. Under the background
   term's own magnitude, survival 16→50 is ~e⁻⁹ ≈ 0.01% — the age curve is
   unreachable dead code. The two curves in one function disagree about what
   world they live in by ~2 Φ-powers.
2. **Vestigial job.** R51's stated purpose was population reduction (~1,505
   deaths/day to push 494K → 400K). That job is now done by R97-2's soft birth
   cap — the correct mechanism (taper births, don't kill adults). The hot
   background rate is a leftover population-control device.
3. **The design anticipated this fix.** R51's own tuning note:
   *"if too aggressive, reduce background by one Φ power (Agnosis⁵ base)."*

**Projector verification** (flags `-kidcoh`, `-bgpow`, `-onset` added to
`cmd/pop_projector`; defaults reproduce the 2026-05-26 runs exactly):

| Scenario (R97_full, kidcoh=0.53) | yr-20 pop | age structure | verdict |
|---|---|---|---|
| prod (Agnosis⁴, onset 16) | rebound w/ echoes | 70-99% kids, maxAge ~25, pool whipsaw | echo machine |
| onset 16→18, rate kept | 388K (at cap) | **99% kids by yr 12, pool 242↔8.4K, maxAge 31** | band-aid |
| bgpow 5 (one Φ-power down) | 376K | 87% kids, maxAge 36, pool 52K | viable |
| **bgpow 6 (Agnosis⁶/Agnosis⁷)** | **396K** | **adults 40-50%, pool ~200K, births steady, maxAge 41+, age curve expresses** | **the fix** |
| bgpow 6 @ pessimistic kidcoh 0.40 | 396K | pool 50K+, stable | robust |

Calibration check: at kidcoh=0.53 the projector reproduces observed deaths
(~119/day projected vs ~108 live at year 3). Births lag 4× vs projection due to
settlement granularity (200×500 modeled vs 786×~345 real — the ≥2 co-located
eligible parents gate passes far more easily in the model).

**Sage-ripple verdict:** faithfully implemented per R88 (−Agnosis·0.05 × c,
settlement-wide, un-gated) — a true second-order casualty of mortality
miscalibration. At ~30 sage deaths/day it drains world coherence with no
counter-flow: under-16s (83% of pop) have zero coherence inflow (practice gated
age ≥ 16, baseline drift gated age > 20, witness gains relationship-gated).
Recalibrating mortality cuts sage deaths ~15×, returning the ripple to its
designed rare-event regime. Watch; don't gate it yet. The inflow asymmetry is a
design-review item for later, not load-bearing.

**Projector verdict (premise corrected):** it models the background mortality
faithfully — the original report's claim that it omitted the term was wrong.
Its actual gaps: coherence is static and was seeded optimistically (kids 0.85
vs live 0.53 and falling; no ripple dynamics), and coarse settlement
granularity inflates birth co-location. The new `-kidcoh` flag covers the
first; consider 786-settlement seeding for the second.

**R98 recommendation:** reduce background scatter to `Agnosis⁶` base /
`Agnosis⁷` floor (two clicks of R51's documented knob), keep onset at 16,
change neither the 18-45 birth gate nor the ripple. Re-validate at
kidcoh 0.40-0.45 before deploy.

---

## Addendum 2 (same day): projector v2 + rate corrections

**Rate corrections.** Two unit errors in the original report, caught while
building the hindcast fixture: stats_history rows are exactly 1440 ticks =
1 sim-day apart, so live rates are **deaths ~155/day, births ~39/day**
(7-day trailing; the report said ~108/28 by misconverting ticks/day), and the
post-R97 window is **778 sim-days** on the 1440-tick agent-age calendar (the
"3.1 sim-years" used the separate 360,000-tick season calendar). The implied
16+ death rate is therefore ~0.34%/day — higher than the curve at the world
mean coherence 0.53, implying the 16+ cohort's own coherence is well below the
world mean (the under-16 mass carries the legacy high coherence). Structural
conclusions unchanged; the wall is somewhat *worse* than first quoted.

**Projector v2** (`cmd/pop_projector`, committed this session): dynamic
coherence (sage-death ripple, baseline drift, practice with samatha+insight,
newborn inheritance capped at Matter), live full-scale seeding from
`/api/v1/agents?tier=0` + `/api/v1/settlements` dumps (real settlement-size
distribution — fixes the 4× births overestimate from uniform buckets),
`-hindcast` backtest mode, `-runs N` envelopes, and a Nous constant fix
(0.7639 → Φ²=2.618, mirroring phi.Nous in the R97-1 health target).

**Hindcast (May-26 seed → 778 days, pre-R98 mortality, vs today's actuals):**
population +1.0%, under-16 −1.3%, age 16-17 +10.2%, births/day +9.8%, mean
coherence 0.576 vs 0.533 (+8%) — the ripple spiral reproduces. Residuals:
tail deaths −30% and 18-29 2× over (coherence falls slightly too slowly);
30-45/46+ bands are sampling noise (≈120 real agents at 1:3 scale). The v1
static model predicted a *rebound* at this epoch; v2 predicts the decline.

**Live-seeded R98 forecast (271K real agents, real settlements, 20 yr):**
STABLE — recovery to the cap in ~3 sim-years, repro pool 1,159 → ~180K by
yr 5, echo waves at yr 16-20 absorbed. 5-seed envelope: 5/5 STABLE, final pop
398,646-398,752. New structural prediction: mean coherence settles toward
~0.26 long-run as Matter-capped newborns dilute the legacy cohort — today's
53%-Liberated world was a transient; the steady state has liberation rare,
consistent with Doc 25 §3.0's calibrated ~1.2%. Expect the Liberated count to
keep falling post-R98 *without* that being a pathology signal.

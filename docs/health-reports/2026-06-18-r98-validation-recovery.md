# Health Report — 2026-06-18 — R98 Validated: From Collapse to Boom

**Tick 8,071,856 — Summer Day 43, Year 23** (+9 wall-days / 502 sim-days after R98
deploy at tick ~7,348,330, 2026-06-09 21:12 UTC)

## Verdict

R98 is a decisive, clean success. The W-22 adolescent mortality wall is gone: the
single-constant background-mortality recalibration (Agnosis⁴→Agnosis⁶) dropped
deaths from ~150/day to ~15-25/day exactly as projected, and the world flipped from
−115/day decline to **+150–250/day growth**. Population recovered 270,254 → **360,201**.
The reproductive generation — the thing the wall was destroying — refilled from 1,038
to **15,392** agents aged 18-29. No collateral economic damage. The only declining
metric, mean coherence (0.533 → 0.480), is the predicted and documented
newborn-dilution composition effect, not a pathology.

## The numbers

| Metric | R98 deploy (06-09) | Now (06-18) | Trend |
|---|---|---|---|
| Population | 270,254 | **360,201** | ↑ +90K (+33%) |
| Deaths/sim-day | ~150 → 12 | ~15-25 | ↓↓ ~6-10× cut, holding |
| Births/sim-day | ~39 | ~180-260 | ↑↑ |
| Net/sim-day | −115 | **+150 to +250** | reversed |
| Age 18-29 | 1,038 | **15,392** | ↑ 15× |
| Repro 18-45 | 1,159 | 15,466 | ↑ |
| Age <16 | 225,047 (83%) | 263,973 (73%) | still cohort-dominated |
| Age 16-17 | 44,821 | 80,335 | the next bulge |
| Max age | 59 | 47 | legacy elders died; new regime not yet aged in |
| Mean coherence | 0.533 | **0.480** | ↓ by-design dilution |
| Liberated (c≥0.7) | 18.3% | 12.4% | ↓ by-design |
| Satisfaction | 0.697 | 0.697 | flat — material life untouched |
| Mood / Alignment | 0.531 / 0.323 | 0.515 / 0.300 | ↓ downstream of coherence |
| Market health | 0.932 | 0.917 | healthy |
| Work rate | 0.495 | 0.507 | ↑ |
| Crowns (conserved) | 1.4039B | 1.4069B | closed ✓ |
| Trade routes | 686 | 697 | ↑ |
| Council governance | 92.1% | 92.2% | stable, <95% |
| Factions (largest) | Crown 76.0K | Crown 94.9K / Verdant 93.1K | balanced, all treasuries + |

## What's working

- **The mortality fix landed to spec.** ~15-25 deaths/day at 360K ≈ 0.005%/day, now
  dominated by the (unchanged) age sigmoid rather than background scatter.
  avg_survival flat at 0.390 confirms zero starvation — these are age deaths, the
  intended regime. Sage deaths fell to ~2/day (was ~30), so the coherence-ripple
  drain returned to its designed rare-event scale.
- **Reproduction is self-sustaining for the first time.** 15.4K adults aged 18-29 now
  reproduce and survive long enough to do it; births outpace deaths ~10:1.
- **Economy rode the +90K population swing without strain** — crowns conserved, work
  rate up, routes up, factions balanced, governance stable.

## Expected, not concerning (flagged so it isn't misread)

- **Coherence decline is the documented dilution mechanism**, not agents losing
  coherence. 264K under-16 (73% of pop) plus ~200 newborns/day all enter Matter-capped
  (≤0.618, most near Agnosis 0.236), re-weighting the *mean* downward by composition.
  The projector's long-run number is ~0.26; the Liberated count will keep falling to
  its calibrated ~1.2% steady state (Doc 25 §3.0). **Mood (0.515) and alignment (0.300)
  drift with it** because EffectiveMood blends alignment by c² — same root, not separate
  issues. **This should not trigger a coherence watch.** A sentinel `coherence_drift`
  alert firing in the coming weeks would be a false positive; worth a threshold note.

## Genuinely worth watching

- **The 400K cap is ~3 wall-days out.** At +12K/wall-day the soft birth cap (R97-2) is
  already tapering (engaged since 305K) and will throttle births as the world nears
  400K. Forward forecast (v2.1, seeded from today): smooth approach to ~399K within ~1
  sim-year, **no overshoot, no cliff** — the soft cap's first real boundary exam. Watch
  it lands smoothly rather than producing a new synchronized cohort.
- **The echo wave is real and visible.** Age structure is still one marching pulse:
  <16 = 73%, 16-17 = 22%, almost all adults are 18-29 (the 30-45 bucket is 74 agents).
  The 80K-strong 16-17 band becomes a reproductive bulge in ~2 sim-years, then a trough
  behind it. Forward forecast shows the repro pool oscillating 15K→40K→140K→180K before
  damping to ~150K over ~8 sim-years. Healthy, but expect oscillation, not a smooth fill.
- **Max age climbing** is the proof the age pyramid is finally forming — under the gentle
  regime the oldest should age past 47 into their 50s-60s over coming sim-years.

## Why the projector undershot — diagnosis + fix (v2.1)

The 06-09 ROADMAP re-observe prediction ("births/day → ~95, 18-29 pool > 5K") fell short
of reality (births ~200, pool 15.4K). Two distinct causes, one a real model bug:

1. **Real bug — hard-coded birth prosperity multiplier (primary, ~1.8×).** Production
   `processBirths` computes `birthChance = parents/30 × (0.5 + prosperityMod) × capFactor`
   where `prosperityMod = min(1.0, log1p(treasury/pop) × Agnosis)`. The v2 projector
   hard-coded `(0.5 + prosperityMod) = 0.75` ("mid prosperity"). But live settlement
   treasuries are healthy enough that prosperityMod **pins at its 1.0 cap in most
   settlements** → real factor **mean 1.35, median 1.50** (measured across 791 live
   settlements). The 0.75 constant understated per-settlement births ~1.8×.
2. **Reporting error — wall↔sim-day horizon mislabel (secondary).** The cited "~95
   births/day, pool >5K" were the projector's **year-1 (360-sim-day)** outputs, quoted
   as a "3-5 wall-day" check. But 1 wall-day ≈ 56 sim-days, so 9 wall-days = 502 sim-days
   = year 1.4 — past year 1. The projector's own year-2 outputs (pool 41K, births
   244/day) actually bracket reality; I compared against the wrong row.

**Fix (v2.1, `cmd/pop_projector`):** seed each settlement's real treasury from the
`/api/v1/settlements` dump and compute `birthProsperityFactor()` per settlement exactly
as production does, replacing the 0.75 constant. Legacy histogram + hindcast paths keep
0.75 (no treasury data; pre-R98 births were parent-starved not prosperity-limited, so the
validated 2026-06-09 hindcast is unchanged).

**Validation** — re-hindcast of the post-deploy window (seed from 06-09 dumps, run 502
sim-days, compare to today):

| Forecast at day 502 | Population | Error vs actual 360,201 |
|---|---|---|
| v2 (0.75 constant) | ~333,000 | −7.5% |
| **v2.1 (real treasuries)** | ~355,000 | **−1.6%** |

The prosperity fix cut the forecast error 4.7× (−7.5% → −1.6%). Residual −1.6% is within
seed/stochastic noise (treasury is held at its seeded level rather than evolved — a
deliberate 80/20 choice; a full treasury-dynamics model is not worth the complexity).

**Lesson recorded:** always convert wall-days↔sim-days explicitly when setting re-observe
expectations (1 wall-day ≈ 56 sim-days at ~1 TPS); and when a model uses a "representative"
constant for a quantity production computes from state, seed the real distribution instead.

## Suggested actions (ranked)

1. **Hold — do nothing structural.** R98 is validated; the system converges as modeled.
   Resist "fixing" the coherence decline; it's the intended trajectory.
2. **Pre-empt a sentinel/operator misread (doc/threshold only).** Note in the sentinel
   coherence check that a falling Liberated count during post-R98 recovery is expected
   dilution, not regression.
3. **Watch the cap handoff, don't tune it.** Let population reach ~400K and observe the
   soft-cap birth taper before deciding whether the echo needs damping. The R65c-3
   contingency (`birthChance *= 1/max(1,pp)`) stays in reserve if the echo proves sharp.

## Monitor next session (~3-5 wall-days)

- **Cap approach:** smooth settle at ~399K with births tapering, or overshoot/oscillate?
  First real soft-cap boundary test.
- **Coherence floor:** tracking toward ~0.26-0.39 and *decelerating*, not free-falling.
- **Max age** climbing past 47 — proof the age pyramid is forming.
- **Echo timing:** 16-17 → 18-29 transition over ~2 sim-years; births bulge then trough.
- **Satisfaction (0.697):** if it ever moves, *that's* a real economic signal —
  coherence-driven mood/alignment drift is not.

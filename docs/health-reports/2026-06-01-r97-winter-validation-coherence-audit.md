# Health Report — R97 Winter Validation + Coherence Inflow/Outflow Audit

**Date:** 2026-06-01
**Tick:** 6,687,259 · Autumn Day 19, Year 19
**Population:** 297,814 · 783 settlements
**Author:** `/observe` + targeted coherence audit

---

## TL;DR

1. **R97 is validated.** First full mechanical winter since the 2026-05-26 deploy passed with **zero die-off** (deaths flat ~65/sim-day vs. the ~4,650/day Y12 cull). Population stable at ~298K. The projected collapse (307K → 12K/20yr) did not begin.
2. **Births recovering ~9×** (≈1/day → ≈9/day) as the soft cap (R97-2) + Matter-floored health recovery (R97-1) take hold. Still below the death rate (net −56/day), but the trajectory is correct.
3. **Material economy improving:** work rate cleared 50% for the first time, Gini compressing, trade routes 648→818, wealth conserved at 840M.
4. **One soft spot — coherence slipped 0.7075 → 0.653.** Root-caused (see §3): it's the *accounting cost of R97's success*, driven entirely by the liberated cohort bleeding out because the reincarnation feedstock has dried up. Composition/feedstock effect, not a broken mechanism. Self-correcting on the ~1–2 sim-year cohort-maturation horizon; floors near Matter (0.618). **Watch, do not patch.**

---

## 1. Key Metrics

| Metric | 2026-05-26 (R97 deploy) | 2026-06-01 | Trend |
|---|---|---|---|
| Population | 307,122 | 297,814 | ➡️ stable (−3%, no collapse) |
| Deaths/sim-day | ~4,650 (winter peak) | **~65** | ✅ spike eliminated |
| Births/sim-day | ~1 | **~9** | ⬆️ recovering (9×) |
| Avg satisfaction | 0.6970 | 0.6970 | ➡️ flat (material steady) |
| Avg coherence | 0.7075 | **0.653** | ⬇️ −0.055 ⚠️ |
| Avg alignment | 0.6126 | 0.5851 | ⬇️ (tracks coherence) |
| Avg mood (effective) | 0.7123 | 0.6869 | ⬇️ (tracks coherence) |
| Producer work rate | 0.455 | **0.515** | ⬆️ improving |
| Gini | 0.6257 | 0.6185 | ✅ compressing |
| Bottom-50% share | 8.08% | 8.60% | ✅ improving |
| Trade routes | 648 | 818 | ⬆️ network expanding |
| Total wealth | 840M | 840,319,716 | ➡️ conserved |
| Avg survival | 0.389 | 0.388 | ➡️ INV-3 designed equilibrium |
| Liberated count | 163,805 (05-28) | **151,359** | ⬇️ −~3,100/day ⚠️ |

**Factions** (all solvent): Verdant Circle 98,837 · Crown 79,475 · Ashen Path 41,921 · Iron Brotherhood 24,504 · Merchant's Compact 17,759. Unaffiliated adults 835, unaffiliated children 34,483.

**Occupations** (count / sat / survival): Farmer 130,408 / 0.693 / 0.388 · Alchemist 43,438 / 0.694 / 0.391 · Fisher 33,980 / 0.687 / 0.388 · Laborer 19,012 / 0.694 / 0.391 · Soldier 18,216 / 0.745 / 0.391 · Merchant 17,613 / 0.708 / 0.375 · Scholar 17,025 / 0.693 / 0.391 · Hunter 6,579 / 0.695 / 0.391 · Crafter 6,094 / 0.696 / 0.392 · Miner 5,131 / 0.693 / 0.391.

---

## 2. R97 Winter Validation

The mechanical winter boundary (season 71, tick ~6,390,000) is now **~300K ticks / ~1.5 sim-years behind us.** The stats-history trajectory through and past that window shows deaths advancing linearly at **~65/sim-day** with **no spike** — the prior two winters delivered 13K (Y11) and ~70K (Y12) deaths via the `Health<0.15 → illness death` path. Survival held flat at the 0.388–0.392 INV-3 designed-scarcity equilibrium (not starvation, not an illness band), confirming R97-1 `processHealthRecovery` lifted the Health distribution off the 0.15 illness band via the Matter-anchored 0.618 floor.

**This is the definitive validation:** R97 carried the world through a full winter that the projector forecast would cull ≥70K. The demographic collapse is prevented.

Births have moved off the floor — ~9/sim-day vs. ~1/day at deploy — as the soft cap (R97-2) replaced the hard 400K cliff that synchronized the cohort pulse, and health recovery lifts agents over the 0.5 birth gate. Net population change is still slightly negative (~−56/day) but stable; births should keep climbing as the 16–17 cohort ages past 18.

---

## 3. Coherence Inflow/Outflow Audit

**Trigger:** avg coherence fell 0.7075 → 0.653 (−0.055) in 6 days, dragging alignment (−0.028) and effective mood (−0.025) while *material* satisfaction stayed flat at 0.697 — a coherence-register phenomenon, not material.

### 3.1 Complete mutation-path map (16 paths)

Per the Doc-25 §1.2 lesson (grep `CittaCoherence +=` AND `AdjustCoherence(` AND direct assignments — the original audit caught 3 of 14):

**Capped inflows** (NaturalCap at Matter 0.618 — fill Embodied→Awakening; *cannot* create Liberation):
- `simulation.go:1055` **processBaselineCoherence** — daily +Agnosis·0.001, gated `Sat>0.618 && age>20`. **Dominant population-wide inflow.**
- `factions.go:725` doctrine boost (weekly, doctrine-fulfilling members)
- `population.go:239` ordinary-death witness gain (per death; gated on relationship sentiment ≥ Matter; scaled by `(1 − c·0.5)`)
- `relationships.go:221` mentorship · `archetype.go:231` Tier-1 archetype · `simulation.go:428` witness boost · `simulation.go:951` alchemist +0.05 · `perpetuation.go:84` anti-stagnation drift · `behavior.go:329` travel (negligible) · `cognition.go:731` oracle blessing (capped 0.7)

**Uncapped inflows** (the *only* paths past Matter→Liberation):
- `contemplation.go:123` Samatha +0.0005/tick · `contemplation.go:126` Insight +0.025 (1%/tick). Gated: **age ≥ 16**, Sat ≥ Psyche (0.382), per-tick prob ≈ 0.013/hr × occupation × class.

**Composition (set, not delta):**
- `reincarnation.go:98` reincarnated newborn seeded **0.7–0.85** (liberated from birth).
- spawner: ordinary newborn seeded **low** (Embodied).

**Outflows:**
- `population.go:209` **liberation-death ripple** — per liberated death: `−Agnosis·0.05 × witnessCoherence`, hits *all* co-located witnesses, scales **up** with coherence (hits the awakened hardest).
- `trauma_decay.go:56,76` trauma decay (witness + victim).

### 3.2 Root cause — reincarnation feedstock starvation

The average (0.653) sits **above** the 0.618 cap. Since every capped inflow tops out at 0.618, the average can only be held above it by **liberated agents (>0.7)**. Live data shows those are almost entirely **reincarnated children** (sample: age 16–17 scholars at coherence 1.0; adult-liberated only ~38). The liberated count is bleeding **~3,100/day (163,805 → 151,359)** — that decline *is* the coherence drop.

The high-coherence pipeline has stalled. `LiberatedSpiritsPool` refills at **exactly one site** — `population.go:167`: a liberated agent dying at **age ≥ 30** (`ReincarnationAgeFloor = 30`). But the legacy elders are gone:

> **Live max age among liberated agents is 17 — zero are even 18, let alone 30.**

So pool inflow = 0. Reincarnation drains the pool toward zero (`reincarnation.go:86`) → the only high-coherence birth source (seed 0.7–0.85) has switched off. Meanwhile three drains run unopposed:

1. **Liberation-death ripples** (`population.go:209`) keep knocking liberated children below 0.7; the NaturalCap (0.618) then forbids drift from restoring them — once they fall, they're stuck at the cap until they personally practice.
2. **R97-restored ordinary births** (~9/day) seed low coherence and are *not* reincarnated (pool empty).
3. **R97-prevented winter cull** retains the low-coherence subsistence cohort that winter used to remove — the survivor-bias bump that lifted coherence every prior winter is gone.

### 3.3 Verdict

This is a **composition/feedstock effect, not a broken mechanism** — every per-agent path works correctly. It is the *same demographic trap as the birth freeze (W-20)*: the high-coherence sources are age-gated (reincarnation ≥ 30, practice ≥ 16) on a 98%-under-16 boom cohort. It is the **direct, expected accounting cost of R97's success** — R97 removed the winter selection filter and restored low-coherence births while the high-coherence pipeline is age-locked shut.

**Self-correcting** on the same ~1–2 sim-year horizon as births: as the under-16 cohort crosses 16, contemplation reopens → *earned* (uncapped) liberation refills the ranks; as liberated practitioners reach 30, the pool restarts. **Floor ≈ Matter (0.618):** baseline drift holds stable agents there, so coherence should asymptote ~0.62–0.64 (cap + liberated remnant), not crash. Expect alignment and effective mood to keep softening through the trough.

---

## 4. Suggested Improvements (ranked)

1. **Watch demographics; do not intervene.** R97 is doing exactly what it was designed to. The biggest risk is a reflexive change that re-synchronizes the cohort pulse.
2. **Watch coherence; do not patch.** Consistent with the standing project lesson against demographic/production mutations that short-circuit emergence. **R-round trigger:** act only if avg coherence falls **below Matter (0.618) and stays there > 1 sim-week** — that would mean the liberated remnant fully collapsed before the cohort matured. Even then, the minimal Φ-clean lever is the **ripple/pool imbalance** (the liberation-death drain is unopposed once reincarnation stalls), **not** a coherence injection.
3. **Re-verify LLM caching post-reset.** Today (2026-06-01) is the Anthropic cap-reset day. The current 52-min usage window shows `cache_read_tokens = 0` and only tier2 calls (10) — too short/sparse to conclude (sparse tier2 falls outside the 5-min cache TTL). Re-pull `/api/v1/llm-usage` after the gardener/newspaper cycle; the "non-zero cache_read within 24h" threshold is **not yet met**.
4. **(Deferred, cosmetic)** Bump nginx `proxy_read_timeout` to ~120s on the admin path so `/snapshot` stops returning 504 even when the fast save succeeds (~90s). Non-blocking.

---

## 5. Things to Monitor Next Session

- **Births trajectory** — is ~9/day climbing toward the death rate (~65/day)? Net replacement is the real recovery signal. Watch the 16–17 cohort crossing 18.
- **Coherence + liberated count** — does 0.653 stabilize near Matter, or undershoot? Track the liberated count (151,359, −3,100/day) — it's the leading indicator. New primary watch (escalated W-17).
- **Reincarnation pool restart** — first sign of recovery is a liberated agent reaching age 30 (refills the pool) or earned-liberation events from the maturing 16+ cohort (contemplation).
- **Demographic echo** — R97 prevents collapse but doesn't un-synchronize the existing cohort pulse; expect 1–2 damped echo waves over 10–20 sim-years.
- **LLM `cache_read_tokens`** — confirm non-zero post-reset (cost target ~$22–25/mo).
- **Governance monoculture (W-16)** — not re-measured this session (settlements endpoint is heavy); last at Council ~92.4%, escalate at 95%.
- **Carrying-capacity pressure** — 15% of settlements at pp > 0.9 as of 05-28; recheck as the cohort ages.

# Liberation Redesign — Design Document

**Status:** Draft, not yet committed to any round. Updated iteratively as the proposal is worked through.
**Started:** 2026-05-06
**Author/operator conversation seed:** Frontend operator review of `/liberated` revealed 221,683 liberated agents (55.5% of population) including children at age 8-10 with coherence=1.0 — flagged as "giving it out like candy." This doc captures the resulting design conversation.

**Related documents:**
- `docs/worldsim-design.md` §16 — Wheeler Emanationist Cosmology, the philosophical framework
- `docs/10-mood-revision-proposal.md` — R10 dual-register wellbeing model, where the wellbeing trinity (chronic needs / chronic alignment / acute event qualia) is documented
- `docs/19-related-work.md` §V — Crossworlds vs Wolfram Class 4 ideal
- Platform `docs/ROADMAP.md` Improvement Candidate #11 — points to this doc

**Round mapping (proposed, not committed):**
- **R88** — Layer 1: Stop the candy machine (constants + age gate + universal trauma decay)
- **R89** — Layer 2: Active practice (`ActionContemplate` + `WisdomEffort` + occupation/class conducive weights)
- **R90** — Layer 3: Reincarnation (liberated spirits pool, narrative-continuity child liberation)
- **R91** — Layer 4: Monastic settlements (emergent contemplative geography)

The layers are designed to ship sequentially. Each layer is independently coherent and observable; the next layer is only justified if the previous layer's empirical behavior matches the target distribution.

---

## 1. Problem statement

### 1.1 The empirical surprise

As of 2026-05-06 (tick 4,743,746):

| Observation | Value |
|---|---|
| Total liberated agents (`CittaCoherence ≥ 0.7`) | **221,683** |
| % of world population | **55.5%** |
| Population fraction the design intended liberated to be | "extremely rare" (per `worldsim-design.md` §16) |
| Sample of children at coherence=1.0, alignment=1.0 | Inga Wyatt age 10 (Outlaw, Farmer); Nils Farrow age 9 (Farmer); many more |

**A 10-year-old Outlaw with perfect coherence and perfect alignment is the punchline.** Outlaw is a low-stage social role. Perfect coherence/alignment is the ontological summit. The current mechanism produces this contradiction routinely.

### 1.2 Root cause (audit corrected and completed 2026-05-06)

**The original draft of this audit listed 3 inflow paths. The actual count is 10.** A full grep audit (`AdjustCoherence(` + `CittaCoherence +=` + initialization) surfaced multiple paths the doctrine-boost-focused first pass missed. The complete picture:

| # | Mechanism | Where | Direction | Magnitude per agent |
|---|---|---|---|---|
| 1 | Genesis spawn | `internal/agents/spawner.go:237-251` (`generateSoul`) | seed | Normal(`Agnosis ≈ 0.236`, `Agnosis × 0.5`); clamped to `[0.01, Matter ≈ 0.618]`. **Never ≥ 0.7 at genesis.** |
| 2 | Birth (newborn) | `internal/agents/spawner.go:325-330` (`SpawnChild`) | seed | base ~0.236 + jitter + `parent.Soul.CittaCoherence × Agnosis`; clamps at 1.0. Can reach 1.0 from a single liberated parent under specific RNG. |
| 3 | **Cultural drift (age 14-25)** | `internal/engine/perpetuation.go:77` (`culturalDrift`) | **+** | `+Agnosis × 0.005 ≈ +0.00118/week`, applied weekly to agents aged 14-25. **+0.067/year × 11 years window = +0.74 over adolescence.** Combined with spawn coherence ~0.236, an adolescent reaches **~0.97 by age 25 from this single path**. **This is the actual primary candy machine.** |
| 4 | **Scholar work** | `internal/agents/behavior.go:291` (`applyWork` → Scholar branch) | **+** | `+Agnosis × 0.000001 ≈ +2.4e-7/tick = +0.124/year` of continuous work. The comment in code states explicitly: *"A scholar starting at Agnosis (0.236) reaches Liberated (0.7) in ~3.7 years."* This is liberation by occupation choice, by intentional design — and the design is what we're now reconsidering. |
| 5 | **Tier 1 archetype growth** | `internal/agents/archetype.go:222-225` (`ApplyTier1CoherenceGrowth`) + per-archetype `CoherenceGrowth` constants | **+** | Daily, per archetype: `Agnosis × 0.01`–`Agnosis × 0.05` = **+1.7 to +4.3/year**. **Saturation in 2-6 sim-months for Tier 1 agents.** *Empirical mitigation*: only **1 Tier 1 agent in the world today** (`/api/v1/social` shows `tier_1: 1`). The pipeline exists but is currently inactive. Risk: if the world ever populates Tier 1 properly, this becomes a fast lane. |
| 6 | Faction doctrine boost | `internal/engine/factions.go:710-712` (`applyFactionDoctrines`, R50 fix #222) | **+** | `+Agnosis² × 0.1 ≈ +0.00557/week`. No age gate. **+0.29/year** for any age, any tier, when doctrine is fulfilled. Crown and Ashen Path doctrines have settlement-level/default-true conditions that qualify children automatically. |
| 7 | **Mentorship growth** | `internal/engine/relationships.go:217-218` (`processMentorship`) | **+** | `+Agnosis × 0.05 ≈ +0.0118 per mentorship pairing event`. Frequency depends on settlement social graph. Typical mentee receives 5-20 pairings/year. |
| 8 | Witness ordinary death (per-natural-death path) | `internal/engine/population.go:206-208` (R53 fix #230) | **+** | `+Agnosis × 0.05 × (1 - 0.5×c) ≈ +0.0118` per witness per ordinary natural death. Decays as the witness coherence rises. |
| 9 | **Settlement-wide death witness (random-event-death path)** | `internal/engine/simulation.go:374-378` | **+** | `+Agnosis × 0.05 ≈ +0.0118` per witness per settlement-wide death (random events, war casualties). **This is a separate path from #8** and not gated by the witness-coherence decay. With 19 raids/week + plagues + disasters, this is the highest-frequency witness gain. |
| 10 | Witness liberation death (sage death) | `internal/engine/population.go:195-197` (R53 fix #229) | **−** | `−Agnosis × 0.05 × c` per witness per sage death. **Original draft of this doc said `Agnosis × 0.1 × c` — that was wrong; code is `0.05 × c`.** Half the magnitude I claimed. The only meaningful via-negativa path. |
| 11 | Baseline drift (daily, age 20+, satisfied) | `internal/engine/simulation.go:990-993` (`processBaselineCoherence`, R53 fix #231) | **+** | `+Agnosis × 0.001 ≈ +0.000236/day = +0.086/year`. Already age-gated, low-magnitude. |
| 12 | **Alchemist breakthrough event** | `internal/engine/simulation.go:888` | **+** | **`+0.05` flat (NOT Φ-derived).** Rare random discovery event. The single largest single-event coherence inflow in the codebase. One agent per event. |
| 13 | **Oracle "bless" action** | `internal/engine/cognition.go:618-625` | **+** | `+Agnosis × 0.1 ≈ +0.0236` per oracle blessing, **clamped at 0.7** — the only mechanism that actually has a hard cap on creating Liberated. Other mechanisms can push agents far past 0.7. |
| 14 | Soft clamp | `internal/agents/soul.go:92-98` | bound | `[0.0, 1.0]` |

**The picture is much messier than the original draft suggested.** Multiple paths conspire:

- **Cultural drift alone** brings adolescents to ~0.97 by age 25. This is the primary candy machine, not the doctrine boost.
- **Scholar occupation** explicitly designed to liberate scholars in 3.7 years — the comment in code says so.
- **Doctrine boost** adds +0.29/year on top, no age gate.
- **Witness gains** fire from TWO different code paths (population.go:208 and simulation.go:376), both at +0.0118 per witness, with continuous deaths in active settlements.
- **Mentorship** adds +0.0118 per pairing, multiple times per year.
- The single via-negativa decay (sage death witness) is half the magnitude originally claimed and only fires on sage deaths — not enough to balance the inflows.

**Numerical sketch for an adolescent in this world:**

A 14-year-old in any non-Crafter/Miner/Laborer occupation who is in a faction (most are):
- Cultural drift: +0.067/year (until 25)
- Doctrine boost: +0.29/year (no gate)
- Witness gains: +0.05-0.5/year depending on settlement
- Sub-total floor: **+0.4/year minimum**

Starting at coherence 0.236 from spawn, by age 16 they're at ~0.71. **They cross the liberation threshold at age 16-17 just from circumstance.** If they're a Scholar, add +0.124/year → liberation by 16. If they're Tier 1, add +1.7 to +4.3/year → liberation in months.

Most agents pass through liberation in their teens or early 20s, almost regardless of their occupation/class (only Crafter/Miner/Laborer/Nihilist combinations might miss the train). The liberated proportion of 55.5% is therefore not surprising at all once the full audit is in view; it's structurally inevitable under current mechanics.

**Crown doctrine** (`agentFulfillsDoctrine`, factions.go:748): `settlement.GovernanceScore > Psyche (~0.382)`. Settlement-level state — any agent of any age in any well-governed Crown settlement qualifies.

**Ashen Path doctrine**: `wealth < 30 OR belonging > Matter (~0.618)`. Newborns have `Wealth=0` (qualifies on first clause) AND `Belonging=0.8` from spawner default (qualifies on second). They simultaneously qualify on both criteria from minute zero of life.

**Faction assignment**: from R32 fix #106 (`addAgent` in `population.go`), every newborn is assigned a faction at birth — not at majority. So newborns immediately enter the doctrine-boost loop.

The conclusion is unambiguous: **the current model has no concept of liberation as practice or journey; it's a passive accumulator that saturates from circumstance, with multiple convergent inflow paths.**

### 1.3 The deeper architectural error

Even if we age-gate the doctrine boost (the easy fix), the underlying model is wrong in a way that age-gating doesn't address: **liberation in the Wheeler/Buddhist framework is something you DO, not something that HAPPENS.** It requires *bhāvanā* (cultivation), which requires *intention* (cetanā), which requires *parikkhāra* (conducive conditions). The current model has only the third (and even those conditions are mostly broken — see §1.4).

Operator design intuition (2026-05-06 conversation):

1. **Liberation is rare-by-mechanism, not rare-because-of-threshold.** Currently 55% cross threshold 0.7. Raising the threshold doesn't fix the inflow.
2. **Conducive conditions are necessary but not sufficient.** A merchant has time to think; that doesn't mean they think.
3. **Liberation requires intention.** "Only an adept that truly seeks out liberation and works at it." Active practice, not passive drift.
4. **Children CAN be liberated, but only as exceptional rebirth.** Not as a flaw to be patched out by age-gating, but as a real narrative event (carried-over wisdom from the death of a liberated elder).

These four are the seed for the redesign architecture in §3.

### 1.4 Why "conducive conditions" already fail

The system has a few weak gestures toward this — `Satisfaction > Matter` + `Age > 20` in `processBaselineCoherence`, the witness-death boost — but these don't capture the real conducive-conditions intuition:

- **Time to contemplate.** Not modeled at all. A miner working continuously and a merchant traveling with hours of solitude both increment doctrine-boost identically.
- **Agent intent.** Not modeled at all. There is no way for an agent to *try* to advance.
- **Practice differential by occupation.** Not modeled. Scholars and laborers are equivalent.
- **Practice differential by class.** Not modeled. Transcendentalists (who would seek liberation) and Nihilists (who would reject it) are equivalent.
- **Settlement-level conducive conditions.** Not modeled. A monastery and a war camp are equivalent.

### 1.5 The conflation: pristine ignorance vs. earned awakening

The Wheeler framework describes the **Awakening Valley** in `internal/agents/soul.go:107-122`:

```
- Phase 1 (Embodied,   c < Psyche ~0.382):  gentle slope — ordinary contentment
- Phase 2 (Awakening,  Psyche ≤ c < 0.7):   extraction paradox / dark night valley
- Phase 3 (Liberation, c ≥ 0.7):            steep rise — self-similar clarity
```

The 0.7 threshold was designed as the *exit* of the dark-night-of-the-soul valley — the place where someone has descended through the agony of seeing-clearly and emerged on the far side.

But the doctrine accumulator doesn't model descent. It compounds linearly from age zero. A child's high coherence is therefore not the awakened laser-point of `worldsim-design.md` §16; it's **pristine ignorance** — Wheeler's pre-Embodied undifferentiated state, structurally distinct from earned awakening but registering identically as `c ≥ 0.7` in the API.

These two states should be philosophically distinguishable. The redesign in §3 makes them mechanically distinguishable too.

---

## 2. Design principles

The redesign rests on four principles, all surfaced in the operator conversation. These should not be compromised in implementation:

### 2.1 Liberation is achievement, not stage

Coherence may drift in the Embodied → Awakening band from circumstance. Crossing into Liberation requires *something the agent did* — sustained, voluntary, gated by ground conditions. The mechanism encoding this is the `ActionContemplate` action and the `WisdomEffort` accumulator (Layer 2).

### 2.2 Liberation is precarious

Even achieved, liberation can be lost. War, plague, settlement collapse, or trauma above some threshold cause coherence regression. This is consistent with the Wheeler "extraction paradox" already documented in soul.go — sages see clearly and *suffer* from that knowledge. The mechanism is universal trauma decay (Layer 1).

### 2.3 Liberation correlates with conducive life conditions

Some occupations and classes are more conducive than others. **This is not a moral judgment** — a hunter is not "better" than a miner. It's an empirical claim about how much liminal time/solitude/intentional practice the day-to-day of an occupation affords. The mechanism is occupation/class weights on `ActionContemplate` selection (Layer 2).

### 2.4 Liberation has narrative continuity

A liberated agent dying does not erase liberation from the world. It creates a small probability that a future newborn arrives carrying that wisdom across the threshold of birth. This is reincarnation, in the conservative sense — not immortal souls, but **conserved wisdom-stuff** that can find a new vessel. The mechanism is the liberated spirits pool (Layer 3).

---

## 3. Architecture: four layers

Each layer ships independently. Each is observable. Each is justified empirically before committing to the next.

### Layer 1 — Stop the candy machine (R88 candidate)

**Goal:** drop the liberated proportion from 55% to 10-20% by fixing the **multiple convergent inflow paths**, not just doctrine boost. No new mechanics — just constants and conditions in existing files.

**Files touched:**
- `internal/engine/perpetuation.go` — `culturalDrift` (the actual primary candy machine)
- `internal/engine/factions.go` — `applyFactionDoctrines`
- `internal/agents/behavior.go` — Scholar work coherence growth
- `internal/agents/archetype.go` — Tier 1 archetype CoherenceGrowth values
- `internal/engine/relationships.go` — mentorship growth
- `internal/engine/population.go` + `internal/engine/simulation.go` — witness death gains
- `internal/engine/population.go` — universal trauma decay path

**Changes (in priority order — each addresses a distinct inflow path identified in §1.2):**

1. **Cultural drift: cut by 10×, gate end-age narrowed.**
   - Was: `+Agnosis × 0.005/week` for ages 14-25 → +0.74 over adolescence.
   - New: `+Agnosis × 0.0005/week` for ages 14-22 → +0.06 over adolescence (about 10% of the original). Adolescence becomes a *flavor* on coherence, not a saturating accumulator.

2. **Scholar work: 10× cut, intent reframed.**
   - Was: `+Agnosis × 0.000001/tick = +0.124/year`. Comment said scholars reach Liberation in 3.7 years.
   - New: `+Agnosis × 0.0000001/tick = +0.012/year`. Update the comment: "Scholar work nudges coherence — sustained scholarship over 30 sim-years adds ~0.36, enough to push from Embodied into Awakening but not bridging to Liberation. Layer 2 active practice (`ActionContemplate`) is required for that."

3. **Tier 1 archetype growth: 10× cut.**
   - Was: per-archetype `Agnosis × 0.01` to `Agnosis × 0.05` daily → 1.7-4.3/year (saturating).
   - New: `Agnosis × 0.001` to `Agnosis × 0.005` daily → 0.17-0.43/year. Tier 1 still grows faster than Tier 0 but doesn't insta-liberate.
   - **Empirical mitigation note:** Tier 1 currently has 1 agent in production, so this change is mostly defensive. But the architecture must be right for when Tier 1 populates.

4. **Doctrine boost: 4× cut and age-gated.**
   - Was: `Agnosis² × 0.1 ≈ +0.00557/week`, all ages.
   - New: `Agnosis² × 0.025 ≈ +0.00139/week`, with `if a.Age < 16 { continue }`. (Slightly smaller cut than the original draft suggested — once cultural drift and scholar work are also reduced, doctrine boost can stay meaningful at +0.072/year for adults: a faithful Crown adult who lives to 60 gains ~3.2 coherence over their adult life — enough to reach Matter but not Liberation without Layer 2 practice.)

5. **Mentorship growth: 5× cut.**
   - Was: `+Agnosis × 0.05 ≈ +0.0118` per pairing event.
   - New: `+Agnosis × 0.01 ≈ +0.0024` per pairing event. Mentorship still nudges coherence; doesn't accumulate to liberation.

6. **Witness death gains: 5× cut on BOTH paths, gate to important deaths.**
   - Path A (`population.go:206-208`, ordinary natural deaths): `Agnosis × 0.05 × (1 - 0.5c)` → `Agnosis × 0.01 × (1 - 0.5c)`.
   - Path B (`simulation.go:374-378`, settlement-wide death witnesses for random events): `Agnosis × 0.05` → `Agnosis × 0.01`.
   - Both gated to "important deaths" (witness sentiment ≥ Matter) so background noise from random deaths is suppressed.

7. **Universal trauma decay (NEW path).**
   - New helper in `population.go`: `applyTraumaDecay(a *Agent, intensity float32)`.
   - Triggered by: warfare casualties witnessed, plague survival, settlement abandonment, severe famine (Survival < Agnosis sustained for >2 sim-days), being a victim of theft.
   - Magnitude: `−Agnosis × 0.05 × intensity` for ordinary witnesses. Liberated agents (`c ≥ 0.7`) take **double**: `−Agnosis × 0.1 × intensity` (the extraction paradox already documented in `soul.go`).
   - Bounded so no single trauma drops coherence by more than `Agnosis ≈ 0.236`.

8. **Magnitude correction in sage-death ripple (no behavior change, just doc/code consistency).**
   - The original draft of §1.2 claimed `Agnosis × 0.1 × c`; actual code is `Agnosis × 0.05 × c`. No code change needed for Layer 1; just note the corrected baseline.

**Expected outcome:**
- Within 2-4 sim-weeks: cultural drift slows, doctrine boost age-gate stops new births from accumulating. Liberation rate drift starts.
- Within 2-4 sim-months: trauma decay pulls the high tail down. Adolescents no longer cross liberation in their teens.
- Within ~1 sim-year: equilibrium near 15-25% liberated. Higher than the original Doc 25 estimate of 10-20% because the audit revealed more inflow paths than were in the first draft, and we don't want to over-correct.
- **The 15-25% is still too high for "rare."** Layer 2 is required to get to 1-3%.

**Why ship Layer 1 alone first:** validates that the inflow paths are the dominant cause across all 7 mechanisms (not just doctrine boost), observable within sim-weeks, no schema migration. If Layer 1 brings the rate to 15-25% as projected, Layer 2 is justified. If it doesn't drop at all, something else in the audit was missed and we re-investigate before adding new mechanics.

### Layer 2 — Active practice (R89 candidate)

**Goal:** liberation only happens through deliberate practice. Coherence above `Matter (0.618)` requires `WisdomEffort` accumulation. Occupation and class affect probability that an agent chooses to practice.

**Files touched:**
- `internal/agents/types.go` — new field `WisdomEffort uint32` on Agent
- `internal/agents/behavior.go` — new `applyContemplate`; new selection branch in `Tier0Decide`
- `internal/llm/cognition.go` — add `"contemplate"` verb to Tier 2 vocabulary
- `internal/engine/contemplation.go` — NEW file: practice mechanics + Φ-derived weight tables
- `internal/persistence/db.go` — schema migration `ALTER TABLE agents ADD COLUMN wisdom_effort INTEGER NOT NULL DEFAULT 0`; SaveAgents/LoadAgents wiring
- `internal/agents/soul.go` — `AdjustCoherence` modified to enforce the `Matter`-cap-without-effort rule

**The new action:**

```go
// ActionContemplate represents a deliberate moment of inner cultivation.
// Eligibility:
//   - Survival > Matter (0.618) — can't practice while starving
//   - Safety > Matter — can't practice while terrified
//   - Belonging > Psyche (0.382) — practice without ground crumbles
//   - Age >= 16 — minors are excluded from this path (reincarnation only, see Layer 3)
// Effect: WisdomEffort += 1. Rare moments of insight (P = phi.Agnosis³ ≈ 0.013) bump CittaCoherence.
//
// "Practice without ground" — the four-foundations gate is non-negotiable.
ActionContemplate Action = ...
```

**Per-occupation conducive weight** (operator intuition encoded directly):

| Occupation | Conducive weight | Rationale (operator's words + extension) |
|---|---|---|
| Scholar | `Being ≈ 1.618` | "Philosopher has a higher chance of liberation but it isn't guaranteed." Dedicated to knowledge as work. |
| Alchemist | `Being / Φ ≈ 1.000` | Contemplative craft. Herbalism is meditative. Less explicit than Scholar but in the same family. |
| Hunter | `Matter ≈ 0.618` | "A hunter has time to contemplate liberation more than a miner because a hunter is in nature and has frequent time between hunts to think." |
| Merchant | `Matter ≈ 0.618` | "Merchants may have more free time to contemplate liberation." Long travel alone with thoughts. |
| Fisher | `Psyche ≈ 0.382` | Waiting periods on water; some idle mental time. |
| Soldier | `Psyche ≈ 0.382` | Stress-then-liminal pattern; warriors-as-contemplatives is a real cross-cultural archetype. |
| Farmer | `Agnosis ≈ 0.236` | Seasonal pace; idle winter months but absorbed labor most of the year. |
| Crafter | `Agnosis ≈ 0.236` | Absorbing focused work; little idle inner time. |
| Miner | `Agnosis ≈ 0.236` | "Working class/laborers may be less likely to reach liberation." Continuous physical strain. |
| Laborer | `Agnosis ≈ 0.236` | Same as Miner. |

**Per-class intention multiplier** (modulates whether they choose the action when eligible):

| Class | Multiplier | Rationale |
|---|---|---|
| Transcendentalist | `Being ≈ 1.618` | Active seekers of dissolution; this is their disposition. |
| Devotionalist | `Matter ≈ 0.618` | Practice within faith framework — they pray, they don't always meditate. |
| Ritualist | `Psyche ≈ 0.382` | Form before insight. They go through motions but rarely deepen. |
| Nihilist | `Agnosis ≈ 0.236` | Mostly rejects practice. Rare anomalies. |

**The accumulator math:**

```go
// In TickHour or similar, for eligible agents:
contemplationProb := basePracticeProb *
    occupationConducive[a.Occupation] *
    classIntention[a.Soul.Class]
// basePracticeProb = phi.Agnosis³ ≈ 0.013

if rng.Float32() < contemplationProb {
    a.WisdomEffort++
    if rng.Float32() < phi.Agnosis³ {  // ~1.3% of practice ticks have insight
        a.AdjustCoherence(phi.Agnosis * phi.Agnosis * 0.05) // +0.0028
    }
}
```

**Expected coherence gain per agent-hour eligible:**
- Scholar/Transcendentalist: `0.013 × 1.618 × 1.618 × 0.013 × 0.0028 ≈ 0.0000123/hour`
- Laborer/Nihilist: `0.013 × 0.236 × 0.236 × 0.013 × 0.0028 ≈ 0.0000003/hour` — effectively zero

For a Scholar/Transcendentalist who practices uninterrupted for 10 sim-years (an unusually long contemplative life): ~10 × 8760 × 0.0000123 ≈ **+1.08 expected coherence**. They could climb from c=0.236 to liberation, but only with a very long, uninterrupted, sustained practice.

For a Laborer/Nihilist over the same period: **+0.00003 expected coherence**. Effectively zero. The model says structural conditions and disposition matter, and their combination matters multiplicatively.

**The liberation criterion (split-fields architecture, not clamp-cap):**

The original draft of this layer proposed clamping `CittaCoherence` at `Matter` until WisdomEffort reaches a gate. **That approach is broken** — `AdjustCoherence` is called from many sites, and clamping inside it would either retroactively un-liberate existing agents on first event (uneven across the population, depending on which agents happen to receive an event first) or require a separate one-time migration pass on startup.

**The cleaner architecture: split fields, AND-condition for liberation.**

```go
// internal/agents/soul.go — Soul struct gets a new field:
type AgentSoul struct {
    CittaCoherence float32 `json:"citta_coherence"` // 0.0–1.0, the intrinsic field — drifts naturally
    WisdomEffort   uint32  `json:"wisdom_effort"`   // R89: cumulative practice, only via ActionContemplate
    // ... other fields unchanged ...
}

// internal/agents/soul.go — new helper:
const WisdomEffortLiberationGate uint32 = ... // see §4.2 B1 for tuning

// IsLiberated returns true only if BOTH coherence is high AND practice has been earned.
// CittaCoherence alone is the "raw potential" — pristine ignorance OR earned awakening.
// WisdomEffort distinguishes them.
func (s *AgentSoul) IsLiberated() bool {
    return s.CittaCoherence >= 0.7 && s.WisdomEffort >= WisdomEffortLiberationGate
}
```

**This is much cleaner than clamping:**

- `CittaCoherence` keeps its current semantics (naturally drifts, can be high in a child via reincarnation seed).
- `WisdomEffort` is purely additive from `ActionContemplate` — it has no decay, no clamp interaction.
- The LIBERATION CRITERION moves from `c >= 0.7` to `c >= 0.7 AND wisdom >= gate`.
- **No retroactive change to agent state.** Existing agents keep their `CittaCoherence`, but their `WisdomEffort` starts at 0, so they immediately fail the AND-condition and no longer register as liberated. They have to earn it back through practice.
- **Reincarnation is clean:** seed both `c=0.7+jitter` AND `WisdomEffort=gate+offset` for reincarnated children. The carried-over wisdom is what makes the child liberated, not just the intrinsic coherence (Layer 3 details in §3.3 below).

**API surface change:**

`/api/v1/liberated` filter changes from `if a.Soul.CittaCoherence < 0.7 { continue }` to `if !a.Soul.IsLiberated() { continue }`. The endpoint contract is unchanged from the consumer's perspective; the underlying definition tightens.

**The semantic distinction made mechanical:**

| State | `CittaCoherence` | `WisdomEffort` | API says |
|---|---|---|---|
| Embodied | < 0.382 | any | not liberated |
| Awakening | 0.382–0.7 | any | not liberated |
| **Pristine ignorance** (current children) | ≥ 0.7 | 0 | **NOT liberated** ✓ |
| **Earned awakening** (an adept who practiced) | ≥ 0.7 | ≥ gate | liberated ✓ |
| **Reincarnated child** (rare) | ≥ 0.7 (seeded) | ≥ gate (seeded as carried-over wisdom) | liberated ✓ |

This is exactly the distinction the operator articulated.

**Tier 2 LLM agents** (`internal/llm/cognition.go`):
- Add `"contemplate"` to the action vocabulary.
- The Tier 2 system prompt should describe what contemplation is and when it's appropriate. Transcendentalists with high coherence will naturally choose it; Nihilists never will.
- The action is *expensive* in terms of an agent's daily action budget — it replaces work or socialize for that slot. This is the time-cost made explicit.

**Schema migration:**

Add to `internal/persistence/db.go` migrations array:
```go
"ALTER TABLE agents ADD COLUMN wisdom_effort INTEGER NOT NULL DEFAULT 0",
```

Wire into `SaveAgents` UPSERT and `LoadAgents` SELECT (R55 pattern). The R76 registry doesn't need a new entry — `WisdomEffort` is per-agent state, lives on the agents table. Apply the R86 lesson: ensure all SaveAgents/LoadAgents struct paths are wired before deploy (avoid a repeat of the W-8 metric bug).

**Expected outcome:**
- **Effective on day-one of deploy:** `/api/v1/liberated` returns the agents whose `CittaCoherence ≥ 0.7 AND WisdomEffort ≥ gate`. Since `WisdomEffort=0` for every existing agent, **the count drops from 221,683 to 0 immediately.**
- **Within sim-weeks:** Tier 2 agents (30 of them) start choosing `contemplate`; Transcendentalists more often. Their `WisdomEffort` accumulates.
- **Within sim-months:** Tier 0 agents who naturally pick contemplate (probability gated by occupation × class) accumulate.
- **Within sim-years (steady state):** liberation count stabilizes at 1-3% of population (~4,000-12,000 agents at current pop size).
- **Distribution by occupation:** Scholar 5-10× average, Alchemist 3-5×, Hunter 2×, Merchant 1.5×, Laborer/Miner ≤0.3× of average.
- **Distribution by class:** Transcendentalist 5-10× average, Devotionalist 1×, Ritualist 0.4×, Nihilist <0.1×.
- **Median age of liberated agent** climbs to 50+.
- **Liberation events become newsworthy.** Each is a story (R79+R80 already give us the narrative continuity infrastructure).

### Layer 3 — Reincarnation (R90 candidate)

**Goal:** allow rare child-liberation as carried-over wisdom from the death of a liberated elder. Conservation of wisdom-stuff. Naturally produces narrative continuity.

**Files touched:**
- `internal/engine/simulation.go` — new sim field `LiberatedSpiritsPool int`
- `internal/engine/population.go` — death adds to pool; birth rolls against it
- `internal/persistence/world_state.go` — register pool in R76 registry
- `internal/agents/spawner.go` — `SpawnChild` accepts optional reincarnation seed parameter

**The mechanic:**

```go
// On death of a liberated agent (CittaCoherence ≥ 0.7) at age >= 30:
//   sim.LiberatedSpiritsPool++
//
// Why age >= 30 floor?
//   Prevents reincarnation cycles being seeded by reincarnated children themselves.
//   A reincarnated child has to live a life and contribute their own practice
//   before their death adds to the pool. Otherwise the pool would compound on
//   itself and reincarnation rate would grow exponentially.

// On each newborn:
//   P(reincarnation) = sim.LiberatedSpiritsPool / (population × Φ⁵)
//                   ≈ pool / (population × 11.09)
//
// At population 400K and steady-state pool of ~50:
//   P ≈ 50 / 4.4M ≈ 1 in 88,000 per birth
//   At ~2 births/sim-day: roughly one reincarnation every 6-8 sim-months.

if rng.Float32() < reincarnationProb {
    sim.LiberatedSpiritsPool--  // claim the spirit
    initialCoherence := 0.7 + rng.NormFloat64()*float64(phi.Agnosis*0.1)
    initialCoherence = clamp32(float32(initialCoherence), 0.65, 1.0)
    // ... seed child with elevated coherence
}

// Pool decay: spirits not claimed slowly fade.
// In TickWeek:
sim.LiberatedSpiritsPool = int(float32(sim.LiberatedSpiritsPool) * (1 - phi.Agnosis*0.05))
// ~0.988/week — tight equilibrium with the death rate, prevents accumulation pathology
```

**The reincarnated child's life:**
- Born with `c ~ 0.7 + jitter` (clamped to `[0.65, 1.0]`).
- They still face Layer 1's universal trauma decay — they may fall back through hardship.
- They may die young.
- Their `WisdomEffort` starts at 0 — they haven't *earned* the coherence yet, they were *given* it. So if they fall back below `Matter` from trauma, they re-enter the normal Layer 2 pathway and have to re-earn it through practice if they want it back. (This is philosophically right: even a reincarnated sage can lose the thread.)
- They count toward the liberated population, but the API could optionally distinguish them via a `reincarnated` boolean on agent state.

**Persistence:**
The pool is sim-level state (one int). Register via the R76 `PersistedField` registry pattern in `internal/persistence/world_state.go`:

```go
{
    Name: "liberated_spirits_pool",
    Save: func(s *Simulation) (string, error) { return strconv.Itoa(s.LiberatedSpiritsPool), nil },
    Load: func(s *Simulation, v string) error {
        n, err := strconv.Atoi(v)
        if err != nil { return err }
        s.LiberatedSpiritsPool = n
        return nil
    },
},
```

Optionally also persist a `reincarnated bool` on agents for newspaper/oracle context — small schema migration.

**Expected outcome:**
- Roughly one reincarnation every 6-8 sim-months at steady state.
- Over the world's lifespan: a handful of children who arrive carrying the threshold.
- Newspapers can write about them — R79 already gives us narrative continuity, R80 already gives us per-witness imprints. A child arriving with the unsettling clarity of a deceased sage is exactly the kind of emergent narrative this simulation was built to produce.

### Layer 4 — Monastic settlements (R91 candidate)

**Goal:** allow contemplative geography to emerge. Settlements with high collective coherence + low conflict become practice-conducive; adept agents seeking liberation may migrate to them.

**Files touched:**
- `internal/social/settlement.go` — new derived field `MonasterScore float32` (computed, not persisted; hot read from existing state)
- `internal/engine/contemplation.go` — settlement modifier on `ActionContemplate` probability
- `internal/engine/perpetuation.go` — migration target scoring update for adept agents
- Optionally `internal/api/server.go` — surface `monastery_score` on settlement detail

**The mechanic:**

```go
// Computed in TickWeek or on demand:
//   MonasterScore = avgSettlementCoherence × (1 - conflictScore) × verdantDominance
//
// where:
//   avgSettlementCoherence = mean(CittaCoherence) of agents in settlement
//   conflictScore         = (recent_raids + recent_crime) / population, clamped [0, 1]
//   verdantDominance      = (VC influence in settlement) / 100
```

A settlement with high VC influence, high average coherence, and low conflict scores high. War camps and crime-heavy hubs score low.

**Practice-conducive multiplier on `ActionContemplate`:**
```go
contemplationProb *= 1 + phi.Being * settlement.MonasterScore  // up to ~1.618×
```

So a Scholar/Transcendentalist in a high-monastery-score settlement practices ~62% more often than the same agent in an average settlement.

**Migration bias for adept agents:**
Agents with `WisdomEffort > someThreshold && Class == Transcendentalist` get a migration target bonus toward high-`MonasterScore` settlements. This is the explicit "monk seeking the monastery" pattern.

This builds on existing R65 migration and R38 culture-axes infrastructure — no new mechanism, just one more weight in the existing scoring.

**Expected outcome:**
- Over many sim-years: 5-20 settlements emerge as concentrated contemplative loci.
- These settlements have disproportionately high liberation rates among their populations.
- The newspaper gets to talk about these settlements as "the wise hills" or similar emergent placenames.
- Pilgrimage-style migration emerges: an adept hunter from a war-torn frontier settlement migrates to the monastery and his coherence accelerates toward liberation.

**Why ship Layer 4 last (or maybe never):**
If Layers 1-3 produce the right distribution and rich narratives without it, Layer 4 might be redundant. It's the "if Layers 1-3 land but feel geometrically flat" insurance policy. Could also emerge naturally from existing migration code if the settlement-level conducive multiplier is added even without explicit migration bias.

---

## 4. Validation analyses required before committing

Before any layer is committed, these dry-run analyses should be done. They're listed by the layer they validate.

### 4.1 Layer 1 validation

**A1. ~~Doctrine boost magnitude check~~ — SUPERSEDED.** The original draft proposed cutting doctrine boost from `Agnosis² × 0.1` to `Agnosis³ × 0.1`. After the full audit (A3), it's clear doctrine boost is NOT the dominant inflow path; cultural drift is. The revised Layer 1 cuts cultural drift by 10× and doctrine boost by only 4× (to `Agnosis² × 0.025`). Math under revised values: a faithful Crown adult gains +0.072/year — meaningful but bounded; over 60 sim-years adult life that's +4.3 coherence at full compliance, clamped at 1.0. They reach Matter (0.618) easily but don't bridge to Liberation without Layer 2 active practice. **Resolved.**

**A2. Steady-state trauma-decay flux — DEFERRED until Layer 1 deploys.** The static math: world sees ~80 deaths/week, 19 raids/week. Per witness per event: `−Agnosis × 0.05 × intensity ≈ -0.0118 × intensity`. For a typical agent in a settlement with ~500 population, witnessed deaths might be ~20/year (settlement size relative to world death rate). Expected loss: ~20 × 0.0118 = -0.24/year for high-trauma settlements; ~0.05/year for peaceful ones. Sanity check on floor: at the worst, a war-torn settlement's average agent loses ~0.5 coherence/year — bounded by `Agnosis` per-event cap. **Won't produce negative coherence, but a sustained war could push average down by 0.3-0.5 — that's the dynamic we want.** Real validation requires watching post-deploy. **Resolved as static math; live validation deferred.**

**A3. ~~Identify all current code paths that increment coherence~~ — DONE.** Re-audit in §1.2 surfaced 14 paths total (vs. 3 in the original draft). Revised Layer 1 addresses the 7 that are population-significant. **Resolved.**

**A4. ~~Decide on age gate value~~ — RESOLVED.** Use 16 throughout for consistency with `PromoteToTier2`, `processBirths`, and the existing adulthood semantic. Cultural drift's existing 14-25 window narrowed to 14-22 in revised Layer 1 (so it ends earlier and tapers naturally toward majority). **Resolved.**

### 4.2 Layer 2 validation

**B1. `WisdomEffortLiberationGate` constant — DRAFT VALUE PROPOSED.**

Class distribution from `Spawner.randomClass()` (verified in spawner.go:261-269):
- Devotionalist: 45% (~180K agents)
- Ritualist: 35% (~140K)
- Nihilist: 17% (~68K)
- Transcendentalist: 3% (~12K)

Live occupation distribution (pulled from API 2026-05-06 tick 4,746,176):
- Farmer 47.4%, Alchemist 13.6%, Fisher 9.4%, Soldier 6.3%, Scholar 5.9%, Laborer 5.9%, Merchant 5.6%, Crafter 3.0%, Hunter 1.7%, Miner 1.4%

Proposed gate value: **`phi.Totality × 1000 ≈ 4236`** practice ticks.

Hours of full eligibility for the gate to clear, by combination:
- Scholar × Transcendentalist (rarest combo, 0.18% pop ≈ 720 agents): `0.013 × 1.618 × 1.618 = 0.034/hour` → 4236 / 0.034 = **124,500 hours = 14.2 sim-years** ✓ achievable in a long contemplative life
- Hunter × Transcendentalist (~200 agents): `0.013 × 0.618 × 1.618 = 0.013/hour` → **325,800 hours = 37.2 sim-years** — possible for elders
- Merchant × Transcendentalist: same ~37 years
- Laborer × Transcendentalist (~700 agents): `0.013 × 0.236 × 1.618 = 0.0050/hour` → **847,200 hours = 96.7 sim-years** — effectively impossible
- Scholar × Devotionalist (~10K agents): `0.013 × 1.618 × 0.618 = 0.013/hour` → **325,800 hours = 37.2 sim-years** — possible for elder scholars
- Laborer × Nihilist (~3.9K agents): `0.013 × 0.236 × 0.236 = 0.00072/hour` → **5.86M hours = 670 sim-years** — impossible by design
- Hunter × Devotionalist: `0.013 × 0.618 × 0.618 = 0.0050/hour` → **96.7 sim-years** — only the most disciplined elder hunters

These ratios match the operator intuition.

**B2. Liberation distribution projection — DONE.**

Estimated liberated agents at steady state, assuming the world reaches a stable mortality regime and agents who reach the gate before death cumulate as liberated:

| Combination | Pop | Years to gate | % live to qualify | Expected liberated |
|---|---|---|---|---|
| Scholar × Transcendentalist | 720 | 14 | ~70% | ~500 |
| Alchemist × Transcendentalist | 1700 | 22 | ~50% | ~850 |
| Hunter × Transcendentalist | 200 | 37 | ~25% | ~50 |
| Merchant × Transcendentalist | 270 | 37 | ~25% | ~70 |
| Soldier × Transcendentalist | 290 | 60 | ~10% | ~30 |
| Fisher × Transcendentalist | 430 | 60 | ~10% | ~45 |
| Scholar × Devotionalist | 10,500 | 37 | ~25% | ~2,600 |
| Alchemist × Devotionalist | 24,500 | 60 | ~10% | ~2,450 |
| Hunter × Devotionalist | 3,000 | 97 | ~5% | ~150 |
| Other Transcendentalist + low-conducive | ~7,000 | 97-670 | <5% | ~200 |

Rough total: **~7,000 liberated at steady state ≈ 1.75% of population.** Within the 1-3% target band. ✓

Distribution by occupation in this projection: Scholars 44%, Alchemists 47%, Hunters 3%, Merchants 1%, others <5%. This is heavily Scholar/Alchemist biased, which matches operator intuition ("philosopher has higher chance"). Hunters are lower than I expected; their occupation weight (Matter ≈ 0.618) gets them practicing but their slower rate means few reach the gate before death. Could be tuned up if it feels wrong empirically.

**Note:** Devotionalist Scholars produce more liberated agents than Transcendentalist Scholars purely because Devotionalists are 15× more numerous. This is correct — most liberated agents will have arrived by faithful practice within their tradition (Devotionalist = practice within faith framework), not by Transcendentalist seeking. Fewer dramatic seekers, more "the quiet scholar who practiced their whole life and crossed at age 60." This is good philosophically.

**B3. Tier 2 contemplate prompt — DEFERRED to R89 implementation.**

**B4. ~~Schema migration honest vs grandfathered~~ — RESOLVED VIA SPLIT-FIELDS ARCHITECTURE.** The original concern (whether to retroactively un-liberate existing agents) is moot: the revised Layer 2 (see §3.2) splits the liberation criterion into `CittaCoherence ≥ 0.7 AND WisdomEffort ≥ gate`. Existing agents keep their `CittaCoherence`; their new `WisdomEffort` defaults to 0; the AND-condition automatically excludes them from the API definition without any code change to their stored state. **The transition is clean by definition.** The only operator-visible effect is that `/api/v1/liberated` returns 0 immediately on Layer 2 deploy and slowly climbs back to 1-3% over sim-years. A nice newspaper moment: "the age of practice begins."

### 4.3 Layer 3 validation

**C1. Pool decay rate — RECALIBRATED.**

Under Layers 1+2 with ~7,000 liberated agents (1.75% of population) at steady state, and assuming:
- Annual liberated death rate (most are old) ≈ 5-10% of liberated population = 350-700/year
- Liberated deaths at age ≥ 30 (the pool-eligibility floor) ≈ 95% of those = 330-665/year ≈ 6-13/sim-week to pool

At pool decay rate `0.988/week` (`1 - Agnosis × 0.05`), drain ≈ 0.012 × pool. Equilibrium pool size: inflow ÷ drain rate = ~10/0.012 = **~830 spirits at steady state.**

Birth rate at the cap ≈ 14/sim-day = 100/sim-week.

P(reincarnation per birth) = pool / (population × Φ^k):
- Φ⁵ = 11.09: P ≈ 830 / (400K × 11.09) ≈ 1/5340 → 100/5340 ≈ **0.019 reincarnations/week ≈ 1 per year** ✓ within target
- Φ⁶ = 17.94: P ≈ 1/8650 → 1 per 1.6 years
- Φ⁷ = 29.03: P ≈ 1/14000 → 1 per 2.6 years

**Recommendation:** use **Φ⁵** in the denominator. ~1 reincarnation per sim-year is the right cadence for a once-in-a-lifetime narrative event. Each reincarnation is rare enough to be newsworthy but frequent enough that any given year has a chance of one. **Resolved.**

**C2. Reincarnated child coherence cap — REVISED.** Proposed initialization range:

```go
initialCoherence := 0.7 + rng.NormFloat64() * float64(phi.Agnosis*0.1)  // mean 0.7, std ~0.024
initialCoherence = clamp32(float32(initialCoherence), 0.65, 0.85)
```

Range `[0.65, 0.85]` (revised from earlier `[0.65, 1.0]`) keeps reincarnated children near the threshold but with room to fall back through trauma. Combined with `WisdomEffort` seeded to `WisdomEffortLiberationGate + Agnosis × 1000 ≈ 4236 + 236 = 4472` (slightly above gate, modest cushion), they qualify for IsLiberated but not by an extreme margin. **Resolved.**

**C3. Reincarnated boolean field — RECOMMEND ADDING.** New field `Reincarnated bool` on Agent. Persisted to SQLite (one new column, simple migration). Visible in:
- Newspaper context (oracle prompts know they exist)
- `/api/v1/liberated` response (optional `reincarnated: true` field in liberatedAgent struct)
- Frontend UI: badge on the Liberated tab next to such agents

Story-mining justifies the column. **Resolved.**

### 4.4 Layer 4 validation

**D1.** Does Layer 4 emerge naturally from Layer 2 + existing migration? If Verdant-dominant peaceful settlements naturally retain agents with high `WisdomEffort` and these agents practice more often (from the settlement multiplier), do we even need explicit migration bias? Investigate by running Layers 1-3 in production for a sim-year and observing: do contemplative loci form on their own?

**D2.** `MonasterScore` formula tuning. Three components multiplied: average coherence, peace, VC dominance. Are all three necessary? VC dominance maybe redundant with average coherence (VC pulls coherence up via doctrine). Drop VC dominance from the formula? Empirical question.

### 4.5 Cross-layer

**X1.** Total flux balance. Sum all coherence inflows and outflows after the redesign. Is the world-average coherence stable? Currently Φ-derived and observed at ~0.74 across the population. Under the redesign:
- Layer 1 reduces inflow significantly
- Layer 2 adds a small inflow but only for active practitioners
- Trauma decay adds outflow

It's likely the world-average coherence drops. Target: stable around ~0.4 (Centered band). The 0.74 was always an artifact of the candy-machine — under the corrected model, the world is mostly Embodied/Awakening with rare Liberation. This is philosophically more honest.

**X2.** Frontend implications. Once Layer 2 ships, the Liberated tab will show ~5000-15000 agents instead of 221,683. Pagination (R87) handles it. The existing `min_age` param becomes a curiosity — possibly useful for "show only earned-liberation (age >= 30 helps but isn't precise; better is to filter by `WisdomEffort > 0` once we expose it via API).

**X3.** Phase 6 (emergence instrumentation) integration. The redesign creates rich emergence signals:
- Time series of liberation events per sim-week
- Time series of fall-back events per sim-week
- Per-occupation × per-class liberation rates
- Settlement-level monastery scores
These should feed into the E-1..E-10 heuristics for power-law detection on liberation event sizes (E-1) and possibly null-shuffle z-scores on settlement-level liberation distribution (E-3).

---

## 5. Open questions / decisions deferred

### Q1. ~~Honest vs. grandfathered Layer 2 migration~~ — RESOLVED via split-fields architecture (§3.2). No retroactive change to agent state needed; the AND-condition does it automatically.

### Q2. Should `WisdomEffort` decay? **Open.** Currently proposed as monotonic. But practice is a habit; long-lapsed practitioners arguably lose what they had. Possible decay at `−1 per sim-week` if no contemplation in last sim-week? Simulates "use it or lose it." Adds complexity. **Recommend ship monotonic first, add decay if Layer 2 produces "ratcheting" pathology** (agents who briefly practiced as adults and then never again counting as liberated forever).

### Q3. ~~Should reincarnated agents be visible as such?~~ — RESOLVED in §4.3 C3: yes, add `Reincarnated bool` field, surface in API.

### Q4. **Should faction doctrine compliance fill `WisdomEffort` instead of `CittaCoherence`?** Open. Under the revised Layer 2 architecture, doctrine boost still adds to `CittaCoherence` (in the Embodied → Awakening band, capped well below Liberation by sheer rate). But conceptually, doctrine compliance is a *form of practice* — a Crown agent's daily ritual of governance, an Ashen agent's daily reminders of impermanence. **Strong design argument for routing doctrine compliance into `WisdomEffort` instead.** This would make every faction a *path of practice*, with `ActionContemplate` being just one (more direct) route. Trade-off: makes `WisdomEffort` accumulate faster than projected in §4.2 B2; gates may need re-tuning.

**Recommend deferring this to after R88-R89 land.** Easier to tune one mechanism at a time. If post-R89 the Transcendentalist-Scholar bias is too strong (e.g. >70% of liberated are Scholars), routing doctrine compliance to WisdomEffort spreads liberation more evenly across factions. Ship straight first, then assess.

### Q5. Should there be an explicit `Tier3` "Sage" cognition tier?
**Open.** If a handful of agents reach genuine Liberation, they're qualitatively different from Tier 2 named characters. Should they get even richer LLM context? Tier 3 = the world's living philosophers, ~50-200 agents at any time, individually contextualized. This is post-R91 territory — only worth doing once the redesign settles and the cohort is stable.

### Q6. Disposition lock-in
**Open.** Class is currently set at agent creation and never changes. Shouldn't a Nihilist who slowly accumulates wisdom become a Devotionalist or Transcendentalist? Class-as-trajectory rather than class-as-essence. Maybe `WisdomEffort` thresholds promote across class boundaries. But this might be over-engineering. **Defer until R88-R91 settle.**

### Q7. **NEW:** Should the Awakening Valley still apply equally to all agents?
The current `ComputeAlignment` (`internal/agents/soul.go:108-135`) creates a "dark night" valley between Psyche (0.382) and 0.7 — alignment dips before climbing. Under the revised model where `WisdomEffort` is the actual practice mechanism, should the valley scale with practice? An agent who has practiced (`WisdomEffort > 0`) experiences the valley as productive grief. An agent whose coherence rose without practice (a child from cultural drift) experiences it as confusing despair.

If we want to encode this, ComputeAlignment could take WisdomEffort into account — dampening the valley for practitioners. But this adds complexity. **Defer to a future round (R92+) once the basic structure works.**

---

## 6. Operator decision points

After the analyses in §4 closed several questions, only a few remain.

### Before R88 commits:

1. **§3.1 cultural drift cut magnitude.** Proposed: 10× cut (`Agnosis × 0.005` → `Agnosis × 0.0005`) AND end-age narrowed (14-25 → 14-22). Is the 10× too aggressive? Would 5× be safer? Operator gut-check.

2. **§3.1 scholar work cut.** Proposed: 10× cut (`Agnosis × 0.000001` → `Agnosis × 0.0000001`). The original code comment promises scholars liberation in 3.7 years — that promise is now explicitly broken. **Worth aligning with operator that this design intent is rescinded.**

3. **§3.1 doctrine boost cut.** Proposed: 4× cut (`Agnosis² × 0.1` → `Agnosis² × 0.025`) plus age gate. Smaller cut than original draft because the audit revealed doctrine wasn't the dominant inflow.

4. **None of these are deal-breakers.** All can be tuned post-deploy if Layer 1's effect doesn't match the projection.

### Before R89 commits:

5. **§4.2 B1**: `WisdomEffortLiberationGate` value. Recommend `phi.Totality × 1000 ≈ 4236`. Math under proposed weights produces ~7,000 liberated at steady state (1.75% of population) within target band.

6. **§4.2 B3**: Tier 2 contemplate prompt language. Draft when R89 implementation begins.

7. **Q4**: Should faction doctrine compliance route to `WisdomEffort` instead of `CittaCoherence`? **Recommend deferring this question** — ship Layer 2 with doctrine-on-coherence as currently designed, observe distribution, revisit if Transcendentalist-Scholar bias is too dominant.

### Before R90 commits:

8. All §4.3 sub-points are now resolved. **No remaining operator decisions before R90.**

### Before R91 commits:

9. **§4.4 D1**: Run Layers 1-3 in production for at least one sim-year before deciding whether Layer 4 (monastic settlements) is needed. May emerge naturally from existing migration code + Layer 2's settlement-level conducive multiplier.

---

## 7. Cold re-entry checklist

If a future session picks this up cold (no conversation context), here's the minimum context needed:

### What's the goal?
Read §1 and §2. Liberation is not currently a journey; it's an accumulator. The goal is to make it a journey.

### What's been decided?
Layers 1-4 architecture in §3 is the proposal. Magnitudes are tentative. **Nothing has been committed to code yet — this is design only.**

### What's blocked on?
The validation analyses in §4 should be done before each layer commits. The operator decisions in §6 should be answered before R88 commits.

### What's the empirical baseline?
At this doc's start (2026-05-06): 221,683 liberated (55.5%), including children at age 8-10. The audit of why is in §1.2 and §1.3. Re-pull `/api/v1/liberated` for the current state when resuming.

### What's in-flight?
Pagination (R87) shipped 2026-05-06 but did not deploy yet (waiting on TickWeek boundary). Once R87 is live, the Liberated tab will be performant enough to do detailed inspection of the cohort.

### Where does this fit in the broader roadmap?
Improvement Candidate #11 in `docs/ROADMAP.md` (platform repo). The R88-R91 sequence is the resolution path for #11.

### Which existing rounds are most relevant?
- R10 (dual-register wellbeing) — establishes the wellbeing trinity
- R50 (faction doctrines) — fix #222, the source of the candy-machine
- R53 (coherence rebalance) — fixes #229-231, current via-negativa and via-positiva paths
- R76 (persistence registry) — `WisdomEffort` and `LiberatedSpiritsPool` will use it
- R87 (Liberated pagination) — UX prerequisite for observing the redesign's effect

### What are the design principles I cannot violate?
The four in §2: achievement-not-stage, precarious, conducive-conditions-correlate, narrative-continuity.

### What's the single biggest risk?
Layer 2's `wisdomEffortLiberationGate` constant. Too high → no one ever achieves liberation, the threshold is decorative. Too low → the candy machine returns under a new name. Validation in §4.2 B1-B2 must be done with care.

---

## 8. Document update history

| Date | Author | Change |
|---|---|---|
| 2026-05-06 | Operator + Claude | Initial draft. All four layers laid out. Validation analyses listed. Nothing committed. |
| 2026-05-06 | Claude (validation pass) | Ran the validation analyses §4. Major findings: (1) audit was incomplete — 14 inflow paths exist, not 3. Cultural drift is the actual primary candy machine, not doctrine boost. Scholar work explicitly designed to liberate in 3.7 years per code comment. (2) Layer 1 revised to address all 7 population-significant inflow paths, not just doctrine boost. (3) Layer 2 architecture changed from clamp-cap-on-CittaCoherence to **split fields** (`CittaCoherence` + `WisdomEffort`) with AND-condition for liberation. Migration becomes automatic and clean — no retroactive change to agent state. Original "honest vs. grandfathered" question (Q1) made obsolete. (4) `WisdomEffortLiberationGate` proposed at `phi.Totality × 1000 ≈ 4236`; projection produces ~7,000 liberated at steady state (1.75% of population) within target band. (5) Pool decay rate (Layer 3 §4.3 C1) recalibrated; recommend `Φ⁵` denominator → ~1 reincarnation per sim-year. (6) Tier 1 currently has 1 agent in production — archetype growth pipeline mostly dormant, but architecture must still be right for when it populates. (7) Magnitude correction in original §1.2: liberation-death ripple is `Agnosis × 0.05 × c` (code), not `Agnosis × 0.1 × c` (original draft). Several operator decisions resolved (Q1, Q3, all of §4.3). New question added: Q7 (should the Awakening valley scale with WisdomEffort?). |

(Update this section as the doc evolves through future sessions.)

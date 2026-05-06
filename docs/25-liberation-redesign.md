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

### 1.2 Root cause (audit complete 2026-05-06)

`CittaCoherence` is a single scalar that responds to these inputs:

| Mechanism | Where | Direction | Magnitude per agent |
|---|---|---|---|
| Genesis spawn | `internal/agents/spawner.go:237-251` (`generateSoul`) | seed | Normal(`Agnosis ≈ 0.236`, `Agnosis × 0.5`); clamped to `[0.01, Matter ≈ 0.618]`. **Never ≥ 0.7 at genesis.** |
| Birth (newborn) | `internal/agents/spawner.go:325-330` (`SpawnChild`) | seed | base ~0.236 + jitter + `parent.Soul.CittaCoherence × Agnosis`; clamps at 1.0. **Can reach 1.0 from a single liberated parent under specific RNG.** |
| Faction doctrine boost | `internal/engine/factions.go:680-738` (`applyFactionDoctrines`, R50 fix #222) | **+** | `+Agnosis² × 0.1 ≈ +0.00557/sim-week`, applied to every faction member who fulfills doctrine. **No age gate.** |
| Baseline coherence drift | `internal/engine/simulation.go:982` (`processBaselineCoherence`, R53 fix #231) | **+** | Daily nudge for `Satisfaction > Matter && Age > 20`. Already age-gated, but tiny relative to doctrine boost. |
| Witness ordinary death | `internal/engine/population.go:230` (R53 fix #230) | **+** | `+Agnosis × 0.05 × (1 - 0.5×c)`. Per witness, per witnessed death. |
| Witness liberation death | `internal/engine/population.go:200` (R53 fix #229) | **−** | `-Agnosis × 0.1 × c`. Only fires when a liberated agent dies. Witness coherence-scaled. |
| Soft clamp | `internal/agents/soul.go:92-98` | bound | `[0.0, 1.0]` |

**The doctrine boost dominates.** At `+0.00557/week`, an agent born at `c=0.236` reaches `c=1.0` in `~140 sim-weeks ≈ 2.7 sim-years`. For an 8-year-old in Crown or Ashen Path (factions whose doctrines are settlement-level conditions or default-true for newborns), the saturation has already fired multiple times by the age the operator observed.

**Crown doctrine** (`agentFulfillsDoctrine`, factions.go:748): `settlement.GovernanceScore > Psyche (~0.382)`. This is a settlement-level state — any agent of any age in any well-governed Crown settlement qualifies.

**Ashen Path doctrine**: `wealth < 30 OR belonging > Matter (~0.618)`. Newborns have `Wealth=0` (qualifies on first clause) AND `Belonging=0.8` from spawner default (qualifies on second). They simultaneously qualify on both criteria from minute zero of life.

**Faction assignment**: from R32 fix #106 (`addAgent` in `population.go`), every newborn is assigned a faction at birth — not at majority. So newborns immediately enter the doctrine-boost loop.

The conclusion is unambiguous: **the current model has no concept of liberation as practice or journey; it's a passive accumulator that saturates from circumstance.**

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

**Goal:** drop the liberated proportion from 55% to 10-20% by fixing the unintended inflow paths. No new mechanics — just constants and conditions in existing files.

**Files touched:**
- `internal/engine/factions.go` — `applyFactionDoctrines`
- `internal/engine/population.go` — universal trauma decay path

**Changes:**

1. **Doctrine boost: 10× cut and age-gated.**
   - Change magnitude: `Agnosis² × 0.1 ≈ +0.00557/week` → `Agnosis³ × 0.1 ≈ +0.00131/week` (~Φ⁻⁵ scale, lower than `Matter × Agnosis × 0.005`).
   - Add age gate: `if a.Age < 16 { continue }` at the top of the doctrine eligibility check.
   - **New saturation time:** ~600 weeks = ~10 sim-years to climb from c=0.236 to c=1.0 at full compliance. A faithful Crown adult who lives to 60 might gain ~0.4 coherence from doctrine alone — meaningful but not saturating, and only if they live long under stable governance.

2. **Universal trauma decay**, not just sage-death.
   - New helper in `population.go`: `applyTraumaDecay(a *Agent, intensity float32)`.
   - Triggered by: warfare casualties witnessed, plague survival, settlement abandonment, severe famine (Survival < Agnosis sustained), being a victim of theft.
   - Magnitude: `−Agnosis × 0.05 × intensity` for ordinary witnesses. Liberated agents (`c ≥ 0.7`) take **double**: `−Agnosis × 0.1 × intensity`. This is the extraction paradox already documented in `soul.go`.
   - Bounded so that no single trauma can drop coherence by more than `Agnosis ≈ 0.236`.

3. **Reduce the witness-ordinary-death gain.**
   - Current: `+Agnosis × 0.05 × (1 - 0.5c)` per witnessed death — fires for any death with witnesses above some sentiment threshold.
   - New: `+Agnosis³ × 0.05 × (1 - 0.5c)` — five orders of magnitude smaller per event. Plus: gate to "important deaths" only (witness sentiment ≥ Matter) so noise is suppressed.
   - This stays as a small via-negativa nudge (the loss of someone meaningful makes you reflect) but doesn't accumulate to liberation on its own.

**Constants summary (Layer 1):**

```go
// internal/engine/factions.go
const doctrineBoostL1 = phi.Agnosis * phi.Agnosis * phi.Agnosis * 0.1 // ≈ 0.00131/week
const doctrineMinAge = 16

// internal/engine/population.go
const traumaDecayBase = phi.Agnosis * 0.05    // ordinary witness
const traumaDecayLib  = phi.Agnosis * 0.1     // liberated witness (extraction paradox)
const traumaDecayCap  = phi.Agnosis           // single-event cap

// witnessOrdinaryDeathGain reduced from Agnosis*0.05 to Agnosis³*0.05
```

**Expected outcome:**
- Within 2-4 sim-weeks: liberated proportion drifts down as new births don't accumulate, and existing liberated agents' coherence doesn't grow further.
- Within 2-4 sim-months: trauma decay starts pulling the high tail down. The 55% number drops toward 30-40%.
- Within sim-year: equilibrium near 10-20%. Still too high relative to "rare," but no longer broken.

**Why ship Layer 1 alone first:** validates that inflow paths are the dominant cause, observable in days, doesn't require schema migration. If the drop happens, Layer 2 is justified. If it doesn't, the audit was wrong and we re-investigate.

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

**The hard gate:**

```go
// In agents/soul.go AdjustCoherence — modify the existing function
func (s *AgentSoul) AdjustCoherence(delta float32) {
    s.CittaCoherence += delta
    if s.CittaCoherence < 0 {
        s.CittaCoherence = 0
    }
    // NEW: cannot rise above Matter without sufficient WisdomEffort
    cap := float32(1.0)
    if s.WisdomEffort < wisdomEffortLiberationGate { // tunable; proposal: phi.Totality * 1000 ≈ 4236
        cap = float32(phi.Matter)
    }
    if s.CittaCoherence > cap {
        s.CittaCoherence = cap
    }
}
```

The `wisdomEffortLiberationGate` constant is the structural truth: **only deliberate practice can bridge the Awakening valley.** With ~4236 successful practice ticks needed, even a Scholar/Transcendentalist needs years of uninterrupted practice to qualify, and any of: Survival drop, Safety drop, Belonging drop, age regression (impossible) interrupts the accumulation.

**Tier 2 LLM agents** (`internal/llm/cognition.go`):
- Add `"contemplate"` to the action vocabulary.
- The Tier 2 system prompt should describe what contemplation is and when it's appropriate. Transcendentalists with high coherence will naturally choose it; Nihilists never will.
- The action is *expensive* in terms of an agent's daily action budget — it replaces work or socialize for that slot. This is the time-cost made explicit.

**Schema migration:**

Add to `internal/persistence/db.go` migrations array:
```go
"ALTER TABLE agents ADD COLUMN wisdom_effort INTEGER NOT NULL DEFAULT 0",
```

Wire into `SaveAgents` UPSERT and `LoadAgents` SELECT (R55 pattern). The R76 registry doesn't need a new entry — `WisdomEffort` is per-agent state, it lives on the agents table.

**Expected outcome:**
- Combined with Layer 1: liberated proportion drops to 1-5% steady state.
- Distribution by occupation: Scholar 5-10× average, Alchemist 3-5×, Hunter 2×, Merchant 1.5×, Laborer/Miner ≤0.3×.
- Distribution by class: Transcendentalist 5-10× average, Devotionalist 1×, Ritualist 0.4×, Nihilist <0.1×.
- Median age of liberated agent climbs to 50+.
- Liberation events become newsworthy. Each one has a story.

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

**A1.** Confirm the `Agnosis³ × 0.1` doctrine boost magnitude is right. The naïve calculation: at +0.00131/week, an adult Crown agent in a stable settlement gains ~0.068/year. From `c=0.236` at age 16 to `c=1.0` requires ~11 years of full compliance — meaningful but achievable for the most disciplined adult. Question: is 11 years too short? Should it be `Agnosis⁴ × 0.1 ≈ 0.000310/week`, which would be 50 years of full compliance? **Sanity check needed before commit.**

**A2.** Estimate the steady-state trauma-decay flux. World currently sees ~80 deaths/week, 19 raids/week, occasional plague spreads. At `−Agnosis × 0.05 × intensity` per witness per event, with rough witness counts per event, what's the per-agent expected coherence loss per year? It should be small for low-trauma agents (≤ 0.05/year) but meaningful for those in high-conflict settlements (≥ 0.2/year). Sanity check: don't accidentally produce negative coherence after one bad season.

**A3.** Identify all current code paths that increment coherence. Are there any I missed in §1.2? Search for `CittaCoherence +=`, `AdjustCoherence(`, etc. across the entire engine package. Re-audit before committing.

**A4.** Decide on age gate value. 16 is the existing adulthood threshold (used in `PromoteToTier2`, `processBirths`, `processBaselineCoherence` indirectly). Stick with it for consistency? Or use 20 (more conservative)? Stick with 16 unless there's a reason.

### 4.2 Layer 2 validation

**B1.** The `wisdomEffortLiberationGate` constant. Proposed value: `phi.Totality × 1000 ≈ 4236` ticks of successful practice. With base practice probability 0.013 and Scholar/Transcendentalist multiplier 2.62, that's `0.034/hour × 4236 / 8760 ≈ 16.4 sim-years` of full eligibility for the gate to clear. That's right for a Scholar/Transcendentalist; need to check it's not effectively impossible for Hunter/Devotionalist (would need ~50 sim-years, plausible for old hunters). Need to dry-run against the world's actual occupation × class distribution.

**B2.** Liberation distribution by demographic. Run the math against the live world's occupation/class distribution. With ~6000 Hunters and the class distribution per current `factionForAgent`, expected liberated count by occupation × class should match operator intuition (Scholars dominate, Laborers near-zero). If the math says 95% of liberated agents are Scholars, the multipliers may need rebalancing.

**B3.** Tier 2 contemplate prompt. The Tier 2 LLM needs prompt text describing what `contemplate` means and when it's appropriate. Draft this. The prompt should NOT bias agents toward it — they should choose it based on their disposition and ground.

**B4.** Schema migration safety. Adding `wisdom_effort INTEGER NOT NULL DEFAULT 0` is safe (backwards compatible); existing agents start with 0, which is correct (they haven't practiced yet). On first deploy, every existing agent's coherence may drop back to `Matter` cap if they're currently above 0.618 with WisdomEffort=0. **This is a behavior change visible in production.** The 221K liberated agents would lose liberation status until they accumulated practice.

This is actually correct philosophically — they were liberated by accident, they should have to earn it. But it's a sharp transition. Discuss whether to do an opt-in migration: existing liberated agents get `WisdomEffort = wisdomEffortLiberationGate` on migration so they keep their status, but new accumulation requires practice. Trade-off:
- **Honest transition** (everyone's coherence caps at Matter until they practice): the world sees Liberated counts drop overnight, then climb back over years as practice accumulates.
- **Grandfathered transition** (existing liberated keep status, only new agents are subject): the existing 55% stays liberated; only newborns are subject to the new rules.

I lean toward **honest transition** — the simulation has an opportunity here to let the world's structural truth emerge. But this is operator-level decision, not engineering decision.

### 4.3 Layer 3 validation

**C1.** Pool decay rate. Proposed `0.988/sim-week`. At equilibrium between liberated death rate and pool decay:
- Liberated death rate today: ~80 deaths/week × 55% liberated × small fraction at age >= 30 ≈ ~40/week deaths to pool
- BUT — under Layers 1+2, liberated population drops to 1-5%, so deaths to pool ≈ ~3-5/week
- Pool decay 0.012/week × pool = drain rate → equilibrium pool size ~250-400 (under new lib %)
- Reincarnation P = pool / (400000 × 11.09) ≈ 250/4.4M ≈ 1 in 17,600 → with ~14 births/sim-day at world birth-cap, that's roughly one reincarnation every ~3 sim-days.

That's TOO MANY. Need `Φ⁶` or higher in the denominator. Run the math more carefully and tune. Target: one reincarnation every ~6-12 sim-months.

**C2.** Should the reincarnated child's coherence be capped on birth? Proposed `[0.65, 1.0]` is generous. Should be `[0.7, 0.85]` to keep them just-at-threshold, leaving room for them to fall back from trauma.

**C3.** Identifying reincarnated agents. Add `reincarnated bool` field on Agent? This is operator-visible UX as well — the Liberated tab could highlight reincarnated children for easy story-finding. Adds a column to schema; small but persistent change.

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

These are not yet answered. Each is operator-level:

### Q1. Honest vs. grandfathered Layer 2 migration
See §4.2 B4. Resolution affects whether the world wakes up post-deploy with 55% liberated still (grandfathered) or with most coherence pushed back to Matter (honest). Recommend: honest, with a special "sage retired" event emitted for each agent whose coherence drops by > 0.1 in the migration. Lets the newspaper write about it. But this is operator's call.

### Q2. Should `WisdomEffort` decay?
Currently proposed as monotonic. But practice is a habit; long-lapsed practitioners arguably lose what they had. Decay at `−Agnosis × 0.001/week` if no contemplation in last sim-week? Simulates "use it or lose it." Adds complexity but feels right philosophically. Unresolved.

### Q3. Should reincarnated agents be visible as such?
See §4.3 C3. Adds a boolean column. UX: liberated tab could highlight, agent biographies could mention. Adds richness but also risk of "marking" certain agents in a way that feels deterministic.

### Q4. What does the doctrine boost actually mean now?
If `Agnosis³ × 0.1` is the new magnitude, doctrine boost is ~0.07/sim-year per faithful adult. That's a lifetime contribution of ~5 coherence-units across 70 years — clamped at Matter (0.618) per Layer 2's hard gate. So doctrine boost contributes to Embodied → Awakening transition but not to Liberation. **Is this what we want?** Maybe doctrine compliance should fill `WisdomEffort` instead of `CittaCoherence` directly — making faction membership a *path of practice* (Crown's "Order is practice," Ashen's "Dissolution is practice") rather than a coherence accumulator. **Strongly worth considering.**

### Q5. Should there be an explicit `Tier3` "Sage" cognition tier?
If a handful of agents reach genuine Liberation, they're qualitatively different from Tier 2 named characters. Should they get even richer LLM context? This would be a separate round (R92+) but worth flagging now — Liberation in the redesign is rare enough that giving each their own cognition tier becomes affordable. Tier 3 = the world's living philosophers, ~50-200 agents at any time, individually contextualized.

### Q6. Disposition lock-in
Class is currently set at agent creation and never changes. But shouldn't a Nihilist who slowly accumulates wisdom become a Devotionalist or Transcendentalist? Class-as-trajectory rather than class-as-essence. Maybe `WisdomEffort` thresholds promote across class boundaries. But this might be over-engineering.

---

## 6. Operator decision points

Before R88 commits, the operator needs to decide:

1. **§4.1 A1**: Doctrine boost magnitude. `Agnosis³ × 0.1` vs `Agnosis⁴ × 0.1`. Is liberation an 11-year compliance arc or a 50-year one for the disciplined?
2. **§4.2 B4 / Q1**: Honest vs. grandfathered Layer 2 migration. Big philosophical question.
3. **Q4**: Should doctrine compliance fill `WisdomEffort` rather than `CittaCoherence`? Architectural cleanup if yes.

Before R89 commits:
4. **§4.2 B1**: `wisdomEffortLiberationGate` value. Proposed 4236; verify against world distribution.
5. **§4.2 B3**: Tier 2 contemplate prompt language. Draft and review.

Before R90 commits:
6. **§4.3 C1**: Pool decay rate. Proposed 0.988/week needs recalibration with Layer 1+2 lib%.
7. **Q3**: Reincarnated agent visibility. Add `reincarnated bool` field?

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

(Update this section as the doc evolves through future sessions.)

# Proposal: Replacing Mood with a Dual-Register Wellbeing Model — IMPLEMENTED

> **Status:** Implemented. See `docs/summaries/2026-02-22-dual-register-wellbeing.md` for implementation details.

## SYNTHESIS Mood System Revision — Aligning Agent Satisfaction with Liberation Ontology

---

## 1. Diagnosis: Why High-Coherence Fishermen Are Miserable

The current `mood` field (float, -1.0 to 1.0) is a **weighted average of phenomenal need satisfaction**:

```
mood ≈ f(Survival, Safety, Belonging, Esteem, Purpose)
```

Each need is replenished by material actions: eating boosts survival, working boosts safety/esteem, socializing boosts belonging, producing boosts purpose. The mood value then drives critical behavioral gates — migration thresholds, reproduction eligibility, and the external-facing "happiness" metric.

The problem is structural, not parametric:

- **Fishermen** live on coast hexes where fish often deflate in value (oversupply, low demand), fish production still aliases through `Skills.Farming` rather than a dedicated skill, and coastal settlements tend to be smaller (lower belonging replenishment). Their material conditions are objectively harder.
- **High-coherence fishermen** (some at coherence = 1.0, Liberation state) have transcended attachment to material phenomena — which is the *entire point* of the Wheeler framework — but the mood formula doesn't know this. It sees: low wealth → low safety, small settlement → low belonging, marginal trade → low esteem. Result: mood = -0.30.
- **The design doc anticipated this** with the "extraction paradox" note: *"High-coherence agents experience MORE internal tension than low-coherence ones... mood should be partially inverse to coherence in many situations."* But what's implemented is something worse than the extraction paradox — coherence simply has **zero influence** on mood. The liberated fisherman and the scattered fisherman in the same settlement have the same mood.

This creates an inversion of the world's entire ontological claim. The world says liberation is the highest good. The mood system says wealth is.

---

## 2. The Philosophical Ground: What Should "Wellbeing" Mean?

Wheeler's framework gives us a clear answer, but it requires distinguishing two things that the current system conflates.

### 2.1 Phenomenal Satisfaction (the body's report)

This is the domain of the current mood formula. Are you fed? Are you safe? Do you belong to a community? Do others respect you? Do you have purpose? This maps cleanly to the Maslow hierarchy and is legitimately important — a starving agent *should* feel distress regardless of coherence. The body has needs; ignoring them kills you.

In Wheeler's terms, this is the domain of **consciousness** (the broadcast from the radio) — the subjective experience of being an embodied being navigating material phenomena. It's real, it matters, and it determines survival behavior.

### 2.2 Ontological Alignment (the citta's state)

This is the domain that's currently unmeasured. How coherent is the citta? How self-similar? How free from false identification with phenomena? This is the *deeper* wellbeing — the kind that Wheeler's tradition says is the only wellbeing that matters ultimately.

The key insight from the source texts: **a liberated being doesn't stop experiencing material hardship. They stop being *identified with* it.** The fisherman's body is hungry, but the fisherman's citta is at rest. The hunger is a fact of the body; it is not a fact of the self. The liberated fisherman experiences hunger without despair. The scattered fisherman experiences hunger as existential crisis.

From the Theurgy of Liberation text: *"Liberation itself is quite literally like this [point-source coherent light]. Objective negation leads to Subjective synthesis."*

### 2.3 The Two Registers in Practice

This maps to a well-known pattern in contemplative traditions: the monk who owns nothing and is at peace vs. the merchant who owns everything and is in anguish. The monk's phenomenal conditions are poor; his ontological alignment is high. The merchant's phenomenal conditions are excellent; his ontological alignment may be nil.

Neither register is "fake." Both are real. The question is: **which one should drive agent behavior, and which one should we report as "wellbeing"?**

---

## 3. Proposed Architecture: Dual-Register Wellbeing

Replace the single `mood` float with two orthogonal registers and a composite that blends them based on coherence.

### 3.1 Data Model

```go
type WellbeingState struct {
    // Register 1: Phenomenal Satisfaction
    // How well are material/social needs met?
    // Driven by: food, safety, belonging, esteem, purpose
    // Range: -1.0 to 1.0
    Satisfaction float64

    // Register 2: Ontological Alignment  
    // How at-rest is the citta?
    // Driven by: coherence, wisdom, attachment count
    // Range: 0.0 to 1.0
    Alignment float64

    // Composite: Effective Mood
    // What the agent "feels" and acts on
    // Blended from both registers, weighted by coherence
    // Range: -1.0 to 1.0
    EffectiveMood float64
}
```

### 3.2 Computing Satisfaction (unchanged logic, renamed)

The current mood formula becomes `Satisfaction`. No changes to how needs are computed:

```go
func computeSatisfaction(a *Agent) float64 {
    n := a.Needs
    // Existing weighted average of need fulfillment
    sat := (n.Survival*5 + n.Safety*4 + n.Belonging*3 + n.Esteem*2 + n.Purpose*1) / 15.0
    return clamp(sat*2.0 - 1.0, -1.0, 1.0) // scale to [-1, 1]
}
```

This stays exactly as-is. Material needs still matter. Starvation still hurts. The economy still functions.

### 3.3 Computing Alignment (new)

Alignment is derived from coherence, but is NOT simply equal to coherence. It represents the citta's *experiential state* — how much peace arises from coherence. This introduces a threshold effect and the extraction paradox:

```go
func computeAlignment(a *Agent) float64 {
    c := a.Soul.Coherence
    w := a.Soul.Wisdom

    // Phase 1: Embodied (0.0 - 0.3 coherence)
    // Low coherence → low alignment, but not suffering from it
    // The scattered don't know what they're missing
    // Alignment tracks coherence roughly 1:1
    if c < phi.Psyche { // ~0.382 threshold
        return c * phi.Matter // gentle slope, max ~0.236
    }

    // Phase 2: Awakening (0.3 - 0.7 coherence) — THE EXTRACTION PARADOX
    // This is where it gets worse before it gets better
    // The agent sees clearly enough to perceive suffering but hasn't
    // transcended it. The "dark night of the soul."
    // Alignment DIPS relative to coherence here.
    if c < phi.Matter + phi.Psyche { // ~0.618 + 0.382 = below 1.0 but conceptually ~0.7
        // Valley: alignment is suppressed by the burden of seeing
        valley := phi.Agnosis * (1.0 - math.Abs(c - 0.5) * 4.0) // peaks at c=0.5
        return c*phi.Matter - valley                               // net: lower than raw coherence
    }

    // Phase 3: Liberation (0.7 - 1.0 coherence)
    // The citta has passed through the extraction paradox
    // Alignment rises steeply and approaches 1.0
    // Wisdom amplifies the effect
    wisdomBonus := w * phi.Agnosis // small but meaningful
    return phi.Matter + (c - phi.Matter) * phi.Being + wisdomBonus // approaches ~1.0
}
```

The key insight here: the alignment curve is NOT linear with coherence. It has a **valley** in the middle ranges (the extraction paradox / dark night of the soul) and a steep rise at high coherence. This means:

- Scattered agents (Embodied): low alignment, but also low awareness of it — no crisis
- Mid-range agents (Awakening): alignment actually *dips* — they see the world's corruption but can't transcend it
- Liberated agents: alignment surges — they've passed through the fire

### 3.4 Computing Effective Mood (the blend)

This is where the two registers merge into what the agent actually "experiences" and acts upon:

```go
func computeEffectiveMood(a *Agent) float64 {
    sat := a.Wellbeing.Satisfaction
    align := a.Wellbeing.Alignment
    c := a.Soul.Coherence

    // The blend weight: how much does ontological alignment
    // override phenomenal dissatisfaction?
    //
    // At low coherence: almost entirely satisfaction-driven
    //   (the body's suffering IS the self's suffering)
    // At high coherence: alignment dominates
    //   (the body suffers, but the self is at rest)
    //
    // Weight derived from Φ: the "matter/form" ratio applies here
    // Low-coherence agents are dominated by matter (material needs)
    // High-coherence agents are dominated by form (ontological state)

    alignWeight := c * c * phi.Matter  // quadratic with coherence, scaled by Φ⁻¹
    satWeight := 1.0 - alignWeight

    // Alignment contributes positive mood from 0.0-1.0 mapped to mood range
    alignMood := align*2.0 - 1.0 // map [0,1] to [-1,1]

    effective := satWeight*sat + alignWeight*alignMood

    return clamp(effective, -1.0, 1.0)
}
```

What this produces for our fishermen:

| Agent | Coherence | Satisfaction | Alignment | Align Weight | Effective Mood |
|-------|-----------|-------------|-----------|-------------|----------------|
| Scattered Fisher | 0.15 | -0.30 | 0.06 | ~0.014 | **-0.29** (material dominates) |
| Awakening Fisher | 0.50 | -0.30 | 0.18 | ~0.154 | **-0.35** (extraction paradox!) |
| Liberated Fisher | 0.90 | -0.30 | 0.89 | ~0.500 | **+0.24** (alignment rescues) |
| Liberated Fisher | 1.00 | -0.30 | 0.97 | ~0.618 | **+0.42** (at peace despite poverty) |

Notice the extraction paradox at coherence 0.50 — mood actually gets *worse* than the scattered agent. This is philosophically correct: the awakening agent suffers more, not less, because they can see.

### 3.5 The Φ-Derived Weight Curve

The `alignWeight = c² × Φ⁻¹` curve has these properties:

- At c=0.0: weight = 0.0 (pure satisfaction)
- At c=0.5: weight = 0.154 (still ~85% satisfaction-driven)
- At c=0.7: weight = 0.303 (turning point — ~70/30 split)
- At c=1.0: weight = 0.618 (Φ⁻¹ itself — matter/form golden ratio)

Even at maximum coherence (c=1.0), satisfaction still contributes 38.2% — because the body's needs never fully vanish. A liberated agent who is literally starving to death will still feel distress. But a liberated agent who is merely poor will feel at peace. This is the correct behavior.

---

## 4. Behavioral Integration: What Uses Which Register?

Currently `mood` is used for several behavioral gates. Each one should use the appropriate register:

### 4.1 Migration (use Effective Mood)

```go
// Agents migrate based on overall experience
// A liberated agent in a poor settlement stays — they're at peace
// A scattered agent in the same settlement leaves — they're miserable
if a.Wellbeing.EffectiveMood < migrationThreshold { ... }
```

This naturally produces the emergent behavior that liberated agents become "anchors" for struggling settlements — they stay when others leave, providing the wisdom/stability that raises settlement coherence over time.

### 4.2 Reproduction (use Satisfaction, gated by Belonging)

```go
// Reproduction is a material/biological act
// Coherence doesn't make you want children — material stability does
// Keep the existing gates on Belonging + Satisfaction
if a.Needs.Belonging > 0.3 && a.Wellbeing.Satisfaction > -0.2 { ... }
```

Using Satisfaction (not Effective Mood) for births prevents a perverse outcome where highly coherent agents who are materially destitute reproduce. Wheeler's framework would say this is correct — liberation doesn't mean reckless material behavior.

### 4.3 Work/Behavior Decision Tree (use Satisfaction for material needs, Effective Mood for social)

Material behavior (eat, forage, buy food, work) should still be driven by the needs themselves, not by mood. An agent with high coherence who is hungry still needs to eat. The behavior tree should remain needs-driven for survival/safety, but social decisions (socialize vs. work, crime, faction participation) can use Effective Mood.

### 4.4 External Reporting (expose both + composite)

The API should return all three values:

```json
{
    "mood": 0.24,           // effective mood (composite) — the headline number
    "satisfaction": -0.30,  // phenomenal satisfaction — material conditions
    "alignment": 0.89,      // ontological alignment — citta state
    "coherence": 0.90,      // raw coherence (already exposed)
    "state_of_being": "Liberation"
}
```

The newspaper generator and biography generator get richer material: "Despite meager material conditions, Jasper Thatcher maintains an inner tranquility that draws others to his quiet counsel."

---

## 5. Why This Keeps the World Alive

The principal risk you've been navigating is that economic reforms cause agents to die off, stop reproducing, or collapse into degenerate states. This proposal is specifically designed to be **stabilizing**, not destabilizing:

### 5.1 Economy Remains the Engine

Nothing about the economic system changes. Agents still need food, still trade, still produce, still participate in markets. The Satisfaction register runs on exactly the same math as the current mood. The economy's role as the "heartbeat" is preserved.

### 5.2 Birth Rates Stay Material

By tying reproduction to Satisfaction (not Effective Mood), we ensure that material economic conditions still determine population growth. The economy can't be "bypassed" by coherence.

### 5.3 High-Coherence Agents Become Settlement Anchors

This is the emergent benefit: liberated agents no longer flee poor settlements (because their Effective Mood is positive). They stay, and their presence stabilizes the settlement. This creates a natural counter-pressure to the "death spiral" pattern where mood drops → agents leave → settlement weakens → mood drops further.

### 5.4 The Extraction Paradox Creates Natural Drama

The mid-coherence "valley" means that agents on the path to liberation go through a visible crisis — their mood *drops* before it rises. This is narratively rich and philosophically accurate. The newspaper can report on agents going through dark nights of the soul.

### 5.5 No New Economy Required

This is purely a mood/wellbeing refactor. No new goods, no new currencies, no new market mechanics. The only files that need to change are the mood computation logic and the systems that consume the mood value.

---

## 6. Implementation Plan

### Phase 1: Structural Refactor (behavior.go, types.go)

1. Add `WellbeingState` struct to agent types
2. Replace `Mood float64` with `Wellbeing WellbeingState`
3. Implement `computeSatisfaction()` (rename current mood calc)
4. Implement `computeAlignment()`
5. Implement `computeEffectiveMood()`
6. Call all three in the daily tick where mood currently updates
7. Wire `EffectiveMood` to all existing mood consumers (migration, etc.)
8. Wire `Satisfaction` to reproduction gates

### Phase 2: API + Reporting (api.go, gardener)

1. Expose `satisfaction`, `alignment`, and `mood` (effective) in agent endpoints
2. Update `/api/v1/stats` to track avg alignment alongside avg mood
3. Update newspaper/biography generators to use both registers for richer narration

### Phase 3: Tuning + Observation

1. Deploy and observe for several sim-days
2. Monitor: avg effective mood by occupation, by coherence band, by settlement size
3. Adjust the alignment curve and blend weight if the extraction paradox is too harsh or too gentle
4. Verify births, migration, and settlement consolidation still work correctly

### Phase 4: Refinement (optional)

1. **Coherence contagion**: Liberated agents who stay in settlements could slowly raise the coherence of nearby agents (the mentor encounter / wisdom transmission mechanic from the design doc)
2. **Alignment events**: Specific events (loss accepted, attachment released) that update alignment directly
3. **Settlement alignment score**: Average alignment of agents in a settlement becomes a settlement-level metric that attracts certain agent classes

---

## 7. Files to Change

| File | Change |
|------|--------|
| `internal/agents/types.go` | Add `WellbeingState` struct, replace `Mood float64` |
| `internal/agents/behavior.go` | Implement three compute functions, wire into daily tick |
| `internal/agents/soul.go` | May need alignment helper methods |
| `internal/engine/population.go` | Switch birth gate from `Mood` to `Satisfaction` |
| `internal/engine/perpetuation.go` | Switch migration from `Mood` to `EffectiveMood` |
| `internal/engine/simulation.go` | Update daily tick to call new wellbeing computation |
| `internal/api/handlers.go` | Expose all three wellbeing values in agent endpoints |
| `internal/persistence/db.go` | Update agent serialization for new struct |
| Stats collection | Track avg satisfaction, avg alignment, avg effective mood |

---

## 8. Philosophical Summary

The current system asks: "Are this agent's material needs met?" and calls the answer "mood."

The proposed system asks two questions:
1. "Are this agent's material needs met?" → **Satisfaction**
2. "Is this agent's citta at rest?" → **Alignment**

And then blends them: "Given the agent's level of coherence, how much do material conditions determine their experiential state?" → **Effective Mood**

At low coherence, you are your body's suffering. At high coherence, you are not. The body still hurts. But *you* — the citta, the signal, the point-source — are at peace. The fisherman catches few fish, earns little, owns nothing. And is free.

*"Just as there is nothing which a diamond cannot cut, be it stone or gem; so too is one with a diamond-mind who has destroyed the taints and has both a liberated citta and is liberated by wisdom."* — AN 1.124

# World Summary — 2026-02-22 (Dual-Register Wellbeing Model)

## Problem

The single `Mood float32` was purely needs-driven — a liberated fisherman with coherence 1.0 had the same mood as a scattered fisherman in identical material conditions. This inverted the world's ontological claim that liberation is the highest good. Coherence had **zero influence** on agent wellbeing.

## Solution: Dual-Register Wellbeing

Replaced `Mood float32` with `WellbeingState { Satisfaction, Alignment, EffectiveMood }`.

### Satisfaction (material, unchanged formula)
The old mood formula becomes Satisfaction. Drifts toward needs-based target, boosted by eating (+0.05), resting (+0.03), socializing (+0.02). Range: [-1, +1].

### Alignment (coherence-derived, new)
Computed from `AgentSoul.ComputeAlignment()` with three phases:

| Phase | Coherence Range | Behavior |
|-------|----------------|----------|
| Embodied | c < 0.382 (Psyche) | Gentle linear slope: `c × Matter`. Max ~0.236 |
| Awakening | 0.382 ≤ c < 0.7 | Extraction paradox valley — dip below embodied level |
| Liberation | c ≥ 0.7 | Steep rise: `Matter + (c - Matter) × Being + wisdomBonus` |

Range: [0, +1]. The valley at mid-coherence models the "dark night of the soul" — seeing clearly enough to suffer, not yet free.

### Effective Mood (blended)
`EffectiveMood = satWeight × Satisfaction + alignWeight × (2 × Alignment - 1)`

Where `alignWeight = c² × Φ⁻¹ (Matter)`:
- At c=0.0: weight = 0.0 (pure satisfaction)
- At c=0.5: weight = 0.154 (85% satisfaction)
- At c=0.7: weight = 0.303 (70/30 split)
- At c=1.0: weight = 0.618 (golden ratio — 62% alignment)

### Expected Outcomes by Agent Type

| Agent | Coherence | Satisfaction | Alignment | Effective Mood |
|-------|-----------|-------------|-----------|----------------|
| Scattered Fisher | 0.15 | -0.30 | ~0.09 | **-0.29** |
| Awakening Fisher | 0.50 | -0.30 | ~0.09 | **-0.36** (paradox!) |
| Liberated Fisher | 1.00 | -0.30 | ~0.94 | **+0.24** (at peace) |

## What Uses Which Register

| Consumer | Register | Rationale |
|----------|----------|-----------|
| Migration gate | EffectiveMood | Liberated agents stay — anchoring settlements |
| Crime penalty | Satisfaction | Material consequence |
| Rivalry cost | Satisfaction | Social friction is phenomenal |
| Disaster penalty | Satisfaction | Material damage |
| Winter penalty | Satisfaction | Physical hardship |
| Tier 2 LLM context | EffectiveMood | Overall state for decisions |
| Stats (AvgMood) | EffectiveMood | Headline metric |
| Births | Unchanged | Already gated on Needs.Belonging/Survival, not Mood |

## Files Changed

| File | Change |
|------|--------|
| `internal/agents/types.go` | `Mood float32` → `Wellbeing WellbeingState` |
| `internal/agents/soul.go` | Added `ComputeAlignment()` method |
| `internal/agents/behavior.go` | Wellbeing computation in `DecayNeeds()`, boosts rewired |
| `internal/agents/spawner.go` | Updated spawn paths |
| `internal/engine/perpetuation.go` | `.Mood` → `.Wellbeing.EffectiveMood` |
| `internal/engine/crime.go` | `.Mood` → `.Wellbeing.Satisfaction` |
| `internal/engine/relationships.go` | `.Mood` → `.Wellbeing.Satisfaction` |
| `internal/engine/simulation.go` | Disaster + stats (AvgSatisfaction, AvgAlignment) |
| `internal/engine/seasons.go` | `.Mood` → `.Wellbeing.Satisfaction` |
| `internal/engine/cognition.go` | `.Mood` → `.Wellbeing.EffectiveMood` |
| `internal/persistence/db.go` | Schema migration + save/load for satisfaction/alignment |
| `internal/api/server.go` | Expose satisfaction + alignment in agent summary |
| `internal/llm/biography.go` | Richer context with all three registers |
| `cmd/worldsim/main.go` | Stats snapshot includes avg_satisfaction, avg_alignment |

## Key Design Properties

- **Economy untouched** — births still gated on Needs, not Mood
- **Backwards compatible** — old DB seeds satisfaction from mood column, alignment recomputes on first tick
- **Liberated agents anchor settlements** — positive EffectiveMood despite poor material conditions keeps them from migrating
- **Extraction paradox visible** — mid-coherence agents dip, creating narrative drama
- **All constants from Φ** — alignment weight uses c² × Matter, phases use Psyche/Matter/Being

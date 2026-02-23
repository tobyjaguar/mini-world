// Package agents — AgentSoul implements the Wheeler coherence model.
// See design doc Sections 16.2–16.4.
package agents

import (
	"math"

	"github.com/talgya/mini-world/internal/phi"
)

// StateOfBeing represents the agent's coherence band.
// Named for the philosophical condition, not the subjective experience.
// An Embodied farmer with a full belly and warm home is content — they're just
// identified with phenomena rather than self-similar. See Wheeler's Codex on Evil:
// "For the fool no such separation exists, nor this form of torture."
type StateOfBeing uint8

const (
	// Embodied: low coherence (0.0–0.3). Consciousness scattered among phenomena.
	// Identified with body, desires, routines. The default condition of embodied beings.
	// Not suffering — simply ordinary life.
	Embodied StateOfBeing = iota

	// Centered: medium coherence (0.4–0.6). Stable, introspective, materially
	// successful. The meditator — calm but not transcendent.
	Centered

	// Liberated: high coherence (0.7–1.0). Extremely rare. Self-similar,
	// point-source. Disproportionate influence on world events.
	Liberated
)

// AgentClass represents the agent's fundamental behavioral orientation.
type AgentClass uint8

const (
	// Devotionalist: driven by loyalty, tradition, social conformity. Most common.
	Devotionalist AgentClass = iota
	// Ritualist: driven by routine, established patterns. Stable economic actors.
	Ritualist
	// Nihilist: driven by pure self-interest, accumulation, zero-sum thinking.
	Nihilist
	// Transcendentalist: wisdom-seeking, driven by subtraction. Extremely rare.
	Transcendentalist
)

// ElementType classifies agents on the Mass × Gauss axes (Section 16.3).
type ElementType uint8

const (
	ElementHelium   ElementType = iota // Low mass, low drive — inert, stable
	ElementHydrogen                    // Low mass, high drive — volatile, transformative
	ElementGold                        // High mass, low drive — wealthy, passive
	ElementUranium                     // High mass, high drive — powerful, unstable
)

// AgentSoul holds the Wheeler coherence model state for an agent.
type AgentSoul struct {
	// Core identity — the "citta" (signal, not the radio).
	CittaCoherence float32 `json:"citta_coherence"` // 0.0–1.0, master variable

	// Mass × Gauss classification.
	Mass  float32 `json:"mass"`  // 0.0–1.0: accumulated capability, wealth, social weight
	Gauss float32 `json:"gauss"` // 0.0–1.0: ambition, drive, field intensity

	// Derived state.
	State StateOfBeing `json:"state_of_being"`
	Class AgentClass   `json:"agent_class"`

	// Via negativa accumulator.
	WisdomScore float32 `json:"wisdom_score"` // Accumulated through subtraction
}

// StateFromCoherence derives the StateOfBeing from a coherence value.
func StateFromCoherence(coherence float32) StateOfBeing {
	switch {
	case coherence >= 0.7:
		return Liberated
	case coherence >= 0.4:
		return Centered
	default:
		return Embodied
	}
}

// ClassifyElement returns the elemental type based on mass and gauss.
func ClassifyElement(mass, gauss float32) ElementType {
	highMass := mass > 0.5
	highGauss := gauss > 0.5
	switch {
	case !highMass && !highGauss:
		return ElementHelium
	case !highMass && highGauss:
		return ElementHydrogen
	case highMass && !highGauss:
		return ElementGold
	default:
		return ElementUranium
	}
}

// Element returns this soul's elemental classification.
func (s *AgentSoul) Element() ElementType {
	return ClassifyElement(s.Mass, s.Gauss)
}

// UpdateState recalculates derived state from coherence.
func (s *AgentSoul) UpdateState() {
	s.State = StateFromCoherence(s.CittaCoherence)
}

// AdjustCoherence modifies coherence by delta (positive = via negativa gain,
// negative = attachment/dilution). Clamps to [0, 1].
func (s *AgentSoul) AdjustCoherence(delta float32) {
	s.CittaCoherence += delta
	if s.CittaCoherence < 0 {
		s.CittaCoherence = 0
	}
	if s.CittaCoherence > 1 {
		s.CittaCoherence = 1
	}
	s.UpdateState()
}

// WisdomContribution returns the cultural imprint this soul would leave on death.
// High-coherence agents leave disproportionate marks.
func (s *AgentSoul) WisdomContribution() float64 {
	return float64(s.WisdomScore) * phi.Psyche
}

// ComputeAlignment derives inner harmony from coherence.
// Three phases model the Wheeler journey:
//   - Phase 1 (Embodied, c < Psyche ~0.382): gentle slope — ordinary contentment
//   - Phase 2 (Awakening, Psyche <= c < 0.7): extraction paradox / dark night valley
//   - Phase 3 (Liberation, c >= 0.7): steep rise — self-similar clarity
func (s *AgentSoul) ComputeAlignment() float32 {
	c := float64(s.CittaCoherence)

	var alignment float64
	switch {
	case c < phi.Psyche:
		// Embodied: linear climb, max ~0.236 at boundary
		alignment = c * phi.Matter
	case c < 0.7:
		// Awakening: dip into the valley. The extraction paradox —
		// seeing clearly enough to suffer, not yet free.
		// Quadratic valley with minimum near c=0.55
		mid := (phi.Psyche + 0.7) / 2 // ~0.541
		depth := 0.15                  // valley depth
		norm := (c - mid) / (0.7 - mid)
		alignment = phi.Psyche*phi.Matter - depth*(1-norm*norm)
	default:
		// Liberation: steep ascent. Wisdom bonus from via negativa.
		wisdomBonus := math.Min(float64(s.WisdomScore)*0.1, 0.1)
		alignment = phi.Matter + (c-phi.Matter)*phi.Being + wisdomBonus
	}

	// Clamp to [0, 1]
	if alignment < 0 {
		alignment = 0
	}
	if alignment > 1 {
		alignment = 1
	}

	return float32(alignment)
}

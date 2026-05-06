package engine

import (
	"fmt"
	"math/rand"

	"github.com/talgya/mini-world/eventproto"
	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// R90 (Doc 25 Layer 3): Reincarnation.
//
// When a liberated agent dies past age 30, their accumulated wisdom is
// conserved as spirit-stuff in the LiberatedSpiritsPool (see
// processNaturalDeaths in population.go). Each newborn rolls a small
// probability of inheriting from the pool — if so, they're seeded with
// elevated coherence and inherited WisdomEffort, beginning life past the
// Awakening Valley.
//
// This is the ONLY path for an under-16 agent to be liberated. Without it,
// children's coherence is structurally capped below Matter (Layer 1) and
// they cannot practice (Layer 2 four-foundations gate). The narrative beat:
// rare children arrive carrying the unsettling clarity of a deceased sage.
//
// Calibrated cadence (validated by cmd/lib_projector): ~1 reincarnation per
// 80 sim-years per 10K agents. Scaling to production's 400K population:
// roughly 1 reincarnation every 2 sim-years. Right narrative beat.
//
// Reincarnated children are NOT guaranteed to maintain liberation. Trauma
// can still drop them below 0.7. Per projector validation: ~50% maintain
// liberation through adult life; ~50% fall back. The carried wisdom is
// real but the agent must still choose to practice when they reach age 16.

const (
	// ReincarnationAgeFloor — only deceased agents at or above this age
	// contribute to the spirits pool. Prevents reincarnated-child cycles.
	ReincarnationAgeFloor = 30
)

// ReincarnationDenomFactor returns Φ⁵ ≈ 11.09. Tuning knob: P = pool / (pop × Φ⁵).
func ReincarnationDenomFactor() float64 {
	return phi.Phi * phi.Phi * phi.Phi * phi.Phi * phi.Phi
}

// PoolDecayPerWeek returns 1.2% per week. Spirits not claimed slowly fade,
// keeping the pool in equilibrium with the death rate.
func PoolDecayPerWeek() float64 {
	return phi.Agnosis * 0.05
}

// processSpiritsPoolDecay decays the pool weekly. Called from TickWeek.
func (s *Simulation) processSpiritsPoolDecay() {
	if s.LiberatedSpiritsPool <= 0 {
		return
	}
	// Apply 1.2% decay using Bernoulli on each spirit (cheap for typical
	// pool sizes 0–1000). Equivalent to multiplicative decay in expectation
	// but stays integer-valued without rounding bias.
	rng := rand.New(rand.NewSource(int64(s.LastTick)))
	pDecay := PoolDecayPerWeek()
	remaining := 0
	for i := 0; i < s.LiberatedSpiritsPool; i++ {
		if rng.Float64() >= pDecay {
			remaining++
		}
	}
	s.LiberatedSpiritsPool = remaining
}

// rollReincarnation determines whether a newborn is reincarnated. Returns
// (yes, seedCoherence, seedWisdom). Decrements the pool if yes.
func (s *Simulation) rollReincarnation(rng *rand.Rand) (bool, float32, uint32) {
	if s.LiberatedSpiritsPool <= 0 {
		return false, 0, 0
	}
	pop := len(s.Agents)
	if pop == 0 {
		return false, 0, 0
	}
	pReincarn := float64(s.LiberatedSpiritsPool) / (float64(pop) * ReincarnationDenomFactor())
	if rng.Float64() >= pReincarn {
		return false, 0, 0
	}
	s.LiberatedSpiritsPool--
	// Seed coherence: [0.7, 0.85] — past the threshold but with room to fall back.
	seedC := 0.7 + rng.Float64()*0.15
	// Seed WisdomEffort: [100, 300] — carried-over practice from prior life.
	seedW := uint32(100 + rng.Intn(200))
	return true, float32(seedC), seedW
}

// applyReincarnation modifies a freshly-born agent into a reincarnated soul.
// Should be called immediately after the spawner returns a new child if the
// reincarnation roll succeeded.
func (s *Simulation) applyReincarnation(child *agents.Agent, seedCoherence float32, seedWisdom uint32, tick uint64) {
	child.Soul.CittaCoherence = seedCoherence
	child.Soul.WisdomEffort = seedWisdom
	child.Soul.Reincarnated = true
	child.Soul.UpdateState()

	settName := "the wilderness"
	hex := world.HexCoord{}
	if child.HomeSettID != nil {
		if sett, ok := s.SettlementIndex[*child.HomeSettID]; ok {
			settName = sett.Name
			hex = sett.Position
		}
	}

	s.EmitEvent(Event{
		Tick:        tick,
		Description: fmt.Sprintf("%s is born in %s carrying the clarity of an elder sage — a reincarnated soul", child.Name, settName),
		Category:    eventproto.CategoryBirth,
		Meta: map[string]any{
			"event_type":      "reincarnation",
			"agent_id":        child.ID,
			"agent_name":      child.Name,
			"settlement_id":   child.HomeSettID,
			"settlement_name": settName,
			"hex_q":           hex.Q,
			"hex_r":           hex.R,
			"seed_coherence":  fmt.Sprintf("%.3f", seedCoherence),
			"seed_wisdom":     seedWisdom,
		},
	})
}

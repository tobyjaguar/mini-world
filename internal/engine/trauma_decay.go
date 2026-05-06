package engine

import (
	"math/rand"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// Layer 1 trauma decay magnitudes. Computed at init (phi.Agnosis is float64).
var (
	traumaWitnessBase = float32(phi.Agnosis * 0.05)  // ≈0.0118
	traumaWitnessCap  = float32(phi.Agnosis * 0.5)   // ≈0.118
	traumaVictimBase  = float32(phi.Agnosis * 0.025) // half of settlement-level
	traumaVictimCap   = float32(phi.Agnosis * 0.25)
)

const traumaLibMult float32 = 2.0 // extraction paradox

// applyTraumaToWitnesses applies coherence loss to all alive agents in a
// settlement after a traumatic event (war, plague, severe famine).
//
// R88 (Doc 25 Layer 1): universal trauma decay path. Liberated agents
// (CittaCoherence ≥ 0.7) take 2× the loss — the Wheeler "extraction paradox"
// already documented in soul.go: sages see clearly enough to suffer from
// what's lost. Single-event decay capped at Agnosis*0.5 ≈ 0.118.
//
// This is the negative force that balances Layer 2 practice gains and makes
// liberation precarious — even those who have crossed the Awakening Valley
// can fall back through trauma.
//
// Intensity: 0.0–1.0 scale of how severe the event is. A skirmish raid
// might be 0.3; a settlement-wide plague might be 0.8.
func (s *Simulation) applyTraumaToWitnesses(settID uint64, intensity float32, tick uint64) {
	if intensity <= 0 {
		return
	}
	witnesses, ok := s.SettlementAgents[settID]
	if !ok {
		return
	}
	rng := rand.New(rand.NewSource(int64(tick) + int64(settID)))
	for _, w := range witnesses {
		if !w.Alive {
			continue
		}
		jitter := 0.7 + rng.Float32()*0.3 // 0.7–1.0
		decay := traumaWitnessBase * intensity * jitter
		if w.Soul.CittaCoherence >= 0.7 {
			decay *= traumaLibMult
		}
		if decay > traumaWitnessCap {
			decay = traumaWitnessCap
		}
		w.Soul.AdjustCoherence(-decay)
	}
}

// applyTraumaToVictim applies coherence loss to a single agent after a
// personal traumatic event (theft, assault). Smaller magnitude than
// settlement-level trauma but still meaningful for liberated victims.
func (s *Simulation) applyTraumaToVictim(victim *agents.Agent, intensity float32, tick uint64) {
	if !victim.Alive || intensity <= 0 {
		return
	}
	rng := rand.New(rand.NewSource(int64(tick) + int64(victim.ID)))
	jitter := 0.7 + rng.Float32()*0.3
	decay := traumaVictimBase * intensity * jitter
	if victim.Soul.CittaCoherence >= 0.7 {
		decay *= traumaLibMult
	}
	if decay > traumaVictimCap {
		decay = traumaVictimCap
	}
	victim.Soul.AdjustCoherence(-decay)
}

// settlementHasTrauma returns true if the settlement has experienced enough
// recent conflict to qualify as a "high trauma zone" — used by Layer 4
// monastic settlement detection (the inverse of monastic peace).
func settlementHasTrauma(sett *social.Settlement, recentRaids int, recentDeaths int) bool {
	if sett == nil {
		return false
	}
	// Settlement is "trauma-experiencing" if recent raids per population
	// exceed Agnosis (a meaningful fraction).
	if sett.Population == 0 {
		return false
	}
	rate := float64(recentRaids+recentDeaths) / float64(sett.Population)
	return rate > phi.Agnosis*0.05
}

package agents

import (
	"math/rand"

	"github.com/talgya/mini-world/internal/phi"
)

// R89 (Doc 25 Layer 2): Active practice mechanics.
//
// The simulation's view of liberation: a difficult journey of deliberate
// practice (bhāvanā/cultivation), with conducive conditions correlating
// but not guaranteeing. Coherence drift caps at Matter (R88 NaturalCap);
// only practice insights bypass the cap to bridge the Awakening Valley.
//
// Practice is gated by the four foundations:
//   - Survival > Matter (~0.618)  — can't practice while starving
//   - Safety > Matter             — can't practice while terrified
//   - Belonging > Psyche (~0.382) — practice without ground crumbles
//   - Age >= 16                   — adult journey only (children only via Layer 3 reincarnation)
//
// Eligible agents may choose contemplation. Per-occupation conducive weights
// encode the operator intuition: Scholar/Alchemist > Hunter/Merchant >
// Laborer/Miner. Per-class intention multipliers: Transcendentalist seeks
// liberation; Nihilist mostly rejects it.
//
// Constants validated by cmd/lib_projector across 5 seeds (Doc 25 §3.0):
// Layer 1+2 produces 1.20% liberated mean, distribution Scholar 12% / Alchemist
// 3% / Hunter 0.6% / Merchant 0.2% / Laborer 0.02%, median age 47, zero
// non-reincarnated children liberated.

const (
	// ContemplateMinAge — children excluded from this path. Layer 3 reincarnation
	// is the only way for an under-16 agent to be liberated.
	ContemplateMinAge = 16

	// SamathaPerTick — gradual deepening per practice tick. Calibrated.
	SamathaPerTick float32 = 0.0005

	// InsightProb — per practice tick, probability of vipassanā insight.
	InsightProb = 0.01

	// InsightCoherenceGain — size of insight bump (when it fires).
	InsightCoherenceGain float32 = 0.025
)

// occupationConducive returns how conducive each occupation is to contemplative
// practice. Φ-derived; encodes operator intuition (philosopher highest, laborer
// lowest).
var occupationConducive = [...]float64{
	OccupationFarmer:    phi.Agnosis,         // 0.236 — absorbed seasonal labor
	OccupationMiner:     phi.Agnosis,         // 0.236 — continuous physical strain
	OccupationCrafter:   phi.Agnosis,         // 0.236 — focused absorbing work
	OccupationMerchant:  phi.Matter,          // 0.618 — long travel alone with thoughts
	OccupationSoldier:   phi.Psyche,          // 0.382 — stress + post-action liminal time
	OccupationScholar:   phi.Being,           // 1.618 — dedicated to knowledge as work
	OccupationAlchemist: phi.Being / phi.Phi, // 1.000 — contemplative craft, herbalism is meditative
	OccupationLaborer:   phi.Agnosis,         // 0.236 — same as miner
	OccupationFisher:    phi.Psyche,          // 0.382 — waiting periods on water
	OccupationHunter:    phi.Matter,          // 0.618 — solitude in nature, liminal between hunts
}

// classIntention returns how disposed each class is to choose contemplation
// when eligible. Transcendentalist actively seeks; Nihilist mostly rejects.
var classIntention = [...]float64{
	Devotionalist:     phi.Matter,  // 0.618 — practice within faith framework
	Ritualist:         phi.Psyche,  // 0.382 — form before insight
	Nihilist:          phi.Agnosis, // 0.236 — rare anomalies
	Transcendentalist: phi.Being,   // 1.618 — active seekers
}

// ContemplationEligible returns true if the agent meets the four-foundations
// gate for deliberate practice. This is checked at action selection time;
// the cost of practice is the action slot itself (replaces work/socialize).
func ContemplationEligible(a *Agent) bool {
	if a.Age < ContemplateMinAge {
		return false
	}
	if a.Needs.Survival < float32(phi.Matter) {
		return false
	}
	if a.Needs.Safety < float32(phi.Matter) {
		return false
	}
	if a.Needs.Belonging < float32(phi.Psyche) {
		return false
	}
	return true
}

// ContemplationProbability returns the per-tick probability that an eligible
// agent chooses to contemplate, given their occupation and class.
//
// Base rate phi.Agnosis³ ≈ 0.013/hour, then occupation × class weights.
// The probability is per-tick (sim-minute); over a sim-hour of eligibility,
// expected ticks = probability × 60. For Scholar/Transcendentalist (highest
// combo): 0.013 × 1.618 × 1.618 × 60 ≈ 2.04 ticks/hour. For Laborer/Nihilist
// (lowest): 0.013 × 0.236 × 0.236 × 60 ≈ 0.043 ticks/hour.
func ContemplationProbability(a *Agent) float64 {
	base := phi.Agnosis * phi.Agnosis * phi.Agnosis // ≈0.013 per hour
	occW := occupationConducive[a.Occupation]
	clsW := classIntention[a.Soul.Class]
	// Per-tick: divide by 60 (60 ticks per sim-hour).
	return base * occW * clsW / 60.0
}

// applyContemplate runs one tick of practice. Increments WisdomEffort and
// adds the gradual samatha gain. May produce a vipassanā insight (rare,
// per InsightProb) which adds a larger bump and bypasses the Matter cap
// (the only path to Liberation).
//
// Practice gives small needs replenishment: Belonging (community of
// fellow practitioners), Purpose (the work of cultivation itself).
func applyContemplate(a *Agent, rng *rand.Rand) []string {
	a.Soul.WisdomEffort++
	// Samatha — gradual deepening, bypasses Matter cap.
	a.Soul.AdjustCoherenceUncapped(SamathaPerTick)
	// Vipassanā — rare breakthrough.
	if rng.Float64() < InsightProb {
		a.Soul.AdjustCoherenceUncapped(InsightCoherenceGain)
	}
	// Small needs replenishment.
	a.Needs.Belonging += float32(phi.Agnosis * 0.001)
	if a.Needs.Belonging > 1 {
		a.Needs.Belonging = 1
	}
	a.Needs.Purpose += float32(phi.Agnosis * 0.001)
	if a.Needs.Purpose > 1 {
		a.Needs.Purpose = 1
	}
	return nil
}

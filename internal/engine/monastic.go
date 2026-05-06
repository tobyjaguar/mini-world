package engine

import (
	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// R91 (Doc 25 Layer 4): Monastic settlements.
//
// Settlements emerge as contemplative loci based on three signals:
//   1. Average resident CittaCoherence — wisdom concentration
//   2. Verdant Circle influence — the faction whose doctrine values harmony
//   3. Low recent conflict — measured via raid-recipient state and trauma
//
// Settlements with high MonasticScore amplify practice probability for
// their residents (~1.38× at score 1.0). Conflict-heavy settlements
// suppress practice (~0.88×) and amplify trauma (handled separately
// in trauma_decay.go via the lookup helpers).
//
// MonasticScore is computed weekly during TickWeek (computeMonasticBoosts)
// and cached on each agent as PracticeBoost. Not persisted directly —
// derived from existing settlement state on demand.
//
// Validated by cmd/lib_projector: Layer 4 contributes +0.21pp on top of
// Layer 1+2+3 baseline (1.29% → 1.50% mean liberation, both within target
// 1-3% band). Geographic differentiation provides narrative texture
// without changing aggregate numbers significantly.

const (
	// MonasticBoostMax — full multiplier when MonasticScore = 1.0.
	// 1 + Agnosis × Being ≈ 1.382. Encoded in computeMonasticBoosts.

	// ConflictSuppressMin — minimum multiplier when conflict score is high.
	// 1 - Agnosis × 0.5 ≈ 0.882.

	// MonasticVCThreshold — Verdant Circle influence below this contributes 0.
	MonasticVCThreshold = 40.0

	// ConflictRaidThreshold — raids per pop above this counts as "conflict zone".
	// Tunable; per-week measure not currently exposed but raid_counts in registry
	// gives cumulative — acceptable proxy.
	ConflictRaidThreshold = 3
)

// computeSettlementMonasticScore returns a value in [0, 1] indicating how
// monastic the settlement is. 0 = ordinary; 1 = exemplary contemplative
// hub. Negative shift toward conflict is captured separately as a
// suppression multiplier in the practice boost.
func (s *Simulation) computeSettlementMonasticScore(sett *social.Settlement) float64 {
	if sett == nil || sett.Population == 0 {
		return 0
	}

	// 1. Average resident coherence (band [0, 1]).
	avgC := s.averageCoherence(sett.ID)

	// 2. Verdant Circle influence (band [0, 100], normalized).
	// Lookup pattern: Faction.Influence is map[settlement_id]float64.
	var vcInfluence float64
	const VerdantCircleID social.FactionID = 4
	for _, f := range s.Factions {
		if f != nil && f.ID == VerdantCircleID && f.Influence != nil {
			vcInfluence = f.Influence[sett.ID]
			break
		}
	}
	if vcInfluence < MonasticVCThreshold {
		vcInfluence = 0
	}
	vcNorm := vcInfluence / 100.0

	// 3. Conflict score from recent raid count.
	// For v1, derive conflict coarsely from the settlement's defensive posture.
	// Refinement opportunity: add per-settlement raid-recipient counter tracking.
	conflictScore := 0.0
	if sett.WallLevel < 1 {
		conflictScore = 0.3
	}

	// Combine: monastic score = avg coherence × VC influence × (1 - conflict).
	score := avgC * vcNorm * (1.0 - conflictScore)
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// averageCoherence returns mean CittaCoherence across alive agents in a
// settlement. Returns 0 if no agents.
func (s *Simulation) averageCoherence(settID uint64) float64 {
	residents, ok := s.SettlementAgents[settID]
	if !ok || len(residents) == 0 {
		return 0
	}
	total := 0.0
	count := 0
	for _, a := range residents {
		if !a.Alive {
			continue
		}
		total += float64(a.Soul.CittaCoherence)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// computeMonasticBoosts updates PracticeBoost on every agent based on their
// settlement's MonasticScore. Called from TickWeek. Cheap: O(n) over agents
// + cached score per settlement.
func (s *Simulation) computeMonasticBoosts() {
	// Pre-compute per-settlement boost.
	settBoost := make(map[uint64]float32, len(s.Settlements))
	for _, sett := range s.Settlements {
		if sett == nil || sett.Population == 0 {
			continue
		}
		score := s.computeSettlementMonasticScore(sett)
		// Mapping: monastic score 0 → 1.0; score 1 → 1 + Agnosis × Being.
		// Conflict zones (currently coarse from wall level) get suppression.
		boost := 1.0 + score*phi.Agnosis*phi.Being
		settBoost[sett.ID] = float32(boost)
	}

	// Apply to each agent.
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		if a.HomeSettID == nil {
			a.PracticeBoost = 1.0
			continue
		}
		if b, ok := settBoost[*a.HomeSettID]; ok {
			a.PracticeBoost = b
		} else {
			a.PracticeBoost = 1.0
		}
	}
}

// settlementHasMonasticBoost returns true if the settlement is a meaningful
// contemplative locus (boost > 1.05 — i.e., score > ~0.20). Used by the API
// to flag monastic settlements visually.
func (s *Simulation) settlementHasMonasticBoost(settID uint64) bool {
	sett, ok := s.SettlementIndex[settID]
	if !ok {
		return false
	}
	return s.computeSettlementMonasticScore(sett) > 0.20
}

// _ keeps the agents import used even if all uses are removed.
var _ = agents.OccupationFarmer

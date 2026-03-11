// Land governance — Phase 7B: hex claims, infrastructure investment, coherence-based extraction.
// Implements Ostrom commons governance principles with Φ-derived constants.
// See docs/15-land-management-proposal.md for the research proposal.
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// initSettlementClaims assigns hex claims for all active settlements.
// Each settlement claims its home hex + up to 6 neighbors (non-ocean, unclaimed).
// Called once on startup and when new settlements are founded.
func (s *Simulation) initSettlementClaims() {
	claimed := 0
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}
		claimed += s.claimHexesForSettlement(sett.ID)
	}
	if claimed > 0 {
		slog.Info("hex claims initialized", "hexes_claimed", claimed)
	}
}

// claimHexesForSettlement claims the home hex + unclaimed neighbors for a settlement.
// Returns the number of newly claimed hexes.
func (s *Simulation) claimHexesForSettlement(settID uint64) int {
	sett, ok := s.SettlementIndex[settID]
	if !ok {
		return 0
	}

	claimed := 0
	claimHex := func(coord world.HexCoord) {
		h := s.WorldMap.Get(coord)
		if h == nil || h.Terrain == world.TerrainOcean {
			return
		}
		if h.ClaimedBy == nil {
			h.ClaimedBy = &settID
			claimed++
		}
	}

	claimHex(sett.Position)
	for _, nc := range sett.Position.Neighbors() {
		claimHex(nc)
	}
	return claimed
}

// releaseSettlementClaims releases all hex claims for an abandoned settlement.
func (s *Simulation) releaseSettlementClaims(settID uint64) {
	for _, hex := range s.WorldMap.Hexes {
		if hex.ClaimedBy != nil && *hex.ClaimedBy == settID {
			hex.ClaimedBy = nil
		}
	}
}

// processLandInvestment handles weekly infrastructure investment by settlements.
// Settlements with sufficient treasury and population invest in irrigation and
// conservation on their claimed hexes. One upgrade per settlement per week.
//
// Irrigation boosts resource regen: factor = 1 + level × Φ⁻¹ (at level 5: +3.09×).
// Conservation reduces extraction damage: factor = 1 - level × Φ⁻² (at level 5: -59% damage).
//
// Cost per level: level × pop × Agnosis crowns (~0.236 × pop per level).
// Governance quality affects investment likelihood: GovernanceScore must exceed Psyche (0.382).
func (s *Simulation) processLandInvestment(tick uint64) {
	invested := 0
	for _, sett := range s.Settlements {
		if sett.Population == 0 || sett.Treasury < 100 {
			continue
		}

		// Governance quality gate: only well-governed settlements invest wisely.
		if sett.GovernanceScore < phi.Psyche {
			continue
		}

		// Budget: Agnosis fraction of treasury per week (~23.6%), capped at pop×10.
		budget := uint64(float64(sett.Treasury) * phi.Agnosis * 0.1)
		maxBudget := uint64(sett.Population) * 10
		if budget > maxBudget {
			budget = maxBudget
		}
		if budget < 50 {
			continue
		}

		// Find the claimed hex most in need of investment.
		// Priority: lowest health hex that can be upgraded.
		var bestHex *world.Hex
		bestScore := math.MaxFloat64
		upgradeType := "" // "irrigation" or "conservation"

		for _, hex := range s.WorldMap.Hexes {
			if hex.ClaimedBy == nil || *hex.ClaimedBy != sett.ID {
				continue
			}
			if hex.Terrain == world.TerrainOcean {
				continue
			}

			// Prefer irrigation on productive hexes, conservation on degraded ones.
			if hex.IrrigationLevel < 5 && hex.Health > phi.Agnosis {
				// Irrigation score: lower health = higher priority (more benefit from regen boost).
				score := hex.Health + float64(hex.IrrigationLevel)*0.2
				if score < bestScore {
					bestScore = score
					bestHex = hex
					upgradeType = "irrigation"
				}
			}
			if hex.ConservationLevel < 5 && hex.Health < phi.Matter {
				// Conservation score: lower health = higher priority (protect degraded land).
				score := hex.Health + float64(hex.ConservationLevel)*0.2 - 0.5
				if score < bestScore {
					bestScore = score
					bestHex = hex
					upgradeType = "conservation"
				}
			}
		}

		if bestHex == nil {
			continue
		}

		// Compute cost: next_level × Agnosis × pop (scales with settlement size).
		var nextLevel uint8
		if upgradeType == "irrigation" {
			nextLevel = bestHex.IrrigationLevel + 1
		} else {
			nextLevel = bestHex.ConservationLevel + 1
		}
		cost := uint64(float64(nextLevel) * phi.Agnosis * float64(sett.Population) * 0.1)
		if cost < 50 {
			cost = 50
		}
		if cost > budget || cost > sett.Treasury {
			continue
		}

		// Execute upgrade.
		sett.Treasury -= cost
		if upgradeType == "irrigation" {
			bestHex.IrrigationLevel = nextLevel
		} else {
			bestHex.ConservationLevel = nextLevel
		}
		invested++

		s.EmitEvent(Event{
			Tick: tick,
			Description: fmt.Sprintf("%s invests in %s (level %d) on hex (%d,%d)",
				sett.Name, upgradeType, nextLevel, bestHex.Coord.Q, bestHex.Coord.R),
			Category: "economy",
			Meta: map[string]any{
				"settlement_id":   sett.ID,
				"settlement_name": sett.Name,
				"type":            upgradeType,
				"level":           nextLevel,
				"cost":            cost,
				"hex_q":           bestHex.Coord.Q,
				"hex_r":           bestHex.Coord.R,
			},
		})
	}

	if invested > 0 {
		slog.Info("land investment", "settlements_invested", invested)
	}
}

// processInfrastructureDecay decays irrigation and conservation levels weekly.
// Represents natural entropy — settlements must maintain improvements.
// Decay chance per hex per week: Agnosis × 0.05 (~1.18%). At level 5, expected
// decay every ~85 weeks. Low enough that maintained settlements stay upgraded
// but abandoned improvements gradually return to nature.
func (s *Simulation) processInfrastructureDecay(tick uint64) {
	simWeek := tick / TicksPerSimWeek
	decayed := 0
	for coord, hex := range s.WorldMap.Hexes {
		if hex.IrrigationLevel == 0 && hex.ConservationLevel == 0 {
			continue
		}

		// Deterministic decay check: based on week + hex coordinate.
		decayRoll := float64((simWeek*uint64(coord.Q+500)*7 + uint64(coord.R+500)*13) % 1000) / 1000.0
		if decayRoll > phi.Agnosis*0.05 {
			continue // No decay this week
		}

		// Claimed hexes with active settlements decay slower (maintenance).
		if hex.ClaimedBy != nil {
			if sett, ok := s.SettlementIndex[*hex.ClaimedBy]; ok && sett.Population > 0 {
				// Maintained: 50% chance to resist decay.
				maintainRoll := float64((simWeek*uint64(coord.Q+300)*11 + uint64(coord.R+300)*17) % 100) / 100.0
				if maintainRoll < 0.5 {
					continue
				}
			}
		}

		if hex.IrrigationLevel > 0 {
			hex.IrrigationLevel--
			decayed++
		}
		if hex.ConservationLevel > 0 {
			hex.ConservationLevel--
			decayed++
		}
	}

	if decayed > 0 {
		slog.Info("infrastructure decay", "levels_decayed", decayed)
	}
}

// IrrigationRegenFactor returns the regen multiplier from irrigation level.
// factor = 1 + level × Φ⁻¹. At level 0: 1.0. At level 5: 4.09.
func IrrigationRegenFactor(level uint8) float64 {
	return 1.0 + float64(level)*phi.Matter // Φ⁻¹ = Matter = 0.618
}

// ConservationDamageFactor returns the extraction damage multiplier from conservation level.
// factor = 1 - level × Φ⁻² (Agnosis). At level 0: 1.0. At level 5: ~0.882.
func ConservationDamageFactor(level uint8) float64 {
	f := 1.0 - float64(level)*phi.Agnosis*0.1
	if f < 0.1 {
		f = 0.1 // Never reduce damage below 10%
	}
	return f
}

// coherenceExtractionMod returns a modifier on extraction damage based on
// settlement governance quality and average agent coherence.
// Well-governed settlements with coherent agents extract more carefully.
// Modifier range: 0.618 (excellent governance) to 1.236 (poor governance).
func (s *Simulation) coherenceExtractionMod(settID uint64) float64 {
	sett, ok := s.SettlementIndex[settID]
	if !ok {
		return 1.0
	}

	// Average coherence of settlement agents.
	settAgents := s.SettlementAgents[settID]
	if len(settAgents) == 0 {
		return 1.0
	}
	var totalCoherence float64
	alive := 0
	for _, a := range settAgents {
		if a.Alive {
			totalCoherence += float64(a.Soul.CittaCoherence)
			alive++
		}
	}
	if alive == 0 {
		return 1.0
	}
	avgCoherence := totalCoherence / float64(alive)

	// Governance quality: GovernanceScore * avgCoherence.
	// High scores = careful extraction (lower damage).
	// Low scores = reckless extraction (higher damage).
	quality := sett.GovernanceScore * avgCoherence

	// Map quality 0→1.236, quality 1→0.618 (inverse relationship).
	// At quality 0.5 (average): 0.927 (slight reduction).
	return 1.0 + phi.Agnosis*(1.0-2.0*quality)
}

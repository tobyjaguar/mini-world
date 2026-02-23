// Perpetuation safeguards — anti-stagnation, economic circuit breakers, cultural drift.
// See design doc Section 9.3–9.4.
package engine

import (
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processAntiStagnation runs weekly checks against economic and social stagnation.
func (s *Simulation) processAntiStagnation(tick uint64) {
	s.economicCircuitBreaker(tick)
	s.culturalDrift(tick)
	s.reassignMismatchedProducers(tick)
	s.updateSettlementPopulations()
}

// economicCircuitBreaker prevents extreme inflation or deflation.
// See design doc Section 9.4.
func (s *Simulation) economicCircuitBreaker(tick uint64) {
	for _, sett := range s.Settlements {
		if sett.Market == nil {
			continue
		}

		for _, entry := range sett.Market.Entries {
			ratio := entry.Price / entry.BasePrice

			// Hyperinflation: price > 5x base → inject supply.
			if ratio > phi.Completion {
				entry.Supply += entry.Demand * 0.5
				entry.Price = entry.ResolvePrice(1.0, 1.0)
				slog.Debug("circuit breaker: inflation correction",
					"settlement", sett.Name,
					"good", entry.Good,
					"ratio", ratio,
				)
			}

			// Deflation: price < 20% of base → reduce supply.
			if ratio < phi.Agnosis {
				entry.Supply *= 0.5
				if entry.Supply < 1 {
					entry.Supply = 1
				}
				entry.Price = entry.ResolvePrice(1.0, 1.0)
				slog.Debug("circuit breaker: deflation correction",
					"settlement", sett.Name,
					"good", entry.Good,
					"ratio", ratio,
				)
			}
		}
	}
}

// culturalDrift makes younger agents slightly different from older ones.
// Prevents the world from reaching perfect equilibrium.
func (s *Simulation) culturalDrift(tick uint64) {
	for _, a := range s.Agents {
		if !a.Alive || a.Age > 30 {
			continue
		}

		if a.Age >= 14 && a.Age <= 25 {
			a.Soul.AdjustCoherence(float32(phi.Agnosis * 0.005))
			a.Needs.Belonging += float32(phi.Agnosis * 0.01)
			if a.Needs.Belonging > 1 {
				a.Needs.Belonging = 1
			}
		}
	}
}

// updateSettlementPopulations recalculates settlement populations from actual alive agents.
func (s *Simulation) updateSettlementPopulations() {
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		alive := uint32(0)
		for _, a := range settAgents {
			if a.Alive {
				alive++
			}
		}
		sett.Population = alive
	}
}

// processSeasonalMigration moves desperate agents toward prosperous settlements.
// For tiny settlements (pop < 25), the mood threshold is relaxed to accelerate absorption.
func (s *Simulation) processSeasonalMigration(tick uint64) {
	// Find the most prosperous settlement (global fallback target).
	var bestSettID uint64
	bestProsperity := 0.0

	// Cache alive counts per settlement for reuse.
	settAliveCounts := make(map[uint64]int, len(s.Settlements))
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		aliveCount := 0
		for _, a := range settAgents {
			if a.Alive {
				aliveCount++
			}
		}
		settAliveCounts[sett.ID] = aliveCount
		if aliveCount == 0 {
			continue
		}
		prosperity := float64(sett.Treasury) / float64(aliveCount+1)
		if prosperity > bestProsperity {
			bestProsperity = prosperity
			bestSettID = sett.ID
		}
	}

	if bestSettID == 0 {
		return
	}

	bestSett := s.SettlementIndex[bestSettID]
	if bestSett == nil {
		return
	}

	// Desperate agents migrate.
	migrated := false
	for _, sett := range s.Settlements {
		if sett.ID == bestSettID {
			continue
		}

		aliveCount := settAliveCounts[sett.ID]
		isTiny := aliveCount > 0 && aliveCount < 25

		// For tiny settlements, lower mood threshold from -0.3 to 0.0
		// (any non-positive mood triggers migration).
		moodThreshold := float32(-0.3)
		if isTiny {
			moodThreshold = 0.0
		}

		settAgents := s.SettlementAgents[sett.ID]
		for _, a := range settAgents {
			// For tiny settlements, use Satisfaction (not EffectiveMood) for
			// migration decisions. A liberated agent with high alignment but
			// negative satisfaction should still leave a dying village —
			// liberation doesn't mean staying somewhere you can't eat.
			// Also remove survival gate — isolation is deprivation even when fed.
			if isTiny {
				if !a.Alive || a.Wellbeing.Satisfaction > 0.0 {
					continue
				}
			} else {
				if !a.Alive || a.Wellbeing.EffectiveMood > moodThreshold || a.Needs.Survival > 0.3 {
					continue
				}
			}

			// For tiny settlements, migrate to nearest viable settlement
			// (pop >= 25) within 5 hexes instead of global best.
			target := bestSett
			if isTiny {
				if nearby := s.findNearestViableSettlement(sett, 5); nearby != nil {
					target = nearby
				}
			}

			newID := target.ID
			a.HomeSettID = &newID
			a.Position = target.Position
			migrated = true
		}
	}

	// Rebuild SettlementAgents map if any agents migrated.
	// Without this, population counts stay stale and settlements never consolidate.
	if migrated {
		s.rebuildSettlementAgents()
	}
}

// findNearestViableSettlement finds the closest settlement with pop >= 50
// within maxDist hex distance. Returns nil if none found.
func (s *Simulation) findNearestViableSettlement(from *social.Settlement, maxDist int) *social.Settlement {
	var best *social.Settlement
	bestDist := maxDist + 1

	for _, sett := range s.Settlements {
		if sett.ID == from.ID || sett.Population < 50 {
			continue
		}
		dist := world.Distance(from.Position, sett.Position)
		if dist <= maxDist && dist < bestDist {
			bestDist = dist
			best = sett
		}
	}
	return best
}

// reassignMismatchedProducers checks resource-producing agents and reassigns
// those whose hex lacks their required resource. A fisher on a plains hex can
// never produce fish — they should become a farmer instead.
func (s *Simulation) reassignMismatchedProducers(tick uint64) {
	reassigned := 0
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		resType, needsResource := occupationResource[a.Occupation]
		if !needsResource {
			continue // Crafters, laborers, etc. don't need hex resources.
		}

		hex := s.WorldMap.Get(a.Position)
		if hex == nil {
			continue
		}

		// Check if the hex has any of the required resource.
		if hex.Resources[resType] >= 1.0 {
			continue // Resource available — no reassignment needed.
		}

		// Hex lacks the required resource. Reassign to match hex terrain.
		newOcc := bestOccupationForHex(hex)
		if newOcc == a.Occupation {
			continue
		}

		old := a.Occupation
		a.Occupation = newOcc
		reassigned++

		slog.Debug("reassigned mismatched producer",
			"agent", a.Name,
			"from", old,
			"to", newOcc,
			"terrain", hex.Terrain,
		)
	}

	if reassigned > 0 {
		slog.Info("reassigned mismatched producers", "count", reassigned, "tick", tick)
	}
}

// bestOccupationForHex returns the primary resource-producing occupation
// that matches the hex's available resources. Falls back to Farmer (most
// versatile) if no clear match.
func bestOccupationForHex(hex *world.Hex) agents.Occupation {
	// Check which resources the hex actually has, pick the richest.
	type candidate struct {
		occ agents.Occupation
		amt float64
	}
	candidates := []candidate{
		{agents.OccupationFarmer, hex.Resources[world.ResourceGrain]},
		{agents.OccupationMiner, hex.Resources[world.ResourceIronOre]},
		{agents.OccupationFisher, hex.Resources[world.ResourceFish]},
		{agents.OccupationHunter, hex.Resources[world.ResourceFurs]},
	}

	best := candidates[0] // Default to Farmer.
	for _, c := range candidates[1:] {
		if c.amt > best.amt {
			best = c
		}
	}

	// If the best resource is still 0, fall back to Farmer — grain is
	// the most universally available resource.
	if best.amt < 1.0 {
		return agents.OccupationFarmer
	}
	return best.occ
}

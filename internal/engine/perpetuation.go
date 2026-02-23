// Perpetuation safeguards — anti-stagnation, economic circuit breakers, cultural drift.
// See design doc Section 9.3–9.4.
package engine

import (
	"log/slog"

	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processAntiStagnation runs weekly checks against economic and social stagnation.
func (s *Simulation) processAntiStagnation(tick uint64) {
	s.economicCircuitBreaker(tick)
	s.culturalDrift(tick)
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
			// For tiny settlements, remove survival gate — agents migrate
			// seeking community, not just food. Isolation is deprivation
			// even when fed.
			if isTiny {
				if !a.Alive || a.Mood > moodThreshold {
					continue
				}
			} else {
				if !a.Alive || a.Mood > moodThreshold || a.Needs.Survival > 0.3 {
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

// findNearestViableSettlement finds the closest settlement with pop >= 25
// within maxDist hex distance. Returns nil if none found.
func (s *Simulation) findNearestViableSettlement(from *social.Settlement, maxDist int) *social.Settlement {
	var best *social.Settlement
	bestDist := maxDist + 1

	for _, sett := range s.Settlements {
		if sett.ID == from.ID || sett.Population < 25 {
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

// Perpetuation safeguards — anti-stagnation, economic circuit breakers, cultural drift.
// See design doc Section 9.3–9.4.
package engine

import (
	"log/slog"

	"github.com/talgya/mini-world/internal/phi"
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
func (s *Simulation) processSeasonalMigration(tick uint64) {
	// Find the most prosperous settlement.
	var bestSettID uint64
	bestProsperity := 0.0

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		aliveCount := 0
		for _, a := range settAgents {
			if a.Alive {
				aliveCount++
			}
		}
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
	for _, sett := range s.Settlements {
		if sett.ID == bestSettID {
			continue
		}
		settAgents := s.SettlementAgents[sett.ID]
		for _, a := range settAgents {
			if !a.Alive || a.Mood > -0.3 || a.Needs.Survival > 0.3 {
				continue
			}
			newID := bestSett.ID
			a.HomeSettID = &newID
			a.Position = bestSett.Position
		}
	}
}

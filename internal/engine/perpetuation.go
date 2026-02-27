// Perpetuation safeguards — anti-stagnation, economic circuit breakers, cultural drift.
// See design doc Section 9.3–9.4.
package engine

import (
	"log/slog"
	"math"
	"sort"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processAntiStagnation runs weekly checks against economic and social stagnation.
func (s *Simulation) processAntiStagnation(tick uint64) {
	s.economicCircuitBreaker(tick)
	s.culturalDrift(tick)
	s.rebalanceSettlementProducers(tick)
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
			s.reassignIfMismatched(a, newID)
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

// reassignIfMismatched checks a single agent after movement and reassigns their
// occupation if the 7-hex neighborhood of the destination settlement lacks
// their required resource. Called at all movement sites (migration, diaspora,
// consolidation, viability force-migration) so agents are productive immediately.
func (s *Simulation) reassignIfMismatched(a *agents.Agent, settID uint64) {
	if !a.Alive {
		return
	}

	// Determine required resource. Alchemist needs herbs but isn't in occupationResource.
	resType, needsResource := occupationResource[a.Occupation]
	if !needsResource {
		if a.Occupation == agents.OccupationAlchemist {
			resType = world.ResourceHerbs
		} else {
			return // Crafter, Merchant, Soldier, Scholar — no hex resource needed.
		}
	}

	// Check 7-hex neighborhood for the required resource.
	sett, ok := s.SettlementIndex[settID]
	if !ok {
		return
	}
	found := false
	checkHex := func(coord world.HexCoord) {
		if found {
			return
		}
		h := s.WorldMap.Get(coord)
		if h != nil && h.Resources[resType] >= 1.0 {
			found = true
		}
	}
	checkHex(sett.Position)
	for _, nc := range sett.Position.Neighbors() {
		checkHex(nc)
	}
	if found {
		return
	}

	// No resource nearby — reassign to match settlement position hex.
	hex := s.WorldMap.Get(sett.Position)
	if hex == nil {
		return
	}
	newOcc := bestOccupationForHex(hex)
	if newOcc == a.Occupation {
		return
	}

	old := a.Occupation
	a.Occupation = newOcc
	slog.Debug("reassigned occupation on move",
		"agent", a.Name,
		"from", old,
		"to", newOcc,
		"settlement", sett.Name,
		"terrain", hex.Terrain,
	)
}

// rebalanceSettlementProducers gradually reassigns excess resource producers
// to non-producer occupations when the settlement has more producers than
// its carrying capacity can support. Runs weekly via processAntiStagnation.
func (s *Simulation) rebalanceSettlementProducers(tick uint64) {
	for settID, settAgents := range s.SettlementAgents {
		target := s.settlementProducerTarget(settID)

		// Count current resource producers.
		var producerAgents []*agents.Agent
		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			if _, ok := occupationResource[a.Occupation]; ok {
				producerAgents = append(producerAgents, a)
			}
		}

		excess := len(producerAgents) - target
		if excess <= 0 {
			continue
		}

		// Reassign phi.Agnosis (~23.6%) of excess per week — gradual, not disruptive.
		toReassign := int(math.Ceil(float64(excess) * phi.Agnosis))

		// Pick lowest-satisfaction producers (they're suffering most on depleted hexes).
		sort.Slice(producerAgents, func(i, j int) bool {
			return producerAgents[i].Wellbeing.Satisfaction < producerAgents[j].Wellbeing.Satisfaction
		})

		reassigned := 0
		for _, a := range producerAgents {
			if reassigned >= toReassign {
				break
			}
			a.Occupation = bestNonProducerOccupation(settAgents)
			reassigned++
		}

		if reassigned > 0 {
			slog.Info("rebalanced producers", "settlement", settID,
				"target", target, "were", len(producerAgents), "reassigned", reassigned)
		}
	}
}

// bestNonProducerOccupation returns the most underrepresented non-producer
// occupation relative to ideal distribution weights.
func bestNonProducerOccupation(settAgents []*agents.Agent) agents.Occupation {
	counts := make(map[agents.Occupation]int)
	for _, a := range settAgents {
		if a.Alive {
			counts[a.Occupation]++
		}
	}

	// Non-producer occupations and their ideal weights.
	nonProducers := []struct {
		occ    agents.Occupation
		weight float64
	}{
		{agents.OccupationCrafter, 0.40},
		{agents.OccupationSoldier, 0.20},
		{agents.OccupationScholar, 0.20},
		{agents.OccupationMerchant, 0.15},
		{agents.OccupationAlchemist, 0.05},
	}

	// Pick the most underrepresented relative to ideal weight.
	best := agents.OccupationCrafter
	bestDeficit := -math.MaxFloat64
	total := 0.0
	for _, a := range settAgents {
		if a.Alive {
			total++
		}
	}
	for _, np := range nonProducers {
		ideal := np.weight * total * (1 - phi.Matter) // scale by non-producer share
		actual := float64(counts[np.occ])
		deficit := ideal - actual
		if deficit > bestDeficit {
			bestDeficit = deficit
			best = np.occ
		}
	}
	return best
}

// reassignMismatchedProducers is a weekly safety net that catches any agents
// whose occupation doesn't match their hex resources. With reassignIfMismatched()
// called at all movement sites, this should fire with count=0. If it doesn't,
// we missed a movement path.
func (s *Simulation) reassignMismatchedProducers(tick uint64) {
	reassigned := 0
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		resType, needsResource := occupationResource[a.Occupation]
		if !needsResource {
			continue // Crafters, merchants, etc. don't need hex resources.
		}

		// Check the 7-hex neighborhood (home + 6 neighbors) for the required
		// resource. bestProductionHex picks the healthiest hex with the resource,
		// but falls back to sett.Position even when no resource exists — so we
		// must verify the returned hex actually has the resource.
		prodHex := s.bestProductionHex(a)
		if prodHex != nil && prodHex.Resources[resType] >= 1.0 {
			continue // Can produce in the neighborhood — no reassignment needed.
		}

		// No resource anywhere nearby. Reassign to match position hex terrain.
		hex := s.WorldMap.Get(a.Position)
		if hex == nil {
			continue
		}
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
		slog.Warn("safety-net reassigned mismatched producers", "count", reassigned, "tick", tick)
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
		{agents.OccupationLaborer, hex.Resources[world.ResourceStone]},
		{agents.OccupationAlchemist, hex.Resources[world.ResourceHerbs]},
	}

	best := candidates[0] // Default to Farmer.
	for _, c := range candidates[1:] {
		if c.amt > best.amt {
			best = c
		}
	}

	// If the best resource is still 0, fall back to Crafter — crafters
	// don't need hex resources and won't deplete further. The old Farmer
	// fallback fed a doom loop: depleted hex → Farmer → more depletion.
	if best.amt < 1.0 {
		return agents.OccupationCrafter
	}
	return best.occ
}

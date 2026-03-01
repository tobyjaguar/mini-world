// Perpetuation safeguards — anti-stagnation, economic circuit breakers, cultural drift.
// See design doc Section 9.3–9.4.
package engine

import (
	"fmt"
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
	// Round 24: disabled — forced reassignment destroyed occupation diversity.
	// Producers should migrate to compatible resources, not become crafters.
	// s.rebalanceSettlementProducers(tick)
	// s.reassignMismatchedProducers(tick)
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

// reassignIfMismatched is a no-op as of Round 24.
// Occupation is identity — a farmer whose field is fallow should MOVE to better land,
// not become a crafter. Forced reassignment destroyed occupation diversity (82% crafters).
// All 4 call sites (migration, diaspora, viability, consolidation) handle movement correctly;
// the only side effect was the unwanted occupation change.
func (s *Simulation) reassignIfMismatched(a *agents.Agent, settID uint64) {
	// Round 24: disabled. Agents keep their occupation on move.
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

// --- Round 24: Occupation Persistence & Resource-Seeking Migration ---

// processResourceMigration moves idle resource producers to settlements with compatible resources.
// A farmer whose field is fallow should MOVE to better land, not become a crafter.
// Grace period: 2 sim-weeks unproductive. Cap: 10% of settlement producers per week (min 1).
func (s *Simulation) processResourceMigration(tick uint64) {
	twoWeeks := uint64(TicksPerSimDay * 14)
	migrated := false

	for settID, settAgents := range s.SettlementAgents {
		sett, ok := s.SettlementIndex[settID]
		if !ok {
			continue
		}

		// Find idle resource producers in this settlement.
		var idleProducers []*agents.Agent
		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			resType, isProducer := occupationResource[a.Occupation]
			if !isProducer {
				continue
			}
			// Check grace period: idle for 2+ weeks.
			if tick-a.LastWorkTick < twoWeeks {
				continue
			}
			// Verify settlement lacks their resource in 7-hex neighborhood.
			if s.settlementHasResource(sett, resType) {
				continue
			}
			idleProducers = append(idleProducers, a)
		}

		if len(idleProducers) == 0 {
			continue
		}

		// Cap: max 10% of settlement producers per week (min 1).
		producerCount := 0
		for _, a := range settAgents {
			if a.Alive {
				if _, ok := occupationResource[a.Occupation]; ok {
					producerCount++
				}
			}
		}
		maxMigrate := producerCount / 10
		if maxMigrate < 1 {
			maxMigrate = 1
		}

		moved := 0
		for _, a := range idleProducers {
			if moved >= maxMigrate {
				break
			}
			resType := occupationResource[a.Occupation]
			target := s.findResourceSettlement(sett, resType, 5)
			if target == nil {
				target = s.findResourceSettlement(sett, resType, 10)
			}
			if target == nil {
				continue // Fallow tolerance: no compatible settlement found, agent waits.
			}

			newID := target.ID
			a.HomeSettID = &newID
			a.Position = target.Position
			migrated = true
			moved++

			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s migrated to %s seeking %s", a.Name, target.Name, resourceName(resType)),
				Category:    "social",
				Meta: map[string]any{
					"agent_id":        a.ID,
					"agent_name":      a.Name,
					"settlement_id":   target.ID,
					"settlement_name": target.Name,
					"occupation":      a.Occupation,
				},
			})
		}

		if moved > 0 {
			slog.Info("resource migration",
				"from", sett.Name,
				"moved", moved,
				"idle_producers", len(idleProducers),
			)
		}
	}

	if migrated {
		s.rebuildSettlementAgents()
	}
}

// settlementHasResource checks if a settlement's 7-hex neighborhood has a given resource.
func (s *Simulation) settlementHasResource(sett *social.Settlement, resType world.ResourceType) bool {
	check := func(coord world.HexCoord) bool {
		h := s.WorldMap.Get(coord)
		return h != nil && h.Resources[resType] >= 1.0
	}
	if check(sett.Position) {
		return true
	}
	for _, nc := range sett.Position.Neighbors() {
		if check(nc) {
			return true
		}
	}
	return false
}

// findResourceSettlement finds the nearest settlement within maxDist hexes
// that has a given resource in its 7-hex neighborhood.
func (s *Simulation) findResourceSettlement(from *social.Settlement, resType world.ResourceType, maxDist int) *social.Settlement {
	var best *social.Settlement
	bestDist := maxDist + 1

	for _, sett := range s.Settlements {
		if sett.ID == from.ID || sett.Population < 25 {
			continue
		}
		dist := world.Distance(from.Position, sett.Position)
		if dist > maxDist || dist >= bestDist {
			continue
		}
		if s.settlementHasResource(sett, resType) {
			bestDist = dist
			best = sett
		}
	}
	return best
}

// resourceName returns a human-readable name for a resource type.
func resourceName(r world.ResourceType) string {
	switch r {
	case world.ResourceGrain:
		return "farmland"
	case world.ResourceIronOre:
		return "iron deposits"
	case world.ResourceFish:
		return "fishing waters"
	case world.ResourceFurs:
		return "hunting grounds"
	case world.ResourceStone:
		return "quarries"
	case world.ResourceHerbs:
		return "herb gardens"
	default:
		return "resources"
	}
}

// processCrafterRecovery transitions idle crafters to producer occupations
// matching the richest resource in their settlement's 7-hex neighborhood.
// Cap: 10% of settlement idle crafters per week (min 1).
func (s *Simulation) processCrafterRecovery(tick uint64) {
	oneWeek := uint64(TicksPerSimDay * 7)

	for settID, settAgents := range s.SettlementAgents {
		sett, ok := s.SettlementIndex[settID]
		if !ok {
			continue
		}

		// Find idle crafters (no materials, idle 7+ sim-days).
		var idleCrafters []*agents.Agent
		for _, a := range settAgents {
			if !a.Alive || a.Occupation != agents.OccupationCrafter {
				continue
			}
			if tick-a.LastWorkTick < oneWeek {
				continue
			}
			idleCrafters = append(idleCrafters, a)
		}

		if len(idleCrafters) == 0 {
			continue
		}

		// Find best producer occupation for neighborhood.
		newOcc := bestProducerOccupationForNeighborhood(s, sett)
		if newOcc == agents.OccupationCrafter {
			continue // No viable producer role nearby.
		}

		// Cap: 10% of idle crafters per week (min 1).
		maxRetrain := len(idleCrafters) * 10 / 100
		if maxRetrain < 1 {
			maxRetrain = 1
		}

		retrained := 0
		for _, a := range idleCrafters {
			if retrained >= maxRetrain {
				break
			}
			a.Occupation = newOcc
			setMinimumSkill(a, newOcc)
			retrained++

			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s begins retraining as a %s in %s", a.Name, occupationLabel(newOcc), sett.Name),
				Category:    "social",
				Meta: map[string]any{
					"agent_id":        a.ID,
					"agent_name":      a.Name,
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"occupation":      newOcc,
				},
			})
		}

		if retrained > 0 {
			slog.Info("crafter recovery",
				"settlement", sett.Name,
				"retrained", retrained,
				"idle_crafters", len(idleCrafters),
				"new_occupation", occupationLabel(newOcc),
			)
		}
	}
}

// bestProducerOccupationForNeighborhood checks all 7 hexes for the richest resource
// and returns the corresponding producer occupation.
func bestProducerOccupationForNeighborhood(s *Simulation, sett *social.Settlement) agents.Occupation {
	type candidate struct {
		occ agents.Occupation
		amt float64
	}
	var best candidate

	checkHex := func(coord world.HexCoord) {
		h := s.WorldMap.Get(coord)
		if h == nil {
			return
		}
		for resType, occ := range resourceOccupation() {
			amt := h.Resources[resType]
			if amt > best.amt {
				best = candidate{occ: occ, amt: amt}
			}
		}
	}

	checkHex(sett.Position)
	for _, nc := range sett.Position.Neighbors() {
		checkHex(nc)
	}

	if best.amt < 1.0 {
		return agents.OccupationCrafter // No viable resource.
	}
	return best.occ
}

// resourceOccupation returns the reverse of occupationResource: resource → occupation.
func resourceOccupation() map[world.ResourceType]agents.Occupation {
	return map[world.ResourceType]agents.Occupation{
		world.ResourceGrain:   agents.OccupationFarmer,
		world.ResourceIronOre: agents.OccupationMiner,
		world.ResourceFish:    agents.OccupationFisher,
		world.ResourceFurs:    agents.OccupationHunter,
		world.ResourceStone:   agents.OccupationLaborer,
		world.ResourceHerbs:   agents.OccupationAlchemist,
	}
}

// setMinimumSkill ensures an agent has at least 0.2 in the primary skill for their new occupation.
func setMinimumSkill(a *agents.Agent, occ agents.Occupation) {
	switch occ {
	case agents.OccupationFarmer, agents.OccupationFisher:
		if a.Skills.Farming < 0.2 {
			a.Skills.Farming = 0.2
		}
	case agents.OccupationMiner, agents.OccupationLaborer:
		if a.Skills.Mining < 0.2 {
			a.Skills.Mining = 0.2
		}
	case agents.OccupationHunter, agents.OccupationSoldier:
		if a.Skills.Combat < 0.2 {
			a.Skills.Combat = 0.2
		}
	case agents.OccupationAlchemist, agents.OccupationScholar, agents.OccupationCrafter:
		if a.Skills.Crafting < 0.2 {
			a.Skills.Crafting = 0.2
		}
	case agents.OccupationMerchant:
		if a.Skills.Trade < 0.2 {
			a.Skills.Trade = 0.2
		}
	}
}

// processCareerTransition handles chronically idle producers (30+ sim-days)
// who have no compatible settlement within 10 hexes.
// Transitions to skill-adjacent occupation if settlement has resources for it.
func (s *Simulation) processCareerTransition(tick uint64) {
	thirtyDays := uint64(TicksPerSimDay * 30)
	sixtyDays := uint64(TicksPerSimDay * 60)

	for settID, settAgents := range s.SettlementAgents {
		sett, ok := s.SettlementIndex[settID]
		if !ok {
			continue
		}

		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			_, isProducer := occupationResource[a.Occupation]
			if !isProducer {
				continue
			}
			idleTicks := tick - a.LastWorkTick
			if idleTicks < thirtyDays {
				continue
			}
			// Only transition if no compatible settlement exists within 10 hexes.
			resType := occupationResource[a.Occupation]
			if s.findResourceSettlement(sett, resType, 10) != nil {
				continue // Resource migration should handle this instead.
			}

			// Try skill-adjacent occupation.
			newOcc := skillAdjacentOccupation(a.Occupation)
			if newOcc == a.Occupation {
				// No skill-adjacent option. After 60+ days, fall back to Crafter.
				if idleTicks >= sixtyDays {
					newOcc = agents.OccupationCrafter
				} else {
					continue
				}
			}

			old := a.Occupation
			a.Occupation = newOcc
			setMinimumSkill(a, newOcc)

			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s transitions from %s to %s in %s", a.Name, occupationLabel(old), occupationLabel(newOcc), sett.Name),
				Category:    "social",
				Meta: map[string]any{
					"agent_id":        a.ID,
					"agent_name":      a.Name,
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"occupation":      newOcc,
				},
			})

			slog.Debug("career transition",
				"agent", a.Name,
				"from", occupationLabel(old),
				"to", occupationLabel(newOcc),
				"idle_days", idleTicks/uint64(TicksPerSimDay),
			)
		}
	}
}

// skillAdjacentOccupation returns the closest occupation that shares skills.
func skillAdjacentOccupation(occ agents.Occupation) agents.Occupation {
	switch occ {
	case agents.OccupationFarmer:
		return agents.OccupationFisher // Both use Farming
	case agents.OccupationFisher:
		return agents.OccupationFarmer // Both use Farming
	case agents.OccupationMiner:
		return agents.OccupationLaborer // Both use Mining
	case agents.OccupationLaborer:
		return agents.OccupationMiner // Both use Mining
	case agents.OccupationHunter:
		return agents.OccupationSoldier // Both use Combat
	case agents.OccupationAlchemist:
		return agents.OccupationScholar // Both use Crafting
	default:
		return occ // No skill-adjacent option.
	}
}

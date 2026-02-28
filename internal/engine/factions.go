// Faction dynamics — membership assignment, influence updates, inter-faction relations.
// See design doc Section 6.
package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// InitFactions creates seed factions and assigns agents to them based on occupation/class.
func (s *Simulation) InitFactions() {
	s.Factions = social.SeedFactions()

	// Set initial inter-faction relations.
	// Crown vs Merchant: mild tension. Crown vs Iron Brotherhood: alliance.
	// Ashen Path: distrusted by all.
	s.setRelation(1, 2, -20) // Crown ↔ Merchants: tension
	s.setRelation(1, 3, 30)  // Crown ↔ Iron Brotherhood: allied
	s.setRelation(1, 4, 10)  // Crown ↔ Verdant Circle: neutral-positive
	s.setRelation(1, 5, -50) // Crown ↔ Ashen Path: hostile
	s.setRelation(2, 3, -10) // Merchants ↔ Iron Brotherhood: mild tension
	s.setRelation(2, 4, 20)  // Merchants ↔ Verdant Circle: positive
	s.setRelation(2, 5, -30) // Merchants ↔ Ashen Path: negative
	s.setRelation(3, 4, -20) // Iron Brotherhood ↔ Verdant Circle: tension
	s.setRelation(3, 5, -40) // Iron Brotherhood ↔ Ashen Path: hostile
	s.setRelation(4, 5, -60) // Verdant Circle ↔ Ashen Path: very hostile

	// Assign agents to factions based on occupation, class, and governance.
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		govType := social.GovCommune // default
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				govType = sett.Governance
			}
		}
		fid := factionForAgent(a, govType)
		if fid > 0 {
			factionID := uint64(fid)
			a.FactionID = &factionID
		}
	}

	// Calculate initial faction influence per settlement.
	s.updateFactionInfluence()

	slog.Info("factions initialized", "count", len(s.Factions))
}

// factionForAgent determines which faction an agent naturally belongs to.
// Not all agents join factions (~60% do). Governance type influences assignment
// for common folk — monarchies produce more Crown loyalists, merchant republics
// produce more Compact members.
func factionForAgent(a *agents.Agent, govType social.GovernanceType) social.FactionID {
	// Occupation-based primary assignment.
	switch a.Occupation {
	case agents.OccupationSoldier:
		return 3 // Iron Brotherhood
	case agents.OccupationMerchant:
		return 2 // Merchant's Compact
	case agents.OccupationScholar:
		return 4 // Verdant Circle
	case agents.OccupationAlchemist:
		if a.Soul.Class == agents.Nihilist || a.Soul.Class == agents.Transcendentalist {
			return 5 // Ashen Path — mystics and nihilists
		}
		return 4 // Verdant Circle — regular alchemists
	case agents.OccupationHunter:
		// Hunters with combat skill lean military, others unaffiliated.
		if a.Skills.Combat > 0.15 {
			return 3 // Iron Brotherhood
		}
		return 0
	default:
		// Governance-based assignment for common folk (before trait-based).
		if govType == social.GovMonarchy && (a.Wealth > 50 || a.Soul.CittaCoherence > 0.3) {
			return 1 // Crown — monarchy subjects with wealth or coherence
		}
		if govType == social.GovMerchantRepublic && (a.Skills.Trade > 0.1 || a.Wealth > 80) {
			return 2 // Merchant's Compact — republic traders
		}

		// Trait-based secondary assignment for common folk.
		switch a.Soul.Class {
		case agents.Devotionalist:
			if a.Role == agents.RoleNoble || a.Role == agents.RoleLeader {
				return 1 // Crown
			}
			// Devout high-coherence farmers/crafters → Verdant Circle.
			if a.Soul.CittaCoherence > 0.35 {
				return 4 // Verdant Circle
			}
			return 0 // Unaffiliated
		case agents.Ritualist:
			// Ritualists with combat aptitude → Iron Brotherhood.
			if a.Skills.Combat > 0.12 {
				return 3 // Iron Brotherhood
			}
			// Ritualists with wealth → Crown loyalists.
			if a.Wealth > 150 && a.Soul.CittaCoherence > 0.3 {
				return 1 // Crown
			}
			return 0 // Unaffiliated
		case agents.Nihilist:
			return 5 // Ashen Path
		case agents.Transcendentalist:
			return 4 // Verdant Circle — seekers
		default:
			return 0 // Unaffiliated
		}
	}
}

// SetFactions loads previously saved factions into the simulation.
func (s *Simulation) SetFactions(factions []*social.Faction) {
	s.Factions = factions
	slog.Info("factions loaded from database", "count", len(factions))
}

// updateFactionInfluence recalculates faction influence per settlement.
func (s *Simulation) updateFactionInfluence() {
	// Reset all influence.
	for _, f := range s.Factions {
		for k := range f.Influence {
			delete(f.Influence, k)
		}
	}

	// Build faction index for quick lookup.
	factionIndex := make(map[social.FactionID]*social.Faction)
	for _, f := range s.Factions {
		factionIndex[f.ID] = f
	}

	// Count faction members per settlement.
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		factionCounts := make(map[social.FactionID]int)
		aliveCount := 0

		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			aliveCount++
			if a.FactionID != nil {
				factionCounts[social.FactionID(*a.FactionID)]++
			}
		}

		if aliveCount == 0 {
			continue
		}

		// Influence = (members / total) * 100, plus governance alignment bonus.
		for fid, count := range factionCounts {
			f, ok := factionIndex[fid]
			if !ok {
				continue
			}
			influence := float64(count) / float64(aliveCount) * 100.0

			// Governance alignment bonus: matching faction-governance pairs get +15/+10.
			switch fid {
			case 1: // Crown benefits from monarchies.
				if sett.Governance == social.GovMonarchy {
					influence += 15
				}
			case 2: // Merchant's Compact benefits from merchant republics.
				if sett.Governance == social.GovMerchantRepublic {
					influence += 15
				}
			case 4: // Verdant Circle benefits from councils.
				if sett.Governance == social.GovCouncil {
					influence += 10
				}
			}

			f.Influence[sett.ID] = influence
		}
	}
}

// processWeeklyFactions runs weekly faction updates: influence shifts, relations drift,
// policy advocacy, faction dues, and inter-faction tension.
func (s *Simulation) processWeeklyFactions(tick uint64) {
	// Assign factions to unaffiliated agents (catches agents born before the addAgent fix).
	assigned := 0
	for _, a := range s.Agents {
		if !a.Alive || a.FactionID != nil {
			continue
		}
		govType := social.GovCommune
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				govType = sett.Governance
			}
		}
		if fid := factionForAgent(a, govType); fid > 0 {
			factionID := uint64(fid)
			a.FactionID = &factionID
			assigned++
		}
	}
	if assigned > 0 {
		slog.Info("faction assignment sweep", "assigned", assigned)
	}

	s.updateFactionInfluence()
	s.collectFactionDues(tick)
	s.applyFactionPolicies(tick)
	s.checkFactionTensions(tick)

	// Relations drift toward zero over time (grudges fade, alliances weaken).
	for _, f := range s.Factions {
		for otherID, rel := range f.Relations {
			drift := rel * phi.Agnosis * 0.1 // ~2.4% decay toward neutral
			f.Relations[otherID] = rel - drift
		}
	}

	// Log faction state.
	for _, f := range s.Factions {
		totalInfluence := 0.0
		for _, inf := range f.Influence {
			totalInfluence += inf
		}
		slog.Info("faction update",
			"faction", f.Name,
			"total_influence", int(totalInfluence),
			"treasury", f.Treasury,
		)
	}
}

// collectFactionDues collects weekly dues from faction members.
// Members contribute Wealth * Agnosis * 0.01 if they have >30 crowns.
func (s *Simulation) collectFactionDues(tick uint64) {
	factionIndex := make(map[social.FactionID]*social.Faction)
	for _, f := range s.Factions {
		factionIndex[f.ID] = f
	}

	for _, a := range s.Agents {
		if !a.Alive || a.FactionID == nil || a.Wealth <= 30 {
			continue
		}
		f, ok := factionIndex[social.FactionID(*a.FactionID)]
		if !ok {
			continue
		}
		dues := uint64(float64(a.Wealth) * phi.Agnosis * 0.01)
		if dues < 1 {
			dues = 1
		}
		if dues > a.Wealth {
			dues = a.Wealth
		}
		a.Wealth -= dues
		f.Treasury += dues
	}
}

// applyFactionPolicies nudges settlement governance based on dominant faction preferences.
func (s *Simulation) applyFactionPolicies(tick uint64) {
	factionIndex := make(map[social.FactionID]*social.Faction)
	for _, f := range s.Factions {
		factionIndex[f.ID] = f
	}

	for _, sett := range s.Settlements {
		// Find the dominant faction in this settlement.
		var dominantFaction *social.Faction
		highestInfluence := 0.0

		for _, f := range s.Factions {
			inf, ok := f.Influence[sett.ID]
			if ok && inf > highestInfluence {
				highestInfluence = inf
				dominantFaction = f
			}
		}

		if dominantFaction == nil || highestInfluence < 15 {
			continue // No faction has meaningful influence
		}

		// Scale nudge by influence strength (0-100 → 0-1, then small multiplier).
		strength := highestInfluence / 100.0 * phi.Agnosis * 0.1

		// Tax nudge.
		sett.TaxRate += dominantFaction.TaxPreference * strength
		if sett.TaxRate < 0.01 {
			sett.TaxRate = 0.01
		}
		if sett.TaxRate > 0.30 {
			sett.TaxRate = 0.30
		}

		// Governance score nudge based on faction type.
		switch dominantFaction.ID {
		case 1: // Crown: +governance
			sett.GovernanceScore += strength * 0.5
		case 2: // Merchant's Compact: +market
			if sett.MarketLevel < 5 && highestInfluence > 40 {
				// Small chance to upgrade market level.
				if tick%uint64(sett.MarketLevel+2) == 0 {
					sett.MarketLevel++
				}
			}
		case 3: // Iron Brotherhood: +governance
			sett.GovernanceScore += strength * 0.3
		case 4: // Verdant Circle: +governance, resist market growth
			sett.GovernanceScore += strength * 0.4
		case 5: // Ashen Path: -governance (corruption)
			sett.GovernanceScore -= strength * 0.5
		}

		// Clamp governance score.
		if sett.GovernanceScore < 0 {
			sett.GovernanceScore = 0
		}
		if sett.GovernanceScore > 1 {
			sett.GovernanceScore = 1
		}
	}
}

// checkFactionTensions logs tension events when two factions contest the same settlement.
func (s *Simulation) checkFactionTensions(tick uint64) {
	for _, sett := range s.Settlements {
		// Find factions with >40 influence in this settlement.
		var contesting []*social.Faction
		for _, f := range s.Factions {
			if inf, ok := f.Influence[sett.ID]; ok && inf > 40 {
				contesting = append(contesting, f)
			}
		}

		if len(contesting) < 2 {
			continue
		}

		// Tension between the top two factions.
		for i := 0; i < len(contesting)-1; i++ {
			for j := i + 1; j < len(contesting); j++ {
				f1, f2 := contesting[i], contesting[j]
				// Accelerate relations decay.
				f1.Relations[f2.ID] -= phi.Agnosis * 5
				f2.Relations[f1.ID] -= phi.Agnosis * 5

				s.EmitEvent(Event{
					Tick: tick,
					Description: fmt.Sprintf("Tension rises between %s and %s in %s",
						f1.Name, f2.Name, sett.Name),
					Category: "political",
					Meta: map[string]any{
						"settlement_id":   sett.ID,
						"settlement_name": sett.Name,
						"faction_1":       f1.Name,
						"faction_2":       f2.Name,
					},
				})
			}
		}
	}
}

// adjustFactionInfluenceFromCrime boosts Ashen Path and reduces Crown influence
// when crimes occur in a settlement.
func (s *Simulation) adjustFactionInfluenceFromCrime(settID uint64) {
	for _, f := range s.Factions {
		switch f.ID {
		case 1: // Crown loses credibility
			if inf, ok := f.Influence[settID]; ok && inf > 1 {
				f.Influence[settID] = inf - 0.5
			}
		case 5: // Ashen Path gains from chaos
			f.Influence[settID] += 0.3
		}
	}
}

// setRelation sets a symmetric relation between two factions.
func (s *Simulation) setRelation(a, b social.FactionID, value float64) {
	for _, f := range s.Factions {
		if f.ID == a {
			f.Relations[b] = value
		}
		if f.ID == b {
			f.Relations[a] = value
		}
	}
}

// Faction dynamics — membership assignment, influence updates, inter-faction relations.
// See design doc Section 6.
package engine

import (
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// initFactions creates seed factions and assigns agents to them based on occupation/class.
func (s *Simulation) initFactions() {
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

	// Assign agents to factions based on occupation and class.
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		fid := factionForAgent(a)
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
// Not all agents join factions (~60% do).
func factionForAgent(a *agents.Agent) social.FactionID {
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
	default:
		// Common folk: some join Crown loyalists, most are unaffiliated.
		switch a.Soul.Class {
		case agents.Devotionalist:
			if a.Role == agents.RoleNoble || a.Role == agents.RoleLeader {
				return 1 // Crown
			}
			return 0 // Unaffiliated
		case agents.Nihilist:
			return 5 // Ashen Path
		default:
			return 0 // Unaffiliated
		}
	}
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

		// Influence = (members / total) * 100, scaled by agent quality.
		for fid, count := range factionCounts {
			f, ok := factionIndex[fid]
			if !ok {
				continue
			}
			influence := float64(count) / float64(aliveCount) * 100.0
			f.Influence[sett.ID] = influence
		}
	}
}

// processWeeklyFactions runs weekly faction updates: influence shifts, relations drift.
func (s *Simulation) processWeeklyFactions(tick uint64) {
	s.updateFactionInfluence()

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

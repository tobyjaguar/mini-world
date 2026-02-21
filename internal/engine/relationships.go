// Relationship dynamics — social bonds, family formation, trust building.
// See design doc Section 4.1 and 16.4.
package engine

import (
	"fmt"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
)

// processRelationships runs daily relationship updates per settlement.
// Agents build bonds through proximity and shared settlement life.
func (s *Simulation) processRelationships(tick uint64) {
	simDay := tick / TicksPerSimDay

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]

		// Collect living adults.
		var alive []*agents.Agent
		for _, a := range settAgents {
			if a.Alive && a.Age >= 10 {
				alive = append(alive, a)
			}
		}
		if len(alive) < 2 {
			continue
		}

		// Each day, a few agents in each settlement form or strengthen bonds.
		// Number of interactions scales with population.
		interactions := len(alive) / 10
		if interactions < 1 {
			interactions = 1
		}
		if interactions > 20 {
			interactions = 20
		}

		for i := 0; i < interactions; i++ {
			// Deterministic pairing using day and settlement.
			idx1 := int((simDay*uint64(i+1) + uint64(sett.ID)*7) % uint64(len(alive)))
			idx2 := int((simDay*uint64(i+1)*3 + uint64(sett.ID)*13 + 1) % uint64(len(alive)))
			if idx1 == idx2 {
				idx2 = (idx2 + 1) % len(alive)
			}

			a1 := alive[idx1]
			a2 := alive[idx2]

			// Strengthen or create relationship.
			strengthenBond(a1, a2)
			strengthenBond(a2, a1)
		}

		// Family formation: adults with strong mutual bonds can pair up.
		if simDay%7 == 0 { // Check weekly
			s.formFamilies(sett, alive, tick)
		}
	}
}

// strengthenBond increases the relationship between two agents.
func strengthenBond(from, to *agents.Agent) {
	// Find existing relationship.
	for i := range from.Relationships {
		if from.Relationships[i].TargetID == to.ID {
			// Strengthen existing bond.
			from.Relationships[i].Sentiment += float32(phi.Agnosis * 0.1) // +~0.024
			from.Relationships[i].Trust += float32(phi.Agnosis * 0.05)    // +~0.012
			if from.Relationships[i].Sentiment > 1 {
				from.Relationships[i].Sentiment = 1
			}
			if from.Relationships[i].Trust > 1 {
				from.Relationships[i].Trust = 1
			}
			return
		}
	}

	// Create new relationship (cap at 20 to prevent memory bloat).
	if len(from.Relationships) >= 20 {
		return
	}
	from.Relationships = append(from.Relationships, agents.Relationship{
		TargetID:  to.ID,
		Sentiment: float32(phi.Agnosis * 0.2), // Start slightly positive
		Trust:     float32(phi.Agnosis * 0.1),
	})
}

// formFamilies pairs up compatible single adults with strong bonds.
func (s *Simulation) formFamilies(sett interface{ }, alive []*agents.Agent, tick uint64) {
	for _, a := range alive {
		if a.Age < 18 || a.Age > 50 {
			continue
		}
		// Skip if already in a strong family bond (has a relationship with sentiment > 0.7).
		hasPartner := false
		for _, rel := range a.Relationships {
			if rel.Sentiment > 0.7 && rel.Trust > 0.5 {
				hasPartner = true
				break
			}
		}
		if hasPartner {
			continue
		}

		// Find best compatible match among relationships.
		var bestMatch *agents.Agent
		bestScore := float32(0.4) // Minimum threshold

		for _, rel := range a.Relationships {
			if rel.Sentiment <= bestScore {
				continue
			}
			partner, ok := s.AgentIndex[rel.TargetID]
			if !ok || !partner.Alive || partner.Age < 18 || partner.Sex == a.Sex {
				continue
			}

			// Check if partner also has positive feelings back.
			for _, partnerRel := range partner.Relationships {
				if partnerRel.TargetID == a.ID && partnerRel.Sentiment > 0.3 {
					bestMatch = partner
					bestScore = rel.Sentiment
					break
				}
			}
		}

		if bestMatch != nil {
			// Boost both relationships to family-level bond.
			boostRelationship(a, bestMatch.ID, 0.3, 0.2)
			boostRelationship(bestMatch, a.ID, 0.3, 0.2)

			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s and %s have formed a family", a.Name, bestMatch.Name),
				Category:    "social",
			})
		}
	}
}

// boostRelationship increases sentiment and trust for a specific relationship.
func boostRelationship(a *agents.Agent, targetID agents.AgentID, sentimentBoost, trustBoost float32) {
	for i := range a.Relationships {
		if a.Relationships[i].TargetID == targetID {
			a.Relationships[i].Sentiment += sentimentBoost
			a.Relationships[i].Trust += trustBoost
			if a.Relationships[i].Sentiment > 1 {
				a.Relationships[i].Sentiment = 1
			}
			if a.Relationships[i].Trust > 1 {
				a.Relationships[i].Trust = 1
			}
			return
		}
	}
}

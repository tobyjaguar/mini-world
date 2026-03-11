// Crime and conflict — theft, smuggling, law enforcement.
// See design doc Section 6.3.
package engine

import (
	"fmt"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
)

// processCrime checks for criminal activity in settlements daily.
// Agents with critically unmet needs may steal. Law enforcement deters proportionally.
func (s *Simulation) processCrime(tick uint64) {
	simDay := tick / TicksPerSimDay

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		if len(settAgents) == 0 {
			continue
		}

		// Count soldiers for military deterrence bonus.
		soldierCount := 0
		for _, sa := range settAgents {
			if sa.Alive && sa.Occupation == agents.OccupationSoldier {
				soldierCount++
			}
		}
		soldierRatio := float64(soldierCount) / (float64(sett.Population) + 1)
		militaryBonus := 1.0 + soldierRatio*phi.Being*10 // At 7% soldiers: ~2.13x

		// Law enforcement effectiveness based on treasury, governance, military, walls, and culture.
		// Walls provide structural deterrence: each level adds Psyche (~38%) to guard strength.
		// Militarism axis adds martial discipline: +Agnosis*0.5 per point (max ±11.8%).
		wallBonus := 1.0 + float64(sett.WallLevel)*phi.Psyche
		cultureBonus := 1.0 + float64(sett.CultureMilitarism)*phi.Agnosis*0.5
		diplomacyBonus := 1.0 + s.GetDiplomacyCrimeBonus(sett.ID)
		guardStrength := float64(sett.Treasury) / (float64(sett.Population) + 1) * sett.GovernanceScore * militaryBonus * wallBonus * cultureBonus * diplomacyBonus
		// Deterrence: 0.0 (no law) to 1.0 (perfect enforcement)
		deterrence := guardStrength / (guardStrength + phi.Totality)

		// Soldiers gain purpose from protecting people. The more effective the
		// deterrence they contribute to, the more meaningful their role feels.
		// Purpose boost = deterrence * Agnosis * 0.3 per day (~0.017 at 25% deterrence).
		if soldierCount > 0 {
			purposeBoost := float32(deterrence * phi.Agnosis * 0.3)
			for _, sa := range settAgents {
				if sa.Alive && sa.Occupation == agents.OccupationSoldier {
					sa.Needs.Purpose += purposeBoost
					if sa.Needs.Purpose > 1 {
						sa.Needs.Purpose = 1
					}
				}
			}
		}

		for i, a := range settAgents {
			if !a.Alive {
				continue
			}

			// Crime motivation: desperate agents with low survival/safety and low coherence.
			if a.Needs.Survival > 0.3 && a.Needs.Safety > 0.2 {
				continue // Not desperate enough
			}
			if a.Soul.CittaCoherence > float32(phi.Matter) {
				continue // Too coherent to resort to crime
			}

			// Deterministic crime check: combine day, agent ID, deterrence.
			crimeChance := (1.0 - float64(a.Needs.Survival)) * (1.0 - deterrence) * phi.Agnosis
			threshold := float64((simDay*uint64(a.ID))%100) / 100.0
			if crimeChance < threshold {
				continue
			}

			// Attempt theft: steal food or wealth from a random neighbor.
			victimIdx := int((simDay + uint64(i)*7) % uint64(len(settAgents)))
			victim := settAgents[victimIdx]
			if victim.ID == a.ID || !victim.Alive {
				continue
			}

			// Steal food if hungry.
			if a.Needs.Survival < 0.2 {
				stolen := false
				if victim.Inventory[agents.GoodGrain] > 1 {
					victim.Inventory[agents.GoodGrain]--
					a.Inventory[agents.GoodGrain]++
					stolen = true
				} else if victim.Inventory[agents.GoodFish] > 1 {
					victim.Inventory[agents.GoodFish]--
					a.Inventory[agents.GoodFish]++
					stolen = true
				}
				if stolen {
					// Damage relationship.
					damageRelationship(victim, a.ID, 0.3, 0.2)
					s.adjustFactionInfluenceFromCrime(sett.ID)
				}
			} else if a.Wealth < 5 && victim.Wealth > 20 {
				// Steal crowns.
				stolen := uint64(3)
				if stolen > victim.Wealth {
					stolen = victim.Wealth
				}
				victim.Wealth -= stolen
				a.Wealth += stolen
				damageRelationship(victim, a.ID, 0.4, 0.3)
				s.adjustFactionInfluenceFromCrime(sett.ID)
			}

			// Check for faction betrayal (crime against fellow faction member).
			s.ProcessBetrayalExpulsion(a, victim, tick)

			// Caught? Deterrence chance of being caught → become outlaw.
			if deterrence > 0.3 && float64((simDay+uint64(a.ID)*3)%100)/100.0 < deterrence {
				a.Role = agents.RoleOutlaw
				a.Wellbeing.Satisfaction -= 0.2
				// Fine: lose some wealth.
				fine := uint64(float64(a.Wealth) * phi.Agnosis)
				if fine > a.Wealth {
					fine = a.Wealth
				}
				a.Wealth -= fine
				sett.Treasury += fine

				s.EmitEvent(Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s was caught stealing and branded an outlaw in %s", a.Name, sett.Name),
					Category:    "crime",
					Meta: map[string]any{
						"agent_id":        a.ID,
						"agent_name":      a.Name,
						"settlement_id":   sett.ID,
						"settlement_name": sett.Name,
					},
				})
			}
		}
	}
}

// damageRelationship decreases sentiment and trust for a specific relationship.
func damageRelationship(a *agents.Agent, targetID agents.AgentID, sentimentDmg, trustDmg float32) {
	for i := range a.Relationships {
		if a.Relationships[i].TargetID == targetID {
			a.Relationships[i].Sentiment -= sentimentDmg
			a.Relationships[i].Trust -= trustDmg
			if a.Relationships[i].Sentiment < -1 {
				a.Relationships[i].Sentiment = -1
			}
			if a.Relationships[i].Trust < 0 {
				a.Relationships[i].Trust = 0
			}
			return
		}
	}
	// Create negative relationship if none exists.
	if len(a.Relationships) < 20 {
		a.Relationships = append(a.Relationships, agents.Relationship{
			TargetID:  targetID,
			Sentiment: -sentimentDmg,
			Trust:     0,
		})
	}
}

// Governance mechanics — leader assignment, governance decay, and revolutions.
// See design doc Section 6.2.
package engine

import (
	"fmt"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// processGovernance runs daily governance updates: leader assignment, governance decay,
// and revolution checks.
func (s *Simulation) processGovernance(tick uint64) {
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]

		// Collect living adults.
		var alive []*agents.Agent
		for _, a := range settAgents {
			if a.Alive && a.Age >= 16 {
				alive = append(alive, a)
			}
		}
		if len(alive) == 0 {
			continue
		}

		// Leader assignment: if no leader or leader is dead, pick one.
		s.ensureLeader(sett, alive, tick)

		// Governance decay: score drifts toward leader-dependent target.
		s.decayGovernance(sett)

		// Revolution check.
		s.checkRevolution(sett, alive, tick)
	}
}

// ensureLeader assigns a leader if the settlement doesn't have one or the current leader is dead.
func (s *Simulation) ensureLeader(sett *social.Settlement, alive []*agents.Agent, tick uint64) {
	if sett.LeaderID != nil {
		leader, ok := s.AgentIndex[agents.AgentID(*sett.LeaderID)]
		if ok && leader.Alive {
			return // Leader exists and is alive
		}
		// Leader has died — succession crisis.
		sett.GovernanceScore -= 0.2
		if sett.GovernanceScore < 0 {
			sett.GovernanceScore = 0
		}
		sett.LeaderID = nil

		leaderName := "the leader"
		var leaderFaction string
		if ok {
			leaderName = leader.Name
			leaderFaction = s.agentFactionName(leader)
		}
		meta := map[string]any{
			"settlement_id":   sett.ID,
			"settlement_name": sett.Name,
		}
		if leaderFaction != "" {
			meta["faction_name"] = leaderFaction
		}
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s of %s has died, causing a succession crisis", leaderName, sett.Name),
			Category:    "political",
			Meta:        meta,
		})
	}

	// Select new leader based on governance type.
	var newLeader *agents.Agent

	switch sett.Governance {
	case social.GovMonarchy:
		// Highest-wealth Tier 2 agent, or highest-coherence if no Tier 2.
		newLeader = selectLeaderByWealth(alive, agents.Tier2)
		if newLeader == nil {
			newLeader = selectLeaderByCoherence(alive)
		}
	case social.GovCouncil:
		// Highest-coherence adult.
		newLeader = selectLeaderByCoherence(alive)
	case social.GovMerchantRepublic:
		// Wealthiest agent.
		newLeader = selectLeaderByWealth(alive, agents.Tier0) // Any tier
	case social.GovCommune:
		// Deterministic "random" adult based on tick.
		idx := int(tick % uint64(len(alive)))
		newLeader = alive[idx]
	}

	if newLeader != nil {
		id := uint64(newLeader.ID)
		sett.LeaderID = &id
		newLeader.Role = agents.RoleLeader

		leaderMeta := map[string]any{
			"agent_id":        newLeader.ID,
			"agent_name":      newLeader.Name,
			"settlement_id":   sett.ID,
			"settlement_name": sett.Name,
			"governance":      sett.Governance,
		}
		if fname := s.agentFactionName(newLeader); fname != "" {
			leaderMeta["faction_name"] = fname
		}
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s becomes leader of %s", newLeader.Name, sett.Name),
			Category:    "political",
			Meta:        leaderMeta,
		})
	}
}

// selectLeaderByWealth picks the wealthiest agent at or above the given tier.
func selectLeaderByWealth(alive []*agents.Agent, minTier agents.CognitionTier) *agents.Agent {
	var best *agents.Agent
	for _, a := range alive {
		if a.Tier < minTier {
			continue
		}
		if best == nil || a.Wealth > best.Wealth {
			best = a
		}
	}
	return best
}

// selectLeaderByCoherence picks the highest-coherence adult.
func selectLeaderByCoherence(alive []*agents.Agent) *agents.Agent {
	var best *agents.Agent
	for _, a := range alive {
		if best == nil || a.Soul.CittaCoherence > best.Soul.CittaCoherence {
			best = a
		}
	}
	return best
}

// decayGovernance drifts GovernanceScore toward a leader-dependent target.
func (s *Simulation) decayGovernance(sett *social.Settlement) {
	target := 0.3 // Base target without leader

	if sett.LeaderID != nil {
		leader, ok := s.AgentIndex[agents.AgentID(*sett.LeaderID)]
		if ok && leader.Alive {
			// Target = 0.3 + leader_coherence * 0.5
			target = 0.3 + float64(leader.Soul.CittaCoherence)*0.5
		}
	}

	// Drift toward target at Agnosis rate.
	drift := (target - sett.GovernanceScore) * phi.Agnosis * 0.05
	sett.GovernanceScore += drift

	if sett.GovernanceScore < 0 {
		sett.GovernanceScore = 0
	}
	if sett.GovernanceScore > 1 {
		sett.GovernanceScore = 1
	}
}

// checkRevolution fires a revolution if conditions are met:
// GovernanceScore < 0.4 AND a faction has >40 influence AND a Tier 1+ agent with coherence > 0.4 exists.
func (s *Simulation) checkRevolution(sett *social.Settlement, alive []*agents.Agent, tick uint64) {
	if sett.GovernanceScore >= 0.4 {
		return
	}

	// Find a faction with >40 influence.
	var dominantFaction *social.Faction
	for _, f := range s.Factions {
		if inf, ok := f.Influence[sett.ID]; ok && inf > 40 {
			if dominantFaction == nil || inf > f.Influence[sett.ID] {
				dominantFaction = f
			}
		}
	}
	if dominantFaction == nil {
		return
	}

	// Find a revolutionary: Tier 1+ agent with coherence > 0.4.
	var revolutionary *agents.Agent
	for _, a := range alive {
		if a.Tier >= agents.Tier1 && a.Soul.CittaCoherence > 0.4 {
			if revolutionary == nil || a.Soul.CittaCoherence > revolutionary.Soul.CittaCoherence {
				revolutionary = a
			}
		}
	}
	if revolutionary == nil {
		return
	}

	// Revolution fires!
	oldGov := sett.Governance

	// New governance based on dominant faction.
	switch dominantFaction.ID {
	case 1: // Crown → Monarchy
		sett.Governance = social.GovMonarchy
	case 2: // Merchant's Compact → Merchant Republic
		sett.Governance = social.GovMerchantRepublic
	case 3: // Iron Brotherhood → Council
		sett.Governance = social.GovCouncil
	case 4: // Verdant Circle → Commune
		sett.Governance = social.GovCommune
	case 5: // Ashen Path → Commune (anarchy)
		sett.Governance = social.GovCommune
	}

	// Depose old leader.
	if sett.LeaderID != nil {
		if oldLeader, ok := s.AgentIndex[agents.AgentID(*sett.LeaderID)]; ok && oldLeader.Alive {
			oldLeader.Role = agents.RoleCommoner
		}
	}

	// Revolutionary becomes new leader.
	newLeaderID := uint64(revolutionary.ID)
	sett.LeaderID = &newLeaderID
	revolutionary.Role = agents.RoleLeader

	// Seize 30% of treasury.
	seized := uint64(float64(sett.Treasury) * 0.3)
	sett.Treasury -= seized
	dominantFaction.Treasury += seized

	// Reset governance score.
	sett.GovernanceScore = 0.5

	govNames := map[social.GovernanceType]string{
		social.GovMonarchy:        "Monarchy",
		social.GovCouncil:         "Council",
		social.GovMerchantRepublic: "Merchant Republic",
		social.GovCommune:         "Commune",
	}

	s.EmitEvent(Event{
		Tick: tick,
		Description: fmt.Sprintf("REVOLUTION in %s! %s leads uprising backed by %s. Governance changes from %s to %s. %d crowns seized.",
			sett.Name, revolutionary.Name, dominantFaction.Name,
			govNames[oldGov], govNames[sett.Governance], seized),
		Category: "political",
		Meta: map[string]any{
			"settlement_id":   sett.ID,
			"settlement_name": sett.Name,
			"old_governance":  govNames[oldGov],
			"new_governance":  govNames[sett.Governance],
			"faction_name":    dominantFaction.Name,
		},
	})
}

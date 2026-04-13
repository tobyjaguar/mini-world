// Faction dynamics — membership assignment, influence updates, inter-faction relations.
// See design doc Section 6.
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
)

// factionNameByID returns the faction name for a given faction ID, or "" if not found.
func (s *Simulation) factionNameByID(id uint64) string {
	for _, f := range s.Factions {
		if uint64(f.ID) == id {
			return f.Name
		}
	}
	return ""
}

// agentFactionName returns the faction name for an agent, or "" if unaffiliated.
func (s *Simulation) agentFactionName(a *agents.Agent) string {
	if a.FactionID == nil {
		return ""
	}
	return s.factionNameByID(*a.FactionID)
}

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
			// Psyche threshold (~0.382): only genuinely awakening Devotionalists join.
			// Lower thresholds created a self-reinforcing loop where VC's easy doctrine
			// boosted coherence → more agents crossed threshold → VC grew unboundedly.
			if float64(a.Soul.CittaCoherence) > phi.Psyche {
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
	// Iron Brotherhood soldiers count with a martial discipline bonus (Being weight
	// per soldier) — soldiers exert outsized political influence through military presence.
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		factionCounts := make(map[social.FactionID]float64)
		aliveCount := 0

		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			aliveCount++
			if a.FactionID != nil {
				fid := social.FactionID(*a.FactionID)
				weight := 1.0
				// Iron Brotherhood soldiers count as Being (~1.618) members each.
				// Martial discipline = outsized political weight.
				if fid == 3 && a.Occupation == agents.OccupationSoldier {
					weight = phi.Being
				}
				factionCounts[fid] += weight
			}
		}

		if aliveCount == 0 {
			continue
		}

		// Influence = (weighted members / total) * 100, plus governance alignment bonus.
		for fid, weightedCount := range factionCounts {
			f, ok := factionIndex[fid]
			if !ok {
				continue
			}
			influence := weightedCount / float64(aliveCount) * 100.0

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
			case 3: // Iron Brotherhood benefits from councils (structured governance).
				if sett.Governance == social.GovCouncil {
					influence += 5
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
// processFactionMaintenance runs weekly faction updates: influence recomputation,
// dues collection, patronage distribution, policy effects, tension checks, and
// inter-faction relations drift. Does NOT perform the unaffiliated assignment
// sweep — that happens in processFactionAssignmentFallback after defection and
// recruitment have had their turn.
func (s *Simulation) processFactionMaintenance(tick uint64) {
	s.updateFactionInfluence()
	s.collectFactionDues(tick)
	s.distributeFactionPatronage(tick)
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

// processFactionAssignmentFallback is the final safety net for unaffiliated
// agents. Runs after defection and influence-based recruitment, so it only
// catches agents that recruitment couldn't place. This is the emergent pathway:
// influence competition decides faction membership first; this lookup handles
// edge cases (newborns missed by addAgent, defectors in factionless territory).
//
// Agents whose factionForAgent() returns 0 (no natural affinity) remain
// unaffiliated — that is a valid state representing agents who don't fit any
// faction's identity.
func (s *Simulation) processFactionAssignmentFallback(tick uint64) {
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
		slog.Info("faction assignment fallback", "assigned", assigned)
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

		// Culture drift: dominant faction nudges settlement culture axes.
		// Rate: strength * 0.5 per week. Slow drift, faction-shaped identity.
		cultureDrift := float32(strength * 0.5)
		switch dominantFaction.ID {
		case 1: // Crown: traditional, neutral openness, mildly martial
			sett.CultureTradition += cultureDrift
			sett.CultureMilitarism += cultureDrift * 0.3
		case 2: // Merchant's Compact: progressive, cosmopolitan, mercantile
			sett.CultureTradition -= cultureDrift
			sett.CultureOpenness += cultureDrift
			sett.CultureMilitarism -= cultureDrift
		case 3: // Iron Brotherhood: traditional, isolationist, very martial
			sett.CultureTradition += cultureDrift * 0.5
			sett.CultureOpenness -= cultureDrift * 0.5
			sett.CultureMilitarism += cultureDrift
		case 4: // Verdant Circle: progressive, cosmopolitan, mercantile
			sett.CultureTradition -= cultureDrift * 0.5
			sett.CultureOpenness += cultureDrift * 0.5
			sett.CultureMilitarism -= cultureDrift * 0.5
		case 5: // Ashen Path: progressive, isolationist, neutral
			sett.CultureTradition -= cultureDrift
			sett.CultureOpenness -= cultureDrift * 0.5
		}
		// Clamp culture axes to [-1, 1].
		if sett.CultureTradition < -1 {
			sett.CultureTradition = -1
		} else if sett.CultureTradition > 1 {
			sett.CultureTradition = 1
		}
		if sett.CultureOpenness < -1 {
			sett.CultureOpenness = -1
		} else if sett.CultureOpenness > 1 {
			sett.CultureOpenness = 1
		}
		if sett.CultureMilitarism < -1 {
			sett.CultureMilitarism = -1
		} else if sett.CultureMilitarism > 1 {
			sett.CultureMilitarism = 1
		}

		// Faction mismatch pressure: when the dominant faction's preferred
		// governance doesn't match the current type, they agitate for change.
		// This creates revolution windows that the static system never produced.
		// Rate: influence/100 * Agnosis * 0.05 per week. At 40 influence: ~0.0047/week.
		// Must compete with daily drift (~0.008/week toward target), so mismatch alone
		// won't trigger revolution — but it widens the window after leader death (-0.2).
		preferredGov := factionPreferredGov(dominantFaction.ID)
		if preferredGov != social.GovernanceType(0) && preferredGov != sett.Governance {
			mismatchDecay := highestInfluence / 100.0 * phi.Agnosis * 0.05
			sett.GovernanceScore -= mismatchDecay
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

// factionPreferredGov returns the governance type a faction prefers.
// Returns 0 (zero value) for factions with no strong preference.
func factionPreferredGov(factionID social.FactionID) social.GovernanceType {
	switch factionID {
	case 1: // Crown → Monarchy
		return social.GovMonarchy
	case 2: // Merchant's Compact → Merchant Republic
		return social.GovMerchantRepublic
	case 4: // Verdant Circle → Council (already dominant, neutral)
		return social.GovCouncil
	case 3: // Iron Brotherhood → Council (military fraternity, structured governance)
		return social.GovCouncil
	default: // Ashen Path: no preference (corruption doctrine erodes governance directly)
		return 0
	}
}

// distributeFactionPatronage distributes a fraction of each faction's treasury
// back to its members using ideology-specific weight functions.
// Budget: treasury * Agnosis * 0.1 per week (~2.36% of treasury).
func (s *Simulation) distributeFactionPatronage(tick uint64) {
	factionIndex := make(map[social.FactionID]*social.Faction)
	for _, f := range s.Factions {
		factionIndex[f.ID] = f
	}

	// Build per-faction member lists with weights.
	type weightedAgent struct {
		agent  *agents.Agent
		weight float64
	}
	factionMembers := make(map[social.FactionID][]weightedAgent)

	for _, a := range s.Agents {
		if !a.Alive || a.FactionID == nil {
			continue
		}
		fid := social.FactionID(*a.FactionID)
		w := factionPatronageWeight(fid, a)
		if w > 0 {
			factionMembers[fid] = append(factionMembers[fid], weightedAgent{a, w})
		}
	}

	for _, f := range s.Factions {
		if f.Treasury == 0 {
			continue
		}
		members := factionMembers[f.ID]
		if len(members) == 0 {
			continue
		}

		budget := uint64(float64(f.Treasury) * phi.Agnosis * 0.1)
		if budget == 0 {
			continue
		}

		// Sum weights.
		totalWeight := 0.0
		for _, m := range members {
			totalWeight += m.weight
		}
		if totalWeight == 0 {
			continue
		}

		// Distribute proportionally.
		paid := uint64(0)
		for _, m := range members {
			payment := uint64(float64(budget) * m.weight / totalWeight)
			if payment == 0 {
				continue
			}
			if paid+payment > budget {
				break
			}
			m.agent.Wealth += payment
			paid += payment
			applyPatronageNeeds(f.ID, m.agent)
		}

		f.Treasury -= paid

		s.EmitEvent(Event{
			Tick: tick,
			Description: fmt.Sprintf("%s distributes %d crowns in patronage to %d members",
				f.Name, paid, len(members)),
			Category: "economy",
			Meta: map[string]any{
				"faction_name": f.Name,
				"amount":       paid,
				"recipients":   len(members),
			},
		})
	}
}

// factionPatronageWeight returns the ideology-specific distribution weight
// for an agent within their faction. Returns 0 to exclude from distribution.
func factionPatronageWeight(factionID social.FactionID, a *agents.Agent) float64 {
	coherence := float64(a.Soul.CittaCoherence)
	wealth := float64(a.Wealth)

	switch factionID {
	case 1: // Crown — royal stipends: rewards hierarchy and inner order.
		loyaltyBonus := 1.0
		if a.Role == agents.RoleNoble || a.Role == agents.RoleLeader {
			loyaltyBonus = phi.Being
		}
		return (1.0 + coherence*phi.Being) * loyaltyBonus

	case 2: // Merchant's Compact — trade grants: invests in aspiring traders, anti-wealth bias.
		merchantBonus := 1.0
		if a.Occupation == agents.OccupationMerchant {
			merchantBonus = phi.Being
		}
		return (float64(a.Skills.Trade)*phi.Being + phi.Agnosis) * merchantBonus / (1.0 + math.Log1p(wealth)*phi.Agnosis)

	case 3: // Iron Brotherhood — military payroll: pragmatic payment for martial skill.
		soldierBonus := 0.0
		switch a.Role {
		case agents.RoleSoldier:
			soldierBonus = phi.Nous
		case agents.RoleOutlaw:
			soldierBonus = phi.Agnosis
		}
		return float64(a.Skills.Combat)*phi.Nous + soldierBonus

	case 4: // Verdant Circle — agricultural grants: nurtures land workers and inner growth.
		var producerMul float64
		switch a.Occupation {
		case agents.OccupationFarmer, agents.OccupationFisher, agents.OccupationLaborer, agents.OccupationMiner, agents.OccupationAlchemist:
			producerMul = phi.Nous
		case agents.OccupationHunter:
			producerMul = phi.Being
		case agents.OccupationSoldier, agents.OccupationMerchant:
			producerMul = phi.Agnosis * 0.5 // Token patronage — they pay dues, so they get a small share.
		default:
			producerMul = phi.Monad
		}
		return producerMul * (1.0 + coherence*phi.Psyche)

	case 5: // Ashen Path — chaos fund: anti-wealth redistribution, rewards seekers.
		antiWealth := 1.0 - math.Min(wealth/5000.0, 1.0)
		classBonus := 0.0
		switch a.Soul.Class {
		case agents.Nihilist, agents.Transcendentalist:
			classBonus = phi.Nous
		}
		if a.Role == agents.RoleOutlaw {
			classBonus += phi.Agnosis
		}
		return phi.Being*antiWealth + classBonus
	}

	return 0
}

// applyPatronageNeeds applies faction-specific needs boosts from patronage.
func applyPatronageNeeds(factionID social.FactionID, a *agents.Agent) {
	switch factionID {
	case 1: // Crown
		a.Needs.Esteem += 0.015
		a.Needs.Belonging += 0.010
		a.Needs.Safety += 0.005
	case 2: // Merchant's Compact
		a.Needs.Purpose += 0.020
		a.Needs.Esteem += 0.010
		a.Needs.Safety += 0.008
	case 3: // Iron Brotherhood
		a.Needs.Safety += 0.020
		a.Needs.Belonging += 0.015
		a.Needs.Esteem += 0.005
	case 4: // Verdant Circle
		a.Needs.Purpose += 0.020
		a.Needs.Belonging += 0.015
		a.Needs.Survival += 0.003
	case 5: // Ashen Path
		a.Needs.Belonging += 0.020
		a.Needs.Purpose += 0.015
		a.Needs.Safety -= 0.005
	}
	clampAgentNeeds(&a.Needs)
}

// applyFactionDoctrines applies weekly coherence boosts to faction members
// who fulfill their faction's philosophical doctrine. Each faction has a
// qualification criterion; qualifying agents receive a small coherence boost
// of Agnosis² × 0.1 (~0.00557/week). Over months, this creates measurable
// coherence divergence aligned with each faction's philosophical identity.
//
// Doctrines:
//   Crown (Order): settlement GovernanceScore > Psyche
//   Merchant (Exchange): active merchant (worked within 7 sim-days)
//   Iron Brotherhood (Discipline): soldier in governed settlement (GovernanceScore > Agnosis)
//   Verdant Circle (Harmony): worked recently + home hex health > Psyche
//   Ashen Path (Dissolution): wealth < 30 or belonging > Matter
func (s *Simulation) applyFactionDoctrines(tick uint64) {
	boost := float32(phi.Agnosis * phi.Agnosis * 0.1) // ~0.00557
	awakenings := 0

	for _, a := range s.Agents {
		if !a.Alive || a.FactionID == nil {
			continue
		}

		var sett *social.Settlement
		if a.HomeSettID != nil {
			sett = s.SettlementIndex[*a.HomeSettID]
		}

		if !s.agentFulfillsDoctrine(a, social.FactionID(*a.FactionID), sett, tick) {
			continue
		}

		oldState := agents.StateFromCoherence(a.Soul.CittaCoherence)
		a.Soul.AdjustCoherence(boost)
		newState := agents.StateFromCoherence(a.Soul.CittaCoherence)

		// Emit event on state transition (Tier 1+ only, avoid flooding).
		if newState != oldState && a.Tier >= 1 {
			awakenings++
			factionName := s.factionNameByID(*a.FactionID)
			s.EmitEvent(Event{
				Tick: tick,
				Description: fmt.Sprintf("%s experiences a doctrine awakening through %s teachings",
					a.Name, factionName),
				Category: "spiritual",
				Meta: map[string]any{
					"event_type":    "doctrine_awakening",
					"agent_id":      a.ID,
					"agent_name":    a.Name,
					"faction_name":  factionName,
					"settlement_id": a.HomeSettID,
				},
			})
		}
	}

	if awakenings > 0 {
		slog.Info("doctrine awakenings", "count", awakenings)
	}
}

// agentFulfillsDoctrine checks whether an agent meets their faction's doctrine.
// Used by both applyFactionDoctrines (coherence boosts) and processFactionDefection
// (defection tracking). Each faction has a philosophical criterion:
//
//	Crown (Order): settlement GovernanceScore > Psyche
//	Merchant (Exchange): active merchant (worked within 7 sim-days)
//	Iron Brotherhood (Discipline): soldier in governed settlement
//	Verdant Circle (Harmony): worked recently + home hex health > Psyche
//	Ashen Path (Dissolution): poor or deeply belonging
func (s *Simulation) agentFulfillsDoctrine(a *agents.Agent, fid social.FactionID, sett *social.Settlement, tick uint64) bool {
	switch fid {
	case 1: // Crown — Order
		return sett != nil && sett.GovernanceScore > phi.Psyche
	case 2: // Merchant's Compact — Exchange
		return a.Occupation == agents.OccupationMerchant && tick-a.LastWorkTick < TicksPerSimDay*7
	case 3: // Iron Brotherhood — Discipline
		return a.Occupation == agents.OccupationSoldier && sett != nil && sett.GovernanceScore > phi.Agnosis
	case 4: // Verdant Circle — Harmony
		if tick-a.LastWorkTick >= TicksPerSimDay*7 || sett == nil {
			return false
		}
		hex := s.WorldMap.Get(sett.Position)
		return hex != nil && hex.Health > phi.Psyche
	case 5: // Ashen Path — Dissolution
		return a.Wealth < 30 || a.Needs.Belonging > float32(phi.Matter)
	default:
		return false
	}
}

// processFactionDefection checks all faction members for consecutive doctrine
// failure. After 4+ weeks, agents have an Agnosis (~23.6%) chance per week
// to leave their faction (become unaffiliated). Creates natural faction churn —
// factions that don't serve their members shrink.
func (s *Simulation) processFactionDefection(tick uint64) {
	defections := 0

	for _, a := range s.Agents {
		if !a.Alive || a.FactionID == nil {
			delete(s.DoctrineFailWeeks, a.ID)
			continue
		}

		var sett *social.Settlement
		if a.HomeSettID != nil {
			sett = s.SettlementIndex[*a.HomeSettID]
		}

		fid := social.FactionID(*a.FactionID)
		if s.agentFulfillsDoctrine(a, fid, sett, tick) {
			delete(s.DoctrineFailWeeks, a.ID)
			continue
		}

		// Increment failure counter.
		s.DoctrineFailWeeks[a.ID]++
		failWeeks := s.DoctrineFailWeeks[a.ID]

		if failWeeks < 4 {
			continue
		}

		// Defection check: Agnosis probability per week.
		// Deterministic hash matches existing mortality pattern.
		hash := (uint64(a.ID)*2654435761 + tick*40503) % 100000
		if float64(hash)/100000.0 >= phi.Agnosis {
			continue
		}

		// Defect — get faction name before clearing.
		factionName := s.factionNameByID(*a.FactionID)
		a.FactionID = nil
		delete(s.DoctrineFailWeeks, a.ID)
		defections++

		if a.Tier >= 1 {
			s.EmitEvent(Event{
				Tick: tick,
				Description: fmt.Sprintf("%s left %s after failing to fulfill its doctrine",
					a.Name, factionName),
				Category: "political",
				Meta: map[string]any{
					"event_type":   "faction_defection",
					"agent_id":     a.ID,
					"agent_name":   a.Name,
					"faction_name": factionName,
				},
			})
		}
	}

	if defections > 0 {
		slog.Info("faction defection", "defections", defections)
	}
}

// processFactionRecruitmentByInfluence lets factions compete for unaffiliated
// agents in settlements where they have influence. Agents are more likely to
// join factions matching their natural affinity (from factionForAgent).
// Complements existing relationship-based recruitment in processFactionRecruitment.
// Capped at Agnosis fraction of unaffiliated agents per settlement per week.
func (s *Simulation) processFactionRecruitmentByInfluence(tick uint64) {
	recruited := 0

	// Diagnostic counters to understand recruitment behavior.
	settsTotal := 0
	settsWithUnaff := 0
	settsSkippedNoInflu := 0
	totalUnaffSeen := 0
	totalCapSum := 0
	hashPassed := 0
	hashFailed := 0

	for _, sett := range s.Settlements {
		settsTotal++
		settAgents := s.SettlementAgents[sett.ID]

		// Collect unaffiliated alive adults.
		var unaffiliated []*agents.Agent
		for _, a := range settAgents {
			if a.Alive && a.FactionID == nil && a.Age >= 16 {
				unaffiliated = append(unaffiliated, a)
			}
		}
		if len(unaffiliated) == 0 {
			continue
		}
		settsWithUnaff++
		totalUnaffSeen += len(unaffiliated)

		// Recruitment cap: Agnosis fraction, min 1.
		cap := int(math.Max(1, float64(len(unaffiliated))*phi.Agnosis))

		// Collect factions with local influence.
		type factionScore struct {
			fid       social.FactionID
			influence float64
		}
		var active []factionScore
		totalInfluence := 0.0
		for _, f := range s.Factions {
			if inf, ok := f.Influence[sett.ID]; ok && inf > 0 {
				active = append(active, factionScore{f.ID, inf})
				totalInfluence += inf
			}
		}
		if len(active) == 0 || totalInfluence == 0 {
			settsSkippedNoInflu++
			continue
		}
		totalCapSum += cap

		settRecruited := 0
		for _, a := range unaffiliated {
			if settRecruited >= cap {
				break
			}

			// Determine natural affinity via the same logic used at birth.
			affinityFID := factionForAgent(a, sett.Governance)

			// Each faction's score = influence × affinity bonus.
			// Being bonus for matching natural affinity, Monad otherwise.
			bestScore := 0.0
			var bestFID social.FactionID
			for _, fs := range active {
				score := fs.influence
				if fs.fid == affinityFID {
					score *= phi.Being
				}
				if score > bestScore {
					bestScore = score
					bestFID = fs.fid
				}
			}

			if bestFID == 0 {
				continue
			}

			// Recruitment probability scales with faction's share of local influence.
			// Fully-dominated settlement + affinity match: prob = Being × Psyche ≈ Matter (62%).
			// Fully-dominated + no affinity: prob = Psyche (38%).
			// Typical (50% share + affinity): ~31%.
			// Typical (50% share + no affinity): ~19%.
			prob := bestScore / totalInfluence * phi.Psyche

			// Deterministic check.
			hash := (uint64(a.ID)*2654435761 + tick*40503 + sett.ID*7) % 100000
			if float64(hash)/100000.0 >= prob {
				hashFailed++
				continue
			}
			hashPassed++

			// Recruit.
			factionID := uint64(bestFID)
			a.FactionID = &factionID
			settRecruited++
			recruited++

			if a.Tier >= 1 {
				factionName := s.factionNameByID(factionID)
				s.EmitEvent(Event{
					Tick: tick,
					Description: fmt.Sprintf("%s joined %s through local influence in %s",
						a.Name, factionName, sett.Name),
					Category: "political",
					Meta: map[string]any{
						"event_type":      "faction_recruitment",
						"agent_id":        a.ID,
						"agent_name":      a.Name,
						"faction_name":    factionName,
						"settlement_name": sett.Name,
					},
				})
			}
		}
	}

	// Diagnostic log: always emitted so we can see what's happening even when recruited=0.
	slog.Info("recruitment stats",
		"recruited", recruited,
		"setts_total", settsTotal,
		"setts_with_unaff", settsWithUnaff,
		"setts_no_influ", settsSkippedNoInflu,
		"total_unaff_seen", totalUnaffSeen,
		"cap_sum", totalCapSum,
		"hash_passed", hashPassed,
		"hash_failed", hashFailed,
	)

	if recruited > 0 {
		slog.Info("influence-based recruitment", "recruited", recruited)
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

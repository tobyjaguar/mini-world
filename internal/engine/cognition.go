// Tier 2 cognition processing — bridges LLM decisions into simulation effects.
// See design doc Section 4.2 and Section 8.5 (call budget).
package engine

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processTier2Decisions runs LLM-powered decisions for a batch of Tier 2 agents.
// Called daily, processing ~5 agents per day to spread the ~30 agents across a week.
func (s *Simulation) processTier2Decisions(tick uint64) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return
	}

	// Collect living Tier 2 agents.
	var tier2 []*agents.Agent
	for _, a := range s.Agents {
		if a.Alive && a.Tier == agents.Tier2 {
			tier2 = append(tier2, a)
		}
	}

	if len(tier2) == 0 {
		return
	}

	// Stagger: process a slice of agents each day.
	// Day number within the week (0-6).
	dayInWeek := (tick / TicksPerSimDay) % 7
	batchSize := (len(tier2) + 6) / 7 // Ceiling division
	start := int(dayInWeek) * batchSize
	if start >= len(tier2) {
		return
	}
	end := start + batchSize
	if end > len(tier2) {
		end = len(tier2)
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}
	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}
	stateNames := map[agents.StateOfBeing]string{
		agents.Embodied: "Embodied", agents.Centered: "Centered", agents.Liberated: "Liberated",
	}

	for _, a := range tier2[start:end] {
		ctx := s.buildTier2Context(a, occNames, govNames, stateNames)
		decisions, err := llm.GenerateTier2Decision(s.LLM, ctx)
		if err != nil {
			slog.Debug("tier 2 decision failed, falling back to tier 0",
				"agent", a.Name, "error", err)
			continue
		}

		for _, d := range decisions {
			s.applyTier2Decision(a, d, tick)
		}
	}
}

func (s *Simulation) buildTier2Context(a *agents.Agent, occNames []string, govNames map[uint8]string, stateNames map[agents.StateOfBeing]string) *llm.Tier2Context {
	occName := "Unknown"
	if int(a.Occupation) < len(occNames) {
		occName = occNames[a.Occupation]
	}

	moodDesc := "content"
	switch {
	case a.Wellbeing.EffectiveMood > 0.5:
		moodDesc = "elated"
	case a.Wellbeing.EffectiveMood > 0.2:
		moodDesc = "content"
	case a.Wellbeing.EffectiveMood > -0.2:
		moodDesc = "uneasy"
	case a.Wellbeing.EffectiveMood > -0.5:
		moodDesc = "anxious"
	default:
		moodDesc = "despairing"
	}

	ctx := &llm.Tier2Context{
		Name:       a.Name,
		Age:        a.Age,
		Occupation: occName,
		Wealth:     a.Wealth,
		Mood:       moodDesc,
		Coherence:  a.Soul.CittaCoherence,
		State:      stateNames[a.Soul.State],
		Archetype:  a.Archetype,
		Season:     SeasonName(s.CurrentSeason),
		Faction:    "unaffiliated",
	}

	// Settlement context.
	if a.HomeSettID != nil {
		if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
			ctx.Settlement = sett.Name
			ctx.Governance = govNames[uint8(sett.Governance)]
			ctx.Treasury = sett.Treasury
		}
	}

	// Weather context.
	if s.CurrentWeather.Description != "" {
		ctx.Weather = s.CurrentWeather.Description
	}

	// Recent memories (up to 10).
	recent := agents.RecentMemories(a, 10)
	for _, m := range recent {
		ctx.Memories = append(ctx.Memories, m.Content)
	}

	// Top 5 relationships by absolute sentiment.
	if len(a.Relationships) > 0 {
		rels := make([]agents.Relationship, len(a.Relationships))
		copy(rels, a.Relationships)
		sort.Slice(rels, func(i, j int) bool {
			ai := rels[i].Sentiment
			if ai < 0 {
				ai = -ai
			}
			aj := rels[j].Sentiment
			if aj < 0 {
				aj = -aj
			}
			return ai > aj
		})
		for i, r := range rels {
			if i >= 5 {
				break
			}
			target, ok := s.AgentIndex[r.TargetID]
			if !ok {
				continue
			}
			ctx.Relationships = append(ctx.Relationships,
				fmt.Sprintf("%s (sentiment: %.1f, trust: %.1f)", target.Name, r.Sentiment, r.Trust))
		}
	}

	// Faction.
	if a.FactionID != nil {
		for _, f := range s.Factions {
			if uint64(f.ID) == *a.FactionID {
				ctx.Faction = f.Name
				break
			}
		}
	}

	// Merchant-specific trade context.
	if a.Occupation == agents.OccupationMerchant {
		ctx.TradeContext = s.buildMerchantTradeContext(a)
	}

	// Round 24: Resource/career context for all agents.
	ctx.ResourceAvailability = s.buildResourceAvailability(a)
	ctx.OccupationSatisfaction = s.buildOccupationSatisfaction(a)
	ctx.SkillSummary = fmt.Sprintf("Farming %.2f, Mining %.2f, Crafting %.2f, Combat %.2f, Trade %.2f",
		a.Skills.Farming, a.Skills.Mining, a.Skills.Crafting, a.Skills.Combat, a.Skills.Trade)

	return ctx
}

// buildMerchantTradeContext provides real market data for merchant LLM decisions.
func (s *Simulation) buildMerchantTradeContext(a *agents.Agent) string {
	if a.HomeSettID == nil {
		return ""
	}
	sett, ok := s.SettlementIndex[*a.HomeSettID]
	if !ok || sett.Market == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("Trade Intelligence:\n")

	// Home market prices.
	b.WriteString("Home market prices: ")
	first := true
	for good, entry := range sett.Market.Entries {
		if !first {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s %.1f", goodName(good), entry.Price)
		first = false
	}
	b.WriteString("\n")

	// Best margins to nearby settlements.
	b.WriteString("Nearby trade routes:\n")
	for _, other := range s.Settlements {
		if other.ID == sett.ID || other.Market == nil {
			continue
		}
		dist := world.Distance(sett.Position, other.Position)
		if dist > 5 {
			continue
		}
		bestMargin := 0.0
		bestGood := agents.GoodType(0)
		for good, homeEntry := range sett.Market.Entries {
			destEntry, ok := other.Market.Entries[good]
			if !ok || homeEntry.Price < 1 {
				continue
			}
			margin := (destEntry.Price - homeEntry.Price) / homeEntry.Price
			if margin > bestMargin {
				bestMargin = margin
				bestGood = good
			}
		}
		if bestMargin > 0 {
			tc := routeCost(sett.Position, other.Position, s.WorldMap)
			fmt.Fprintf(&b, "- %s (dist %d, travel %d ticks): best margin %.0f%% on %s\n",
				other.Name, dist, tc, bestMargin*100, goodName(bestGood))
		}
	}

	// Current status.
	if a.TravelTicksLeft > 0 {
		b.WriteString("Status: Currently traveling\n")
	} else if !a.TradeCargo.IsEmpty() {
		b.WriteString("Status: Carrying cargo\n")
	} else {
		b.WriteString("Status: At home, ready to trade\n")
	}
	fmt.Fprintf(&b, "Trade skill: %.2f, Wealth: %d crowns\n", a.Skills.Trade, a.Wealth)

	return b.String()
}

func (s *Simulation) applyTier2Decision(a *agents.Agent, d llm.Tier2Decision, tick uint64) {
	switch d.Action {
	case "work":
		// Resource producers work the healthiest hex in their settlement neighborhood.
		hex := s.bestProductionHex(a)
		boostMul := 1.0
		if a.HomeSettID != nil {
			boostMul = s.GetSettlementBoost(*a.HomeSettID)
		}
		workAction := agents.Action{Kind: agents.ActionWork}
		ResolveWork(a, workAction, hex, tick, boostMul)

	case "trade":
		// Sell surplus to settlement treasury — closed transfer, no crowns minted.
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				tier2MarketSell(a, sett)
			}
		}

	case "socialize":
		// Boost relationship with target agent (find by name).
		for i := range a.Relationships {
			target, ok := s.AgentIndex[a.Relationships[i].TargetID]
			if ok && target.Name == d.Target {
				a.Relationships[i].Sentiment += 0.1
				if a.Relationships[i].Sentiment > 1.0 {
					a.Relationships[i].Sentiment = 1.0
				}
				a.Relationships[i].Trust += 0.05
				if a.Relationships[i].Trust > 1.0 {
					a.Relationships[i].Trust = 1.0
				}
				break
			}
		}
		a.Needs.Belonging += 0.1

	case "advocate":
		// Push faction policy in settlement — nudge governance score or tax rate.
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				// Small nudge based on coherence (more coherent = more influence).
				influence := float64(a.Soul.CittaCoherence) * 0.02
				sett.GovernanceScore += influence
				if sett.GovernanceScore > 1.0 {
					sett.GovernanceScore = 1.0
				}
			}
		}

	case "invest":
		// Spend wealth on settlement treasury.
		investment := a.Wealth / 10
		if investment > 0 {
			a.Wealth -= investment
			if a.HomeSettID != nil {
				if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
					sett.Treasury += investment
				}
			}
		}

	case "recruit":
		// Try to recruit a named agent to faction.
		if a.FactionID != nil {
			for _, candidate := range s.Agents {
				if candidate.Alive && candidate.Name == d.Target && candidate.FactionID == nil {
					fid := *a.FactionID
					candidate.FactionID = &fid
					factionName := s.agentFactionName(a)
					s.EmitEvent(Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s recruited %s to %s", a.Name, candidate.Name, factionName),
						Category:    "social",
						Meta: map[string]any{
							"agent_id":      a.ID,
							"agent_name":    a.Name,
							"target_name":   candidate.Name,
							"settlement_id": a.HomeSettID,
							"faction_name":  factionName,
						},
					})
					break
				}
			}
		}

	case "scout_route":
		if a.Occupation == agents.OccupationMerchant {
			// Find the named settlement and store as preferred destination.
			for _, sett := range s.Settlements {
				if sett.Name == d.Target {
					id := sett.ID
					a.TradePreferredDest = &id
					break
				}
			}
			a.Needs.Purpose += 0.05
			a.Skills.Trade += 0.005
		}

	case "speak":
		// Generate a quote — stored as an event for narrative color.
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s declares: \"%s\"", a.Name, d.Target),
			Category:    "social",
			Meta: map[string]any{
				"agent_id":      a.ID,
				"agent_name":    a.Name,
				"settlement_id": a.HomeSettID,
			},
		})

	case "relocate":
		// Move to named settlement, keep occupation.
		for _, sett := range s.Settlements {
			if sett.Name == d.Target {
				newID := sett.ID
				a.HomeSettID = &newID
				a.Position = sett.Position
				s.rebuildSettlementAgents()
				agents.AddMemory(a, tick, fmt.Sprintf("I relocated to %s seeking better resources for my work", sett.Name), 0.7)
				break
			}
		}

	case "retrain":
		// Skill-adjacent occupation change.
		newOcc := parseOccupationName(d.Target)
		if newOcc != a.Occupation {
			adj := skillAdjacentOccupation(a.Occupation)
			// Only allow skill-adjacent transitions or to Crafter as last resort.
			if newOcc == adj || newOcc == agents.OccupationCrafter {
				old := a.Occupation
				a.Occupation = newOcc
				setMinimumSkill(a, newOcc)
				agents.AddMemory(a, tick,
					fmt.Sprintf("I retrained from %s to %s — a new chapter begins", occupationLabel(old), occupationLabel(newOcc)), 0.8)
			}
		}
	}

	// Log the decision and create a memory.
	detail := fmt.Sprintf("%s decided to %s (%s): %s", a.Name, d.Action, d.Target, d.Reasoning)
	agents.AddMemory(a, tick, detail, 0.5)
	s.EmitEvent(Event{
		Tick:        tick,
		Description: detail,
		Category:    "agent",
		Meta: map[string]any{
			"agent_id":      a.ID,
			"agent_name":    a.Name,
			"occupation":    a.Occupation,
			"action":        d.Action,
			"settlement_id": a.HomeSettID,
		},
	})

	slog.Debug("tier 2 action", "agent", a.Name, "action", d.Action, "target", d.Target)
}

// processOracleVisions runs weekly LLM visions for Liberated agents.
// ~5 agents max — no batching needed.
func (s *Simulation) processOracleVisions(tick uint64) {
	if s.LLM == nil || !s.LLM.Enabled() {
		return
	}

	// Collect living Liberated Tier 2 agents — these are named characters
	// with individual LLM cognition. ~5 agents, so ~5 Haiku calls/week.
	var oracles []*agents.Agent
	for _, a := range s.Agents {
		if a.Alive && a.Tier == agents.Tier2 && a.Soul.State == agents.Liberated {
			oracles = append(oracles, a)
		}
	}

	if len(oracles) == 0 {
		return
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}
	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}
	stateNames := map[agents.StateOfBeing]string{
		agents.Embodied: "Embodied", agents.Centered: "Centered", agents.Liberated: "Liberated",
	}
	elementNames := map[agents.ElementType]string{
		agents.ElementHelium: "Helium", agents.ElementHydrogen: "Hydrogen",
		agents.ElementGold: "Gold", agents.ElementUranium: "Uranium",
	}

	gini := s.GiniCoefficient()

	for _, a := range oracles {
		ctx := s.buildOracleContext(a, occNames, govNames, stateNames, elementNames, gini)
		vision, err := llm.GenerateOracleVision(s.LLM, ctx)
		if err != nil {
			slog.Debug("oracle vision failed", "agent", a.Name, "error", err)
			continue
		}

		s.applyOracleVision(a, vision, tick)
	}

	slog.Info("oracle visions processed", "count", len(oracles))
}

func (s *Simulation) buildOracleContext(
	a *agents.Agent,
	occNames []string,
	govNames map[uint8]string,
	stateNames map[agents.StateOfBeing]string,
	elementNames map[agents.ElementType]string,
	gini float64,
) *llm.OracleContext {
	occName := "Unknown"
	if int(a.Occupation) < len(occNames) {
		occName = occNames[a.Occupation]
	}

	ctx := &llm.OracleContext{
		Name:      a.Name,
		Age:       a.Age,
		Wealth:    a.Wealth,
		Coherence: a.Soul.CittaCoherence,
		State:     stateNames[a.Soul.State],
		Element:   elementNames[a.Soul.Element()],
		Archetype: a.Archetype,
		Season:    SeasonName(s.CurrentSeason),
		Faction:   "unaffiliated",
		AvgMood:   s.Stats.AvgMood,
		Gini:      gini,
		Occupation: occName,
	}

	// Settlement context.
	if a.HomeSettID != nil {
		if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
			ctx.Settlement = sett.Name
			ctx.Governance = govNames[uint8(sett.Governance)]
			ctx.Treasury = sett.Treasury
			ctx.Population = len(s.SettlementAgents[sett.ID])
		}
	}

	// Weather.
	if s.CurrentWeather.Description != "" {
		ctx.Weather = s.CurrentWeather.Description
	}

	// Top 10 important memories (oracles draw from depth, not recency).
	important := agents.ImportantMemories(a, 10)
	for _, m := range important {
		ctx.Memories = append(ctx.Memories, m.Content)
	}

	// Top 5 relationships by absolute sentiment.
	if len(a.Relationships) > 0 {
		rels := make([]agents.Relationship, len(a.Relationships))
		copy(rels, a.Relationships)
		sort.Slice(rels, func(i, j int) bool {
			ai := rels[i].Sentiment
			if ai < 0 {
				ai = -ai
			}
			aj := rels[j].Sentiment
			if aj < 0 {
				aj = -aj
			}
			return ai > aj
		})
		for i, r := range rels {
			if i >= 5 {
				break
			}
			target, ok := s.AgentIndex[r.TargetID]
			if !ok {
				continue
			}
			ctx.Relationships = append(ctx.Relationships,
				fmt.Sprintf("%s (sentiment: %.1f, trust: %.1f)", target.Name, r.Sentiment, r.Trust))
		}
	}

	// Faction.
	if a.FactionID != nil {
		for _, f := range s.Factions {
			if uint64(f.ID) == *a.FactionID {
				ctx.Faction = f.Name
				break
			}
		}
	}

	// Round 24: Workforce data for guide_migration.
	ctx.WorkforceData = s.buildOracleWorkforceData(a)

	return ctx
}

func (s *Simulation) applyOracleVision(a *agents.Agent, vision *llm.OracleVision, tick uint64) {
	// Record the prophecy as a high-importance memory for the oracle.
	prophecyMemory := fmt.Sprintf("Vision: %s", vision.Prophecy)
	agents.AddMemory(a, tick, prophecyMemory, 0.9)

	// Spread prophecy to Centered and Liberated agents in the same settlement.
	if a.HomeSettID != nil {
		for _, other := range s.SettlementAgents[*a.HomeSettID] {
			if other.Alive && other.ID != a.ID && other.Soul.State >= agents.Centered {
				spreadMemory := fmt.Sprintf("%s spoke a vision: %s", a.Name, vision.Prophecy)
				agents.AddMemory(other, tick, spreadMemory, 0.7)
			}
		}
	}

	// Apply the oracle's action.
	switch vision.Action {
	case "trade":
		// Sell surplus to settlement treasury — closed transfer, no crowns minted.
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				tier2MarketSell(a, sett)
			}
		}

	case "advocate":
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				influence := float64(a.Soul.CittaCoherence) * 0.02
				sett.GovernanceScore += influence
				if sett.GovernanceScore > 1.0 {
					sett.GovernanceScore = 1.0
				}
			}
		}

	case "invest":
		investment := a.Wealth / 10
		if investment > 0 {
			a.Wealth -= investment
			if a.HomeSettID != nil {
				if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
					sett.Treasury += investment
				}
			}
		}

	case "speak":
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s prophesies: \"%s\"", a.Name, vision.Target),
			Category:    "oracle",
			Meta: map[string]any{
				"agent_id":      a.ID,
				"agent_name":    a.Name,
				"settlement_id": a.HomeSettID,
			},
		})

	case "bless":
		if a.HomeSettID != nil {
			for _, candidate := range s.SettlementAgents[*a.HomeSettID] {
				if candidate.Alive && candidate.Name == vision.Target {
					nudge := float32(phi.Agnosis * 0.1) // ~0.024
					candidate.Soul.CittaCoherence += nudge
					if candidate.Soul.CittaCoherence > 0.7 {
						candidate.Soul.CittaCoherence = 0.7 // Can't create Liberated via blessing
					}
					candidate.Soul.UpdateState()
					s.EmitEvent(Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s blessed %s, nudging their coherence toward clarity", a.Name, candidate.Name),
						Category:    "oracle",
						Meta: map[string]any{
							"agent_id":      a.ID,
							"agent_name":    a.Name,
							"target_name":   candidate.Name,
							"settlement_id": a.HomeSettID,
						},
					})
					break
				}
			}
		}

	case "guide_migration":
		// Oracle directs struggling producers to a named settlement with better resources.
		if a.HomeSettID != nil {
			var targetSett *social.Settlement
			for _, sett := range s.Settlements {
				if sett.Name == vision.Target {
					targetSett = sett
					break
				}
			}
			if targetSett != nil {
				guided := 0
				for _, candidate := range s.SettlementAgents[*a.HomeSettID] {
					if guided >= 10 {
						break
					}
					if !candidate.Alive || candidate.ID == a.ID {
						continue
					}
					if candidate.Wellbeing.Satisfaction >= 0 {
						continue // Only guide dissatisfied producers.
					}
					_, isProducer := occupationResource[candidate.Occupation]
					if !isProducer {
						continue
					}
					newID := targetSett.ID
					candidate.HomeSettID = &newID
					candidate.Position = targetSett.Position
					guided++
				}
				if guided > 0 {
					s.rebuildSettlementAgents()
					s.EmitEvent(Event{
						Tick:        tick,
						Description: fmt.Sprintf("Oracle %s guided %d struggling workers to %s", a.Name, guided, targetSett.Name),
						Category:    "oracle",
						Meta: map[string]any{
							"agent_id":        a.ID,
							"agent_name":      a.Name,
							"settlement_id":   targetSett.ID,
							"settlement_name": targetSett.Name,
							"count":           guided,
						},
					})
				}
			}
		}
	}

	// Log the oracle's action.
	detail := fmt.Sprintf("Oracle %s received a vision and chose to %s (%s): %s",
		a.Name, vision.Action, vision.Target, vision.Reasoning)
	agents.AddMemory(a, tick, detail, 0.6)
	s.EmitEvent(Event{
		Tick:        tick,
		Description: fmt.Sprintf("Oracle %s: \"%s\" — %s %s", a.Name, vision.Prophecy, vision.Action, vision.Target),
		Category:    "oracle",
		Meta: map[string]any{
			"agent_id":      a.ID,
			"agent_name":    a.Name,
			"settlement_id": a.HomeSettID,
		},
	})

	slog.Debug("oracle vision applied", "agent", a.Name, "action", vision.Action, "target", vision.Target)
}

// --- Round 24: Context builders for Tier 2 resource/career decisions ---

// buildResourceAvailability describes the local resource situation for an agent.
func (s *Simulation) buildResourceAvailability(a *agents.Agent) string {
	resType, isProducer := occupationResource[a.Occupation]
	if !isProducer {
		return ""
	}
	if a.HomeSettID == nil {
		return "No home settlement"
	}
	sett, ok := s.SettlementIndex[*a.HomeSettID]
	if !ok {
		return ""
	}

	hasLocal := s.settlementHasResource(sett, resType)
	idleDays := uint64(0)
	if s.LastTick > a.LastWorkTick {
		idleDays = (s.LastTick - a.LastWorkTick) / uint64(TicksPerSimDay)
	}

	var b strings.Builder
	if hasLocal {
		fmt.Fprintf(&b, "Local %s available. ", resourceName(resType))
	} else {
		fmt.Fprintf(&b, "No %s in settlement neighborhood. ", resourceName(resType))
	}
	if idleDays > 7 {
		fmt.Fprintf(&b, "Idle for %d days. ", idleDays)
	}

	// List nearby settlements with compatible resources.
	nearby := s.findResourceSettlement(sett, resType, 5)
	if nearby != nil {
		fmt.Fprintf(&b, "Nearby: %s has %s (%d hexes away).",
			nearby.Name, resourceName(resType), world.Distance(sett.Position, nearby.Position))
	}
	return b.String()
}

// buildOccupationSatisfaction describes how well the agent's occupation is working.
func (s *Simulation) buildOccupationSatisfaction(a *agents.Agent) string {
	if a.Wellbeing.Satisfaction > 0.3 {
		return "thriving"
	} else if a.Wellbeing.Satisfaction > 0 {
		return "adequate"
	} else if a.Wellbeing.Satisfaction > -0.3 {
		return "struggling"
	}
	return "suffering"
}

// buildOracleWorkforceData provides workforce information for oracle guide_migration decisions.
func (s *Simulation) buildOracleWorkforceData(a *agents.Agent) string {
	if a.HomeSettID == nil {
		return ""
	}
	sett, ok := s.SettlementIndex[*a.HomeSettID]
	if !ok {
		return ""
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}

	// Count occupations in home settlement.
	occCounts := make(map[agents.Occupation]int)
	dissatisfied := 0
	settAgents := s.SettlementAgents[sett.ID]
	for _, ag := range settAgents {
		if ag.Alive {
			occCounts[ag.Occupation]++
			if ag.Wellbeing.Satisfaction < 0 {
				dissatisfied++
			}
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s workforce (%d agents, %d dissatisfied): ", sett.Name, len(settAgents), dissatisfied)
	for occ := agents.Occupation(0); int(occ) < len(occNames); occ++ {
		if occCounts[occ] > 0 {
			fmt.Fprintf(&b, "%s %d, ", occNames[occ], occCounts[occ])
		}
	}
	b.WriteString("\n")

	// List nearby settlements with abundant resources.
	for _, other := range s.Settlements {
		if other.ID == sett.ID || other.Population < 25 {
			continue
		}
		dist := world.Distance(sett.Position, other.Position)
		if dist > 5 {
			continue
		}
		fmt.Fprintf(&b, "- %s (%d hexes, pop %d)", other.Name, dist, other.Population)
		hex := s.WorldMap.Get(other.Position)
		if hex != nil {
			for _, rt := range []world.ResourceType{world.ResourceGrain, world.ResourceIronOre, world.ResourceFish, world.ResourceFurs, world.ResourceStone, world.ResourceHerbs} {
				if hex.Resources[rt] >= 10.0 {
					fmt.Fprintf(&b, " %s", resourceName(rt))
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// parseOccupationName converts a string to an Occupation constant.
func parseOccupationName(name string) agents.Occupation {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "farm"):
		return agents.OccupationFarmer
	case strings.Contains(lower, "mine") || lower == "miner":
		return agents.OccupationMiner
	case strings.Contains(lower, "craft"):
		return agents.OccupationCrafter
	case strings.Contains(lower, "merch") || strings.Contains(lower, "trade"):
		return agents.OccupationMerchant
	case strings.Contains(lower, "sold") || strings.Contains(lower, "milit"):
		return agents.OccupationSoldier
	case strings.Contains(lower, "schol"):
		return agents.OccupationScholar
	case strings.Contains(lower, "alch"):
		return agents.OccupationAlchemist
	case strings.Contains(lower, "labor"):
		return agents.OccupationLaborer
	case strings.Contains(lower, "fish"):
		return agents.OccupationFisher
	case strings.Contains(lower, "hunt"):
		return agents.OccupationHunter
	default:
		return agents.OccupationCrafter // Safe default
	}
}

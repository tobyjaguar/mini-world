// Tier 2 cognition processing — bridges LLM decisions into simulation effects.
// See design doc Section 4.2 and Section 8.5 (call budget).
package engine

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/phi"
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
	case a.Mood > 0.5:
		moodDesc = "elated"
	case a.Mood > 0.2:
		moodDesc = "content"
	case a.Mood > -0.2:
		moodDesc = "uneasy"
	case a.Mood > -0.5:
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

	return ctx
}

func (s *Simulation) applyTier2Decision(a *agents.Agent, d llm.Tier2Decision, tick uint64) {
	switch d.Action {
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
					s.Events = append(s.Events, Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s recruited %s to their faction", a.Name, candidate.Name),
						Category:    "social",
					})
					break
				}
			}
		}

	case "speak":
		// Generate a quote — stored as an event for narrative color.
		s.Events = append(s.Events, Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s declares: \"%s\"", a.Name, d.Target),
			Category:    "social",
		})
	}

	// Log the decision and create a memory.
	detail := fmt.Sprintf("%s decided to %s (%s): %s", a.Name, d.Action, d.Target, d.Reasoning)
	agents.AddMemory(a, tick, detail, 0.5)
	s.Events = append(s.Events, Event{
		Tick:        tick,
		Description: detail,
		Category:    "agent",
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
		s.Events = append(s.Events, Event{
			Tick:        tick,
			Description: fmt.Sprintf("%s prophesies: \"%s\"", a.Name, vision.Target),
			Category:    "oracle",
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
					s.Events = append(s.Events, Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s blessed %s, nudging their coherence toward clarity", a.Name, candidate.Name),
						Category:    "oracle",
					})
					break
				}
			}
		}
	}

	// Log the oracle's action.
	detail := fmt.Sprintf("Oracle %s received a vision and chose to %s (%s): %s",
		a.Name, vision.Action, vision.Target, vision.Reasoning)
	agents.AddMemory(a, tick, detail, 0.6)
	s.Events = append(s.Events, Event{
		Tick:        tick,
		Description: fmt.Sprintf("Oracle %s: \"%s\" — %s %s", a.Name, vision.Prophecy, vision.Action, vision.Target),
		Category:    "oracle",
	})

	slog.Debug("oracle vision applied", "agent", a.Name, "action", vision.Action, "target", vision.Target)
}

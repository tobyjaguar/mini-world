// Tier 2 cognition processing — bridges LLM decisions into simulation effects.
// See design doc Section 4.2 and Section 8.5 (call budget).
package engine

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/llm"
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
		agents.Torment: "in Torment", agents.WellBeing: "in WellBeing", agents.Liberation: "Liberated",
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
		// Simulate a market transaction — buy or sell at local market.
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				if sett.Market != nil {
					// Simple trade: earn some wealth from trade skill.
					earned := uint64(a.Skills.Trade*5) + 2
					a.Wealth += earned
					a.Skills.Trade += 0.005
				}
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

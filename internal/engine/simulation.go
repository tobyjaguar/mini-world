// Simulation ties together all world systems and runs them each tick.
package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// Simulation holds the complete world state and wires systems together.
type Simulation struct {
	WorldMap    *world.Map
	Agents      []*agents.Agent
	AgentIndex  map[agents.AgentID]*agents.Agent
	Settlements []*social.Settlement
	Events      []Event // Recent events (ring buffer in production)
	LastTick    uint64  // Most recent tick processed

	// Statistics tracked per day.
	Stats SimStats
}

// CurrentTick returns the most recently processed tick number.
func (s *Simulation) CurrentTick() uint64 {
	return s.LastTick
}

// Event is a notable occurrence in the world.
type Event struct {
	Tick        uint64 `json:"tick"`
	Description string `json:"description"`
	Category    string `json:"category"` // "economy", "social", "death", "birth", etc.
}

// SimStats tracks aggregate world statistics.
type SimStats struct {
	TotalPopulation int     `json:"total_population"`
	TotalWealth     uint64  `json:"total_wealth"`
	Deaths          int     `json:"deaths"`
	Births          int     `json:"births"`
	AvgMood         float32 `json:"avg_mood"`
	AvgSurvival     float32 `json:"avg_survival"`
}

// NewSimulation creates a Simulation from generated components.
func NewSimulation(m *world.Map, ag []*agents.Agent, setts []*social.Settlement) *Simulation {
	index := make(map[agents.AgentID]*agents.Agent, len(ag))
	for _, a := range ag {
		index[a.ID] = a
	}
	return &Simulation{
		WorldMap:    m,
		Agents:      ag,
		AgentIndex:  index,
		Settlements: setts,
	}
}

// TickMinute runs every tick (1 sim-minute): agent decisions and need decay.
func (s *Simulation) TickMinute(tick uint64) {
	s.LastTick = tick
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		// Decay needs (passage of time).
		agents.DecayNeeds(a)

		// Agent decides and acts.
		action := agents.Tier0Decide(a)
		events := agents.ApplyAction(a, action)

		// Record notable events.
		for _, desc := range events {
			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: desc,
				Category:    "agent",
			})
		}

		// Check for death.
		if !a.Alive {
			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s has died", a.Name),
				Category:    "death",
			})
		}
	}
}

// TickHour runs every sim-hour: market updates, weather checks.
func (s *Simulation) TickHour(tick uint64) {
	// Placeholder — market resolution will go here.
}

// TickDay runs every sim-day: statistics, daily summary.
func (s *Simulation) TickDay(tick uint64) {
	s.updateStats()

	slog.Info("daily report",
		"tick", tick,
		"time", SimTime(tick),
		"alive", s.Stats.TotalPopulation,
		"deaths", s.Stats.Deaths,
		"avg_mood", fmt.Sprintf("%.3f", s.Stats.AvgMood),
		"avg_survival", fmt.Sprintf("%.3f", s.Stats.AvgSurvival),
		"total_wealth", s.Stats.TotalWealth,
	)

	// Log recent notable events.
	recentStart := 0
	if len(s.Events) > 10 {
		recentStart = len(s.Events) - 10
	}
	for _, e := range s.Events[recentStart:] {
		if e.Category == "death" {
			slog.Info("event", "category", e.Category, "description", e.Description)
		}
	}
}

// TickWeek runs every sim-week: faction updates, diplomatic cycles.
func (s *Simulation) TickWeek(tick uint64) {
	slog.Info("weekly summary",
		"tick", tick,
		"time", SimTime(tick),
		"events_this_week", len(s.Events),
	)
	// Trim old events to prevent unbounded growth (keep last 1000).
	if len(s.Events) > 1000 {
		s.Events = s.Events[len(s.Events)-1000:]
	}
}

// TickSeason runs every sim-season: harvests, seasonal effects.
func (s *Simulation) TickSeason(tick uint64) {
	slog.Info("season change",
		"tick", tick,
		"time", SimTime(tick),
		"population", s.Stats.TotalPopulation,
	)
}

func (s *Simulation) updateStats() {
	alive := 0
	totalWealth := uint64(0)
	totalMood := float32(0)
	totalSurvival := float32(0)
	deaths := 0

	for _, a := range s.Agents {
		if a.Alive {
			alive++
			totalWealth += a.Wealth
			totalMood += a.Mood
			totalSurvival += a.Needs.Survival
		} else {
			deaths++
		}
	}

	s.Stats.TotalPopulation = alive
	s.Stats.TotalWealth = totalWealth
	s.Stats.Deaths = deaths
	if alive > 0 {
		s.Stats.AvgMood = totalMood / float32(alive)
		s.Stats.AvgSurvival = totalSurvival / float32(alive)
	}
}

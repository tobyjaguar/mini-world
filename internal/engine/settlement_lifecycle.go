// Settlement lifecycle — founding via overmass diaspora and abandonment.
// See design doc Section 16.5.3 (galactic jet / M> barrier).
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processSettlementOvermass checks weekly for overmassed settlements and triggers diaspora.
// When a settlement exceeds its governance capacity, ~62% of agents emigrate in
// golden-angle directions. If 10+ emigrants cluster near a habitable hex, a new
// settlement is founded.
func (s *Simulation) processSettlementOvermass(tick uint64) {
	if s.Spawner == nil {
		return
	}

	for _, sett := range s.Settlements {
		if !sett.IsOvermassed() {
			continue
		}

		settAgents := s.SettlementAgents[sett.ID]
		var aliveAgents []*agents.Agent
		for _, a := range settAgents {
			if a.Alive {
				aliveAgents = append(aliveAgents, a)
			}
		}

		if len(aliveAgents) < 20 {
			continue // Not enough people for meaningful diaspora
		}

		// ~62% (Matter ratio) of agents emigrate.
		emigrantCount := int(float64(len(aliveAgents)) * phi.Matter)
		if emigrantCount < 10 {
			continue
		}

		slog.Info("settlement overmass diaspora",
			"settlement", sett.Name,
			"population", len(aliveAgents),
			"emigrants", emigrantCount,
		)

		// Select emigrants: prefer younger, lower-wealth agents (established
		// elites tend to stay).
		emigrants := make([]*agents.Agent, 0, emigrantCount)
		for _, a := range aliveAgents {
			if len(emigrants) >= emigrantCount {
				break
			}
			if a.Age < 50 && a.Role != agents.RoleLeader {
				emigrants = append(emigrants, a)
			}
		}

		// Find habitable hexes in golden-angle directions from settlement.
		var candidateHexes []world.HexCoord
		goldenAngle := 2 * math.Pi * phi.Matter // ~2.399 radians
		for i := 0; i < 6; i++ {
			angle := goldenAngle * float64(i)
			// Search 3-5 hexes out from settlement.
			for dist := 3; dist <= 5; dist++ {
				q := sett.Position.Q + int(math.Round(float64(dist)*math.Cos(angle)))
				r := sett.Position.R + int(math.Round(float64(dist)*math.Sin(angle)))
				coord := world.HexCoord{Q: q, R: r}
				hex := s.WorldMap.Get(coord)
				if hex == nil {
					continue
				}
				// Must be habitable (not ocean/desert/tundra) and unsettled.
				if hex.SettlementID != nil {
					continue
				}
				switch hex.Terrain {
				case world.TerrainPlains, world.TerrainForest, world.TerrainCoast,
					world.TerrainRiver, world.TerrainMountain:
					candidateHexes = append(candidateHexes, coord)
				}
			}
		}

		if len(candidateHexes) == 0 {
			// No suitable founding site — emigrants scatter (become migrants).
			slog.Info("no founding site available, emigrants scatter", "settlement", sett.Name)
			continue
		}

		// Pick the best candidate (first found is fine — they're in golden-angle order).
		foundingHex := candidateHexes[0]

		// Pool emigrant wealth for treasury.
		pooledWealth := uint64(0)
		for _, a := range emigrants {
			contribution := a.Wealth / 3
			a.Wealth -= contribution
			pooledWealth += contribution
		}

		// Found new settlement.
		newSett := s.foundSettlement(foundingHex, emigrants, pooledWealth, tick)

		// Remove emigrants from old settlement.
		sett.Population -= uint32(len(emigrants))

		// Update old settlement agent list.
		emigrantSet := make(map[agents.AgentID]bool, len(emigrants))
		for _, a := range emigrants {
			emigrantSet[a.ID] = true
		}
		remaining := make([]*agents.Agent, 0, len(settAgents)-len(emigrants))
		for _, a := range settAgents {
			if !emigrantSet[a.ID] {
				remaining = append(remaining, a)
			}
		}
		s.SettlementAgents[sett.ID] = remaining

		desc := fmt.Sprintf("%d citizens leave %s and found %s (overmass diaspora)",
			len(emigrants), sett.Name, newSett.Name)
		s.Events = append(s.Events, Event{
			Tick:        tick,
			Description: desc,
			Category:    "political",
		})
		s.createSettlementMemories(sett.ID, tick, desc, 0.8)

		slog.Info("new settlement founded",
			"name", newSett.Name,
			"from", sett.Name,
			"founders", len(emigrants),
			"hex", fmt.Sprintf("%d,%d", foundingHex.Q, foundingHex.R),
		)
	}
}

// processSettlementAbandonment checks weekly for settlements with 0 living population.
// After 2 consecutive weeks of 0 pop, the settlement is abandoned.
func (s *Simulation) processSettlementAbandonment(tick uint64) {
	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		aliveCount := 0
		for _, a := range settAgents {
			if a.Alive {
				aliveCount++
			}
		}

		if aliveCount == 0 {
			s.AbandonedWeeks[sett.ID]++
			if s.AbandonedWeeks[sett.ID] >= 2 {
				// Mark as abandoned — remove from active settlements.
				slog.Info("settlement abandoned", "name", sett.Name, "id", sett.ID)
				s.Events = append(s.Events, Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s has been abandoned — no living souls remain", sett.Name),
					Category:    "political",
				})

				// Clear hex settlement reference.
				hex := s.WorldMap.Get(sett.Position)
				if hex != nil {
					hex.SettlementID = nil
				}

				// Remove from active list (mark population 0, keep in DB).
				sett.Population = 0
			}
		} else {
			// Reset counter if people are alive.
			delete(s.AbandonedWeeks, sett.ID)
		}
	}
}

// foundSettlement creates a new settlement at the given hex with founding agents.
func (s *Simulation) foundSettlement(coord world.HexCoord, founders []*agents.Agent, treasury uint64, tick uint64) *social.Settlement {
	// Generate a unique settlement ID.
	maxID := uint64(0)
	for _, st := range s.Settlements {
		if st.ID > maxID {
			maxID = st.ID
		}
	}
	newID := maxID + 1

	// Generate name using prefix+suffix.
	name := s.generateSettlementName()

	newSett := &social.Settlement{
		ID:              newID,
		Name:            name,
		Position:        coord,
		Population:      uint32(len(founders)),
		Governance:      social.GovCouncil, // New settlements start as councils
		TaxRate:         0.10,
		Treasury:        treasury,
		GovernanceScore: 0.5,
		MarketLevel:     1,
		CultureOpenness: 0.3, // Founders tend to be open-minded
	}

	// Initialize market.
	newSett.Market = economy.NewMarket(newID)

	// Register in indexes.
	s.Settlements = append(s.Settlements, newSett)
	s.SettlementIndex[newID] = newSett

	// Update hex.
	hex := s.WorldMap.Get(coord)
	if hex != nil {
		hex.SettlementID = &newID
	}

	// Move founders to new settlement.
	s.SettlementAgents[newID] = make([]*agents.Agent, 0, len(founders))
	for _, a := range founders {
		a.HomeSettID = &newID
		a.Position = coord
		s.SettlementAgents[newID] = append(s.SettlementAgents[newID], a)
	}

	return newSett
}

// generateSettlementName creates a unique settlement name.
func (s *Simulation) generateSettlementName() string {
	prefixes := []string{
		"Iron", "Green", "Ash", "Stone", "Mill", "Cross", "Black",
		"Silver", "Red", "White", "Dark", "Bright", "High", "Low",
		"Old", "New", "Far", "Deep", "Long", "Broad", "Gold", "Frost",
		"Storm", "Thorn", "Elm", "Oak", "Pine", "Copper", "River",
	}
	suffixes := []string{
		"haven", "ford", "hollow", "wick", "bridge", "gate", "keep",
		"stead", "wood", "field", "dale", "crest", "vale", "port",
		"town", "bury", "marsh", "well", "brook", "cliff", "moor",
		"ridge", "watch", "fall", "rest", "point", "reach", "helm",
	}

	existing := make(map[string]bool)
	for _, st := range s.Settlements {
		existing[st.Name] = true
	}

	// Try random combinations until unique.
	tick := s.LastTick
	for i := 0; i < 1000; i++ {
		pi := int((tick + uint64(i)*17)) % len(prefixes)
		si := int((tick + uint64(i)*31)) % len(suffixes)
		name := prefixes[pi] + suffixes[si]
		if !existing[name] {
			return name
		}
	}
	// Fallback: append ID.
	return fmt.Sprintf("Settlement-%d", tick)
}

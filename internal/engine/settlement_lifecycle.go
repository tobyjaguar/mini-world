// Settlement lifecycle — founding via overmass diaspora and abandonment.
// See design doc Section 16.5.3 (galactic jet / M> barrier).
package engine

import (
	"fmt"
	"log/slog"
	"math"
	"sort"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processSettlementOvermass checks weekly for overmassed settlements and triggers diaspora.
// When a settlement exceeds its governance capacity, ~24% of agents emigrate in
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

		// ~24% (Agnosis ratio) of agents emigrate — smaller diaspora prevents
		// gutting parent settlements and reduces settlement cascade.
		emigrantCount := int(float64(len(aliveAgents)) * phi.Agnosis)
		if emigrantCount < 25 {
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
		s.EmitEvent(Event{
			Tick:        tick,
			Description: desc,
			Category:    "political",
			Meta: map[string]any{
				"source_settlement_id": sett.ID,
				"settlement_name":      newSett.Name,
				"count":                len(emigrants),
			},
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
				// Redistribute treasury to nearest active settlements.
				if sett.Treasury > 0 {
					nearest := s.nearestActiveSettlements(sett.Position, 3)
					if len(nearest) > 0 {
						share := sett.Treasury / uint64(len(nearest))
						for _, neighbor := range nearest {
							neighbor.Treasury += share
						}
						remainder := sett.Treasury - share*uint64(len(nearest))
						nearest[0].Treasury += remainder
						s.EmitEvent(Event{
							Tick:        tick,
							Description: fmt.Sprintf("%s's treasury of %d crowns distributed to neighboring settlements", sett.Name, sett.Treasury),
							Category:    "economy",
							Meta: map[string]any{
								"settlement_id":   sett.ID,
								"settlement_name": sett.Name,
								"amount":          sett.Treasury,
							},
						})
						sett.Treasury = 0
					}
				}

				// Mark as abandoned — remove from active settlements.
				slog.Info("settlement abandoned", "name", sett.Name, "id", sett.ID)
				s.EmitEvent(Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s has been abandoned — no living souls remain", sett.Name),
					Category:    "political",
					Meta: map[string]any{
						"settlement_id":   sett.ID,
						"settlement_name": sett.Name,
					},
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

// nearestActiveSettlements returns the N closest settlements with population > 0.
func (s *Simulation) nearestActiveSettlements(from world.HexCoord, n int) []*social.Settlement {
	type distSett struct {
		dist int
		sett *social.Settlement
	}
	var candidates []distSett
	for _, st := range s.Settlements {
		if st.Population > 0 {
			d := world.Distance(from, st.Position)
			candidates = append(candidates, distSett{d, st})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})
	result := make([]*social.Settlement, 0, n)
	for i := 0; i < n && i < len(candidates); i++ {
		result = append(result, candidates[i].sett)
	}
	return result
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
		s.reassignIfMismatched(a, newID)
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

// processInfrastructureGrowth lets settlements invest treasury into roads and walls weekly.
// Each upgrade requires minimum population and treasury. One upgrade per settlement per week max.
func (s *Simulation) processInfrastructureGrowth(tick uint64) {
	for _, sett := range s.Settlements {
		pop := int(sett.Population)
		if pop == 0 {
			continue
		}

		treasury := sett.Treasury

		// Road upgrade: treasury >= pop×20, pop >= 50, RoadLevel < 5
		if sett.RoadLevel < 5 && pop >= 50 && treasury >= uint64(pop)*20 {
			cost := uint64(pop) * 20
			sett.Treasury -= cost
			sett.RoadLevel++
			slog.Info("infrastructure upgrade: road",
				"settlement", sett.Name,
				"road_level", sett.RoadLevel,
				"cost", cost,
			)
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s upgrades roads to level %d", sett.Name, sett.RoadLevel),
				Category:    "economy",
				Meta: map[string]any{
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"type":            "roads",
					"level":           sett.RoadLevel,
				},
			})
			continue // One upgrade per week max
		}

		// Wall upgrade: treasury >= pop×30, pop >= 100, WallLevel < 5
		if sett.WallLevel < 5 && pop >= 100 && treasury >= uint64(pop)*30 {
			cost := uint64(pop) * 30
			sett.Treasury -= cost
			sett.WallLevel++
			slog.Info("infrastructure upgrade: wall",
				"settlement", sett.Name,
				"wall_level", sett.WallLevel,
				"cost", cost,
			)
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s upgrades walls to level %d", sett.Name, sett.WallLevel),
				Category:    "economy",
				Meta: map[string]any{
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"type":            "walls",
					"level":           sett.WallLevel,
				},
			})
		}
	}
}

// processViabilityCheck tracks settlements with persistently low population.
// After 2 consecutive weeks below 25 pop, the settlement is marked non-viable,
// refugee spawning is disabled, and remaining agents are force-migrated to the
// nearest viable settlement. This accelerates consolidation of the 234
// settlements with pop < 25 that are economic dead zones.
func (s *Simulation) processViabilityCheck(tick uint64) {
	migrated := false
	for _, sett := range s.Settlements {
		aliveCount := 0
		for _, a := range s.SettlementAgents[sett.ID] {
			if a.Alive {
				aliveCount++
			}
		}

		if aliveCount < 25 {
			s.NonViableWeeks[sett.ID]++

			// After 2 weeks non-viable: force-migrate all agents out.
			if s.NonViableWeeks[sett.ID] >= 2 && aliveCount > 0 {
				target := s.findNearestViableSettlement(sett, 8)
				if target == nil {
					continue // No viable target — let them be.
				}
				for _, a := range s.SettlementAgents[sett.ID] {
					if !a.Alive {
						continue
					}
					newID := target.ID
					a.HomeSettID = &newID
					a.Position = target.Position
					s.reassignIfMismatched(a, newID)
					migrated = true
				}
				slog.Info("non-viable settlement force-migration",
					"settlement", sett.Name,
					"population", aliveCount,
					"target", target.Name,
				)
				s.EmitEvent(Event{
					Tick:        tick,
					Description: fmt.Sprintf("The remaining %d souls of %s migrate to %s (settlement non-viable)", aliveCount, sett.Name, target.Name),
					Category:    "social",
					Meta: map[string]any{
						"source_settlement_id":   sett.ID,
						"target_settlement_id":   target.ID,
						"target_settlement_name": target.Name,
						"count":                  aliveCount,
					},
				})
			}
		} else {
			delete(s.NonViableWeeks, sett.ID)
		}
	}
	if migrated {
		s.rebuildSettlementAgents()
	}
}

// Seasonal effects and resource regeneration.
// See design doc Sections 3.4 and 9.2.
package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// Season constants.
const (
	SeasonSpring = 0
	SeasonSummer = 1
	SeasonAutumn = 2
	SeasonWinter = 3
)

// SeasonName returns a human-readable season name.
func SeasonName(season uint8) string {
	switch season {
	case SeasonSpring:
		return "Spring"
	case SeasonSummer:
		return "Summer"
	case SeasonAutumn:
		return "Autumn"
	case SeasonWinter:
		return "Winter"
	default:
		return "Unknown"
	}
}

// SeasonalMarketMod returns a price modifier for goods based on the current season.
func SeasonalMarketMod(season uint8, good uint8) float64 {
	// Food is expensive in winter, cheap after harvest (autumn).
	// Furs are expensive in winter. Herbs peak in summer.
	switch season {
	case SeasonWinter:
		switch good {
		case 0, 4: // Grain, Fish
			return 1.5 // Food scarce in winter
		case 7: // Furs
			return 1.8 // High demand
		case 5: // Herbs
			return 1.4 // Hard to find
		default:
			return 1.1
		}
	case SeasonSpring:
		switch good {
		case 0, 4: // Grain, Fish
			return 1.2 // Still somewhat scarce
		case 5: // Herbs
			return 0.8 // Starting to grow
		default:
			return 1.0
		}
	case SeasonSummer:
		switch good {
		case 5: // Herbs
			return 0.7 // Abundant
		case 7: // Furs
			return 0.7 // Low demand
		default:
			return 0.9
		}
	case SeasonAutumn:
		switch good {
		case 0: // Grain
			return 0.7 // Harvest abundance
		case 4: // Fish
			return 0.8
		case 5: // Herbs
			return 0.9
		default:
			return 1.0
		}
	}
	return 1.0
}

// processSeason handles seasonal transitions: resource regen, crop yields, weather.
func (s *Simulation) processSeason(tick uint64) {
	s.CurrentSeason = uint8((tick / TicksPerSimSeason) % 4)

	slog.Info("season change",
		"tick", tick,
		"time", SimTime(tick),
		"season", SeasonName(s.CurrentSeason),
		"population", s.Stats.TotalPopulation,
	)

	// Regenerate resources on all hexes.
	s.regenerateResources()

	// Seasonal harvest bonus for farmers in autumn.
	if s.CurrentSeason == SeasonAutumn {
		s.autumnHarvest(tick)
	}

	// Winter hardship: increased survival decay.
	if s.CurrentSeason == SeasonWinter {
		s.winterHardship(tick)
	}
}

// regenerateResources replenishes hex resources each season.
func (s *Simulation) regenerateResources() {
	for q := -s.WorldMap.Radius; q <= s.WorldMap.Radius; q++ {
		for r := -s.WorldMap.Radius; r <= s.WorldMap.Radius; r++ {
			coord := world.HexCoord{Q: q, R: r}
			hex := s.WorldMap.Get(coord)
			if hex == nil || hex.Terrain == world.TerrainOcean {
				continue
			}

			// Desertified hexes don't regenerate resources until health recovers.
			if hex.Health < phi.Agnosis {
				continue
			}

			// Regenerate based on terrain type, scaled by hex health.
			for res, qty := range hex.Resources {
				maxQty := ResourceCap(hex.Terrain, res)
				if qty < maxQty {
					// Regrow at rate proportional to Matter, scaled by hex health.
					// Degraded land regenerates slower.
					deficit := maxQty - qty
					regen := deficit * phi.Matter * 0.3 * hex.Health
					hex.Resources[res] = qty + regen
					if hex.Resources[res] > maxQty {
						hex.Resources[res] = maxQty
					}
				}
			}
		}
	}
}

// ResourceCap returns the maximum resource quantity for a terrain/resource combo.
func ResourceCap(terrain world.Terrain, res world.ResourceType) float64 {
	switch terrain {
	case world.TerrainPlains:
		if res == world.ResourceGrain {
			return 100
		}
	case world.TerrainForest:
		switch res {
		case world.ResourceTimber:
			return 80
		case world.ResourceHerbs:
			return 80
		case world.ResourceFurs:
			return 40
		}
	case world.TerrainMountain:
		switch res {
		case world.ResourceIronOre:
			return 60
		case world.ResourceStone:
			return 80
		case world.ResourceCoal:
			return 40
		case world.ResourceGems:
			return 15
		}
	case world.TerrainCoast:
		if res == world.ResourceFish {
			return 70
		}
	case world.TerrainRiver:
		if res == world.ResourceFish {
			return 50
		}
		if res == world.ResourceGrain {
			return 80 // Irrigated
		}
	case world.TerrainSwamp:
		switch res {
		case world.ResourceHerbs:
			return 100
		case world.ResourceExotics:
			return 20
		}
	case world.TerrainTundra:
		if res == world.ResourceFurs {
			return 50
		}
	}
	return 10 // Default small amount
}

// weeklyResourceRegen replenishes hex resources at a smaller rate than seasonal regen.
// Without this, hexes stay depleted for 24 sim-days between seasons. Weekly regen
// recovers ~4.7% of deficit (Agnosis * 0.2) so resources trickle back between seasons.
func (s *Simulation) weeklyResourceRegen() {
	for q := -s.WorldMap.Radius; q <= s.WorldMap.Radius; q++ {
		for r := -s.WorldMap.Radius; r <= s.WorldMap.Radius; r++ {
			coord := world.HexCoord{Q: q, R: r}
			hex := s.WorldMap.Get(coord)
			if hex == nil || hex.Terrain == world.TerrainOcean {
				continue
			}

			// Fallow recovery: un-extracted hexes regain health.
			if hex.LastExtractedTick == 0 || s.LastTick-hex.LastExtractedTick > TicksPerSimDay {
				hex.Health += phi.Agnosis * 0.05 // ~1.2% health per week when fallow
				if hex.Health > 1.0 {
					hex.Health = 1.0
				}
			}

			// Desertified hexes don't regenerate resources until health recovers.
			if hex.Health < phi.Agnosis {
				continue
			}

			for res, qty := range hex.Resources {
				maxQty := ResourceCap(hex.Terrain, res)
				if qty < maxQty {
					deficit := maxQty - qty
					regen := deficit * phi.Agnosis * 0.4 * hex.Health // ~9.4% scaled by health
					hex.Resources[res] = qty + regen
					if hex.Resources[res] > maxQty {
						hex.Resources[res] = maxQty
					}
				}
			}
		}
	}
}

// autumnHarvest gives farmers a seasonal production bonus.
func (s *Simulation) autumnHarvest(tick uint64) {
	harvestCount := 0
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		if a.Occupation == agents.OccupationFarmer {
			// Harvest bonus scaled by farming skill.
			bonus := int(a.Skills.Farming * 10)
			if bonus < 2 {
				bonus = 2
			}
			a.Inventory[agents.GoodGrain] += bonus
			harvestCount++
		}
	}
	if harvestCount > 0 {
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("Autumn harvest: %d farmers bring in the crop", harvestCount),
			Category:    "economy",
			Meta: map[string]any{
				"count": harvestCount,
			},
		})
	}
}

// winterHardship applies cold-weather penalties.
func (s *Simulation) winterHardship(tick uint64) {
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		// Agents without clothing or furs lose health in winter.
		hasWarmth := a.Inventory[agents.GoodClothing] > 0 || a.Inventory[agents.GoodFurs] > 0
		if !hasWarmth {
			a.Health -= 0.05
			a.Wellbeing.Satisfaction -= 0.1
			if a.Health < 0 {
				a.Health = 0
			}
		}
	}
}

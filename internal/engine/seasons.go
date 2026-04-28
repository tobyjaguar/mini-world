// Seasonal effects and resource regeneration.
// See design doc Sections 3.4 and 9.2.
package engine

import (
	"fmt"
	"log/slog"
	"math"

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

// SeasonalProductionMod returns a production multiplier for the current season.
// Spring is the growing season (boost), summer is peak, autumn declines, winter
// is harsh. Creates annual economic cycles — surplus in spring/summer, scarcity
// in winter. Merchant arbitrage across seasons becomes valuable.
// All multipliers are Φ-derived: Spring = 1 + Agnosis (~1.236), Summer = 1.0,
// Autumn = 1 - Agnosis*0.5 (~0.882), Winter = Agnosis + Psyche (~0.618 = Matter).
func SeasonalProductionMod(season uint8) float64 {
	switch season {
	case SeasonSpring:
		return 1.0 + phi.Agnosis // ~1.236 — growing season, land awakening
	case SeasonSummer:
		return 1.0 // Baseline — full productive capacity
	case SeasonAutumn:
		return 1.0 - phi.Agnosis*0.5 // ~0.882 — harvest winding down
	case SeasonWinter:
		return phi.Matter // ~0.618 — frozen ground, short days
	default:
		return 1.0
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

			// Regenerate based on terrain type, scaled by hex health and irrigation.
			irrFactor := IrrigationRegenFactor(hex.IrrigationLevel)
			for res, qty := range hex.Resources {
				maxQty := ResourceCap(hex.Terrain, res)
				if qty < maxQty {
					// Regrow at rate proportional to Matter, scaled by hex health.
					// Degraded land regenerates slower. Irrigation boosts regen.
					deficit := maxQty - qty
					regen := deficit * phi.Matter * 0.3 * hex.Health * irrFactor
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
	// Weather affects fallow recovery: rain accelerates healing, heat slows it.
	fallowMod := 1.0
	w := s.CurrentWeather
	if w.TravelPenalty >= 1.2 && w.TravelPenalty < 2.0 {
		fallowMod = 1.0 + phi.Agnosis // Rain: ~24% faster healing
	}
	if w.TempModifier > 0 {
		fallowMod -= float64(w.TempModifier) * phi.Agnosis * 0.5 // Heat: up to ~12% slower
	}

	for q := -s.WorldMap.Radius; q <= s.WorldMap.Radius; q++ {
		for r := -s.WorldMap.Radius; r <= s.WorldMap.Radius; r++ {
			coord := world.HexCoord{Q: q, R: r}
			hex := s.WorldMap.Get(coord)
			if hex == nil || hex.Terrain == world.TerrainOcean {
				continue
			}

			// Fallow recovery: un-extracted hexes regain health, modified by weather.
			if hex.LastExtractedTick == 0 || s.LastTick-hex.LastExtractedTick > TicksPerSimDay {
				hex.Health += phi.Agnosis * 0.25 * float64(fallowMod) // ~5.9% health per week when fallow
				if hex.Health > 1.0 {
					hex.Health = 1.0
				}
			}

			// Desertified hexes don't regenerate resources until health recovers.
			if hex.Health < phi.Agnosis {
				continue
			}

			irrFactor2 := IrrigationRegenFactor(hex.IrrigationLevel)
			for res, qty := range hex.Resources {
				maxQty := ResourceCap(hex.Terrain, res)
				if qty < maxQty {
					deficit := maxQty - qty
					regen := deficit * phi.Agnosis * 0.4 * hex.Health * irrFactor2 // ~9.4% scaled by health + irrigation
					hex.Resources[res] = qty + regen
					if hex.Resources[res] > maxQty {
						hex.Resources[res] = maxQty
					}
				}
			}
		}
	}
}

// hourlyResourceRegen provides continuous resource recovery every sim-hour.
// Weekly regen alone is consumed within 1-2 ticks by hundreds of producers, keeping
// resources permanently near zero. Hourly micro-regen sustains a steady trickle.
//
// Base rate: deficit * Agnosis * 0.06 * health per hour. Settlement neighborhoods
// get a population-pressure boost: factor = 1 + Agnosis * log2(1 + pressure), where
// pressure = population / carrying_capacity. At pressure 1.0: +24% regen. At 2.0: +37%.
// This represents more intensive land management in denser settlements — a pattern
// observed in real agricultural history. Wilderness hexes regen at base rate.
//
// Weather modifies regen: rain boosts by Agnosis (~24%), hot/dry weather reduces by
// TempModifier * Agnosis * 0.5 (up to ~12% penalty at max heat). Storms suppress
// regen entirely (too dangerous to work the land).
func (s *Simulation) hourlyResourceRegen() {
	// Pre-compute population pressure boost for settlement neighborhoods.
	pressureByHex := make(map[world.HexCoord]float64, len(s.Settlements)*7)
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}
		_, pressure := s.SettlementCarryingCapacity(sett.ID)
		if pressure <= 0 {
			continue
		}
		// Φ-derived logarithmic boost: diminishing returns at high density.
		factor := 1.0 + phi.Agnosis*math.Log2(1.0+pressure)
		coords := sett.Position.Neighbors()
		for _, c := range coords {
			if existing, ok := pressureByHex[c]; !ok || factor > existing {
				pressureByHex[c] = factor
			}
		}
		// Include settlement hex itself.
		if existing, ok := pressureByHex[sett.Position]; !ok || factor > existing {
			pressureByHex[sett.Position] = factor
		}
	}

	// Weather modifier: rain helps growth, heat stresses land, storms suppress regen.
	weatherMod := 1.0
	w := s.CurrentWeather
	if w.TravelPenalty >= 2.0 {
		// Storm: too dangerous to work, land stressed. Suppress regen.
		weatherMod = 1.0 - phi.Agnosis // ~0.764
	} else if w.TravelPenalty >= 1.2 {
		// Rain: nourishes the land.
		weatherMod = 1.0 + phi.Agnosis // ~1.236
	}
	// Heat stress: hot weather dries the land, reducing regen.
	if w.TempModifier > 0 {
		weatherMod -= float64(w.TempModifier) * phi.Agnosis * 0.5 // Up to ~12% penalty at max heat
	}

	for q := -s.WorldMap.Radius; q <= s.WorldMap.Radius; q++ {
		for r := -s.WorldMap.Radius; r <= s.WorldMap.Radius; r++ {
			coord := world.HexCoord{Q: q, R: r}
			hex := s.WorldMap.Get(coord)
			if hex == nil || hex.Terrain == world.TerrainOcean {
				continue
			}

			// Desertified hexes don't regenerate.
			if hex.Health < phi.Agnosis {
				continue
			}

			// Population pressure boost for settlement neighborhoods.
			factor := 1.0
			if f, ok := pressureByHex[coord]; ok {
				factor = f
			}

			// Irrigation boost: improved hexes regenerate faster.
			irrFactor := IrrigationRegenFactor(hex.IrrigationLevel)

			for res, qty := range hex.Resources {
				maxQty := ResourceCap(hex.Terrain, res)
				if qty < maxQty {
					deficit := maxQty - qty
					regen := deficit * phi.Agnosis * 0.06 * hex.Health * factor * weatherMod * irrFactor
					hex.Resources[res] = qty + regen
					if hex.Resources[res] > maxQty {
						hex.Resources[res] = maxQty
					}
				}
			}
		}
	}
}

// applyWeatherHexDamage applies persistent damage to hex health from extreme weather.
// Closes the loop where weather only had transient effects on regen rates and stored
// goods. Storms erode coastal hexes; sustained heat (drought) degrades inland Plains
// and Forest. Complements checkStormDamage (settlement infrastructure) and
// checkCropFailure (stored grain) by damaging the LAND itself — the substrate of
// future production. Conservation level reduces damage; irrigation protects against
// drought.
func (s *Simulation) applyWeatherHexDamage(tick uint64) {
	if s.CurrentWeather.TravelPenalty >= 2.0 {
		s.applyStormErosion(tick)
	}
	if s.HeatStreakHours >= 72 {
		s.applyDroughtDegradation(tick)
	}
}

// applyStormErosion damages coastal hexes during severe weather. Each populated
// coastal settlement has an Agnosis³ (~1.3%) chance per sim-hour of an erosion
// event. On hit, the settlement hex loses Agnosis × 0.01 (~0.236%) health, and the
// six neighboring hexes lose half that. Conservation level reduces damage.
// Deterministic per (settlement, tick) for replay.
func (s *Simulation) applyStormErosion(tick uint64) {
	chance := phi.Agnosis * phi.Agnosis * phi.Agnosis
	regions := 0
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}
		hex := s.WorldMap.Get(sett.Position)
		if hex == nil || hex.Terrain != world.TerrainCoast {
			continue
		}
		hash := (sett.ID*2654435761 + tick*40503 + 7919) % 100000
		if float64(hash)/100000.0 >= chance {
			continue
		}
		s.damageHexHealth(sett.Position, phi.Agnosis*0.01)
		for _, nc := range sett.Position.Neighbors() {
			s.damageHexHealth(nc, phi.Agnosis*0.005)
		}
		regions++
	}
	if regions > 0 {
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("Storm erodes the coast in %d region(s)", regions),
			Category:    "disaster",
			Meta: map[string]any{
				"event_type": "storm_erosion",
				"regions":    regions,
			},
		})
	}
}

// applyDroughtDegradation degrades Plains and Forest hex health during sustained
// heat. Triggered by HeatStreakHours >= 72 (the same threshold that fires crop
// failure, so drought arrives as a one-two punch: stored grain spoils, then the
// land itself dries). Rate Agnosis × 0.0005 (~0.0118%/hour, ~0.28%/day). Irrigation
// level 3+ protects fully; lower levels partial. Conservation reduces remaining damage.
func (s *Simulation) applyDroughtDegradation(tick uint64) {
	rate := phi.Agnosis * 0.0005
	for q := -s.WorldMap.Radius; q <= s.WorldMap.Radius; q++ {
		for r := -s.WorldMap.Radius; r <= s.WorldMap.Radius; r++ {
			coord := world.HexCoord{Q: q, R: r}
			hex := s.WorldMap.Get(coord)
			if hex == nil {
				continue
			}
			if hex.Terrain != world.TerrainPlains && hex.Terrain != world.TerrainForest {
				continue
			}
			if hex.IrrigationLevel >= 3 {
				continue
			}
			irrProtection := 1.0 - float64(hex.IrrigationLevel)/3.0*phi.Agnosis
			damage := rate * ConservationDamageFactor(hex.ConservationLevel) * irrProtection
			hex.Health -= damage
			if hex.Health < 0 {
				hex.Health = 0
			}
		}
	}
}

// damageHexHealth applies a bounded health decrement to a hex, scaled by
// conservation level. Skips ocean hexes. Floors at 0.
func (s *Simulation) damageHexHealth(coord world.HexCoord, baseDamage float64) {
	hex := s.WorldMap.Get(coord)
	if hex == nil || hex.Terrain == world.TerrainOcean {
		return
	}
	hex.Health -= baseDamage * ConservationDamageFactor(hex.ConservationLevel)
	if hex.Health < 0 {
		hex.Health = 0
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

// checkCropFailure tracks sustained heat and triggers crop failure in vulnerable settlements.
// When TempModifier exceeds Agnosis (~0.236) for 72+ consecutive sim-hours (3 sim-days),
// settlements with low irrigation lose a fraction of stored grain. Rewards irrigation
// investment (R42) and creates demand spikes that reward merchant arbitrage.
func (s *Simulation) checkCropFailure(tick uint64) {
	if s.CurrentWeather.TempModifier > float32(phi.Agnosis) {
		s.HeatStreakHours++
	} else {
		s.HeatStreakHours = 0
		return
	}

	// Fire crop failure at 72 sim-hours (3 sim-days of sustained heat).
	// Then reset to 48 so subsequent failures fire every 24 hours of continued heat.
	if s.HeatStreakHours < 72 {
		return
	}
	s.HeatStreakHours = 48

	affected := 0
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}

		// Well-irrigated settlements resist crop failure.
		// Average irrigation across the 7-hex neighborhood.
		avgIrrigation := float64(0)
		hexCount := 0
		neighbors := sett.Position.Neighbors()
		coords := append(neighbors[:], sett.Position)
		for _, c := range coords {
			if h := s.WorldMap.Get(c); h != nil && h.Terrain != world.TerrainOcean {
				avgIrrigation += float64(h.IrrigationLevel)
				hexCount++
			}
		}
		if hexCount > 0 {
			avgIrrigation /= float64(hexCount)
		}

		// Irrigation >= 3 fully protects against crop failure.
		// At 0: full damage. At 1: ~67% damage. At 2: ~33%.
		if avgIrrigation >= 3 {
			continue
		}
		protection := avgIrrigation / 3.0

		// Spoil Agnosis fraction of each farmer's grain, scaled by vulnerability.
		spoilRate := phi.Agnosis * (1.0 - protection) // ~0.236 at zero irrigation
		settAgents := s.SettlementAgents[sett.ID]
		for _, a := range settAgents {
			if !a.Alive {
				continue
			}
			grain := a.Inventory[agents.GoodGrain]
			if grain > 0 {
				lost := int(float64(grain) * spoilRate)
				if lost < 1 {
					lost = 1
				}
				a.Inventory[agents.GoodGrain] -= lost
				if a.Inventory[agents.GoodGrain] < 0 {
					a.Inventory[agents.GoodGrain] = 0
				}
			}
		}
		affected++
	}

	if affected > 0 {
		slog.Info("crop failure from heat wave", "tick", tick, "settlements_affected", affected)
		s.EmitEvent(Event{
			Tick:        tick,
			Description: fmt.Sprintf("Sustained heat wave causes crop failure in %d settlements — stored grain spoils", affected),
			Category:    "disaster",
			Meta: map[string]any{
				"event_type":          "crop_failure",
				"settlements_affected": affected,
			},
		})
	}
}

// checkStormDamage degrades settlement infrastructure during severe weather.
// Storms (TravelPenalty >= 2.0) have an Agnosis² chance (~5.6%) per settlement per
// sim-hour of degrading one infrastructure level. Well-governed settlements resist
// damage (governance score reduces probability). Creates maintenance pressure and
// rewards treasury investment — settlements that don't reinvest slowly decay.
func (s *Simulation) checkStormDamage(tick uint64) {
	if s.CurrentWeather.TravelPenalty < 2.0 {
		return // Not a storm
	}

	damaged := 0
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}

		// Base damage chance: Agnosis² (~5.6%) per sim-hour during storms.
		// Governance reduces chance: multiply by (1 - GovernanceScore * 0.5).
		// At governance 0.8: chance = 5.6% × 0.6 = 3.4%.
		chance := phi.Agnosis * phi.Agnosis * (1.0 - sett.GovernanceScore*0.5)

		// Deterministic check from settlement ID + tick.
		hash := (sett.ID*2654435761 + tick*40503) % 100000
		if float64(hash)/100000.0 >= chance {
			continue
		}

		// Pick a random infrastructure to damage (weighted by current level).
		// Higher-level infrastructure is more exposed to storm damage.
		total := int(sett.RoadLevel) + int(sett.WallLevel) + int(sett.MarketLevel)
		if total == 0 {
			continue // Nothing to damage
		}

		pick := int((sett.ID*31 + tick*17) % uint64(total))
		if pick < int(sett.RoadLevel) && sett.RoadLevel > 0 {
			sett.RoadLevel--
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("Storm damages roads in %s (level %d)", sett.Name, sett.RoadLevel),
				Category:    "disaster",
				Meta: map[string]any{
					"event_type":      "storm_damage",
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"infrastructure":  "roads",
					"level":           sett.RoadLevel,
				},
			})
			damaged++
		} else if pick < int(sett.RoadLevel)+int(sett.WallLevel) && sett.WallLevel > 0 {
			sett.WallLevel--
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("Storm damages walls in %s (level %d)", sett.Name, sett.WallLevel),
				Category:    "disaster",
				Meta: map[string]any{
					"event_type":      "storm_damage",
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"infrastructure":  "walls",
					"level":           sett.WallLevel,
				},
			})
			damaged++
		} else if sett.MarketLevel > 0 {
			sett.MarketLevel--
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("Storm damages market in %s (level %d)", sett.Name, sett.MarketLevel),
				Category:    "disaster",
				Meta: map[string]any{
					"event_type":      "storm_damage",
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"infrastructure":  "market",
					"level":           sett.MarketLevel,
				},
			})
			damaged++
		}
	}

	if damaged > 0 {
		slog.Info("storm damage to infrastructure", "tick", tick, "settlements_damaged", damaged)
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

// Population dynamics — aging, natural death, births, migration, anti-collapse.
// See design doc Section 9.
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// SimDaysPerYear is the number of sim-days in one sim-year (4 seasons × 90 days).
const SimDaysPerYear = 360

// processPopulation handles daily aging, natural death, and births.
func (s *Simulation) processPopulation(tick uint64) {
	simDay := tick / TicksPerSimDay

	// Aging: increment age every sim-year (360 sim-days).
	if simDay > 0 && simDay%SimDaysPerYear == 0 {
		s.ageAgents(tick)
	}

	// Daily: natural death checks and birth checks.
	s.processNaturalDeaths(tick)
	s.processBirths(tick)
	s.processAntiCollapse(tick)
}

// ageAgents increments the age of all living agents by 1 year.
func (s *Simulation) ageAgents(tick uint64) {
	for _, a := range s.Agents {
		if a.Alive {
			a.Age++
		}
	}
	slog.Info("agents aged", "tick", tick, "time", SimTime(tick))
}

// processNaturalDeaths checks for death from old age and disease.
func (s *Simulation) processNaturalDeaths(tick uint64) {
	simDay := tick / TicksPerSimDay

	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		// Old age death: probability increases past age 55.
		if a.Age > 55 {
			// Daily mortality = Agnosis * (age - 55) / 100
			// At 55: ~0%, at 70: ~0.35%/day, at 80: ~0.59%/day
			mortalityRate := phi.Agnosis * float64(a.Age-55) / 100.0
			// Use deterministic check based on tick to avoid randomness.
			// Agent dies when accumulated probability exceeds threshold.
			daysSinceThreshold := simDay % SimDaysPerYear
			if mortalityRate*float64(daysSinceThreshold) > float64(a.ID%100)/100.0 {
				if a.Age > 65 || (a.Age > 55 && a.Health < 0.5) {
					a.Alive = false
					a.Health = 0
					s.Events = append(s.Events, Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s has died of old age at %d", a.Name, a.Age),
						Category:    "death",
					})
					s.inheritWealth(a, tick)
				}
			}
		}

		// Disease: low health agents have a small daily death chance.
		if a.Health < 0.15 && a.Health > 0 {
			// Deterministic: die if health has been critical for a while.
			a.Health -= 0.01
			if a.Health <= 0 {
				a.Alive = false
				s.Events = append(s.Events, Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s has died of illness", a.Name),
					Category:    "death",
				})
				s.inheritWealth(a, tick)
			}
		}
	}
}

// processBirths creates new agents from families in prosperous settlements.
func (s *Simulation) processBirths(tick uint64) {
	if s.Spawner == nil {
		return
	}

	simDay := tick / TicksPerSimDay

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		if len(settAgents) == 0 {
			continue
		}

		// Count eligible parents: adults (18-45) with decent health and survival.
		// Belonging gates birth via sigmoid probability (not hard threshold)
		// to prevent cliff dynamics causing wild birth oscillations.
		var eligibleParents []*agents.Agent
		for _, a := range settAgents {
			if a.Alive && a.Age >= 18 && a.Age <= 45 && a.Health > 0.5 &&
				a.Needs.Survival > 0.3 && birthEligible(a, simDay) {
				eligibleParents = append(eligibleParents, a)
			}
		}

		if len(eligibleParents) < 2 {
			continue
		}

		// Birth rate: based on settlement prosperity and population.
		// One birth per ~30 eligible parents per day, modified by prosperity.
		prosperity := float64(sett.Treasury) / (float64(sett.Population) + 1)
		prosperityMod := math.Log1p(prosperity) * phi.Agnosis
		if prosperityMod > 1.0 {
			prosperityMod = 1.0
		}

		birthChance := float64(len(eligibleParents)) / 30.0 * (0.5 + prosperityMod)
		// Deterministic: use simDay and settlement ID for consistent births.
		birthCount := int(birthChance)
		fractional := birthChance - float64(birthCount)
		if fractional > 0 && (simDay+uint64(sett.ID))%uint64(1.0/fractional+1) == 0 {
			birthCount++
		}

		for i := 0; i < birthCount && i < 3; i++ { // Cap at 3 births per settlement per day
			// Pick two parents deterministically.
			parentIdx := int((simDay + uint64(i)) % uint64(len(eligibleParents)))
			parent := eligibleParents[parentIdx]

			hex := s.WorldMap.Get(sett.Position)
			terrain := world.TerrainPlains
			if hex != nil {
				terrain = hex.Terrain
			}

			child := s.Spawner.SpawnChild(sett.Position, sett.ID, terrain, tick, parent)
			s.addAgent(child)

			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s is born in %s", child.Name, sett.Name),
				Category:    "birth",
			})
			s.Stats.Births++
			sett.Population++
		}
	}
}

// processAntiCollapse prevents settlements from dying out entirely.
// See design doc Section 9.4.
func (s *Simulation) processAntiCollapse(tick uint64) {
	if s.Spawner == nil {
		return
	}

	for _, sett := range s.Settlements {
		settAgents := s.SettlementAgents[sett.ID]
		aliveCount := 0
		starvingCount := 0
		for _, a := range settAgents {
			if a.Alive {
				aliveCount++
				if a.Needs.Survival < 0.1 {
					starvingCount++
				}
			}
		}

		// Minimum population floor: refugees arrive if below 10.
		// Skip for non-viable settlements (pop < 25 for 2+ weeks) — let them naturally decline.
		if aliveCount < 10 && s.NonViableWeeks[sett.ID] < 2 {
			needed := 10 - aliveCount
			hex := s.WorldMap.Get(sett.Position)
			terrain := world.TerrainPlains
			if hex != nil {
				terrain = hex.Terrain
			}
			refugees := s.Spawner.SpawnPopulation(uint32(needed), sett.Position, sett.ID, terrain)
			for _, r := range refugees {
				r.BornTick = tick
				s.addAgent(r)
			}
			sett.Population = uint32(aliveCount + needed)
			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: fmt.Sprintf("%d refugees arrive in %s", needed, sett.Name),
				Category:    "social",
			})
		}

		// Famine relief: if >20% starving, emergency food arrives.
		if aliveCount > 0 && float64(starvingCount)/float64(aliveCount) > 0.2 {
			for _, a := range settAgents {
				if a.Alive && a.Needs.Survival < 0.1 {
					a.Inventory[agents.GoodGrain] += 3
				}
			}
			s.Events = append(s.Events, Event{
				Tick:        tick,
				Description: fmt.Sprintf("Emergency food relief arrives in %s", sett.Name),
				Category:    "economy",
			})
		}
	}
}

// birthEligible uses a sigmoid probability curve on Belonging to determine
// if an agent is eligible to be a parent. This replaces the hard threshold
// (Belonging > 0.3) which caused cliff dynamics — small Belonging shifts
// pushed thousands of agents across the threshold simultaneously, creating
// wild birth oscillations (564 → 5,024 → 564 between snapshots).
//
// Sigmoid centered at 0.3 (old threshold):
//
//	At Belonging 0.15: ~5% chance (hard floor below 0.15)
//	At Belonging 0.20: ~20% chance
//	At Belonging 0.30: ~50% chance (matches old threshold behavior)
//	At Belonging 0.40: ~80% chance
//	At Belonging 0.50: ~95% chance
//
// Uses agent ID + simDay for deterministic per-agent-per-day evaluation.
func birthEligible(a *agents.Agent, simDay uint64) bool {
	b := float64(a.Needs.Belonging)

	// Hard floor: truly isolated agents don't reproduce.
	if b < 0.15 {
		return false
	}

	// Sigmoid: 1 / (1 + exp(-steepness * (b - midpoint)))
	midpoint := 0.3
	steepness := 10.0 * phi.Being // ~16.18
	prob := 1.0 / (1.0 + math.Exp(-steepness*(b-midpoint)))

	// Deterministic check: hash agent ID with simDay for stable daily result.
	hash := (uint64(a.ID)*2654435761 + simDay*40503) % 1000
	return float64(hash)/1000.0 < prob
}

// addAgent registers a new agent in all indexes.
func (s *Simulation) addAgent(a *agents.Agent) {
	s.Agents = append(s.Agents, a)
	s.AgentIndex[a.ID] = a
	if a.HomeSettID != nil {
		s.SettlementAgents[*a.HomeSettID] = append(s.SettlementAgents[*a.HomeSettID], a)
	}
}

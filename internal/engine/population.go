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
					s.EmitEvent(Event{
						Tick:        tick,
						Description: fmt.Sprintf("%s has died of old age at %d", a.Name, a.Age),
						Category:    "death",
						Meta: map[string]any{
							"agent_id":      a.ID,
							"agent_name":    a.Name,
							"settlement_id": a.HomeSettID,
							"cause":         "age",
						},
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
				s.EmitEvent(Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s has died of illness", a.Name),
					Category:    "death",
					Meta: map[string]any{
						"agent_id":      a.ID,
						"agent_name":    a.Name,
						"settlement_id": a.HomeSettID,
						"cause":         "illness",
					},
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

			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%s is born in %s", child.Name, sett.Name),
				Category:    "birth",
				Meta: map[string]any{
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
				},
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
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("%d refugees arrive in %s", needed, sett.Name),
				Category:    "social",
				Meta: map[string]any{
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
					"count":           needed,
				},
			})
		}

		// Famine relief: if >20% starving, emergency food arrives.
		if aliveCount > 0 && float64(starvingCount)/float64(aliveCount) > 0.2 {
			for _, a := range settAgents {
				if a.Alive && a.Needs.Survival < 0.1 {
					a.Inventory[agents.GoodGrain] += 3
				}
			}
			s.EmitEvent(Event{
				Tick:        tick,
				Description: fmt.Sprintf("Emergency food relief arrives in %s", sett.Name),
				Category:    "economy",
				Meta: map[string]any{
					"settlement_id":   sett.ID,
					"settlement_name": sett.Name,
				},
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

// processWeeklyTier2Replenishment fills Tier 2 vacancies by promoting the
// most notable Tier 0 adults. Ensures occupation diversity — prioritizes
// occupations with zero Tier 2 representation so fishers, hunters, and miners
// get individual agency alongside farmers and crafters.
func (s *Simulation) processWeeklyTier2Replenishment() {
	const targetTier2 = 30 // Same as initial PromoteToTier2 count
	const maxPerCycle = 2  // Gradual — don't flood with Tier 2 at once

	// Count alive Tier 2 by occupation.
	tier2ByOcc := make(map[agents.Occupation]int)
	aliveTier2 := 0
	for _, a := range s.Agents {
		if a.Alive && a.Tier == agents.Tier2 {
			aliveTier2++
			tier2ByOcc[a.Occupation]++
		}
	}

	// Collect eligible Tier 0 adults.
	var eligible []*agents.Agent
	for _, a := range s.Agents {
		if a.Alive && a.Tier == agents.Tier0 && a.Age >= 16 {
			eligible = append(eligible, a)
		}
	}
	if len(eligible) == 0 {
		return
	}

	// Priority pass: ensure occupation diversity even when target is met.
	// A world where no fisher has individual agency is structurally incomplete.
	// This runs BEFORE the vacancy check so dead merchants get replaced even
	// when total alive Tier 2 count meets target.
	diversityPromoted := 0
	const maxDiversity = 2
	allOccupations := []agents.Occupation{
		agents.OccupationFarmer, agents.OccupationMiner, agents.OccupationCrafter,
		agents.OccupationMerchant, agents.OccupationLaborer, agents.OccupationFisher,
		agents.OccupationHunter,
	}
	for _, occ := range allOccupations {
		if diversityPromoted >= maxDiversity {
			break
		}
		if tier2ByOcc[occ] > 0 {
			continue // This occupation already has Tier 2 representation
		}
		// Find the best candidate of this occupation.
		var best *agents.Agent
		var bestScore float64
		for _, a := range eligible {
			if a.Occupation != occ {
				continue
			}
			score := float64(a.Soul.CittaCoherence)*phi.Nous +
				float64(a.Soul.Gauss)*phi.Being
			if best == nil || score > bestScore {
				best = a
				bestScore = score
			}
		}
		if best != nil {
			best.Tier = agents.Tier2
			diversityPromoted++
			slog.Info("tier 2 diversity promotion",
				"agent", best.Name,
				"occupation", best.Occupation,
				"coherence", fmt.Sprintf("%.3f", best.Soul.CittaCoherence),
			)
			s.EmitEvent(Event{
				Tick:        s.LastTick,
				Description: fmt.Sprintf("%s rises to prominence in %s", best.Name, occupationLabel(best.Occupation)),
				Category:    "social",
				Meta: map[string]any{
					"agent_id":   best.ID,
					"agent_name": best.Name,
					"occupation": best.Occupation,
				},
			})
		}
	}

	// Standard vacancy fill — only if there are slots beyond diversity promotions.
	vacancies := targetTier2 - aliveTier2 - diversityPromoted
	if vacancies <= 0 {
		return
	}
	if vacancies > maxPerCycle {
		vacancies = maxPerCycle
	}

	promoted := 0

	// Remaining slots: promote top scorers from any occupation.
	if promoted < vacancies {
		remaining := vacancies - promoted
		agents.PromoteToTier2(eligible, remaining)
		for _, a := range eligible {
			if a.Tier == agents.Tier2 && promoted < vacancies {
				// Check if this was just promoted (not from the priority pass).
				// PromoteToTier2 sets Tier but we already set some above.
				// Count new promotions by checking if they weren't already logged.
				alreadyLogged := false
				for _, evt := range s.Events {
					if evt.Tick == s.LastTick && evt.Category == "social" &&
						len(evt.Description) > len(a.Name) &&
						evt.Description[:len(a.Name)] == a.Name {
						alreadyLogged = true
						break
					}
				}
				if !alreadyLogged {
					promoted++
					slog.Info("tier 2 promotion",
						"agent", a.Name,
						"occupation", a.Occupation,
						"coherence", fmt.Sprintf("%.3f", a.Soul.CittaCoherence),
					)
					s.EmitEvent(Event{
						Tick:        s.LastTick,
						Description: fmt.Sprintf("%s rises to prominence in %s", a.Name, occupationLabel(a.Occupation)),
						Category:    "social",
						Meta: map[string]any{
							"agent_id":   a.ID,
							"agent_name": a.Name,
							"occupation": a.Occupation,
						},
					})
				}
			}
		}
	}
}

// occupationLabel returns a human-readable label for an occupation.
func occupationLabel(occ agents.Occupation) string {
	switch occ {
	case agents.OccupationFarmer:
		return "farming"
	case agents.OccupationMiner:
		return "mining"
	case agents.OccupationFisher:
		return "fishing"
	case agents.OccupationHunter:
		return "hunting"
	case agents.OccupationCrafter:
		return "crafting"
	case agents.OccupationMerchant:
		return "trade"
	case agents.OccupationLaborer:
		return "labor"
	case agents.OccupationSoldier:
		return "the military"
	case agents.OccupationScholar:
		return "scholarship"
	case agents.OccupationAlchemist:
		return "alchemy"
	default:
		return "their craft"
	}
}

// addAgent registers a new agent in all indexes.
func (s *Simulation) addAgent(a *agents.Agent) {
	s.Agents = append(s.Agents, a)
	s.AgentIndex[a.ID] = a
	if a.HomeSettID != nil {
		s.SettlementAgents[*a.HomeSettID] = append(s.SettlementAgents[*a.HomeSettID], a)
	}
}

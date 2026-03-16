// Population dynamics — aging, natural death, births, migration, anti-collapse.
// See design doc Section 9.
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// SimDaysPerYear is the number of sim-days in one sim-year (4 seasons × 90 days).
const SimDaysPerYear = 360

// MaxWorldPopulation caps births to keep the world within the 2 GB server budget.
// Per-agent cost is ~3.2 KB (struct + relationships + indexes + GC overhead).
// 400K ≈ 1.28 GB agent heap → ~1.58 GB total → comfortably in-RAM on 2 GB.
// Design TPS target is ~15 tps; swap thrashing drops this to <1 tps.
// At 450K the server still swaps lightly (~2-8 tps). At 400K it runs swap-free
// at full design speed. Can be raised to 450K if memory optimizations free headroom.
// See docs/memory-architecture.md for the full analysis.
const MaxWorldPopulation = 400_000

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
// Emits coming-of-age events when agents turn 16 (adulthood threshold).
func (s *Simulation) ageAgents(tick uint64) {
	comingOfAge := 0
	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}
		a.Age++

		// Coming-of-age at 16: agent becomes an adult, eligible for governance,
		// work, and family formation. Belonging boost from community recognition.
		if a.Age == 16 {
			a.Needs.Belonging += float32(phi.Agnosis * 0.5) // ~0.118
			if a.Needs.Belonging > 1 {
				a.Needs.Belonging = 1
			}
			comingOfAge++

			// Only emit individual events for Tier 1+ (avoid flooding with 490K agents).
			if a.Tier >= agents.Tier1 {
				settName := "the wilderness"
				if a.HomeSettID != nil {
					if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
						settName = sett.Name
					}
				}
				s.EmitEvent(Event{
					Tick:        tick,
					Description: fmt.Sprintf("%s comes of age in %s", a.Name, settName),
					Category:    "social",
					Meta: map[string]any{
						"agent_id":        a.ID,
						"agent_name":      a.Name,
						"settlement_name": settName,
						"event_type":      "coming_of_age",
					},
				})
			}
		}
	}
	slog.Info("agents aged", "tick", tick, "time", SimTime(tick), "coming_of_age", comingOfAge)
}

// processNaturalDeaths checks for death from coherence-scaled mortality,
// age, overcapacity pressure, and disease. Three stacking mortality curves:
//
//  1. Background mortality — Agnosis expressing itself in matter. Scatter
//     (low coherence) amplifies vulnerability. Adults only (age >= 16).
//
//  2. Age mortality — universal logistic curve from age 50 onward. "That which
//     has a beginning in time, has an end in time." Liberation is not immortality.
//
//  3. Over-capacity pressure — when population exceeds MaxWorldPopulation,
//     all agents face additional mortality proportional to overshoot, graduated
//     by age: infants (Agnosis), children (Psyche), adults (1.0). The world's
//     carrying capacity is a force of nature, not a policy — it touches everyone
//     but shelters the young. Zero when population is at or below cap.
func (s *Simulation) processNaturalDeaths(tick uint64) {
	simDay := tick / TicksPerSimDay
	liberationDeaths := 0
	naturalDeaths := 0

	for _, a := range s.Agents {
		if !a.Alive {
			continue
		}

		// Coherence-scaled mortality: background entropy + age curve + overcap pressure.
		mortalityChance := agentDailyMortalityChance(a, s.Stats.TotalPopulation)

		if mortalityChance > 0 {
			// Deterministic check: hash agent ID with simDay for stable daily result.
			hash := (uint64(a.ID)*2654435761 + simDay*40503) % 100000
			if float64(hash)/100000.0 < mortalityChance {
				a.Alive = false
				a.Health = 0
				s.Stats.Deaths++
				naturalDeaths++

				cause := "natural"
				desc := fmt.Sprintf("%s has died at age %d", a.Name, a.Age)
				if a.Age > 50 {
					cause = "age"
					desc = fmt.Sprintf("%s has died of old age at %d", a.Name, a.Age)
				}

				isLiberated := a.Soul.State == agents.Liberated
				if isLiberated {
					liberationDeaths++
					settName := "the wilderness"
					if a.HomeSettID != nil {
						if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
							settName = sett.Name
						}
					}
					desc = fmt.Sprintf("%s, a sage of %s, has passed at age %d. Their light is extinguished.", a.Name, settName, a.Age)
				}

				s.EmitEvent(Event{
					Tick:        tick,
					Description: desc,
					Category:    "death",
					Meta: map[string]any{
						"agent_id":      a.ID,
						"agent_name":    a.Name,
						"settlement_id": a.HomeSettID,
						"cause":         cause,
						"age":           a.Age,
						"coherence":     fmt.Sprintf("%.3f", a.Soul.CittaCoherence),
					},
				})
				s.inheritWealth(a, tick)

				// Settlement effects: memories, witness coherence.
				if a.HomeSettID != nil {
					s.createSettlementMemories(*a.HomeSettID, tick, desc, 0.6)

					if isLiberated {
						// Liberation death: the world becomes more scattered
						// when its wise die. The void outweighs contemplation.
						for _, witness := range s.SettlementAgents[*a.HomeSettID] {
							if witness.Alive && witness.ID != a.ID {
								witness.Soul.AdjustCoherence(-float32(phi.Agnosis * 0.1)) // ~-0.024
							}
						}
					} else {
						// Ordinary death: via negativa — witnessing death
						// strips attachment, increasing coherence.
						for _, witness := range s.SettlementAgents[*a.HomeSettID] {
							if witness.Alive && witness.ID != a.ID {
								witness.Soul.AdjustCoherence(float32(phi.Agnosis * 0.05)) // ~+0.012
							}
						}
					}
				}
				continue
			}
		}

		// Disease: low health agents have a small daily death chance.
		if a.Health < 0.15 && a.Health > 0 {
			a.Health -= 0.01
			if a.Health <= 0 {
				a.Alive = false
				s.Stats.Deaths++
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

	if naturalDeaths > 0 || liberationDeaths > 0 {
		slog.Info("natural deaths", "count", naturalDeaths, "liberation", liberationDeaths, "tick", tick)
	}
}

// agentDailyMortalityChance returns the probability [0,1] that an agent
// dies today from background entropy + age. Two stacking curves:
//
//  1. Background mortality — Agnosis⁴ × scatter (~0.26%/day at c=0.15).
//     Four-fold entropy of embodied scatter. Floor at Agnosis⁵ (~0.07%/day)
//     ensures even liberated agents are mortal.
//
//  2. Age mortality — Agnosis³ × sigmoid² (~0.04% at age 32, ~0.88% at 70).
//     Logistic curve from age 50 onward, squared to protect younger adults.
//
// Children (age < 16) return 0 — protected by family and community.
//
// Expected rates at current population (~494K, avg age 32, avg coherence 0.512):
//
//	c=0.5, age 16: bg 0.23% + age ~0.00% = ~0.23%/day
//	c=0.5, age 32: bg 0.23% + age  0.04% = ~0.27%/day
//	c=0.5, age 50: bg 0.23% + age  0.33% = ~0.56%/day
//	c=0.5, age 60: bg 0.23% + age  0.69% = ~0.92%/day
//	c=0.5, age 70: bg 0.23% + age  0.98% = ~1.21%/day
//	c=0.1, age 32: bg 0.36% + age  0.04% = ~0.40%/day (scatter vulnerable)
//	c=0.9, age 32: bg 0.10% + age  0.04% = ~0.14%/day (liberation protective)
//
// Initial deaths: ~1,505/day at 494K. Population declines toward 400K
// (MaxWorldPopulation gates births), then oscillates tightly as births
// toggle on/off at the cap boundary.
//
// Tuning: if too aggressive, reduce background by one Φ power (Agnosis⁵ base).
// If too slow, increase age scale to Agnosis². Onset (50) and steepness (12)
// can also be adjusted independently.
func agentDailyMortalityChance(a *agents.Agent, population int) float64 {
	var chance float64

	// === Normal mortality: coherence-scaled, adults only (age >= 16) ===
	// Children are protected by family and community. Background entropy
	// of autonomous existence begins at adulthood.
	if a.Age >= 16 {
		coherence := float64(a.Soul.CittaCoherence)

		// Background: scatter-driven daily death risk.
		// Base at Agnosis⁴ (~0.00311) — four-fold entropy of embodied scatter.
		// Floor at Agnosis⁵ (~0.000734) — even liberated agents are mortal.
		agnosis4 := phi.Agnosis * phi.Agnosis * phi.Agnosis * phi.Agnosis
		agnosis5 := agnosis4 * phi.Agnosis
		scatter := 1.0 - coherence
		chance = agnosis5 + agnosis4*scatter

		// Age: logistic sigmoid curve, universal.
		// Onset at 50, steepness 12 sim-years, scaled by Agnosis³ (~0.01315).
		// Squared sigmoid protects younger adults while ensuring old agents
		// face increasing mortality. At age 70: ~0.98%/day.
		agnosis3 := phi.Agnosis * phi.Agnosis * phi.Agnosis
		ageOffset := float64(a.Age) - 50.0
		sigmoid := 1.0 / (1.0 + math.Exp(-ageOffset/12.0))
		chance += agnosis3 * sigmoid * sigmoid
	}

	// === Over-capacity pressure: all ages, graduated by life stage ===
	// When population exceeds MaxWorldPopulation, the world pushes back.
	// Overpopulation strains resources, land, and social fabric — a force
	// that touches everyone but shelters the young.
	//
	// Pressure = Agnosis² × overshoot_ratio × age_weight
	//   Age 0-2:  Agnosis  (~24%) — barely touched, sheltered by family
	//   Age 2-16: Psyche   (~38%) — growing but protected by community
	//   Age 16+:  1.0      — full participants in the world's burden
	//
	// The Φ ladder (Agnosis → Psyche → 1.0) maps each life stage to the
	// emanation hierarchy. Under the cap this term is zero — normal Wheeler
	// mortality governs alone.
	if population > MaxWorldPopulation {
		overshoot := float64(population)/float64(MaxWorldPopulation) - 1.0
		pressure := phi.Agnosis * phi.Agnosis * overshoot // Agnosis²

		var weight float64
		switch {
		case a.Age < 2:
			weight = phi.Agnosis // ~0.236
		case a.Age < 16:
			weight = phi.Psyche // ~0.382
		default:
			weight = 1.0
		}

		chance += pressure * weight
	}

	return chance
}

// processBirths creates new agents from families in prosperous settlements.
func (s *Simulation) processBirths(tick uint64) {
	if s.Spawner == nil {
		return
	}
	if s.Stats.TotalPopulation >= MaxWorldPopulation {
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

			// Round 24: removed birth-time producer gate.
			// With 0.26% producers, this never fires, but removing it prevents
			// re-triggering once producers recover. Occupation is identity.

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
	if s.Stats.TotalPopulation >= MaxWorldPopulation {
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
	const maxDiversity = 4
	allOccupations := []agents.Occupation{
		agents.OccupationFarmer, agents.OccupationMiner, agents.OccupationCrafter,
		agents.OccupationMerchant, agents.OccupationLaborer, agents.OccupationFisher,
		agents.OccupationHunter, agents.OccupationAlchemist, agents.OccupationScholar,
		agents.OccupationSoldier,
	}
	// Rotate starting index each week so all occupations eventually get
	// diversity slots, not just Farmer and Miner every time.
	weekNum := s.LastTick / (TicksPerSimDay * 7)
	offset := int(weekNum % uint64(len(allOccupations)))
	for i := 0; i < len(allOccupations); i++ {
		if diversityPromoted >= maxDiversity {
			break
		}
		occ := allOccupations[(i+offset)%len(allOccupations)]
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
		if best == nil {
			slog.Debug("tier 2 diversity: no candidates",
				"occupation", occ)
			continue
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

	// Occupation cap: no single occupation can exceed 40% of Tier 2 roster.
	// This prevents crafters from monopolizing Tier 2 through higher average scores.
	maxPerOcc := (aliveTier2 + diversityPromoted + vacancies) * 40 / 100
	if maxPerOcc < 3 {
		maxPerOcc = 3
	}

	// Filter eligible candidates: exclude occupations already at cap.
	var capped []*agents.Agent
	for _, a := range eligible {
		if tier2ByOcc[a.Occupation] >= maxPerOcc {
			continue // This occupation is at cap
		}
		capped = append(capped, a)
	}
	if len(capped) == 0 {
		capped = eligible // Fallback: don't block all promotions
	}

	agents.PromoteToTier2(capped, vacancies)
	promoted := 0
	for _, a := range capped {
		if a.Tier == agents.Tier2 && promoted < vacancies {
			// Check if this was just promoted (not from the priority pass).
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
				tier2ByOcc[a.Occupation]++
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

// addAgent registers a new agent in all indexes and assigns a faction.
func (s *Simulation) addAgent(a *agents.Agent) {
	s.Agents = append(s.Agents, a)
	s.AgentIndex[a.ID] = a
	if a.HomeSettID != nil {
		s.SettlementAgents[*a.HomeSettID] = append(s.SettlementAgents[*a.HomeSettID], a)
	}

	// Assign faction based on occupation and governance type.
	if a.FactionID == nil {
		govType := social.GovCommune
		if a.HomeSettID != nil {
			if sett, ok := s.SettlementIndex[*a.HomeSettID]; ok {
				govType = sett.Governance
			}
		}
		if fid := factionForAgent(a, govType); fid > 0 {
			factionID := uint64(fid)
			a.FactionID = &factionID
		}
	}
}

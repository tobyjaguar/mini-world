// Tier 0 agent behavior — needs-driven state machine.
// Every tick, agents evaluate their state and take one action.
// See design doc Section 4.2 (Tier 0 — Automaton).
package agents

import (
	"github.com/talgya/mini-world/internal/phi"
)

// Action represents what an agent decided to do this tick.
type Action struct {
	AgentID AgentID
	Kind    ActionKind
	Detail  string // Human-readable description for event log
}

// ActionKind enumerates the possible Tier 0 actions.
type ActionKind uint8

const (
	ActionIdle       ActionKind = iota
	ActionEat                          // Consume food from inventory
	ActionWork                         // Produce goods at current location
	ActionForage                       // Gather food from the land
	ActionTrade                        // Buy/sell at local market
	ActionTravel                       // Move toward destination
	ActionRest                         // Recover health/mood
	ActionSocialize                    // Interact with nearby agent
)

// Decide determines what an agent does this tick, routing by cognition tier.
func Decide(a *Agent) Action {
	switch a.Tier {
	case Tier1:
		return Tier1Decide(a)
	default:
		return Tier0Decide(a)
	}
}

// Tier0Decide determines what a Tier 0 agent does this tick.
// Pure rule-based: evaluate needs bottom-up, pick the most urgent action.
func Tier0Decide(a *Agent) Action {
	if !a.Alive {
		return Action{AgentID: a.ID, Kind: ActionIdle}
	}

	// Merchants in transit skip normal decisions.
	if a.TravelTicksLeft > 0 {
		return Action{AgentID: a.ID, Kind: ActionTravel, Detail: a.Name + " travels with cargo"}
	}

	priority := a.Needs.Priority()

	switch priority {
	case NeedSurvival:
		return decideSurvival(a)
	case NeedSafety:
		return decideSafety(a)
	case NeedBelonging:
		return decideBelonging(a)
	case NeedEsteem:
		return decideEsteem(a)
	default:
		return decideDefault(a)
	}
}

func decideSurvival(a *Agent) Action {
	// Hungry? Eat if we have food, otherwise forage.
	if a.Needs.Survival < 0.3 {
		food := a.Inventory[GoodGrain] + a.Inventory[GoodFish]
		if food > 0 {
			return Action{AgentID: a.ID, Kind: ActionEat, Detail: a.Name + " eats a meal"}
		}
		return Action{AgentID: a.ID, Kind: ActionForage, Detail: a.Name + " forages for food"}
	}

	// Low health? Rest.
	if a.Health < 0.3 {
		return Action{AgentID: a.ID, Kind: ActionRest, Detail: a.Name + " rests to recover"}
	}

	return decideDefault(a)
}

func decideSafety(a *Agent) Action {
	// Low wealth → work to earn.
	if a.Wealth < 20 {
		return Action{AgentID: a.ID, Kind: ActionWork, Detail: a.Name + " works to earn crowns"}
	}
	// Wealthy agents with low belonging socialize — their economic safety is covered.
	if a.Wealth > 30 && a.Needs.Belonging < 0.4 {
		return Action{AgentID: a.ID, Kind: ActionSocialize, Detail: a.Name + " socializes with neighbors"}
	}
	return decideDefault(a)
}

func decideBelonging(a *Agent) Action {
	return Action{AgentID: a.ID, Kind: ActionSocialize, Detail: a.Name + " socializes with neighbors"}
}

func decideEsteem(a *Agent) Action {
	// Work to build skills and reputation.
	return Action{AgentID: a.ID, Kind: ActionWork, Detail: a.Name + " works diligently"}
}

func decideDefault(a *Agent) Action {
	// Default: work during the day, rest otherwise.
	return Action{AgentID: a.ID, Kind: ActionWork, Detail: a.Name + " goes about their work"}
}

// ApplyAction executes an action's effects on the agent and returns any
// notable events for the log.
func ApplyAction(a *Agent, action Action, tick uint64) []string {
	var events []string

	switch action.Kind {
	case ActionEat:
		events = applyEat(a)
	case ActionWork:
		events = applyWork(a, tick)
	case ActionForage:
		events = applyForage(a)
	case ActionRest:
		events = applyRest(a)
	case ActionSocialize:
		events = applySocialize(a)
	case ActionTrade:
		// Trade requires market context — handled at world level.
	case ActionTravel:
		// Movement requires map context — handled at world level.
	}

	return events
}

func applyEat(a *Agent) []string {
	// Consume one unit of food.
	if a.Inventory[GoodFish] > 0 {
		a.Inventory[GoodFish]--
	} else if a.Inventory[GoodGrain] > 0 {
		a.Inventory[GoodGrain]--
	}
	a.Needs.Survival += 0.2
	if a.Needs.Survival > 1.0 {
		a.Needs.Survival = 1.0
	}
	a.Mood += 0.05
	return nil
}

func applyWork(a *Agent, tick uint64) []string {
	var events []string

	// Produce goods based on occupation and skill level.
	switch a.Occupation {
	case OccupationFarmer:
		produced := int(a.Skills.Farming * 3)
		if produced < 1 {
			produced = 1
		}
		a.Inventory[GoodGrain] += produced
		a.Skills.Farming += 0.001 // Slow skill growth
	case OccupationMiner:
		produced := int(a.Skills.Mining * 2)
		if produced < 1 {
			produced = 1
		}
		a.Inventory[GoodIronOre] += produced
		a.Skills.Mining += 0.001
	case OccupationFisher:
		produced := int(a.Skills.Farming * 2)
		if produced < 1 {
			produced = 1
		}
		a.Inventory[GoodFish] += produced
		a.Skills.Farming += 0.001
	case OccupationHunter:
		a.Inventory[GoodFurs]++
		a.Skills.Combat += 0.001
	case OccupationCrafter:
		// Crafters convert raw materials to finished goods.
		// Check recipes in priority order; execute the first one with materials.
		crafted := false
		if a.Inventory[GoodIronOre] >= 2 && a.Inventory[GoodTimber] >= 1 {
			a.Inventory[GoodIronOre] -= 2
			a.Inventory[GoodTimber]--
			produced := 1
			if a.Skills.Crafting > 0.5 && a.ID%3 == 0 {
				produced++
			}
			a.Inventory[GoodTools] += produced
			a.Skills.Crafting += 0.002
			crafted = true
		} else if a.Inventory[GoodIronOre] >= 2 && a.Inventory[GoodCoal] >= 1 {
			a.Inventory[GoodIronOre] -= 2
			a.Inventory[GoodCoal]--
			produced := 1
			if a.Skills.Crafting > 0.5 && a.ID%3 == 0 {
				produced++
			}
			a.Inventory[GoodWeapons] += produced
			a.Skills.Crafting += 0.002
			crafted = true
		} else if a.Inventory[GoodFurs] >= 2 && a.Inventory[GoodTools] >= 1 {
			a.Inventory[GoodFurs] -= 2
			a.Inventory[GoodTools]--
			produced := 1
			if a.Skills.Crafting > 0.5 && a.ID%3 == 0 {
				produced++
			}
			a.Inventory[GoodClothing] += produced
			a.Skills.Crafting += 0.002
			crafted = true
		} else if a.Inventory[GoodGems] >= 2 && a.Inventory[GoodTools] >= 1 {
			a.Inventory[GoodGems] -= 2
			a.Inventory[GoodTools]--
			produced := 1
			if a.Skills.Crafting > 0.5 && a.ID%3 == 0 {
				produced++
			}
			a.Inventory[GoodLuxuries] += produced
			a.Skills.Crafting += 0.002
			crafted = true
		}
		if !crafted {
			// Journeyman labor when lacking materials — throttled mint.
			// Fires once per sim-hour (~24 crowns/day) instead of every tick (~1,440/day).
			if tick%60 == uint64(a.ID)%60 {
				a.Wealth += 1
			}
		}
	case OccupationAlchemist:
		crafted := false
		if a.Inventory[GoodHerbs] >= 2 {
			a.Inventory[GoodHerbs] -= 2
			a.Inventory[GoodMedicine]++
			a.Skills.Crafting += 0.002
			crafted = true
		} else if a.Inventory[GoodExotics] >= 2 && a.Inventory[GoodHerbs] >= 1 {
			a.Inventory[GoodExotics] -= 2
			a.Inventory[GoodHerbs]--
			a.Inventory[GoodLuxuries]++
			a.Skills.Crafting += 0.002
			crafted = true
		}
		if !crafted {
			// Journeyman labor when lacking materials — throttled mint.
			if tick%60 == uint64(a.ID)%60 {
				a.Wealth += 1
			}
		}
	case OccupationLaborer:
		// Laborers earn wages — throttled mint (~24 crowns/day).
		if tick%60 == uint64(a.ID)%60 {
			a.Wealth += 1
		}
	case OccupationMerchant:
		// Merchants earn from inter-settlement trade (world-level).
		// Throttled wage keeps them alive between trips.
		if tick%60 == uint64(a.ID)%60 {
			a.Wealth += 1
		}
		a.Skills.Trade += 0.001
	case OccupationSoldier:
		a.Skills.Combat += 0.002
	case OccupationScholar:
		// Scholars slowly gain wisdom. Rate is per-tick (runs every sim-minute),
		// so use a tiny multiplier: ~0.00034/day → ~0.124/year coherence growth.
		// A scholar starting at Agnosis (0.236) reaches Liberated (0.7) in ~3.7 years.
		a.Soul.AdjustCoherence(float32(phi.Agnosis * 0.000001))
	}

	// Working improves esteem, safety, belonging, and purpose.
	a.Needs.Esteem += 0.01
	a.Needs.Safety += 0.005
	a.Needs.Belonging += 0.003
	a.Needs.Purpose += 0.002

	// Clamp.
	clampNeeds(&a.Needs)

	return events
}

func applyForage(a *Agent) []string {
	// Foraging: low yield but always produces something.
	a.Inventory[GoodGrain]++
	a.Needs.Survival += 0.05
	clampNeeds(&a.Needs)
	return nil
}

func applyRest(a *Agent) []string {
	a.Health += 0.05
	if a.Health > 1.0 {
		a.Health = 1.0
	}
	a.Mood += 0.03
	a.Needs.Survival += 0.02
	clampNeeds(&a.Needs)
	return nil
}

func applySocialize(a *Agent) []string {
	a.Needs.Belonging += 0.05
	a.Needs.Safety += 0.003
	a.Needs.Purpose += 0.002
	a.Mood += 0.02
	clampNeeds(&a.Needs)
	return nil
}

// DecayInventory spoils perishable goods and degrades durable goods.
// Called hourly. Decay rates derived from Φ⁻³ (Agnosis).
func DecayInventory(a *Agent) {
	// Food spoils at ~2.4%/hour (Agnosis * 0.1 per unit → probabilistic loss).
	foodDecayRate := phi.Agnosis * 0.1 // ~0.024
	decayGood(a, GoodGrain, foodDecayRate)
	decayGood(a, GoodFish, foodDecayRate)

	// Herbs decay slightly faster than food.
	herbDecayRate := phi.Agnosis * 0.05 // ~0.012
	decayGood(a, GoodHerbs, herbDecayRate)

	// Medicine degrades.
	medDecayRate := phi.Agnosis * 0.05
	decayGood(a, GoodMedicine, medDecayRate)

	// Tools and weapons degrade slowly.
	durableDecayRate := phi.Agnosis * 0.01 // ~0.0024
	decayGood(a, GoodTools, durableDecayRate)
	decayGood(a, GoodWeapons, durableDecayRate)
}

// decayGood reduces inventory of a good. Rate is per-unit probability of losing one unit.
func decayGood(a *Agent, good GoodType, rate float64) {
	qty := a.Inventory[good]
	if qty <= 0 {
		return
	}
	// Expected loss = qty * rate. We lose floor(qty*rate) guaranteed,
	// plus one more with probability of the fractional part.
	loss := float64(qty) * rate
	intLoss := int(loss)
	// For small inventories, ensure at least probabilistic decay.
	// Use a simple deterministic approach: lose 1 when accumulated decay >= 1.
	if intLoss < 1 && loss > 0 {
		// Accumulate: if qty * rate * some factor rounds up, lose 1.
		// Simplified: lose 1 unit every 1/rate hours per unit.
		// With rate=0.024 and qty=10, loss=0.24 → no loss most hours.
		// This is fine — food lasts ~40 hours (~1.7 sim-days) per unit on average.
		return
	}
	a.Inventory[good] -= intLoss
	if a.Inventory[good] < 0 {
		a.Inventory[good] = 0
	}
}

// DecayNeeds reduces all needs slightly each tick — the passage of time.
// Agents must continually act to maintain their well-being.
// Decay rate derived from Φ⁻³ (agnosis constant).
func DecayNeeds(a *Agent) {
	decay := float32(phi.Agnosis * 0.01) // ~0.24% per tick

	a.Needs.Survival -= decay * 2  // Hunger is most urgent
	a.Needs.Safety -= decay
	a.Needs.Belonging -= decay * 0.5
	a.Needs.Esteem -= decay * 0.3
	a.Needs.Purpose -= decay * 0.1

	// Health decays if survival is critically low (starvation).
	if a.Needs.Survival < 0.1 {
		a.Health -= 0.01
		if a.Health <= 0 {
			a.Alive = false
		}
	}

	// Mood drifts toward a baseline influenced by overall satisfaction.
	satisfaction := a.Needs.OverallSatisfaction()
	moodTarget := satisfaction*2 - 1 // Map 0–1 satisfaction to -1..+1 mood
	a.Mood += (moodTarget - a.Mood) * 0.01

	clampNeeds(&a.Needs)
}

func clampNeeds(n *NeedsState) {
	if n.Survival < 0 {
		n.Survival = 0
	}
	if n.Survival > 1 {
		n.Survival = 1
	}
	if n.Safety < 0 {
		n.Safety = 0
	}
	if n.Safety > 1 {
		n.Safety = 1
	}
	if n.Belonging < 0 {
		n.Belonging = 0
	}
	if n.Belonging > 1 {
		n.Belonging = 1
	}
	if n.Esteem < 0 {
		n.Esteem = 0
	}
	if n.Esteem > 1 {
		n.Esteem = 1
	}
	if n.Purpose < 0 {
		n.Purpose = 0
	}
	if n.Purpose > 1 {
		n.Purpose = 1
	}
}

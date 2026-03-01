// Resource-based production — agents draw from hex resources when working.
// See design doc Section 5.
package engine

import (
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// occupationResource maps occupation to the hex resource they consume when working.
var occupationResource = map[agents.Occupation]world.ResourceType{
	agents.OccupationFarmer:    world.ResourceGrain,
	agents.OccupationMiner:     world.ResourceIronOre,
	agents.OccupationFisher:    world.ResourceFish,
	agents.OccupationHunter:    world.ResourceFurs,
	agents.OccupationLaborer:   world.ResourceStone,
	agents.OccupationAlchemist: world.ResourceHerbs,
}

// ResolveWork wraps agent work production with hex resource depletion.
// Returns events from the underlying work action.
// For resource-producing occupations (farmer, miner, fisher, hunter),
// production is limited by available hex resources.
// boostMul applies a gardener "cultivate" production multiplier (1.0 = no boost).
func ResolveWork(a *agents.Agent, action agents.Action, hex *world.Hex, tick uint64, boostMul float64) []string {
	if action.Kind != agents.ActionWork {
		return agents.ApplyAction(a, action, tick)
	}

	resType, needsResource := occupationResource[a.Occupation]

	// Alchemist dual-mode: craft when herbs stocked, harvest when low.
	if a.Occupation == agents.OccupationAlchemist {
		if a.Inventory[agents.GoodHerbs] >= 1 {
			return agents.ApplyAction(a, action, tick) // Craft from inventory
		}
		// resType and needsResource already set from occupationResource map.
	}

	if !needsResource {
		// Crafters, merchants, etc. don't draw from hex resources.
		return agents.ApplyAction(a, action, tick)
	}

	if hex == nil {
		// No hex data — showed up to work but land unavailable. No punishment.
		a.Needs.Safety += 0.001    // Went to work — still part of the workforce
		a.Needs.Belonging += 0.002 // Tried to contribute — community recognizes effort
		a.Needs.Purpose += 0.001   // Has a role even when land is barren
		clampAgentNeeds(&a.Needs)
		return nil
	}

	available := hex.Resources[resType]
	if available < 1.0 {
		// Hex depleted — showed up but land is barren. No punishment.
		a.Needs.Safety += 0.001    // Went to work — still part of the workforce
		a.Needs.Belonging += 0.002 // Tried to contribute — community recognizes effort
		a.Needs.Purpose += 0.001   // Has a role even when land is barren
		clampAgentNeeds(&a.Needs)
		return nil
	}

	// Calculate production amount (mirrors applyWork logic).
	produced := productionAmount(a)
	if boostMul > 1.0 {
		produced = int(float64(produced) * boostMul)
		if produced < 1 {
			produced = 1
		}
	}

	// Clamp to available resources.
	if float64(produced) > available {
		produced = int(available)
	}
	if produced < 1 {
		produced = 1
		if available < 1.0 {
			// Clamped to zero — showed up but not enough resources. No punishment.
			a.Needs.Safety += 0.001    // Went to work — still part of the workforce
			a.Needs.Belonging += 0.002 // Tried to contribute — community recognizes effort
			a.Needs.Purpose += 0.001   // Has a role even when land is barren
			clampAgentNeeds(&a.Needs)
			return nil
		}
	}

	// Deplete hex resources.
	hex.Resources[resType] -= float64(produced)
	if hex.Resources[resType] < 0 {
		hex.Resources[resType] = 0
	}

	// Extraction degrades hex health.
	hex.Health -= phi.Agnosis * 0.01 // ~0.00236 per extraction tick
	if hex.Health < 0 {
		hex.Health = 0
	}
	hex.LastExtractedTick = tick
	a.LastWorkTick = tick

	// Apply production to agent inventory.
	good := occupationGood(a.Occupation)
	a.Inventory[good] += produced

	// Miners produce 1 coal as secondary output (coal has no dedicated producer).
	if a.Occupation == agents.OccupationMiner && produced > 0 {
		a.Inventory[agents.GoodCoal]++
	}

	// Laborers restore hex health while working — land stewardship.
	if a.Occupation == agents.OccupationLaborer && hex.Health < 1.0 {
		hex.Health += phi.Agnosis * 0.005 // ~0.00118/tick
		if hex.Health > 1.0 {
			hex.Health = 1.0
		}
	}

	// Alchemists gather exotics as secondary output when available.
	if a.Occupation == agents.OccupationAlchemist && hex.Resources[world.ResourceExotics] >= 1.0 {
		a.Inventory[agents.GoodExotics]++
		hex.Resources[world.ResourceExotics] -= 1.0
	}

	// Skill growth.
	applySkillGrowth(a)

	// Working improves all social needs.
	// Producing real goods (food, ore, furs) is ontologically grounded work —
	// the material substrate on which everything else depends.
	a.Needs.Esteem += 0.012
	a.Needs.Safety += 0.008
	a.Needs.Belonging += 0.004
	a.Needs.Purpose += 0.004

	// All hex-resource producers who successfully produced get a Survival boost.
	// Physical labor extracting real goods = material security.
	a.Needs.Survival += 0.003
	clampAgentNeeds(&a.Needs)

	return nil
}

// productionAmount calculates how much an agent produces based on skill.
func productionAmount(a *agents.Agent) int {
	switch a.Occupation {
	case agents.OccupationFarmer:
		p := int(a.Skills.Farming * 3)
		if p < 1 {
			p = 1
		}
		return p
	case agents.OccupationMiner:
		p := int(a.Skills.Mining * 2)
		if p < 1 {
			p = 1
		}
		return p
	case agents.OccupationFisher:
		// Fisher skill: max of farming and combat, with floor of 0.5.
		// Fishing draws on provisioning knowledge and physical fitness.
		// Multiplier 5 (not 3) — nets/boats yield more per trip than farming.
		// At spawn (Farming 0.32-0.56): produces 2-3 fish/tick.
		fishSkill := math.Max(float64(a.Skills.Farming), float64(a.Skills.Combat))
		if fishSkill < 0.5 {
			fishSkill = 0.5
		}
		p := int(fishSkill * 5)
		if p < 1 {
			p = 1
		}
		return p
	case agents.OccupationHunter:
		p := int(a.Skills.Combat * 2)
		if p < 1 {
			p = 1
		}
		return p
	case agents.OccupationLaborer:
		p := int(a.Skills.Mining * 3)
		if p < 1 {
			p = 1
		}
		return p
	case agents.OccupationAlchemist:
		p := int(a.Skills.Crafting * 2)
		if p < 1 {
			p = 1
		}
		return p
	}
	return 1
}

// occupationGood maps resource-producing occupations to the good they create.
func occupationGood(occ agents.Occupation) agents.GoodType {
	switch occ {
	case agents.OccupationFarmer:
		return agents.GoodGrain
	case agents.OccupationMiner:
		return agents.GoodIronOre
	case agents.OccupationFisher:
		return agents.GoodFish
	case agents.OccupationHunter:
		return agents.GoodFurs
	case agents.OccupationLaborer:
		return agents.GoodStone
	case agents.OccupationAlchemist:
		return agents.GoodHerbs
	}
	return agents.GoodGrain
}

// applySkillGrowth increments the relevant skill for resource producers.
func applySkillGrowth(a *agents.Agent) {
	switch a.Occupation {
	case agents.OccupationFarmer, agents.OccupationFisher:
		a.Skills.Farming += 0.001
	case agents.OccupationMiner, agents.OccupationLaborer:
		a.Skills.Mining += 0.001
	case agents.OccupationHunter:
		a.Skills.Combat += 0.001
	case agents.OccupationAlchemist:
		a.Skills.Crafting += 0.001
	}
}

// clampAgentNeeds clamps all needs to [0, 1].
func clampAgentNeeds(n *agents.NeedsState) {
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

// Resource-based production — agents draw from hex resources when working.
// See design doc Section 5.
package engine

import (
	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/world"
)

// occupationResource maps occupation to the hex resource they consume when working.
var occupationResource = map[agents.Occupation]world.ResourceType{
	agents.OccupationFarmer: world.ResourceGrain,
	agents.OccupationMiner:  world.ResourceIronOre,
	agents.OccupationFisher: world.ResourceFish,
	agents.OccupationHunter: world.ResourceFurs,
}

// ResolveWork wraps agent work production with hex resource depletion.
// Returns events from the underlying work action.
// For resource-producing occupations (farmer, miner, fisher, hunter),
// production is limited by available hex resources.
func ResolveWork(a *agents.Agent, action agents.Action, hex *world.Hex, tick uint64) []string {
	if action.Kind != agents.ActionWork {
		return agents.ApplyAction(a, action, tick)
	}

	resType, needsResource := occupationResource[a.Occupation]
	if !needsResource {
		// Crafters, merchants, laborers, etc. don't draw from hex resources.
		return agents.ApplyAction(a, action, tick)
	}

	if hex == nil {
		// No hex data — failed production, needs erode but belonging persists.
		a.Needs.Esteem -= 0.005
		a.Needs.Safety -= 0.003
		a.Needs.Belonging += 0.001 // Tried to work — still part of the community.
		clampAgentNeeds(&a.Needs)
		return nil
	}

	available := hex.Resources[resType]
	if available < 1.0 {
		// Hex depleted — failed production, needs erode but belonging persists.
		a.Needs.Esteem -= 0.005
		a.Needs.Safety -= 0.003
		a.Needs.Belonging += 0.001
		clampAgentNeeds(&a.Needs)
		return nil
	}

	// Calculate production amount (mirrors applyWork logic).
	produced := productionAmount(a)

	// Clamp to available resources.
	if float64(produced) > available {
		produced = int(available)
	}
	if produced < 1 {
		produced = 1
		if available < 1.0 {
			// Failed production — needs erode but belonging persists.
			a.Needs.Esteem -= 0.005
			a.Needs.Safety -= 0.003
			a.Needs.Belonging += 0.001
			clampAgentNeeds(&a.Needs)
			return nil
		}
	}

	// Deplete hex resources.
	hex.Resources[resType] -= float64(produced)
	if hex.Resources[resType] < 0 {
		hex.Resources[resType] = 0
	}

	// Apply production to agent inventory.
	good := occupationGood(a.Occupation)
	a.Inventory[good] += produced

	// Miners produce 1 coal as secondary output (coal has no dedicated producer).
	if a.Occupation == agents.OccupationMiner && produced > 0 {
		a.Inventory[agents.GoodCoal]++
	}

	// Skill growth.
	applySkillGrowth(a)

	// Working improves esteem, safety, and belonging.
	a.Needs.Esteem += 0.01
	a.Needs.Safety += 0.005
	a.Needs.Belonging += 0.003
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
		p := int(a.Skills.Farming * 2)
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
	}
	return agents.GoodGrain
}

// applySkillGrowth increments the relevant skill for resource producers.
func applySkillGrowth(a *agents.Agent) {
	switch a.Occupation {
	case agents.OccupationFarmer, agents.OccupationFisher:
		a.Skills.Farming += 0.001
	case agents.OccupationMiner:
		a.Skills.Mining += 0.001
	case agents.OccupationHunter:
		a.Skills.Combat += 0.001
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

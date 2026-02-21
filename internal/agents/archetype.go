// Tier 1 archetype-guided cognition — behavioral templates that give 4% of agents personality.
// Each archetype adjusts need thresholds, preferred actions, and coherence growth.
// See design doc Section 4.2 (Tier 1 — Archetype-guided).
package agents

import (
	"github.com/talgya/mini-world/internal/phi"
)

// Archetype constants — the 8 behavioral templates.
const (
	ArchAmbitiousMerchant   = "AmbitiousMerchant"
	ArchDevoutTraditionalist = "DevoutTraditionalist"
	ArchFrontierExplorer    = "FrontierExplorer"
	ArchDisgruntledLaborer  = "DisgruntledLaborer"
	ArchSchemingNoble       = "SchemingNoble"
	ArchHealerSage          = "HealerSage"
	ArchMilitantGuard       = "MilitantGuard"
	ArchCuriousScholar      = "CuriousScholar"
)

// BehaviorTemplate defines how an archetype modifies base Tier 0 behavior.
type BehaviorTemplate struct {
	// PriorityOverrides shifts when needs feel "urgent" (lower = triggers sooner).
	PriorityOverrides map[NeedType]float32

	// PreferredAction is what the agent does when no need is urgent.
	PreferredAction ActionKind

	// CoherenceGrowth is bonus coherence per sim-day (Tier 1 grows faster than Tier 0).
	CoherenceGrowth float32

	// SocialBias adjusts socialization tendency (-1 to +1).
	SocialBias float32
}

// archetypeTemplates maps archetype name to its behavior template.
var archetypeTemplates = map[string]BehaviorTemplate{
	ArchAmbitiousMerchant: {
		PriorityOverrides: map[NeedType]float32{
			NeedSafety:  0.5,  // Tolerates lower safety before acting
			NeedEsteem:  0.15, // Very sensitive to esteem needs
		},
		PreferredAction: ActionTrade,
		CoherenceGrowth: float32(phi.Agnosis * 0.02),
		SocialBias:      0.3,
	},
	ArchDevoutTraditionalist: {
		PriorityOverrides: map[NeedType]float32{
			NeedBelonging: 0.2, // Strongly needs community
		},
		PreferredAction: ActionWork,
		CoherenceGrowth: float32(phi.Agnosis * 0.04),
		SocialBias:      0.5,
	},
	ArchFrontierExplorer: {
		PriorityOverrides: map[NeedType]float32{
			NeedSurvival: 0.2,  // Pushes survival limits
			NeedBelonging: 0.5, // Doesn't need community as much
		},
		PreferredAction: ActionForage,
		CoherenceGrowth: float32(phi.Agnosis * 0.03),
		SocialBias:      -0.3,
	},
	ArchDisgruntledLaborer: {
		PriorityOverrides: map[NeedType]float32{
			NeedSafety:  0.2, // Acutely aware of economic insecurity
			NeedEsteem:  0.2, // Feels disrespected easily
		},
		PreferredAction: ActionWork,
		CoherenceGrowth: float32(phi.Agnosis * 0.01),
		SocialBias:      0.1,
	},
	ArchSchemingNoble: {
		PriorityOverrides: map[NeedType]float32{
			NeedEsteem:    0.1, // Obsessed with status
			NeedBelonging: 0.4, // Uses people, doesn't need them
		},
		PreferredAction: ActionSocialize,
		CoherenceGrowth: float32(phi.Agnosis * 0.02),
		SocialBias:      0.4,
	},
	ArchHealerSage: {
		PriorityOverrides: map[NeedType]float32{
			NeedPurpose: 0.15, // Driven by meaning
		},
		PreferredAction: ActionWork,
		CoherenceGrowth: float32(phi.Agnosis * 0.05),
		SocialBias:      0.2,
	},
	ArchMilitantGuard: {
		PriorityOverrides: map[NeedType]float32{
			NeedSafety:    0.15, // Hypervigilant about security
			NeedBelonging: 0.4,  // Loyal but not needy
		},
		PreferredAction: ActionWork,
		CoherenceGrowth: float32(phi.Agnosis * 0.02),
		SocialBias:      -0.1,
	},
	ArchCuriousScholar: {
		PriorityOverrides: map[NeedType]float32{
			NeedPurpose:  0.1, // Intellectually driven
			NeedSurvival: 0.2, // Forgets to eat
		},
		PreferredAction: ActionWork,
		CoherenceGrowth: float32(phi.Agnosis * 0.04),
		SocialBias:      0.0,
	},
}

// Tier1Decide determines what a Tier 1 agent does this tick.
// Like Tier0Decide but uses archetype template to adjust thresholds and fallback.
func Tier1Decide(a *Agent) Action {
	if !a.Alive {
		return Action{AgentID: a.ID, Kind: ActionIdle}
	}

	// Merchants in transit skip normal decisions.
	if a.TravelTicksLeft > 0 {
		return Action{AgentID: a.ID, Kind: ActionTravel, Detail: a.Name + " travels with cargo"}
	}

	tmpl, ok := archetypeTemplates[a.Archetype]
	if !ok {
		// Fallback to Tier 0 if archetype is unknown.
		return Tier0Decide(a)
	}

	// Evaluate needs with archetype-adjusted thresholds.
	priority := priorityWithOverrides(&a.Needs, tmpl.PriorityOverrides)

	switch priority {
	case NeedSurvival:
		return decideSurvival(a)
	case NeedSafety:
		return decideSafety(a)
	case NeedBelonging:
		// Archetype social bias: more social archetypes socialize more eagerly.
		if tmpl.SocialBias > 0.2 {
			return Action{AgentID: a.ID, Kind: ActionSocialize, Detail: a.Name + " engages the community"}
		}
		return decideBelonging(a)
	case NeedEsteem:
		return decideEsteem(a)
	default:
		// No urgent need — use archetype's preferred action.
		return Action{AgentID: a.ID, Kind: tmpl.PreferredAction, Detail: a.Name + " pursues their calling"}
	}
}

// priorityWithOverrides evaluates needs using archetype-specific thresholds.
func priorityWithOverrides(n *NeedsState, overrides map[NeedType]float32) NeedType {
	threshold := func(need NeedType) float32 {
		if t, ok := overrides[need]; ok {
			return t
		}
		return 0.3 // Default threshold
	}

	if n.Survival < threshold(NeedSurvival) {
		return NeedSurvival
	}
	if n.Safety < threshold(NeedSafety) {
		return NeedSafety
	}
	if n.Belonging < threshold(NeedBelonging) {
		return NeedBelonging
	}
	if n.Esteem < threshold(NeedEsteem) {
		return NeedEsteem
	}
	return NeedPurpose
}

// ApplyTier1CoherenceGrowth gives Tier 1 agents their daily coherence bonus.
func ApplyTier1CoherenceGrowth(a *Agent) {
	if a.Tier != Tier1 || !a.Alive {
		return
	}
	tmpl, ok := archetypeTemplates[a.Archetype]
	if !ok {
		return
	}
	a.Soul.AdjustCoherence(tmpl.CoherenceGrowth)
}

// AssignArchetype determines the best archetype for an agent based on
// occupation, soul class, and element type.
func AssignArchetype(a *Agent) string {
	switch a.Occupation {
	case OccupationMerchant:
		return ArchAmbitiousMerchant
	case OccupationSoldier:
		return ArchMilitantGuard
	case OccupationScholar:
		return ArchCuriousScholar
	case OccupationAlchemist:
		if a.Soul.Class == Transcendentalist || a.Soul.CittaCoherence > 0.4 {
			return ArchHealerSage
		}
		return ArchCuriousScholar
	case OccupationFarmer:
		if a.Soul.CittaCoherence > 0.4 {
			return ArchDevoutTraditionalist
		}
		return ArchDisgruntledLaborer
	case OccupationLaborer:
		if a.Soul.CittaCoherence < 0.35 {
			return ArchDisgruntledLaborer
		}
		return ArchDevoutTraditionalist
	case OccupationHunter:
		return ArchFrontierExplorer
	default:
		// Nobles/leaders → scheming noble, others by class.
		if a.Role == RoleNoble || a.Role == RoleLeader {
			return ArchSchemingNoble
		}
		switch a.Soul.Element() {
		case ElementHydrogen:
			return ArchFrontierExplorer
		case ElementUranium:
			return ArchSchemingNoble
		default:
			return ArchDevoutTraditionalist
		}
	}
}

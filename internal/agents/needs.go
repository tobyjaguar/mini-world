// Package agents — NeedsState implements the Maslow-inspired needs hierarchy.
// See design doc Section 4.3.
package agents

// NeedsState tracks the fulfillment level of each need layer.
// All values range from 0.0 (completely unmet) to 1.0 (fully satisfied).
// Lower needs dominate behavior when unmet.
type NeedsState struct {
	Survival  float32 `json:"survival"`  // Food, water, shelter, health
	Safety    float32 `json:"safety"`    // Physical security, economic stability
	Belonging float32 `json:"belonging"` // Social connections, community
	Esteem    float32 `json:"esteem"`    // Reputation, relative wealth, skill mastery
	Purpose   float32 `json:"purpose"`   // Goals, legacy, meaning (Tier 1+ only)
}

// Priority returns the most urgent unmet need.
// Needs are evaluated bottom-up: a starving merchant doesn't trade, they forage.
func (n *NeedsState) Priority() NeedType {
	const threshold = 0.3 // Below this, the need is urgent

	if n.Survival < threshold {
		return NeedSurvival
	}
	if n.Safety < threshold {
		return NeedSafety
	}
	if n.Belonging < threshold {
		return NeedBelonging
	}
	if n.Esteem < threshold {
		return NeedEsteem
	}
	return NeedPurpose
}

// NeedType enumerates the need hierarchy layers.
type NeedType uint8

const (
	NeedSurvival  NeedType = iota
	NeedSafety
	NeedBelonging
	NeedEsteem
	NeedPurpose
)

// OverallSatisfaction returns a weighted average of all needs,
// with lower needs weighted more heavily.
func (n *NeedsState) OverallSatisfaction() float32 {
	// Weights: survival matters most, purpose least
	return (n.Survival*5 + n.Safety*4 + n.Belonging*3 + n.Esteem*2 + n.Purpose*1) / 15
}

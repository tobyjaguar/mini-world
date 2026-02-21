// Package phi provides the ConjugateField interface — the holographic self-similar
// pattern that repeats at every scale of the simulation (agent, settlement, world economy).
// See design doc Section 16.6.
package phi

// ConjugateField represents any entity with conjugate charge/discharge dynamics.
// The same interface is implemented at agent, settlement, and world economy scales.
type ConjugateField interface {
	// ChargingPressure returns centripetal accumulation pressure (saving, producing, investing).
	ChargingPressure() float64
	// DischargingPressure returns centrifugal expenditure pressure (spending, consuming, distributing).
	DischargingPressure() float64
}

// NullPoint returns the pressure differential — the "gravity" anti-field.
// This is not an attractive force but the resolution toward lowest-pressure equilibrium.
func NullPoint(f ConjugateField) float64 {
	cp := f.ChargingPressure()
	dp := f.DischargingPressure()
	if cp > dp {
		return cp - dp
	}
	return dp - cp
}

// HealthRatio returns 0.0–1.0 indicating how balanced the conjugate pair is.
// Perfect health at ratio = 1.0 (balanced). Uses the golden ratio as the natural tolerance band.
func HealthRatio(f ConjugateField) float64 {
	dp := f.DischargingPressure()
	if dp < Agnosis {
		dp = Agnosis
	}
	ratio := f.ChargingPressure() / dp

	// Healthy when ratio falls in golden band: Φ⁻¹ to Φ
	if ratio >= Matter && ratio <= Being {
		return 1.0
	}

	deviation := ratio - 1.0
	if deviation < 0 {
		deviation = -deviation
	}
	health := 1.0 - (deviation / Totality)
	if health < 0 {
		return 0
	}
	return health
}

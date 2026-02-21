// Package phi provides all simulation constants derived from the golden ratio.
// No arbitrary magic numbers — everything traces back to Φ.
// See design doc Section 16.15.
package phi

import "math"

// Phi is the golden ratio.
const Phi = 1.6180339887498948

// Core emanation constants derived from powers of Phi.
var (
	// Agnosis (Φ⁻³): entropy, error, privation, noise.
	// ~24% — the base rate of imperfection in all systems.
	Agnosis = math.Pow(Phi, -3) // 0.23606...

	// Psyche (Φ⁻²): soul base, coherence seed, transfer rate.
	// ~38% — the threshold of meaningful connection.
	Psyche = math.Pow(Phi, -2) // 0.38197...

	// Matter (Φ⁻¹): material ratio, decay, mortality.
	// ~62% — the fraction that persists through transformation.
	Matter = math.Pow(Phi, -1) // 0.61803...

	// Monad: unity, the One, baseline. Always 1.0.
	Monad = 1.0

	// Being (Φ¹): life, cooperation bonus, growth factor.
	// ~1.618 — the fundamental ratio of living systems.
	Being = Phi // 1.61803...

	// Nous (Φ²): intelligence multiplier, wisdom threshold.
	// ~2.618 — the amplification factor of focused cognition.
	Nous = math.Pow(Phi, 2) // 2.61803...

	// Totality (Φ³): completion, overmass threshold, max ratio.
	// ~4.236 — the ceiling beyond which systems collapse.
	Totality = math.Pow(Phi, 3) // 4.23606...
)

// Sacred angles from Wheeler's geometry.
const (
	// LifeAngle is the plane of inertia — the habitable zone.
	LifeAngle = 85.0

	// ManifestationAngle is the aoristos dyad — market cycle period base.
	ManifestationAngle = 108.0

	// GrowthAngle is the divine growth angle — optimal expansion direction (phyllotaxis).
	GrowthAngle = 137.5077
)

// Structural limits from the Fibonacci trinity.
const (
	// Completion is the pentad — max healthy categories/tiers/accumulation.
	Completion = 5.0

	// Excess is beyond the pentad — threshold where corruption/evil begins.
	Excess = 6.0
)

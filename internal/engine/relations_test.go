package engine

import (
	"math"
	"testing"

	"github.com/talgya/mini-world/internal/phi"
)

// TestFactionRivalryMultiplier locks in the philosophical-opposition
// pairings from R54: Crown↔Ashen Path (order vs dissolution),
// Iron Brotherhood↔Verdant Circle (discipline vs harmony), and
// Merchant's Compact↔Ashen Path (wealth vs detachment) all multiply
// negative-sentiment intensity by Being (~1.618). All other pairs
// (including same-faction) return 1.0.
//
// This test exists because the rivalry mapping is a load-bearing
// piece of the inter-settlement diplomacy/warfare ecology — getting
// it wrong would silently dampen warfare and hostile sentiment for
// the philosophically-opposed pairs the design relies on.
func TestFactionRivalryMultiplier(t *testing.T) {
	// Faction IDs per social/faction.go SeedFactions:
	//   1=Crown, 2=Merchant, 3=Iron Brotherhood, 4=Verdant Circle, 5=Ashen Path
	being := phi.Being

	cases := []struct {
		a, b int
		want float64
		desc string
	}{
		// Philosophically-opposed pairs (order matters NOT — function normalizes).
		{1, 5, being, "Crown vs Ashen — order vs dissolution"},
		{5, 1, being, "Ashen vs Crown — symmetric"},
		{3, 4, being, "Iron Brotherhood vs Verdant Circle — discipline vs harmony"},
		{4, 3, being, "Verdant vs Iron — symmetric"},
		{2, 5, being, "Merchant's Compact vs Ashen — wealth vs detachment"},
		{5, 2, being, "Ashen vs Merchant — symmetric"},

		// Non-rival different-faction pairs return 1.0 (handled by the
		// caller's Matter coefficient, not amplified here).
		{1, 2, 1.0, "Crown vs Merchant — competing but not philosophically opposed"},
		{1, 3, 1.0, "Crown vs Iron Brotherhood — both order-leaning"},
		{1, 4, 1.0, "Crown vs Verdant — temporal vs spiritual, not opposed"},
		{2, 3, 1.0, "Merchant vs Iron — pragmatic neither side"},
		{2, 4, 1.0, "Merchant vs Verdant — material vs ecological"},
		{3, 5, 1.0, "Iron vs Ashen — both martial-tolerant"},
		{4, 5, 1.0, "Verdant vs Ashen — both progressive-leaning"},

		// Same-faction pair (degenerate but defined behavior).
		{1, 1, 1.0, "Crown vs Crown — same faction"},
		{4, 4, 1.0, "Verdant vs Verdant — same faction"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got := factionRivalryMultiplier(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("factionRivalryMultiplier(%d, %d) = %g, want %g  (%s)",
					tc.a, tc.b, got, tc.want, tc.desc)
			}
		})
	}
}

// TestFactionRivalryMultiplierSymmetric is a property test confirming
// the function is invariant to argument order. If symmetry is ever
// broken (e.g. by adding an asymmetric Crown-aggressor rule), the
// rivalry sentiment would be path-dependent in a way the design does
// not intend.
func TestFactionRivalryMultiplierSymmetric(t *testing.T) {
	for a := 1; a <= 5; a++ {
		for b := 1; b <= 5; b++ {
			ab := factionRivalryMultiplier(a, b)
			ba := factionRivalryMultiplier(b, a)
			if math.Abs(ab-ba) > 1e-9 {
				t.Errorf("asymmetry: factionRivalryMultiplier(%d,%d)=%g but (%d,%d)=%g",
					a, b, ab, b, a, ba)
			}
		}
	}
}

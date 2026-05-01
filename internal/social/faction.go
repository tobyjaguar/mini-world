// Factions — political, economic, and philosophical organizations.
// See design doc Section 6.1.
package social

// FactionID is a unique identifier for a faction.
type FactionID uint64

// Faction represents an organization with goals, influence, and membership.
//
// Removed in R81: the FactionKind enum (Political/Economic/Military/Religious/
// Criminal). It was set at seed and persisted but had zero call sites that
// branched on it for behavior — pure label. The actual mechanical philosophy
// of each faction lives in tax/trade/military preferences (see below), the
// per-faction patronage weight functions (R34), the doctrine predicates
// (R50), the rivalry pairings (R54), and the recruitment affinity rules
// (R63). See `docs/21-typology-depth-review.md` for the audit.
type Faction struct {
	ID   FactionID `json:"id"`
	Name string    `json:"name"`

	// Influence per settlement (settlement ID → 0–100).
	Influence map[uint64]float64 `json:"influence"`

	// Relations with other factions (faction ID → -100 to +100).
	Relations map[FactionID]float64 `json:"relations"`

	// Leadership
	LeaderID *uint64 `json:"leader_id,omitempty"`
	Treasury uint64  `json:"treasury"`

	// Policy tendencies
	TaxPreference    float64 `json:"tax_preference"`    // -1 low taxes, +1 high taxes
	TradePreference  float64 `json:"trade_preference"`  // -1 isolationist, +1 free trade
	MilitaryPreference float64 `json:"military_preference"` // -1 pacifist, +1 militarist
}

// SeedFactions creates the 5 initial factions for the world.
func SeedFactions() []*Faction {
	return []*Faction{
		{
			ID:   1,
			Name: "The Crown",
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.3,
			TradePreference:    0.0,
			MilitaryPreference: 0.5,
		},
		{
			ID:   2,
			Name: "Merchant's Compact",
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      -0.5,
			TradePreference:    0.8,
			MilitaryPreference: -0.3,
		},
		{
			ID:   3,
			Name: "Iron Brotherhood",
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.2,
			TradePreference:    -0.2,
			MilitaryPreference: 0.9,
		},
		{
			ID:   4,
			Name: "Verdant Circle",
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.0,
			TradePreference:    -0.3,
			MilitaryPreference: -0.5,
		},
		{
			ID:   5,
			Name: "Ashen Path",
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      -0.8,
			TradePreference:    0.4,
			MilitaryPreference: 0.2,
		},
	}
}

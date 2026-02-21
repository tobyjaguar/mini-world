// Factions — political, economic, and philosophical organizations.
// See design doc Section 6.1.
package social

// FactionID is a unique identifier for a faction.
type FactionID uint64

// Faction represents an organization with goals, influence, and membership.
type Faction struct {
	ID   FactionID `json:"id"`
	Name string    `json:"name"`
	Kind FactionKind `json:"kind"`

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

// FactionKind categorizes the nature of a faction.
type FactionKind uint8

const (
	FactionPolitical  FactionKind = iota // Governance-focused
	FactionEconomic                      // Trade and wealth
	FactionMilitary                      // Martial power
	FactionReligious                     // Spiritual and cultural
	FactionCriminal                      // Underground
)

// SeedFactions creates the 5 initial factions for the world.
func SeedFactions() []*Faction {
	return []*Faction{
		{
			ID:   1,
			Name: "The Crown",
			Kind: FactionPolitical,
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.3,
			TradePreference:    0.0,
			MilitaryPreference: 0.5,
		},
		{
			ID:   2,
			Name: "Merchant's Compact",
			Kind: FactionEconomic,
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      -0.5,
			TradePreference:    0.8,
			MilitaryPreference: -0.3,
		},
		{
			ID:   3,
			Name: "Iron Brotherhood",
			Kind: FactionMilitary,
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.2,
			TradePreference:    -0.2,
			MilitaryPreference: 0.9,
		},
		{
			ID:   4,
			Name: "Verdant Circle",
			Kind: FactionReligious,
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      0.0,
			TradePreference:    -0.3,
			MilitaryPreference: -0.5,
		},
		{
			ID:   5,
			Name: "Ashen Path",
			Kind: FactionCriminal,
			Influence: make(map[uint64]float64),
			Relations: make(map[FactionID]float64),
			TaxPreference:      -0.8,
			TradePreference:    0.4,
			MilitaryPreference: 0.2,
		},
	}
}

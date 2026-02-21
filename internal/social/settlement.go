// Package social provides factions, governance, settlements, and social systems.
// See design doc Sections 3.3 and 6.
package social

import (
	"github.com/talgya/mini-world/internal/economy"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// SettlementID is a unique identifier for a settlement.
type SettlementID = uint64

// GovernanceType represents how a settlement is governed.
type GovernanceType uint8

const (
	GovMonarchy        GovernanceType = iota // One leader, hereditary or seized
	GovCouncil                              // Elected representatives
	GovMerchantRepublic                     // Wealthiest citizens govern
	GovCommune                              // Direct democracy
)

// Settlement represents a population center on the hex grid.
type Settlement struct {
	ID       SettlementID   `json:"id"`
	Name     string         `json:"name"`
	Position world.HexCoord `json:"position"`

	// Demographics
	Population uint32 `json:"population"`

	// Governance
	Governance GovernanceType `json:"governance"`
	LeaderID   *uint64        `json:"leader_id,omitempty"`
	TaxRate    float64        `json:"tax_rate"`
	Treasury   uint64         `json:"treasury"` // Crowns

	// Culture (axes from -1.0 to 1.0)
	CultureTradition  float32 `json:"culture_tradition"`  // -1 progressive, +1 traditional
	CultureOpenness   float32 `json:"culture_openness"`   // -1 isolationist, +1 cosmopolitan
	CultureMilitarism float32 `json:"culture_militarism"` // -1 mercantile, +1 martial

	// Infrastructure
	WallLevel      uint8   `json:"wall_level"`      // 0–5
	RoadLevel      uint8   `json:"road_level"`       // 0–5
	MarketLevel    uint8   `json:"market_level"`     // 0–5
	GovernanceScore float64 `json:"governance_score"` // 0.0–1.0, effectiveness

	// Economy
	Market *economy.Market `json:"-"` // Settlement market (rebuilt on restart)

	// Wheeler integration
	CulturalMemory float64 `json:"cultural_memory"` // Accumulated from wise agents
}

// ChargingPressure implements phi.ConjugateField — production and tax revenue.
func (s *Settlement) ChargingPressure() float64 {
	return float64(s.Population)*0.1 + float64(s.Treasury)*0.001
}

// DischargingPressure implements phi.ConjugateField — consumption and trade outflow.
func (s *Settlement) DischargingPressure() float64 {
	return float64(s.Population) * 0.08
}

// Health returns settlement economic health via conjugate field model.
func (s *Settlement) Health() float64 {
	return phi.HealthRatio(s)
}

// IsOvermassed checks if the settlement has exceeded its governance capacity.
// See design doc Section 16.5.3 (M> barrier).
func (s *Settlement) IsOvermassed() bool {
	capacity := s.GovernanceScore * phi.Totality
	load := float64(s.Population) + float64(s.Treasury)*phi.Agnosis
	return load > capacity*phi.Being
}

// CorruptionScore returns the settlement's corruption level.
// Corruption = absence of good governance, not a positive force.
// See design doc Section 16.9.
func (s *Settlement) CorruptionScore(avgCoherence float64) float64 {
	govDeficit := 1.0 - s.GovernanceScore
	wisdomDeficit := 1.0 - avgCoherence

	score := govDeficit*phi.Psyche + wisdomDeficit*phi.Psyche
	if score > 1.0 {
		return 1.0
	}
	return score
}

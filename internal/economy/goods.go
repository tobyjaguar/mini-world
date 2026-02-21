// Package economy provides market mechanics, trade, and currency systems.
// See design doc Section 5.
package economy

import (
	"github.com/talgya/mini-world/internal/phi"
)

// MarketEntry represents the supply/demand state for one good in one settlement.
type MarketEntry struct {
	Good     string  `json:"good"`
	Supply   float64 `json:"supply"`   // Quantity available
	Demand   float64 `json:"demand"`   // Quantity desired
	Price    float64 `json:"price"`    // Current price in crowns
	BasePrice float64 `json:"base_price"` // Production cost floor
}

// Market holds the economic state for a single settlement.
type Market struct {
	SettlementID uint64                 `json:"settlement_id"`
	Entries      map[string]*MarketEntry `json:"entries"`
}

// ResolvePrice calculates price from supply/demand pressure mediation.
// Price emerges from the interference pattern between conjugate pressures,
// not from a set value. See design doc Section 16.5.1.
func (e *MarketEntry) ResolvePrice(seasonalMod, regionalMod float64) float64 {
	supply := e.Supply
	if supply < phi.Agnosis {
		supply = phi.Agnosis // prevent division by zero
	}

	price := e.BasePrice * (e.Demand / supply) * seasonalMod * regionalMod

	// Price bounded by floor (production cost) and reasonable ceiling.
	floor := e.BasePrice * phi.Agnosis
	ceiling := e.BasePrice * phi.Totality
	if price < floor {
		price = floor
	}
	if price > ceiling {
		price = ceiling
	}

	return price
}

// MarketField implements phi.ConjugateField for a market entry.
// Supply is centripetal (charging), demand is centrifugal (discharging).
type MarketField struct {
	Entry *MarketEntry
}

func (m MarketField) ChargingPressure() float64    { return m.Entry.Supply }
func (m MarketField) DischargingPressure() float64  { return m.Entry.Demand }

// Health returns the market health using the conjugate field model.
func (m MarketField) Health() float64 {
	return phi.HealthRatio(m)
}

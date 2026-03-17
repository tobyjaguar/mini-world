// Persistent trade routes — when merchants repeatedly trade between the same
// settlement pair, the route becomes established infrastructure. Established
// routes provide efficiency bonuses and generate events.
//
// Route levels (Φ-derived thresholds):
//   - Level 1 (Established): sustained trade ≥ 4/week for 2+ consecutive weeks
//   - Level 2 (Flourishing): sustained trade ≥ 8/week for 3+ consecutive weeks at level 1+
//   - Level 3 (Legendary): sustained trade ≥ 16/week for 4+ consecutive weeks at level 2+
//
// Bonuses per level:
//   - Travel cost discount: level × Agnosis × 0.1 (~2.4% per level)
//   - Margin bonus: level × Agnosis × 0.05 (~1.2% per level)
//
// Routes decay if trade drops below 2/week for 2+ consecutive weeks.
package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/phi"
)

// TradeRoute represents a persistent trade connection between two settlements.
type TradeRoute struct {
	Level            uint8   `json:"level"`             // 1=Established, 2=Flourishing, 3=Legendary
	Name             string  `json:"name"`              // Generated name (e.g. "Oakford–Millhaven Road")
	SustainedWeeks   int     `json:"sustained_weeks"`   // Consecutive weeks with sufficient trade
	DormantWeeks     int     `json:"dormant_weeks"`     // Consecutive weeks below threshold
	WeeklyTrade      float64 `json:"weekly_trade"`      // Last week's trade volume (for API)
}

const (
	routeEstablishThreshold = 4  // Min trades/week for route establishment
	routeFlourish           = 8  // Min trades/week for level 2
	routeLegendary          = 16 // Min trades/week for level 3
	routeDecayThreshold     = 2  // Below this, route starts decaying
)

// processTradeRoutes evaluates weekly trade volumes and establishes, upgrades,
// or degrades trade routes. Called weekly after computeSettlementRelations
// (which resets the trade tracker, so we must run BEFORE relations or use
// the tracker before reset).
//
// IMPORTANT: This must be called BEFORE computeSettlementRelations() so we
// can read from TradeTracker before it's reset.
func (s *Simulation) processTradeRoutes(tick uint64) {
	if s.TradeRoutes == nil {
		s.TradeRoutes = make(map[SettRelKey]*TradeRoute)
	}

	established := 0
	upgraded := 0
	degraded := 0

	// Check all pairs with trade this week.
	for key, vol := range s.TradeTracker {
		route, exists := s.TradeRoutes[key]

		if vol >= routeEstablishThreshold {
			if !exists {
				// New route tracking — not yet established.
				route = &TradeRoute{Level: 0, SustainedWeeks: 1}
				route.Name = s.generateRouteName(key)
				s.TradeRoutes[key] = route
			} else {
				route.SustainedWeeks++
				route.DormantWeeks = 0
			}
			route.WeeklyTrade = vol

			// Check for establishment or upgrade.
			newLevel := route.Level
			if route.Level == 0 && route.SustainedWeeks >= 2 && vol >= routeEstablishThreshold {
				newLevel = 1
			}
			if route.Level == 1 && route.SustainedWeeks >= 3 && vol >= routeFlourish {
				newLevel = 2
			}
			if route.Level == 2 && route.SustainedWeeks >= 4 && vol >= routeLegendary {
				newLevel = 3
			}

			if newLevel > route.Level {
				oldLevel := route.Level
				route.Level = newLevel
				if oldLevel == 0 {
					established++
					s.emitRouteEvent(tick, key, route, "established")
				} else {
					upgraded++
					s.emitRouteEvent(tick, key, route, "upgraded")
				}
			}
		} else if exists {
			// Trade below threshold — track dormancy.
			route.WeeklyTrade = vol
			if vol < routeDecayThreshold {
				route.DormantWeeks++
				if route.SustainedWeeks > 0 {
					route.SustainedWeeks--
				}
			} else {
				// Some trade but below growth threshold — hold steady.
				route.DormantWeeks = 0
			}
		}
	}

	// Check existing routes with ZERO trade this week (not in TradeTracker).
	for key, route := range s.TradeRoutes {
		if _, hasTradeThisWeek := s.TradeTracker[key]; !hasTradeThisWeek {
			route.DormantWeeks++
			if route.SustainedWeeks > 0 {
				route.SustainedWeeks--
			}
			route.WeeklyTrade = 0
		}

		// Degrade routes after 2+ dormant weeks.
		if route.DormantWeeks >= 2 && route.Level > 0 {
			route.Level--
			route.DormantWeeks = 0
			if route.Level == 0 {
				degraded++
				s.emitRouteEvent(tick, key, route, "dissolved")
				delete(s.TradeRoutes, key)
			} else {
				degraded++
				s.emitRouteEvent(tick, key, route, "degraded")
			}
		}

		// Clean up pre-established routes that never materialized.
		if route.Level == 0 && route.DormantWeeks >= 3 {
			delete(s.TradeRoutes, key)
		}
	}

	if established > 0 || upgraded > 0 || degraded > 0 {
		slog.Info("trade routes processed",
			"established", established,
			"upgraded", upgraded,
			"degraded", degraded,
			"total", len(s.TradeRoutes),
		)
	}
}

// RouteLevelName returns a human-readable level name.
func RouteLevelName(level uint8) string {
	switch level {
	case 1:
		return "Established"
	case 2:
		return "Flourishing"
	case 3:
		return "Legendary"
	default:
		return "Emerging"
	}
}

// generateRouteName creates a name for a trade route from settlement names.
func (s *Simulation) generateRouteName(key SettRelKey) string {
	nameA, nameB := "Unknown", "Unknown"
	if sett, ok := s.SettlementIndex[key.A]; ok {
		nameA = sett.Name
	}
	if sett, ok := s.SettlementIndex[key.B]; ok {
		nameB = sett.Name
	}
	return fmt.Sprintf("%s–%s Road", nameA, nameB)
}

// emitRouteEvent creates a world event for route changes.
func (s *Simulation) emitRouteEvent(tick uint64, key SettRelKey, route *TradeRoute, action string) {
	nameA, nameB := "Unknown", "Unknown"
	var settIDA, settIDB uint64
	if sett, ok := s.SettlementIndex[key.A]; ok {
		nameA = sett.Name
		settIDA = sett.ID
	}
	if sett, ok := s.SettlementIndex[key.B]; ok {
		nameB = sett.Name
		settIDB = sett.ID
	}

	desc := ""
	switch action {
	case "established":
		desc = fmt.Sprintf("A trade route has been established between %s and %s: the %s", nameA, nameB, route.Name)
	case "upgraded":
		desc = fmt.Sprintf("The %s has grown to %s status", route.Name, RouteLevelName(route.Level))
	case "degraded":
		desc = fmt.Sprintf("The %s has weakened to %s status", route.Name, RouteLevelName(route.Level))
	case "dissolved":
		desc = fmt.Sprintf("The trade route between %s and %s has dissolved from disuse", nameA, nameB)
	}

	s.EmitEvent(Event{
		Tick:        tick,
		Description: desc,
		Category:    "economy",
		Meta: map[string]any{
			"event_type":      "trade_route",
			"action":          action,
			"route_name":      route.Name,
			"route_level":     route.Level,
			"settlement_id":   settIDA,
			"settlement_name": nameA,
			"settlement_b_id": settIDB,
			"settlement_b":    nameB,
		},
	})
}

// GetRouteBonus returns the travel cost discount factor and margin bonus
// for a trade between two settlements if a route exists.
// Returns (1.0, 0.0) if no route.
func (s *Simulation) GetRouteBonus(settA, settB uint64) (travelDiscount float64, marginBonus float64) {
	if s.TradeRoutes == nil {
		return 1.0, 0.0
	}
	key := settRelKey(settA, settB)
	route, ok := s.TradeRoutes[key]
	if !ok || route.Level == 0 {
		return 1.0, 0.0
	}

	// Travel discount: level × Agnosis × 0.1 (~2.4% per level).
	travelDiscount = 1.0 - float64(route.Level)*phi.Agnosis*0.1

	// Margin bonus: level × Agnosis × 0.05 (~1.2% per level).
	marginBonus = float64(route.Level) * phi.Agnosis * 0.05

	return travelDiscount, marginBonus
}

// GetSettlementRoutes returns all trade routes involving a settlement.
func (s *Simulation) GetSettlementRoutes(settID uint64) []TradeRouteInfo {
	var routes []TradeRouteInfo
	for key, route := range s.TradeRoutes {
		if route.Level == 0 {
			continue // Only expose established routes.
		}
		if key.A == settID || key.B == settID {
			otherID := key.B
			if key.B == settID {
				otherID = key.A
			}
			otherName := "Unknown"
			if sett, ok := s.SettlementIndex[otherID]; ok {
				otherName = sett.Name
			}
			routes = append(routes, TradeRouteInfo{
				Name:           route.Name,
				Level:          route.Level,
				LevelName:      RouteLevelName(route.Level),
				SettlementID:   otherID,
				SettlementName: otherName,
				WeeklyTrade:    route.WeeklyTrade,
			})
		}
	}
	return routes
}

// TradeRouteInfo is the API-facing representation of a trade route.
type TradeRouteInfo struct {
	Name           string  `json:"name"`
	Level          uint8   `json:"level"`
	LevelName      string  `json:"level_name"`
	SettlementID   uint64  `json:"settlement_id"`
	SettlementName string  `json:"settlement_name"`
	WeeklyTrade    float64 `json:"weekly_trade"`
}

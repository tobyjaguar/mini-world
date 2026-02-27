package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// ProductionBoost is a temporary production multiplier on a settlement.
type ProductionBoost struct {
	SettlementID uint64
	Multiplier   float64
	ExpiresAt    uint64 // tick
}

// ProvisionSettlement injects goods into a settlement's market supply.
func (s *Simulation) ProvisionSettlement(name, goodName string, quantity int) (string, error) {
	sett := s.findSettlementByName(name)
	if sett == nil {
		return "", fmt.Errorf("settlement %q not found", name)
	}
	if sett.Market == nil {
		return "", fmt.Errorf("settlement %q has no market", name)
	}

	goodType, ok := GoodTypeFromString(goodName)
	if !ok {
		return "", fmt.Errorf("unknown good %q", goodName)
	}

	entry, ok := sett.Market.Entries[goodType]
	if !ok {
		return "", fmt.Errorf("good %q not in market for %q", goodName, name)
	}

	entry.Supply += float64(quantity)
	desc := fmt.Sprintf("A merchant caravan arrives in %s bearing %d units of %s", name, quantity, goodName)

	s.EmitEvent(Event{
		Tick:        s.LastTick,
		Description: desc,
		Category:    "gardener",
		Meta: map[string]any{
			"settlement_name": name,
			"good":            goodName,
			"quantity":         quantity,
		},
	})

	slog.Info("provision intervention", "settlement", name, "good", goodName, "quantity", quantity)
	return desc, nil
}

// CultivateSettlement adds a temporary production boost to a settlement.
func (s *Simulation) CultivateSettlement(name string, multiplier float64, durationDays int) (string, error) {
	sett := s.findSettlementByName(name)
	if sett == nil {
		return "", fmt.Errorf("settlement %q not found", name)
	}

	expiresAt := s.LastTick + uint64(durationDays)*TicksPerSimDay
	s.ActiveBoosts = append(s.ActiveBoosts, ProductionBoost{
		SettlementID: sett.ID,
		Multiplier:   multiplier,
		ExpiresAt:    expiresAt,
	})

	desc := fmt.Sprintf("A bountiful season blesses the workers of %s (%.1fx production for %d days)", name, multiplier, durationDays)
	s.EmitEvent(Event{
		Tick:        s.LastTick,
		Description: desc,
		Category:    "gardener",
		Meta: map[string]any{
			"settlement_name": name,
			"multiplier":      multiplier,
			"duration_days":   durationDays,
		},
	})

	slog.Info("cultivate intervention", "settlement", name, "multiplier", multiplier, "duration_days", durationDays, "expires_tick", expiresAt)
	return desc, nil
}

// ConsolidateSettlement force-migrates agents from a dying settlement to the nearest viable one.
func (s *Simulation) ConsolidateSettlement(name string, count int) (string, error) {
	source := s.findSettlementByName(name)
	if source == nil {
		return "", fmt.Errorf("settlement %q not found", name)
	}

	// Find nearest viable settlement (pop >= 50) within 8 hexes.
	var targetSett *social.Settlement
	bestDist := 999

	for _, sett := range s.Settlements {
		if sett.ID == source.ID || sett.Population < 50 {
			continue
		}
		dist := world.Distance(source.Position, sett.Position)
		if dist <= 8 && dist < bestDist {
			bestDist = dist
			targetSett = sett
		}
	}

	if targetSett == nil {
		return "", fmt.Errorf("no viable target settlement found near %q", name)
	}

	// Move agents.
	moved := 0
	settAgents := s.SettlementAgents[source.ID]
	for _, a := range settAgents {
		if moved >= count {
			break
		}
		if !a.Alive {
			continue
		}
		a.HomeSettID = &targetSett.ID
		a.Position = targetSett.Position
		s.reassignIfMismatched(a, targetSett.ID)
		moved++
	}

	if moved == 0 {
		return "", fmt.Errorf("no alive agents to move from %q", name)
	}

	// Rebuild settlement agents map.
	s.rebuildSettlementAgents()

	desc := fmt.Sprintf("Refugees from %s (%d souls) seek shelter in %s", name, moved, targetSett.Name)
	s.EmitEvent(Event{
		Tick:        s.LastTick,
		Description: desc,
		Category:    "gardener",
		Meta: map[string]any{
			"source_settlement_name": name,
			"target_settlement_name": targetSett.Name,
			"count":                  moved,
		},
	})

	slog.Info("consolidate intervention", "source", name, "target", targetSett.Name, "moved", moved)
	return desc, nil
}

// CleanExpiredBoosts removes production boosts that have expired.
func (s *Simulation) CleanExpiredBoosts(tick uint64) {
	n := 0
	for _, b := range s.ActiveBoosts {
		if b.ExpiresAt > tick {
			s.ActiveBoosts[n] = b
			n++
		}
	}
	if n < len(s.ActiveBoosts) {
		slog.Info("expired production boosts cleaned", "removed", len(s.ActiveBoosts)-n)
	}
	s.ActiveBoosts = s.ActiveBoosts[:n]
}

// GetSettlementBoost returns the production multiplier for a settlement from active boosts.
// Returns 1.0 if no boost is active.
func (s *Simulation) GetSettlementBoost(settlementID uint64) float64 {
	best := 1.0
	for _, b := range s.ActiveBoosts {
		if b.SettlementID == settlementID && b.Multiplier > best {
			best = b.Multiplier
		}
	}
	return best
}

// findSettlementByName looks up a settlement by name (case-sensitive).
func (s *Simulation) findSettlementByName(name string) *social.Settlement {
	for _, sett := range s.Settlements {
		if sett.Name == name {
			return sett
		}
	}
	return nil
}

// GoodTypeFromString maps a good name string to agents.GoodType.
func GoodTypeFromString(name string) (agents.GoodType, bool) {
	m := map[string]agents.GoodType{
		"grain":    agents.GoodGrain,
		"fish":     agents.GoodFish,
		"timber":   agents.GoodTimber,
		"iron_ore": agents.GoodIronOre,
		"stone":    agents.GoodStone,
		"coal":     agents.GoodCoal,
		"herbs":    agents.GoodHerbs,
		"furs":     agents.GoodFurs,
		"gems":     agents.GoodGems,
		"exotics":  agents.GoodExotics,
		"tools":    agents.GoodTools,
		"weapons":  agents.GoodWeapons,
		"clothing": agents.GoodClothing,
		"medicine": agents.GoodMedicine,
		"luxuries": agents.GoodLuxuries,
	}
	g, ok := m[name]
	return g, ok
}

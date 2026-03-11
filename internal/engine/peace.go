// Peace treaties and trade embargoes — conflict resolution and economic warfare.
//
// Peace Treaty:
//   - Forms automatically after 3+ raids between a pair (war exhaustion)
//   - Duration: 8 sim-weeks (Φ ≈ 8 being ~5×Being rounded)
//   - During peace: no raids allowed, sentiment recovers +Agnosis×0.1 per week
//   - Emits events on formation and expiration
//
// Trade Embargo:
//   - Active between hostile pairs (sentiment < -Psyche)
//   - Merchants skip embargoed destinations in route selection
//   - Lifts when sentiment recovers above -Agnosis
//   - No persistence needed — derived from sentiment state
//
// All constants Φ-derived.
package engine

import (
	"fmt"
	"log/slog"

	"github.com/talgya/mini-world/internal/phi"
)

// PeaceTreaty tracks a ceasefire between two settlements.
type PeaceTreaty struct {
	RemainingWeeks int    `json:"remaining_weeks"`
	RaidCount      int    `json:"raid_count"` // Total raids that led to this peace
	FormedAtTick   uint64 `json:"formed_at_tick"`
}

const (
	peaceDurationWeeks    = 8 // ~5 × Being, rounded
	peaceRaidThreshold    = 3 // Raids needed before peace triggers
	embargoSentiment      = -0.382 // phi.Psyche negated
)

// RecordRaid increments the raid counter for a settlement pair.
// Called after each raid resolution.
func (s *Simulation) RecordRaid(settA, settB uint64) {
	if s.RaidCounts == nil {
		s.RaidCounts = make(map[SettRelKey]int)
	}
	key := settRelKey(settA, settB)
	s.RaidCounts[key]++
}

// processPeace evaluates war-weary pairs for peace treaties and ticks existing treaties.
// Called weekly BEFORE processWarfare so peace prevents raids.
func (s *Simulation) processPeace(tick uint64) {
	if s.PeaceTreaties == nil {
		s.PeaceTreaties = make(map[SettRelKey]*PeaceTreaty)
	}

	formed := 0
	expired := 0

	// Check for new peace treaties from raid exhaustion.
	for key, count := range s.RaidCounts {
		if count >= peaceRaidThreshold {
			if _, hasPeace := s.PeaceTreaties[key]; !hasPeace {
				s.PeaceTreaties[key] = &PeaceTreaty{
					RemainingWeeks: peaceDurationWeeks,
					RaidCount:      count,
					FormedAtTick:   tick,
				}
				s.RaidCounts[key] = 0 // Reset counter
				formed++
				s.emitPeaceEvent(tick, key, "formed")
			}
		}
	}

	// Tick existing treaties and apply sentiment recovery.
	for key, treaty := range s.PeaceTreaties {
		treaty.RemainingWeeks--

		// Sentiment recovery during peace: +Agnosis × 0.1 per week.
		if rel, ok := s.Relations[key]; ok {
			rel.Sentiment += phi.Agnosis * 0.1
			if rel.Sentiment > 0 {
				rel.Sentiment = 0 // Peace brings neutrality, not friendship
			}
		}

		if treaty.RemainingWeeks <= 0 {
			expired++
			s.emitPeaceEvent(tick, key, "expired")
			delete(s.PeaceTreaties, key)
		}
	}

	if formed > 0 || expired > 0 {
		slog.Info("peace processed",
			"formed", formed,
			"expired", expired,
			"active", len(s.PeaceTreaties),
		)
	}
}

// HasPeace returns true if two settlements have an active peace treaty.
func (s *Simulation) HasPeace(settA, settB uint64) bool {
	if s.PeaceTreaties == nil {
		return false
	}
	key := settRelKey(settA, settB)
	_, ok := s.PeaceTreaties[key]
	return ok
}

// IsEmbargoed returns true if trade is blocked between two settlements
// due to hostile sentiment.
func (s *Simulation) IsEmbargoed(settA, settB uint64) bool {
	key := settRelKey(settA, settB)
	rel, ok := s.Relations[key]
	if !ok {
		return false
	}
	return rel.Sentiment < embargoSentiment
}

// emitPeaceEvent creates a world event for peace treaty changes.
func (s *Simulation) emitPeaceEvent(tick uint64, key SettRelKey, action string) {
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
	case "formed":
		desc = fmt.Sprintf("After prolonged conflict, %s and %s have agreed to a peace treaty", nameA, nameB)
	case "expired":
		desc = fmt.Sprintf("The peace treaty between %s and %s has expired", nameA, nameB)
	}

	s.EmitEvent(Event{
		Tick:        tick,
		Description: desc,
		Category:    "political",
		Meta: map[string]any{
			"event_type":        "peace_treaty",
			"action":            action,
			"settlement_id":     settIDA,
			"settlement_name":   nameA,
			"settlement_b_id":   settIDB,
			"settlement_b_name": nameB,
		},
	})
}

// GetSettlementPeace returns active peace treaties for a settlement.
func (s *Simulation) GetSettlementPeace(settID uint64) []PeaceTreatyInfo {
	var treaties []PeaceTreatyInfo
	for key, treaty := range s.PeaceTreaties {
		if key.A == settID || key.B == settID {
			otherID := key.B
			if key.B == settID {
				otherID = key.A
			}
			otherName := "Unknown"
			if sett, ok := s.SettlementIndex[otherID]; ok {
				otherName = sett.Name
			}
			treaties = append(treaties, PeaceTreatyInfo{
				SettlementID:   otherID,
				SettlementName: otherName,
				RemainingWeeks: treaty.RemainingWeeks,
			})
		}
	}
	return treaties
}

// PeaceTreatyInfo is the API-facing representation.
type PeaceTreatyInfo struct {
	SettlementID   uint64 `json:"settlement_id"`
	SettlementName string `json:"settlement_name"`
	RemainingWeeks int    `json:"remaining_weeks"`
}

// Inter-settlement diplomacy — formal agreements that emerge from sustained
// positive sentiment between settlement pairs. Agreements provide mechanical
// bonuses and dissolve when sentiment drops.
//
// Agreement types (escalating commitment, Φ-derived thresholds):
//   - Trade Pact:       sentiment > Psyche (0.382) for 2+ weeks
//   - Non-Aggression:   sentiment > Matter (0.618) for 3+ weeks
//   - Alliance:         sentiment > Psyche+Matter (0.764 ≈ Nous-ish) for 4+ weeks
//
// Dissolution thresholds (lower than formation to create hysteresis):
//   - Trade Pact:       dissolves when sentiment < Agnosis (0.236)
//   - Non-Aggression:   dissolves when sentiment < Psyche (0.382)
//   - Alliance:         dissolves when sentiment < Matter (0.618)
//
// Bonuses:
//   - Trade Pact:       +50% trade weight in sentiment computation
//   - Non-Aggression:   culture drift toward each other, +Agnosis crime deterrence
//   - Alliance:         all above + faction influence boost between settlements
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/phi"
)

// AgreementType represents the type of diplomatic agreement.
type AgreementType uint8

const (
	AgreementTradePact      AgreementType = 1
	AgreementNonAggression  AgreementType = 2
	AgreementAlliance       AgreementType = 3
)

// Agreement represents a formal diplomatic agreement between two settlements.
type Agreement struct {
	Type           AgreementType `json:"type"`
	SustainedWeeks int           `json:"sustained_weeks"` // Weeks sentiment has been above formation threshold
	FormedAtTick   uint64        `json:"formed_at_tick"`
}

// AgreementTypeName returns the human-readable name for an agreement type.
func AgreementTypeName(t AgreementType) string {
	switch t {
	case AgreementTradePact:
		return "Trade Pact"
	case AgreementNonAggression:
		return "Non-Aggression Pact"
	case AgreementAlliance:
		return "Alliance"
	default:
		return "Unknown"
	}
}

// Formation thresholds (sentiment must be above these).
var agreementFormThreshold = [4]float64{
	0:                        0,
	AgreementTradePact:       phi.Psyche,                  // 0.382
	AgreementNonAggression:   phi.Matter,                  // 0.618
	AgreementAlliance:        phi.Psyche + phi.Matter,     // ~1.0 (very high)
}

// Formation weeks required.
var agreementFormWeeks = [4]int{
	0:                        0,
	AgreementTradePact:       2,
	AgreementNonAggression:   3,
	AgreementAlliance:        4,
}

// Dissolution thresholds (agreement dissolves below these).
var agreementDissolveThreshold = [4]float64{
	0:                        0,
	AgreementTradePact:       phi.Agnosis,  // 0.236
	AgreementNonAggression:   phi.Psyche,   // 0.382
	AgreementAlliance:        phi.Matter,   // 0.618
}

// processDiplomacy evaluates all settlement relations and forms, upgrades, or
// dissolves diplomatic agreements. Called weekly AFTER computeSettlementRelations()
// so sentiment is fresh.
func (s *Simulation) processDiplomacy(tick uint64) {
	if s.Agreements == nil {
		s.Agreements = make(map[SettRelKey]*Agreement)
	}

	formed := 0
	upgraded := 0
	dissolved := 0

	for key, rel := range s.Relations {
		agreement, exists := s.Agreements[key]
		sentiment := rel.Sentiment

		if sentiment >= phi.Psyche { // Above minimum formation threshold
			if !exists {
				// Start tracking for potential agreement.
				s.Agreements[key] = &Agreement{
					Type:           0, // Not yet formed
					SustainedWeeks: 1,
				}
			} else {
				agreement.SustainedWeeks++

				// Check for formation or upgrade (highest type first).
				for t := AgreementAlliance; t >= AgreementTradePact; t-- {
					if sentiment >= agreementFormThreshold[t] && agreement.SustainedWeeks >= agreementFormWeeks[t] {
						if agreement.Type < t {
							oldType := agreement.Type
							agreement.Type = t
							agreement.FormedAtTick = tick
							if oldType == 0 {
								formed++
								s.emitDiplomacyEvent(tick, key, t, "formed")
							} else {
								upgraded++
								s.emitDiplomacyEvent(tick, key, t, "upgraded")
							}
						}
						break
					}
				}
			}
		} else if exists {
			// Sentiment dropped — check dissolution.
			if agreement.Type > 0 && sentiment < agreementDissolveThreshold[agreement.Type] {
				oldType := agreement.Type
				// Downgrade one level instead of immediate dissolution.
				agreement.Type--
				agreement.SustainedWeeks = 0
				if agreement.Type == 0 {
					dissolved++
					s.emitDiplomacyEvent(tick, key, oldType, "dissolved")
					delete(s.Agreements, key)
				} else {
					dissolved++
					s.emitDiplomacyEvent(tick, key, agreement.Type, "downgraded")
				}
			} else if agreement.Type == 0 {
				// Pre-formation tracking that lost momentum — gradual decay
				// instead of hard reset, so one bad week doesn't erase progress.
				if agreement.SustainedWeeks > 0 {
					agreement.SustainedWeeks--
				}
				if sentiment < phi.Agnosis {
					delete(s.Agreements, key)
				}
			}
		}
	}

	// Clean up agreements for relations that no longer exist.
	for key := range s.Agreements {
		if _, ok := s.Relations[key]; !ok {
			if s.Agreements[key].Type > 0 {
				dissolved++
				s.emitDiplomacyEvent(tick, key, s.Agreements[key].Type, "dissolved")
			}
			delete(s.Agreements, key)
		}
	}

	if formed > 0 || upgraded > 0 || dissolved > 0 {
		active := 0
		for _, a := range s.Agreements {
			if a.Type > 0 {
				active++
			}
		}
		slog.Info("diplomacy processed",
			"formed", formed,
			"upgraded", upgraded,
			"dissolved", dissolved,
			"active", active,
		)
	}
}

// emitDiplomacyEvent creates a world event for diplomatic changes.
func (s *Simulation) emitDiplomacyEvent(tick uint64, key SettRelKey, aType AgreementType, action string) {
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

	typeName := AgreementTypeName(aType)
	desc := ""
	switch action {
	case "formed":
		desc = fmt.Sprintf("%s and %s have signed a %s", nameA, nameB, typeName)
	case "upgraded":
		desc = fmt.Sprintf("The agreement between %s and %s has been elevated to a %s", nameA, nameB, typeName)
	case "downgraded":
		desc = fmt.Sprintf("Relations between %s and %s have cooled; their agreement has been reduced to a %s", nameA, nameB, typeName)
	case "dissolved":
		desc = fmt.Sprintf("The %s between %s and %s has been dissolved", typeName, nameA, nameB)
	}

	s.EmitEvent(Event{
		Tick:        tick,
		Description: desc,
		Category:    "political",
		Meta: map[string]any{
			"event_type":        "diplomacy",
			"action":            action,
			"agreement_type":    int(aType),
			"agreement_name":    typeName,
			"settlement_id":     settIDA,
			"settlement_name":   nameA,
			"settlement_b_id":   settIDB,
			"settlement_b_name": nameB,
		},
	})
}

// GetAgreement returns the agreement between two settlements, if any.
func (s *Simulation) GetAgreement(settA, settB uint64) *Agreement {
	if s.Agreements == nil {
		return nil
	}
	key := settRelKey(settA, settB)
	a, ok := s.Agreements[key]
	if !ok || a.Type == 0 {
		return nil
	}
	return a
}

// GetSettlementAgreements returns all active agreements for a settlement.
func (s *Simulation) GetSettlementAgreements(settID uint64) []AgreementInfo {
	var agreements []AgreementInfo
	for key, a := range s.Agreements {
		if a.Type == 0 {
			continue
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
			agreements = append(agreements, AgreementInfo{
				Type:           a.Type,
				TypeName:       AgreementTypeName(a.Type),
				SettlementID:   otherID,
				SettlementName: otherName,
				FormedAtTick:   a.FormedAtTick,
			})
		}
	}
	return agreements
}

// AgreementInfo is the API-facing representation of an agreement.
type AgreementInfo struct {
	Type           AgreementType `json:"type"`
	TypeName       string        `json:"type_name"`
	SettlementID   uint64        `json:"settlement_id"`
	SettlementName string        `json:"settlement_name"`
	FormedAtTick   uint64        `json:"formed_at_tick"`
}

// --- Mechanical Effects ---

// ApplyDiplomacyEffects applies agreement bonuses during the weekly cycle.
// Called AFTER processDiplomacy, BEFORE other systems that benefit from bonuses.
func (s *Simulation) ApplyDiplomacyEffects() {
	for key, a := range s.Agreements {
		if a.Type == 0 {
			continue
		}
		settA, okA := s.SettlementIndex[key.A]
		settB, okB := s.SettlementIndex[key.B]
		if !okA || !okB {
			continue
		}

		// Non-Aggression Pact+: culture drift toward each other.
		if a.Type >= AgreementNonAggression {
			driftRate := float32(phi.Agnosis * 0.1) // ~0.024/week toward each other
			// Tradition
			if settA.CultureTradition < settB.CultureTradition {
				settA.CultureTradition += driftRate
				settB.CultureTradition -= driftRate
			} else if settA.CultureTradition > settB.CultureTradition {
				settA.CultureTradition -= driftRate
				settB.CultureTradition += driftRate
			}
			// Openness
			if settA.CultureOpenness < settB.CultureOpenness {
				settA.CultureOpenness += driftRate
				settB.CultureOpenness -= driftRate
			} else if settA.CultureOpenness > settB.CultureOpenness {
				settA.CultureOpenness -= driftRate
				settB.CultureOpenness += driftRate
			}
			// Militarism
			if settA.CultureMilitarism < settB.CultureMilitarism {
				settA.CultureMilitarism += driftRate
				settB.CultureMilitarism -= driftRate
			} else if settA.CultureMilitarism > settB.CultureMilitarism {
				settA.CultureMilitarism -= driftRate
				settB.CultureMilitarism += driftRate
			}
		}
	}
}

// GetDiplomacyCrimeBonus returns an additive crime deterrence bonus for a
// settlement based on its Non-Aggression+ agreements.
func (s *Simulation) GetDiplomacyCrimeBonus(settID uint64) float64 {
	if s.Agreements == nil {
		return 0
	}
	bonus := 0.0
	for key, a := range s.Agreements {
		if a.Type < AgreementNonAggression {
			continue
		}
		if key.A == settID || key.B == settID {
			bonus += phi.Agnosis * 0.5 // ~0.118 per non-aggression+ partner
		}
	}
	return bonus
}

// GetDiplomacyTradeBonus returns a multiplicative trade sentiment bonus for
// a settlement pair with a Trade Pact+.
func (s *Simulation) GetDiplomacyTradeBonus(settA, settB uint64) float64 {
	a := s.GetAgreement(settA, settB)
	if a == nil {
		return 1.0
	}
	// Trade Pact: +50% trade sentiment weight. Higher agreements inherit.
	return 1.0 + phi.Psyche*float64(a.Type)*0.5 // ~1.19 for pact, ~1.38 for NAP, ~1.57 for alliance
}

// GetDiplomacyFactionBonus returns an additive faction influence bonus for
// allied settlement pairs.
func (s *Simulation) GetDiplomacyFactionBonus(settA, settB uint64) float64 {
	a := s.GetAgreement(settA, settB)
	if a == nil || a.Type < AgreementAlliance {
		return 0
	}
	return phi.Agnosis * 5 // ~1.18 influence bonus between allied settlements
}

// HasMutualDefense returns true if two settlements have an Alliance.
func (s *Simulation) HasMutualDefense(settA, settB uint64) bool {
	a := s.GetAgreement(settA, settB)
	return a != nil && a.Type >= AgreementAlliance
}

// DiplomacySummary returns world-level diplomacy statistics.
func (s *Simulation) DiplomacySummary() map[string]any {
	tradePacts := 0
	nonAggression := 0
	alliances := 0
	for _, a := range s.Agreements {
		switch a.Type {
		case AgreementTradePact:
			tradePacts++
		case AgreementNonAggression:
			nonAggression++
		case AgreementAlliance:
			alliances++
		}
	}

	// Find the strongest agreement pair.
	var strongestPair *AgreementInfo
	strongestType := AgreementType(0)
	strongestSentiment := 0.0
	for key, a := range s.Agreements {
		if a.Type > strongestType || (a.Type == strongestType && s.Relations[key] != nil && s.Relations[key].Sentiment > strongestSentiment) {
			nameA, nameB := "Unknown", "Unknown"
			if sett, ok := s.SettlementIndex[key.A]; ok {
				nameA = sett.Name
			}
			if sett, ok := s.SettlementIndex[key.B]; ok {
				nameB = sett.Name
			}
			sentiment := 0.0
			if rel, ok := s.Relations[key]; ok {
				sentiment = rel.Sentiment
			}
			strongestType = a.Type
			strongestSentiment = sentiment
			strongestPair = &AgreementInfo{
				Type:           a.Type,
				TypeName:       AgreementTypeName(a.Type),
				SettlementID:   key.A,
				SettlementName: fmt.Sprintf("%s & %s", nameA, nameB),
			}
		}
	}

	result := map[string]any{
		"trade_pacts":     tradePacts,
		"non_aggression":  nonAggression,
		"alliances":       alliances,
		"total":           tradePacts + nonAggression + alliances,
	}
	if strongestPair != nil {
		result["strongest"] = map[string]any{
			"type_name":   strongestPair.TypeName,
			"settlements": strongestPair.SettlementName,
			"sentiment":   math.Round(strongestSentiment*1000) / 1000,
		}
	}
	return result
}

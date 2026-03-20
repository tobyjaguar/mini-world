// Inter-settlement relations — sentiment scores between settlement pairs.
// Computed weekly from shared faction dominance, trade volume, culture similarity, and distance.
// See design doc Section 6 (social systems).
package engine

import (
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/world"
)

// SettlementRelation tracks the relationship between two settlements.
type SettlementRelation struct {
	Sentiment float64 `json:"sentiment"` // -1.0 (hostile) to +1.0 (allied)
	Trade     float64 `json:"trade"`     // Recent trade volume between the pair
}

// SettRelKey is a canonical key for a settlement pair (lower ID first).
type SettRelKey struct {
	A, B uint64
}

func settRelKey(a, b uint64) SettRelKey {
	if a > b {
		a, b = b, a
	}
	return SettRelKey{A: a, B: b}
}

// factionRivalryMultiplier returns an extra tension multiplier for philosophically
// opposed faction pairs. Returns Being (~1.618) for deep rivals, 1.0 for neutral pairs.
// Rivalries derive from faction philosophies, not arbitrary scripting:
//   - Crown (order) vs Ashen Path (dissolution): ontological opposites
//   - Iron Brotherhood (martial discipline) vs Verdant Circle (natural harmony): opposites
//   - Merchant's Compact (wealth accumulation) vs Ashen Path (detachment): opposites
func factionRivalryMultiplier(factionA, factionB int) float64 {
	// Faction IDs: 1=Crown, 2=Merchant, 3=Iron Brotherhood, 4=Verdant Circle, 5=Ashen Path
	a, b := factionA, factionB
	if a > b {
		a, b = b, a
	}
	switch {
	case a == 1 && b == 5: // Crown vs Ashen Path
		return phi.Being
	case a == 3 && b == 4: // Iron Brotherhood vs Verdant Circle
		return phi.Being
	case a == 2 && b == 5: // Merchant's Compact vs Ashen Path
		return phi.Being
	default:
		return 1.0
	}
}

// RecordInterSettlementTrade increments the trade counter between two settlements.
// Called from merchant cargo sale when a trade completes between settlements.
func (s *Simulation) RecordInterSettlementTrade(sourceID, destID uint64) {
	if sourceID == destID {
		return
	}
	if s.TradeTracker == nil {
		s.TradeTracker = make(map[SettRelKey]float64)
	}
	key := settRelKey(sourceID, destID)
	s.TradeTracker[key]++
}

// computeSettlementRelations recalculates all inter-settlement relations weekly.
// Sentiment is a blend of four factors:
//   - Shared faction dominance: same dominant faction → positive, rivals → negative
//   - Trade volume: more trade → more positive (capped at Psyche contribution)
//   - Culture similarity: similar axes → positive, divergent → negative
//   - Distance: nearby settlements have stronger relations (positive or negative)
//
// Relations decay toward 0 weekly (Agnosis decay rate) — they must be maintained.
func (s *Simulation) computeSettlementRelations() {
	if s.Relations == nil {
		s.Relations = make(map[SettRelKey]*SettlementRelation)
	}

	// Pre-compute dominant faction per settlement using faction INFLUENCE
	// (not raw member count). Influence accounts for IB soldier Being-weighting,
	// so IB is dominant in its strongholds even though VC has more raw members
	// globally (~40%). Using raw counts made VC dominant everywhere, creating
	// a universal positive floor that prevented any negative sentiment.
	type factionInfo struct {
		dominantFaction int // -1 if none
		factionStrength float64
	}
	factionData := make(map[uint64]factionInfo, len(s.Settlements))
	for _, sett := range s.Settlements {
		if sett.Population == 0 {
			continue
		}
		bestFaction := -1
		bestInfluence := 0.0
		totalInfluence := 0.0
		for _, f := range s.Factions {
			inf := f.Influence[sett.ID]
			totalInfluence += inf
			if inf > bestInfluence {
				bestInfluence = inf
				bestFaction = int(f.ID)
			}
		}
		strength := 0.0
		if totalInfluence > 0 {
			strength = bestInfluence / totalInfluence
		}
		factionData[sett.ID] = factionInfo{dominantFaction: bestFaction, factionStrength: strength}
	}

	// Compute relations for settlement pairs within interaction range (10 hexes).
	// Uses pre-computed neighbor index to avoid O(n²) all-pairs evaluation.
	updated := 0
	seen := make(map[SettRelKey]bool, len(s.Settlements)*20) // dedup: A→B and B→A

	for _, settA := range s.Settlements {
		if settA.Population == 0 {
			continue
		}
		for _, settB := range s.SettlementNeighbors[settA.ID] {
			if settB.Population == 0 {
				continue
			}

			key := settRelKey(settA.ID, settB.ID)
			if seen[key] {
				continue
			}
			seen[key] = true

			dist := world.Distance(settA.Position, settB.Position)

			// 1. Faction affinity: shared dominant faction → positive, different → rivalry.
			//    Cooperation uses Being (1.618), rivalry uses Matter (0.618).
			//    The cooperation:rivalry ratio is Being/Matter = Φ — emergence-preserving asymmetry.
			//    Philosophically opposed factions get an additional Being multiplier.
			factionScore := 0.0
			fA, fB := factionData[settA.ID], factionData[settB.ID]
			if fA.dominantFaction >= 0 && fB.dominantFaction >= 0 {
				if fA.dominantFaction == fB.dominantFaction {
					// Shared faction: strength of bond = product of dominance strengths.
					factionScore = fA.factionStrength * fB.factionStrength * phi.Being // Up to ~1.618
				} else {
					// Different factions: rivalry scaled by Matter.
					rivalryMul := factionRivalryMultiplier(fA.dominantFaction, fB.dominantFaction)
					factionScore = -fA.factionStrength * fB.factionStrength * phi.Matter * rivalryMul
				}
			}

			// 2. Trade affinity: recent trade volume → positive sentiment.
			//    Trade pacts amplify this via GetDiplomacyTradeBonus().
			tradeScore := 0.0
			if tradeVol, ok := s.TradeTracker[key]; ok && tradeVol > 0 {
				// Logarithmic: diminishing returns. 1 trade = 0, 10 = 0.54, 50 = 0.92.
				tradeScore = math.Log2(1+tradeVol) * phi.Agnosis * 0.1
				tradeScore *= s.GetDiplomacyTradeBonus(settA.ID, settB.ID)
				if tradeScore > phi.Psyche {
					tradeScore = phi.Psyche // Cap at ~0.382
				}
			}

			// 3. Culture similarity: similar axes → positive, divergent → negative.
			cultureScore := 0.0
			tradDiff := math.Abs(float64(settA.CultureTradition - settB.CultureTradition))
			openDiff := math.Abs(float64(settA.CultureOpenness - settB.CultureOpenness))
			milDiff := math.Abs(float64(settA.CultureMilitarism - settB.CultureMilitarism))
			avgDiff := (tradDiff + openDiff + milDiff) / 3.0 // 0 (identical) to 2 (opposite)
			cultureScore = (1.0 - avgDiff) * phi.Agnosis     // -0.236 (opposite) to +0.236 (identical)

			// 4. Distance modifier: nearby relations are amplified.
			distMod := 1.0 / (1.0 + float64(dist)*phi.Agnosis*0.5) // 1.0 at dist 1, ~0.54 at dist 5, ~0.32 at dist 10

			// Blend factors into a weekly sentiment delta.
			// Individual scores are already Φ-scaled; Psyche dampener ensures
			// only genuinely strong pairs reach diplomatic thresholds.
			delta := (factionScore + tradeScore + cultureScore) * distMod * phi.Psyche

			// Get or create relation.
			rel, exists := s.Relations[key]
			if !exists {
				rel = &SettlementRelation{}
				s.Relations[key] = rel
			}

			// Decay existing sentiment toward 0, then apply delta.
			rel.Sentiment *= (1.0 - phi.Agnosis) // ~76.4% retention per week
			rel.Sentiment += delta

			// Store trade volume for API visibility, then reset tracker.
			if tradeVol, ok := s.TradeTracker[key]; ok {
				rel.Trade = tradeVol
			}

			// Clamp to [-1, 1].
			if rel.Sentiment > 1.0 {
				rel.Sentiment = 1.0
			}
			if rel.Sentiment < -1.0 {
				rel.Sentiment = -1.0
			}

			updated++
		}
	}

	// Reset weekly trade tracker.
	s.TradeTracker = make(map[SettRelKey]float64)

	// Clean up relations for dead settlement pairs (both near-zero sentiment and no trade).
	for key, rel := range s.Relations {
		if math.Abs(rel.Sentiment) < 0.001 && rel.Trade == 0 {
			delete(s.Relations, key)
		}
	}

	if updated > 0 {
		slog.Info("settlement relations updated", "pairs", updated, "active", len(s.Relations))
	}
}

// GetSettlementRelations returns all relations for a given settlement.
func (s *Simulation) GetSettlementRelations(settID uint64) map[uint64]*SettlementRelation {
	result := make(map[uint64]*SettlementRelation)
	for key, rel := range s.Relations {
		if key.A == settID {
			result[key.B] = rel
		} else if key.B == settID {
			result[key.A] = rel
		}
	}
	return result
}

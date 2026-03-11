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

	// Pre-compute dominant faction per settlement.
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
		bestCount := 0
		factionCounts := make(map[int]int)
		for _, a := range s.SettlementAgents[sett.ID] {
			if a.Alive && a.FactionID != nil {
				factionCounts[int(*a.FactionID)]++
			}
		}
		for fid, count := range factionCounts {
			if count > bestCount {
				bestCount = count
				bestFaction = fid
			}
		}
		strength := 0.0
		if sett.Population > 0 {
			strength = float64(bestCount) / float64(sett.Population)
		}
		factionData[sett.ID] = factionInfo{dominantFaction: bestFaction, factionStrength: strength}
	}

	// Compute relations for settlement pairs within interaction range (10 hexes).
	// Beyond this, settlements don't meaningfully interact.
	const maxRange = 10
	updated := 0

	for i, settA := range s.Settlements {
		if settA.Population == 0 {
			continue
		}
		for j := i + 1; j < len(s.Settlements); j++ {
			settB := s.Settlements[j]
			if settB.Population == 0 {
				continue
			}

			dist := world.Distance(settA.Position, settB.Position)
			if dist > maxRange {
				continue
			}

			key := settRelKey(settA.ID, settB.ID)

			// 1. Faction affinity: shared dominant faction → positive, different → slight negative.
			factionScore := 0.0
			fA, fB := factionData[settA.ID], factionData[settB.ID]
			if fA.dominantFaction >= 0 && fB.dominantFaction >= 0 {
				if fA.dominantFaction == fB.dominantFaction {
					// Shared faction: strength of bond = product of dominance strengths.
					factionScore = fA.factionStrength * fB.factionStrength * phi.Being // Up to ~1.618
				} else {
					// Different factions: mild rivalry.
					factionScore = -fA.factionStrength * fB.factionStrength * phi.Agnosis // Up to ~-0.236
				}
			}

			// 2. Trade affinity: recent trade volume → positive sentiment.
			tradeScore := 0.0
			if tradeVol, ok := s.TradeTracker[key]; ok && tradeVol > 0 {
				// Logarithmic: diminishing returns. 1 trade = 0, 10 = 0.54, 50 = 0.92.
				tradeScore = math.Log2(1+tradeVol) * phi.Agnosis * 0.1
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
			delta := (factionScore + tradeScore + cultureScore) * distMod * phi.Agnosis

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

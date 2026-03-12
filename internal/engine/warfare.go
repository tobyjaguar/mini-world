// Inter-settlement warfare — raids between hostile settlement pairs.
// Raids emerge from negative sentiment + military capability + faction tensions.
// Defense is strongly favored (walls, deterrence, alliances contribute).
//
// Raid trigger (weekly evaluation):
//   - Sentiment < -Agnosis (hostile)
//   - Attacker has > Agnosis soldier ratio (~2.4%)
//   - Distance ≤ 5 hexes (march range)
//   - No mutual defense pact with target
//   - Iron Brotherhood dominance or high militarism increases likelihood
//   - Deterministic pseudo-random gate per settlement pair per week
//
// Resolution:
//   - Attack = sum(soldier Combat) × militarism × Being
//   - Defense = deterrence × defender soldiers × walls × alliances
//   - Outcome: attack / (attack + defense) = victory probability
//   - Victor plunders treasury, may capture border hex
//   - Casualties proportional to battle intensity on both sides
//
// All constants Φ-derived. Defense-favored by design — wars are costly.
package engine

import (
	"fmt"
	"log/slog"
	"math"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/phi"
	"github.com/talgya/mini-world/internal/social"
	"github.com/talgya/mini-world/internal/world"
)

// processWarfare evaluates hostile settlement pairs for potential raids.
// Called weekly after processDiplomacy so agreement state is fresh.
func (s *Simulation) processWarfare(tick uint64) {
	simWeek := tick / TicksPerSimWeek

	raids := 0
	victories := 0
	defeats := 0

	for key, rel := range s.Relations {
		// Only consider hostile pairs.
		if rel.Sentiment >= -phi.Agnosis {
			continue
		}

		settA, okA := s.SettlementIndex[key.A]
		settB, okB := s.SettlementIndex[key.B]
		if !okA || !okB || settA.Population < 50 || settB.Population < 50 {
			continue
		}

		dist := world.Distance(settA.Position, settB.Position)
		if dist > 5 {
			continue // Beyond march range.
		}

		// Mutual defense prevents aggression.
		if s.HasMutualDefense(settA.ID, settB.ID) {
			continue
		}

		// Active peace treaty prevents raids.
		if s.HasPeace(settA.ID, settB.ID) {
			continue
		}

		// Evaluate both directions — the more aggressive settlement raids.
		for _, pair := range [2][2]*social.Settlement{{settA, settB}, {settB, settA}} {
			attacker, defender := pair[0], pair[1]
			if s.evaluateRaid(tick, simWeek, attacker, defender, rel, dist) {
				raids++
				won := s.resolveRaid(tick, attacker, defender, dist)
				if won {
					victories++
				} else {
					defeats++
				}
				s.RecordRaid(attacker.ID, defender.ID)
				break // Only one raid per pair per week.
			}
		}
	}

	if raids > 0 {
		slog.Info("warfare processed",
			"raids", raids,
			"victories", victories,
			"defeats", defeats,
		)
	}
}

// evaluateRaid checks whether a settlement will raid its neighbor this week.
func (s *Simulation) evaluateRaid(tick, simWeek uint64, attacker, defender *social.Settlement, rel *SettlementRelation, dist int) bool {
	attackAgents := s.SettlementAgents[attacker.ID]

	// Count soldiers and compute soldier ratio.
	soldierCount := 0
	for _, a := range attackAgents {
		if a.Alive && a.Occupation == agents.OccupationSoldier {
			soldierCount++
		}
	}
	soldierRatio := float64(soldierCount) / (float64(attacker.Population) + 1)
	if soldierRatio < phi.Agnosis*0.1 { // Need at least ~2.4% soldiers
		return false
	}

	// Aggression factors:
	// 1. Hostility: how negative is sentiment (0 at threshold, ~1 at -1.0)
	hostility := math.Abs(rel.Sentiment) - phi.Agnosis
	if hostility < 0 {
		hostility = 0
	}

	// 2. Military culture: militarism axis increases raid likelihood
	militarism := float64(attacker.CultureMilitarism)
	if militarism < 0 {
		militarism = 0
	}

	// 3. Iron Brotherhood dominance: martial faction is more aggressive
	ibBonus := 0.0
	for _, f := range s.Factions {
		if f.Name == "Iron Brotherhood" {
			if inf, ok := f.Influence[attacker.ID]; ok && inf > 30 {
				ibBonus = phi.Agnosis // ~0.236 bonus when IB is dominant
			}
		}
	}

	// Combined raid probability: hostility × (militarism + soldier ratio + IB) × Agnosis
	raidChance := hostility * (militarism*phi.Agnosis + soldierRatio + ibBonus) * phi.Agnosis

	// Deterministic gate: hash of week + attacker ID + defender ID
	hash := (simWeek*31 + uint64(attacker.ID)*17 + uint64(defender.ID)*7) % 1000
	threshold := float64(hash) / 1000.0

	return raidChance > threshold
}

// resolveRaid resolves a raid between two settlements. Returns true if attacker wins.
func (s *Simulation) resolveRaid(tick uint64, attacker, defender *social.Settlement, dist int) bool {
	attackAgents := s.SettlementAgents[attacker.ID]
	defendAgents := s.SettlementAgents[defender.ID]

	// Compute attack strength: sum of soldier Combat skills × militarism × Being.
	attackStrength := 0.0
	var attackSoldiers []*agents.Agent
	for _, a := range attackAgents {
		if a.Alive && a.Occupation == agents.OccupationSoldier {
			attackStrength += float64(a.Skills.Combat)
			attackSoldiers = append(attackSoldiers, a)
		}
	}
	if len(attackSoldiers) == 0 {
		return false // No soldiers to send.
	}
	militaristicBonus := 1.0 + float64(attacker.CultureMilitarism)*phi.Agnosis*0.5
	if militaristicBonus < 0.5 {
		militaristicBonus = 0.5
	}
	attackStrength *= militaristicBonus * phi.Being

	// Distance penalty: longer marches weaken the force.
	distPenalty := 1.0 / (1.0 + float64(dist)*phi.Agnosis*0.3)
	attackStrength *= distPenalty

	// Compute defense strength: deterrence formula + soldier combat + walls + alliances.
	defenseStrength := 0.0
	var defenderSoldiers []*agents.Agent
	for _, a := range defendAgents {
		if a.Alive && a.Occupation == agents.OccupationSoldier {
			defenseStrength += float64(a.Skills.Combat)
			defenderSoldiers = append(defenderSoldiers, a)
		}
	}
	// Walls are a massive defensive advantage.
	wallBonus := 1.0 + float64(defender.WallLevel)*phi.Being // ~1.618 per wall level (stronger than crime)
	defenseStrength *= wallBonus

	// Governance quality improves coordination.
	defenseStrength *= (1.0 + defender.GovernanceScore)

	// Alliance reinforcements: mutual defense allies contribute.
	for key, agreement := range s.Agreements {
		if agreement.Type < AgreementAlliance {
			continue
		}
		allyID := uint64(0)
		if key.A == defender.ID {
			allyID = key.B
		} else if key.B == defender.ID {
			allyID = key.A
		}
		if allyID == 0 || allyID == attacker.ID {
			continue
		}
		// Ally soldiers contribute at distance-attenuated strength.
		allySett, ok := s.SettlementIndex[allyID]
		if !ok {
			continue
		}
		allyDist := world.Distance(allySett.Position, defender.Position)
		if allyDist > 8 { // Allies must be within reinforcement range.
			continue
		}
		allyAttenuation := 1.0 / (1.0 + float64(allyDist)*phi.Agnosis*0.5)
		for _, a := range s.SettlementAgents[allyID] {
			if a.Alive && a.Occupation == agents.OccupationSoldier {
				defenseStrength += float64(a.Skills.Combat) * allyAttenuation * phi.Psyche // Allies contribute at Psyche efficiency
			}
		}
	}

	// Defense home advantage: Being bonus.
	defenseStrength *= phi.Being

	// Victory probability: attack / (attack + defense). Defense-favored.
	totalStrength := attackStrength + defenseStrength
	if totalStrength < 0.01 {
		return false // No meaningful military force.
	}
	victoryChance := attackStrength / totalStrength

	// Deterministic outcome from tick + settlement IDs.
	outcomeHash := (tick*13 + uint64(attacker.ID)*23 + uint64(defender.ID)*37) % 1000
	outcome := float64(outcomeHash) / 1000.0
	attackerWins := outcome < victoryChance

	// Battle intensity: determines casualty rate. Higher when forces are balanced.
	balance := math.Min(attackStrength, defenseStrength) / math.Max(attackStrength, defenseStrength)
	intensity := balance * phi.Psyche // 0 (one-sided) to Psyche (balanced battle)

	// --- Apply casualties ---
	loserSoldiers := defenderSoldiers
	winnerSoldiers := attackSoldiers
	if !attackerWins {
		loserSoldiers = attackSoldiers
		winnerSoldiers = defenderSoldiers
	}

	// Loser casualties: intensity × Agnosis of soldiers
	loserCasualtyRate := intensity * phi.Agnosis
	loserCasualties := s.applyCasualties(tick, loserSoldiers, loserCasualtyRate)

	// Winner casualties: half the loser rate
	winnerCasualtyRate := intensity * phi.Agnosis * 0.5
	winnerCasualties := s.applyCasualties(tick, winnerSoldiers, winnerCasualtyRate)

	// --- Apply war outcomes ---
	winnerSett := attacker
	loserSett := defender
	if !attackerWins {
		winnerSett = defender
		loserSett = attacker
	}

	// Treasury plunder: winner takes Agnosis fraction of loser treasury.
	plunder := uint64(float64(loserSett.Treasury) * phi.Agnosis * 0.5)
	if plunder > loserSett.Treasury {
		plunder = loserSett.Treasury
	}
	loserSett.Treasury -= plunder
	winnerSett.Treasury += plunder

	// Hex capture: victorious attacker takes one border hex if available.
	var capturedHex *world.HexCoord
	if attackerWins {
		capturedHex = s.captureHex(tick, winnerSett, loserSett)
	}

	// Morale effects on surviving soldiers.
	for _, a := range winnerSoldiers {
		if a.Alive {
			a.Needs.Purpose += float32(phi.Agnosis * 0.5) // +0.118 purpose (victory)
			a.Needs.Esteem += float32(phi.Agnosis * 0.3)  // +0.071 esteem
		}
	}
	for _, a := range loserSoldiers {
		if a.Alive {
			a.Needs.Belonging += float32(phi.Agnosis * 0.3) // +0.071 belonging (shared adversity)
			a.Needs.Safety -= float32(phi.Agnosis * 0.5)    // -0.118 safety
		}
	}

	// Sentiment impact: war deepens hostility.
	key := settRelKey(attacker.ID, defender.ID)
	if rel, ok := s.Relations[key]; ok {
		rel.Sentiment -= phi.Psyche // -0.382 sentiment from raid
		if rel.Sentiment < -1.0 {
			rel.Sentiment = -1.0
		}
	}

	// Emit battle event.
	result := "defeated"
	if attackerWins {
		result = "raided"
	}
	attackerCas := loserCasualties
	defenderCas := winnerCasualties
	if attackerWins {
		attackerCas = winnerCasualties
		defenderCas = loserCasualties
	}

	desc := fmt.Sprintf("Forces from %s %s %s — %d attacker casualties, %d defender casualties, %d crowns plundered",
		attacker.Name, result, defender.Name, attackerCas, defenderCas, plunder)
	if capturedHex != nil {
		desc += fmt.Sprintf(", captured hex (%d,%d)", capturedHex.Q, capturedHex.R)
	}

	meta := map[string]any{
		"event_type":          "raid",
		"attacker_id":         attacker.ID,
		"attacker_name":       attacker.Name,
		"defender_id":         defender.ID,
		"defender_name":       defender.Name,
		"settlement_id":       defender.ID,
		"settlement_name":     defender.Name,
		"result":              result,
		"attacker_casualties": attackerCas,
		"defender_casualties": defenderCas,
		"plunder":             plunder,
	}
	if capturedHex != nil {
		meta["captured_hex_q"] = capturedHex.Q
		meta["captured_hex_r"] = capturedHex.R
	}

	s.EmitEvent(Event{
		Tick:        tick,
		Description: desc,
		Category:    "warfare",
		Meta:        meta,
	})

	slog.Info("raid resolved",
		"attacker", attacker.Name,
		"defender", defender.Name,
		"result", result,
		"attack_strength", fmt.Sprintf("%.1f", attackStrength),
		"defense_strength", fmt.Sprintf("%.1f", defenseStrength),
		"victory_chance", fmt.Sprintf("%.2f", victoryChance),
		"casualties", winnerCasualties+loserCasualties,
		"plunder", plunder,
	)

	return attackerWins
}

// captureHex transfers one border hex from the loser to the winner after a raid victory.
// Only captures if the defender has >1 claimed hex (never takes the last one).
// Selects the highest-health defender hex adjacent to any attacker hex.
// Infrastructure (irrigation, conservation) is preserved — conquest transfers stewardship.
func (s *Simulation) captureHex(tick uint64, winner, loser *social.Settlement) *world.HexCoord {
	// Count defender's claimed hexes — never take the last one.
	defenderHexes := 0
	for _, h := range s.WorldMap.Hexes {
		if h.ClaimedBy != nil && *h.ClaimedBy == loser.ID {
			defenderHexes++
		}
	}
	if defenderHexes <= 1 {
		return nil
	}

	// Build set of attacker-claimed hex coords for adjacency check.
	attackerClaims := make(map[world.HexCoord]bool)
	for _, h := range s.WorldMap.Hexes {
		if h.ClaimedBy != nil && *h.ClaimedBy == winner.ID {
			attackerClaims[h.Coord] = true
		}
	}

	// Find defender hexes adjacent to attacker territory. Pick highest health.
	var bestHex *world.Hex
	for _, h := range s.WorldMap.Hexes {
		if h.ClaimedBy == nil || *h.ClaimedBy != loser.ID {
			continue
		}
		// Skip the defender's settlement hex — don't capture their home.
		if h.SettlementID != nil && *h.SettlementID == loser.ID {
			continue
		}
		// Check adjacency to attacker territory.
		neighbors := h.Coord.Neighbors()
		adjacent := false
		for _, n := range neighbors[:] {
			if attackerClaims[n] {
				adjacent = true
				break
			}
		}
		if !adjacent {
			continue
		}
		if bestHex == nil || h.Health > bestHex.Health {
			bestHex = h
		}
	}

	if bestHex == nil {
		return nil
	}

	// Transfer claim. Infrastructure preserved.
	winnerID := winner.ID
	bestHex.ClaimedBy = &winnerID
	coord := bestHex.Coord

	slog.Info("hex captured",
		"winner", winner.Name,
		"loser", loser.Name,
		"hex", fmt.Sprintf("(%d,%d)", coord.Q, coord.R),
		"health", fmt.Sprintf("%.2f", bestHex.Health),
	)

	return &coord
}

// applyCasualties kills a fraction of soldiers based on casualty rate.
// Returns the number of casualties.
func (s *Simulation) applyCasualties(tick uint64, soldiers []*agents.Agent, rate float64) int {
	if rate <= 0 || len(soldiers) == 0 {
		return 0
	}

	// Minimum 1 casualty if rate > 0 and soldiers exist.
	targetCasualties := int(math.Ceil(float64(len(soldiers)) * rate))
	if targetCasualties > len(soldiers) {
		targetCasualties = len(soldiers)
	}
	if targetCasualties < 1 {
		targetCasualties = 1
	}

	// Kill soldiers with lowest combat skill first (least experienced fall first).
	// Use deterministic selection: hash each soldier, sort by "unluckiness".
	casualties := 0
	for _, a := range soldiers {
		if casualties >= targetCasualties {
			break
		}
		if !a.Alive {
			continue
		}
		// Lower combat skill → higher casualty chance.
		survivalChance := float64(a.Skills.Combat) * phi.Being
		hash := float64((tick*7+uint64(a.ID)*13)%100) / 100.0
		if hash > survivalChance {
			a.Alive = false
			s.handleAgentDeath(a, tick, "battle")
			casualties++
		}
	}
	return casualties
}

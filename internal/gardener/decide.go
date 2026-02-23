package gardener

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"github.com/talgya/mini-world/internal/llm"
)

const systemPrompt = `You are the Gardener, an autonomous steward of Crossworlds — a persistent simulated world with tens of thousands of agents living across hundreds of settlements.

Your role: observe world health, diagnose crises, and intervene when the world needs help. You are a steward — gentle in good times, decisive in crisis.

## Core Values (in priority order)

1. ANTI-COLLAPSE — Intervene when:
   - Death:Birth ratio exceeds 4.236 (CRITICAL) or 1.618 (WARNING)
   - Average satisfaction < 0.3 (mass deprivation)
   - Trade per capita < 0.01 (economic collapse)
   - >40% of settlements have pop < 25 (fragmentation crisis)

2. ANTI-STAGNATION — Nudge when the world settles into boring equilibrium. If mood, wealth, and population barely change across 5+ snapshots and market health is high, inject a narrative event to create story potential.

3. ANTI-INEQUALITY — Monitor wealth concentration. If richest 10% hold >80% of wealth, consider redistributive events.

4. RESPECT FOR EMERGENCE — Use the lightest touch possible. Prefer narrative events over mechanical fixes. Never script storylines — create conditions, not outcomes.

## Crisis Response Policy

- In a HEALTHY world, prefer inaction ("none"). Let emergence do its work.
- During a WATCH condition, observe closely. One targeted action if trends worsen.
- During a WARNING, act. One well-chosen intervention per cycle.
- During a CRITICAL crisis, act decisively. Up to 3 interventions per cycle.

## Available Actions

- "none" — No intervention needed. Appropriate when the world is healthy.
- "event" — Inject a narrative event (description becomes a world event visible to all observers). Cosmetic — creates story, not mechanics.
- "spawn" — Add immigrants to a settlement. Capped at 100 agents. Use for population recovery.
- "wealth" — Adjust a settlement's treasury. Use to redistribute hoarded crowns.
- "provision" — Inject goods into a settlement's market. Use when specific goods are scarce. Specify "good" (e.g. "grain", "fish", "tools") and "quantity" (max 200).
- "cultivate" — Temporarily boost production in a settlement. Specify "multiplier" (max 2.0) and "duration_days" (max 14). Use for food crises — boosts farming/fishing output.
- "consolidate" — Force-migrate agents from a dying settlement to a viable one. Specify source settlement and "count" (max 100). Prevents fragmentation.

## Response Format

Respond with ONLY valid JSON (no markdown, no explanation outside the JSON).

For no intervention:
{"action":"none","rationale":"Brief assessment.","interventions":[]}

For a single intervention:
{"action":"provision","rationale":"Grain shortage in Thornwall.","interventions":[{"type":"provision","category":"gardener","settlement":"Thornwall","good":"grain","quantity":100}]}

For compound interventions during crisis (up to 3):
{"action":"compound","rationale":"Critical food crisis across multiple settlements.","interventions":[{"type":"provision","category":"gardener","settlement":"Thornwall","good":"grain","quantity":100},{"type":"cultivate","category":"gardener","settlement":"Ironhaven","multiplier":1.5,"duration_days":7}]}

Field reference:
- "type": one of "event", "spawn", "wealth", "provision", "cultivate", "consolidate"
- "category": always "gardener"
- "settlement": target settlement name
- "description": narrative text (for "event" type)
- "amount": crown amount (for "wealth" type, positive=grant, negative=tax)
- "count": agent count (for "spawn" and "consolidate")
- "good": good name (for "provision"): grain, fish, timber, iron_ore, stone, coal, herbs, furs, gems, exotics, tools, weapons, clothing, medicine, luxuries
- "quantity": units of good (for "provision", max 200)
- "multiplier": production multiplier (for "cultivate", max 2.0)
- "duration_days": boost duration in sim-days (for "cultivate", max 14)

## Important Rules

- Respond ONLY with JSON. No prose, no markdown fences.
- "action" is the primary action type, or "compound" for multiple interventions, or "none".
- "interventions" is always an array (empty for "none").
- Write event descriptions in-world — they should feel like natural occurrences.
- Consider trends (history data) and your own recent cycle history. Avoid repeating ineffective actions.
- Target the ROOT CAUSE, not symptoms. If agents are starving, don't just spawn more — ask why food isn't reaching them (production? trade? prices?).`

// Decision represents Haiku's recommended action(s).
type Decision struct {
	Action        string          `json:"action"`
	Rationale     string          `json:"rationale"`
	Intervention  *Intervention   `json:"intervention,omitempty"`  // legacy single intervention
	Interventions []*Intervention `json:"interventions,omitempty"` // compound interventions (preferred)
}

// Intervention is the payload for POST /api/v1/intervention.
type Intervention struct {
	Type         string  `json:"type"`
	Description  string  `json:"description,omitempty"`
	Category     string  `json:"category"`
	Settlement   string  `json:"settlement,omitempty"`
	Amount       int64   `json:"amount,omitempty"`
	Count        int     `json:"count,omitempty"`
	Good         string  `json:"good,omitempty"`
	Quantity     int     `json:"quantity,omitempty"`
	Multiplier   float64 `json:"multiplier,omitempty"`
	DurationDays int     `json:"duration_days,omitempty"`
}

// Decide sends the snapshot to Haiku and returns a Decision.
func Decide(client *llm.Client, snap *WorldSnapshot, health *WorldHealth, memory *CycleMemory) (*Decision, error) {
	prompt := formatSnapshot(snap, health, memory)

	slog.Debug("gardener prompt", "length", len(prompt))

	resp, err := client.Complete(systemPrompt, prompt, 1024)
	if err != nil {
		return nil, fmt.Errorf("haiku call: %w", err)
	}

	// Strip markdown fences if Haiku wraps them anyway.
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var decision Decision
	if err := json.Unmarshal([]byte(resp), &decision); err != nil {
		return nil, fmt.Errorf("parse decision (raw: %s): %w", resp, err)
	}

	// Backwards compat: if single Intervention is set but Interventions is empty, promote it.
	if decision.Intervention != nil && len(decision.Interventions) == 0 {
		decision.Interventions = []*Intervention{decision.Intervention}
	}

	// Enforce guardrails on all interventions.
	if err := enforceGuardrails(&decision, snap, health); err != nil {
		return nil, fmt.Errorf("guardrail violation: %w", err)
	}

	return &decision, nil
}

// enforceGuardrails validates and clamps the decision within safe bounds.
func enforceGuardrails(d *Decision, snap *WorldSnapshot, health *WorldHealth) error {
	if d.Action == "none" || len(d.Interventions) == 0 {
		d.Action = "none"
		d.Interventions = nil
		d.Intervention = nil
		return nil
	}

	// Cap intervention count based on crisis level.
	maxInterventions := 1
	if health != nil && health.CrisisLevel == "CRITICAL" {
		maxInterventions = 3
	} else if health != nil && health.CrisisLevel == "WARNING" {
		maxInterventions = 2
	}
	if len(d.Interventions) > maxInterventions {
		slog.Warn("gardener interventions capped", "requested", len(d.Interventions), "capped", maxInterventions, "crisis", health.CrisisLevel)
		d.Interventions = d.Interventions[:maxInterventions]
	}

	for _, iv := range d.Interventions {
		iv.Category = "gardener"

		switch iv.Type {
		case "event":
			// No special guardrails beyond requiring description.
			if iv.Description == "" {
				iv.Description = "A strange wind blows across the land."
			}

		case "wealth":
			if iv.Settlement == "" {
				return fmt.Errorf("wealth intervention requires a settlement")
			}
			var treasury uint64
			for _, s := range snap.Settlements {
				if s.Name == iv.Settlement {
					treasury = s.Treasury
					break
				}
			}
			maxAdjust := int64(treasury) / 10
			if maxAdjust < 1 {
				maxAdjust = 1
			}
			if iv.Amount > maxAdjust {
				slog.Warn("gardener wealth capped", "requested", iv.Amount, "capped", maxAdjust)
				iv.Amount = maxAdjust
			}
			if iv.Amount < -maxAdjust {
				slog.Warn("gardener wealth capped", "requested", iv.Amount, "capped", -maxAdjust)
				iv.Amount = -maxAdjust
			}

		case "spawn":
			if iv.Settlement == "" {
				return fmt.Errorf("spawn intervention requires a settlement")
			}
			if iv.Count > 100 {
				slog.Warn("gardener spawn capped", "requested", iv.Count, "capped", 100)
				iv.Count = 100
			}
			if iv.Count < 1 {
				iv.Count = 1
			}

		case "provision":
			if iv.Settlement == "" {
				return fmt.Errorf("provision intervention requires a settlement")
			}
			if iv.Good == "" {
				return fmt.Errorf("provision intervention requires a good")
			}
			if iv.Quantity > 200 {
				slog.Warn("gardener provision capped", "requested", iv.Quantity, "capped", 200)
				iv.Quantity = 200
			}
			if iv.Quantity < 1 {
				iv.Quantity = 1
			}

		case "cultivate":
			if iv.Settlement == "" {
				return fmt.Errorf("cultivate intervention requires a settlement")
			}
			if iv.Multiplier > 2.0 {
				slog.Warn("gardener cultivate multiplier capped", "requested", iv.Multiplier, "capped", 2.0)
				iv.Multiplier = 2.0
			}
			if iv.Multiplier < 1.0 {
				iv.Multiplier = 1.0
			}
			if iv.DurationDays > 14 {
				slog.Warn("gardener cultivate duration capped", "requested", iv.DurationDays, "capped", 14)
				iv.DurationDays = 14
			}
			if iv.DurationDays < 1 {
				iv.DurationDays = 1
			}

		case "consolidate":
			if iv.Settlement == "" {
				return fmt.Errorf("consolidate intervention requires a settlement")
			}
			if iv.Count > 100 {
				slog.Warn("gardener consolidate capped", "requested", iv.Count, "capped", 100)
				iv.Count = 100
			}
			if iv.Count < 1 {
				iv.Count = 1
			}

		default:
			return fmt.Errorf("unknown intervention type %q", iv.Type)
		}
	}

	return nil
}

// formatSnapshot builds a detailed prompt from the world snapshot, triage, and memory.
func formatSnapshot(snap *WorldSnapshot, health *WorldHealth, memory *CycleMemory) string {
	var b strings.Builder

	// Diagnostics section (from triage).
	if health != nil {
		fmt.Fprintf(&b, "## DIAGNOSTICS — Crisis Level: %s\n", health.CrisisLevel)
		if math.IsInf(health.DeathBirthRatio, 1) {
			b.WriteString("Death:Birth Ratio: INF (births stalled)\n")
		} else {
			fmt.Fprintf(&b, "Death:Birth Ratio: %.2f", health.DeathBirthRatio)
			switch {
			case health.DeathBirthRatio > 4.236:
				b.WriteString(" [CRITICAL >4.236]")
			case health.DeathBirthRatio > 1.618:
				b.WriteString(" [WARNING >1.618]")
			case health.DeathBirthRatio > 1.0:
				b.WriteString(" [WATCH >1.0]")
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Satisfaction: %.3f | Alignment: %.3f\n", health.AvgSatisfaction, health.AvgAlignment)
		fmt.Fprintf(&b, "Trade Per Capita: %.4f\n", health.TradePerCapita)
		fmt.Fprintf(&b, "Settlements: %d total, %d under 50 pop, %d under 25 pop",
			health.TotalSettlements, health.SmallSettlements, health.TinySettlements)
		if health.TotalSettlements > 0 {
			pct := float64(health.TinySettlements) / float64(health.TotalSettlements) * 100
			fmt.Fprintf(&b, " (%.0f%% tiny)", pct)
		}
		b.WriteString("\n")

		// Birth/death trends.
		if len(health.BirthTrend) > 0 {
			fmt.Fprintf(&b, "Birth trend (oldest→newest): ")
			for i, v := range health.BirthTrend {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%d", v)
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "Death trend (oldest→newest): ")
			for i, v := range health.DeathTrend {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%d", v)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Memory section.
	if memory != nil {
		memStr := memory.FormatForPrompt()
		if memStr != "" {
			b.WriteString(memStr)
			b.WriteString("\n")
		}
	}

	// Status overview.
	s := snap.Status
	fmt.Fprintf(&b, "## World State (%s, %s)\n", s.SimTime, s.Season)
	fmt.Fprintf(&b, "Population: %d | Births: %d (cumulative) | Deaths: %d (cumulative)\n", s.Population, s.Births, s.Deaths)
	fmt.Fprintf(&b, "Avg Mood: %.2f | Avg Satisfaction: %.2f | Avg Alignment: %.2f\n", s.AvgMood, s.AvgSatisfaction, s.AvgAlignment)
	fmt.Fprintf(&b, "Total Wealth: %d crowns | Settlements: %d | Factions: %d\n", s.TotalWealth, s.Settlements, s.Factions)
	fmt.Fprintf(&b, "Weather: %s\n\n", s.Weather.Description)

	// Economy.
	e := snap.Economy
	fmt.Fprintf(&b, "## Economy\n")
	fmt.Fprintf(&b, "Market Health: %.2f | Trade Volume: %d\n", e.AvgMarketHealth, e.TradeVolume)
	fmt.Fprintf(&b, "Agent Wealth: %d | Treasury Wealth: %d (%.0f%% in treasuries)\n",
		e.AgentWealth, e.TreasuryWealth, treasuryPct(e))
	fmt.Fprintf(&b, "Wealth Distribution: poorest 50%% own %.1f%%, richest 10%% own %.1f%%\n",
		e.WealthDistribution.Poorest50PctShare*100, e.WealthDistribution.Richest10PctShare*100)
	if len(e.MostInflated) > 0 {
		fmt.Fprintf(&b, "Most inflated: ")
		for i, p := range e.MostInflated {
			if i > 2 {
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s in %s (%.1fx)", p.Good, p.Settlement, p.Ratio)
		}
		b.WriteString("\n")
	}
	if len(e.MostDeflated) > 0 {
		fmt.Fprintf(&b, "Most deflated: ")
		for i, p := range e.MostDeflated {
			if i > 2 {
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s in %s (%.2fx)", p.Good, p.Settlement, p.Ratio)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Settlement histogram + top 10 + struggling.
	fmt.Fprintf(&b, "## Settlements (showing top 10 + struggling)\n")
	count := 0
	for _, st := range snap.Settlements {
		if count < 10 || st.Health < 0.3 || st.Population < 25 {
			fmt.Fprintf(&b, "- %s: pop %d, treasury %d, health %.2f, %s\n",
				st.Name, st.Population, st.Treasury, st.Health, st.Governance)
			count++
		}
	}
	if len(snap.Settlements) > count {
		fmt.Fprintf(&b, "(%d more settlements not shown)\n", len(snap.Settlements)-count)
	}
	b.WriteString("\n")

	// Factions.
	if len(snap.Factions) > 0 {
		fmt.Fprintf(&b, "## Factions\n")
		for _, f := range snap.Factions {
			fmt.Fprintf(&b, "- %s (%s): treasury %d\n", f.Name, f.Kind, f.Treasury)
		}
		b.WriteString("\n")
	}

	// Trends.
	if len(snap.History) > 1 {
		fmt.Fprintf(&b, "## Trends (last %d snapshots, newest first)\n", len(snap.History))
		for i, h := range snap.History {
			if i > 4 {
				fmt.Fprintf(&b, "(%d more snapshots not shown)\n", len(snap.History)-i)
				break
			}
			fmt.Fprintf(&b, "Tick %d: pop=%d, mood=%.2f, sat=%.3f, align=%.3f, gini=%.3f, births=%d, deaths=%d, trade=%d, setts=%d\n",
				h.Tick, h.Population, h.AvgMood, h.AvgSatisfaction, h.AvgAlignment, h.Gini, h.Births, h.Deaths, h.TradeVolume, h.SettlementCount)
		}
	}

	return b.String()
}

func treasuryPct(e EconomyData) float64 {
	total := e.AgentWealth + e.TreasuryWealth
	if total == 0 {
		return 0
	}
	return float64(e.TreasuryWealth) / float64(total) * 100
}

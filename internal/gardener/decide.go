package gardener

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/talgya/mini-world/internal/llm"
)

const systemPrompt = `You are the Gardener, an autonomous steward of Crossworlds — a persistent simulated world with tens of thousands of agents living across hundreds of settlements.

Your role: observe world health and recommend zero or one gentle intervention per cycle. You are a steward, not a god. You tend the soil so the garden can grow.

## Core Values (in priority order)

1. ANTI-COLLAPSE — Intervene when population crashes (>10% decline between snapshots), mass starvation (avg_survival < 0.3), or settlement death spirals (multiple settlements with health < 0.2) threaten the world's continuity.

2. ANTI-STAGNATION — Nudge when the world settles into boring equilibrium. If mood, wealth, and population barely change across 5+ snapshots and market health is high, inject a narrative event to create story potential.

3. ANTI-INEQUALITY — Monitor wealth concentration. If richest 10% hold >80% of wealth, consider redistributive events (natural disasters hitting wealthy settlements, windfalls to poor ones).

4. RESPECT FOR EMERGENCE — Use the lightest touch possible. Prefer narrative events over mechanical fixes. Never script storylines — create conditions, not outcomes. When in doubt, do nothing.

## Available Actions

- "none" — No intervention needed. This is the RIGHT choice most of the time.
- "event" — Inject a narrative event (description becomes a world event visible to all observers).
- "wealth" — Adjust a settlement's treasury (positive = grant, negative = disaster/tax). Capped at 10% of their current treasury.
- "spawn" — Add immigrants to a settlement. Capped at 20 agents.

## Response Format

Respond with ONLY valid JSON (no markdown, no explanation outside the JSON):
{
  "action": "none",
  "rationale": "Brief explanation of your assessment and why this action (or inaction) is appropriate.",
  "intervention": null
}

For interventions:
{
  "action": "event",
  "rationale": "Population in Ironhaven dropped 15% — injecting refugee caravan to stabilize.",
  "intervention": {
    "type": "event",
    "description": "A caravan of displaced farmers arrives seeking shelter and work.",
    "category": "gardener",
    "settlement": "Ironhaven",
    "amount": 0,
    "count": 0
  }
}

For wealth adjustments, set "type": "wealth", "settlement": "<name>", "amount": <number>.
For spawns, set "type": "spawn", "settlement": "<name>", "count": <number>.

## Important Rules

- Respond ONLY with JSON. No prose, no markdown fences.
- "action" must be one of: "none", "event", "wealth", "spawn"
- When action is "none", set "intervention" to null.
- "category" must always be "gardener".
- Write event descriptions in-world — they should feel like natural occurrences, not divine edicts.
- Consider trends (the history data), not just current state. A world slowly declining needs intervention sooner than one that dipped once.`

// Decision represents Haiku's recommended action.
type Decision struct {
	Action       string        `json:"action"`
	Rationale    string        `json:"rationale"`
	Intervention *Intervention `json:"intervention"`
}

// Intervention is the payload for POST /api/v1/intervention.
type Intervention struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category"`
	Settlement  string `json:"settlement,omitempty"`
	Amount      int64  `json:"amount,omitempty"`
	Count       int    `json:"count,omitempty"`
}

// Decide sends the snapshot to Haiku and returns a Decision.
func Decide(client *llm.Client, snap *WorldSnapshot) (*Decision, error) {
	prompt := formatSnapshot(snap)

	slog.Debug("gardener prompt", "length", len(prompt))

	resp, err := client.Complete(systemPrompt, prompt, 512)
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

	// Enforce guardrails.
	if err := enforceGuardrails(&decision, snap); err != nil {
		return nil, fmt.Errorf("guardrail violation: %w", err)
	}

	return &decision, nil
}

// enforceGuardrails validates and clamps the decision within safe bounds.
func enforceGuardrails(d *Decision, snap *WorldSnapshot) error {
	switch d.Action {
	case "none":
		d.Intervention = nil
		return nil

	case "event", "wealth", "spawn":
		if d.Intervention == nil {
			return fmt.Errorf("action %q requires an intervention payload", d.Action)
		}

	default:
		return fmt.Errorf("unknown action %q", d.Action)
	}

	// Force category to gardener.
	d.Intervention.Category = "gardener"

	// Ensure intervention type matches action.
	d.Intervention.Type = d.Action

	switch d.Action {
	case "wealth":
		if d.Intervention.Settlement == "" {
			return fmt.Errorf("wealth intervention requires a settlement")
		}
		// Cap at 10% of target settlement treasury.
		var treasury uint64
		for _, s := range snap.Settlements {
			if s.Name == d.Intervention.Settlement {
				treasury = s.Treasury
				break
			}
		}
		maxAdjust := int64(treasury) / 10
		if maxAdjust < 1 {
			maxAdjust = 1
		}
		if d.Intervention.Amount > maxAdjust {
			slog.Warn("gardener wealth capped", "requested", d.Intervention.Amount, "capped", maxAdjust)
			d.Intervention.Amount = maxAdjust
		}
		if d.Intervention.Amount < -maxAdjust {
			slog.Warn("gardener wealth capped", "requested", d.Intervention.Amount, "capped", -maxAdjust)
			d.Intervention.Amount = -maxAdjust
		}

	case "spawn":
		if d.Intervention.Settlement == "" {
			return fmt.Errorf("spawn intervention requires a settlement")
		}
		if d.Intervention.Count > 20 {
			slog.Warn("gardener spawn capped", "requested", d.Intervention.Count, "capped", 20)
			d.Intervention.Count = 20
		}
		if d.Intervention.Count < 1 {
			d.Intervention.Count = 1
		}
	}

	return nil
}

// formatSnapshot builds a concise prompt from the world snapshot.
func formatSnapshot(snap *WorldSnapshot) string {
	var b strings.Builder

	// Status overview.
	s := snap.Status
	fmt.Fprintf(&b, "## World State (%s, %s)\n", s.SimTime, s.Season)
	fmt.Fprintf(&b, "Population: %d | Births: %d | Deaths: %d\n", s.Population, s.Births, s.Deaths)
	fmt.Fprintf(&b, "Avg Mood: %.2f | Total Wealth: %d crowns\n", s.AvgMood, s.TotalWealth)
	fmt.Fprintf(&b, "Settlements: %d | Factions: %d\n", s.Settlements, s.Factions)
	fmt.Fprintf(&b, "Weather: %s\n\n", s.Weather.Description)

	// Economy.
	e := snap.Economy
	fmt.Fprintf(&b, "## Economy\n")
	fmt.Fprintf(&b, "Market Health: %.2f | Trade Volume: %d\n", e.AvgMarketHealth, e.TradeVolume)
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

	// Settlements — top 10 by population + any with health < 0.3.
	fmt.Fprintf(&b, "## Settlements (showing top 10 + struggling)\n")
	shown := make(map[uint64]bool)
	count := 0
	for _, st := range snap.Settlements {
		if count < 10 || st.Health < 0.3 {
			fmt.Fprintf(&b, "- %s: pop %d, treasury %d, health %.2f, %s\n",
				st.Name, st.Population, st.Treasury, st.Health, st.Governance)
			shown[st.ID] = true
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
		fmt.Fprintf(&b, "## Trends (last %d snapshots)\n", len(snap.History))
		first := snap.History[0]
		last := snap.History[len(snap.History)-1]
		popDelta := last.Population - first.Population
		popPct := 0.0
		if first.Population > 0 {
			popPct = float64(popDelta) / float64(first.Population) * 100
		}
		fmt.Fprintf(&b, "Population: %d → %d (%+.1f%%)\n", first.Population, last.Population, popPct)
		fmt.Fprintf(&b, "Wealth: %d → %d\n", first.TotalWealth, last.TotalWealth)
		fmt.Fprintf(&b, "Mood: %.2f → %.2f\n", first.AvgMood, last.AvgMood)
		fmt.Fprintf(&b, "Gini: %.3f → %.3f\n", first.Gini, last.Gini)
		fmt.Fprintf(&b, "Settlements: %d → %d\n", first.SettlementCount, last.SettlementCount)
	}

	return b.String()
}

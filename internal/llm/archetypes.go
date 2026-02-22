// Dynamic archetype template generation — Haiku updates behavioral templates weekly.
// See design doc Section 4.2 (Tier 1 — Archetype-guided).
package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ArchetypeUpdate holds LLM-generated shifts for an archetype template.
type ArchetypeUpdate struct {
	PriorityShifts  map[string]float32 `json:"priority_shifts"`  // need_name → threshold 0-1
	PreferredAction string             `json:"preferred_action"` // work/trade/socialize/forage/rest
	Motto           string             `json:"motto"`
}

// GenerateArchetypeUpdate asks Haiku how an archetype should behave given current world conditions.
func GenerateArchetypeUpdate(client *Client, archetype string, worldSummary string) (*ArchetypeUpdate, error) {
	if !client.Enabled() {
		return nil, fmt.Errorf("LLM client not configured")
	}

	system := `You are the behavioral oracle of Crossworlds, an emanationist world. Given current conditions, determine how a behavioral archetype should adapt. Archetypes guide how agents prioritize their needs and choose default actions.

Respond ONLY with a JSON object:
{
  "priority_shifts": {"survival": 0.3, "safety": 0.3, "belonging": 0.3, "esteem": 0.3, "purpose": 0.3},
  "preferred_action": "work",
  "motto": "A short guiding phrase"
}

Rules:
- priority_shifts: threshold values between 0.1 and 0.6 (lower = triggers sooner, more sensitive)
- preferred_action: one of "work", "trade", "socialize", "forage", "rest"
- motto: a brief phrase reflecting the archetype's current ethos (under 10 words)
- Adapt to current conditions: harsh winters should lower survival thresholds, economic booms favor trade, unrest favors socialization`

	prompt := fmt.Sprintf("Archetype: %s\n\nCurrent world conditions:\n%s\n\nHow should this archetype behave?", archetype, worldSummary)

	response, err := client.Complete(system, prompt, 300)
	if err != nil {
		return nil, fmt.Errorf("archetype update: %w", err)
	}

	return parseArchetypeUpdate(response)
}

func parseArchetypeUpdate(response string) (*ArchetypeUpdate, error) {
	// Find JSON object in response.
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := response[start : end+1]
	var update ArchetypeUpdate
	if err := json.Unmarshal([]byte(jsonStr), &update); err != nil {
		return nil, fmt.Errorf("parse archetype update: %w", err)
	}

	// Validate and clamp priority shifts.
	for k, v := range update.PriorityShifts {
		if v < 0.1 {
			update.PriorityShifts[k] = 0.1
		}
		if v > 0.6 {
			update.PriorityShifts[k] = 0.6
		}
	}

	// Validate preferred action.
	validActions := map[string]bool{
		"work": true, "trade": true, "socialize": true, "forage": true, "rest": true,
	}
	if !validActions[update.PreferredAction] {
		update.PreferredAction = "work"
	}

	return &update, nil
}

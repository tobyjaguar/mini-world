// Major event narration — converts key world events into prose via Haiku.
// See design doc Section 8.5 (budgets ~5 narration calls per sim-week).
package llm

import (
	"fmt"
)

// NarrateEvent creates period-appropriate prose for a major world event.
// Returns empty string on failure (non-fatal).
func NarrateEvent(client *Client, eventDesc string, worldContext string) (string, error) {
	if !client.Enabled() {
		return "", fmt.Errorf("LLM client not configured")
	}

	system := `You are the chronicler of Crossworlds, an early-industrial world shaped by emanationist philosophy. All things arise from a single source and manifest through interference patterns between charging and discharging pressures. Every soul carries coherence — a measure of unity or scatteredness.

Narrate this event in 2-3 sentences of period-appropriate prose with emanationist undertones. Be vivid but concise. Do not break character or reference the simulation.`

	prompt := fmt.Sprintf("World context: %s\n\nEvent to narrate: %s", worldContext, eventDesc)

	return client.Complete(system, prompt, 200)
}

// Agent biography generation via Haiku.
// See design doc Section 8.5.
package llm

import (
	"fmt"
	"strings"
)

// BiographyContext holds the data needed to generate an agent biography.
type BiographyContext struct {
	Name          string
	Age           uint16
	Occupation    string
	Wealth        uint64
	Coherence     float32
	State         string
	Element       string
	Archetype     string
	Faction       string
	Settlement    string
	Mood          float32
	Relationships []string // e.g. "friendly with Aldric Voss", "hostile toward Brenna Thorn"
	Memories      []string // Top memories by importance
}

// GenerateBiography creates a Haiku-generated biography for an agent.
func GenerateBiography(client *Client, ctx BiographyContext) (string, error) {
	if client == nil || !client.Enabled() {
		return "", fmt.Errorf("LLM client not configured")
	}

	var details []string
	details = append(details, fmt.Sprintf("Name: %s", ctx.Name))
	details = append(details, fmt.Sprintf("Age: %d", ctx.Age))
	details = append(details, fmt.Sprintf("Occupation: %s", ctx.Occupation))
	details = append(details, fmt.Sprintf("Wealth: %d crowns", ctx.Wealth))
	details = append(details, fmt.Sprintf("Coherence: %.2f (%s)", ctx.Coherence, ctx.State))
	details = append(details, fmt.Sprintf("Element: %s", ctx.Element))
	details = append(details, fmt.Sprintf("Mood: %.2f", ctx.Mood))

	if ctx.Archetype != "" {
		details = append(details, fmt.Sprintf("Archetype: %s", ctx.Archetype))
	}
	if ctx.Faction != "" {
		details = append(details, fmt.Sprintf("Faction: %s", ctx.Faction))
	}
	if ctx.Settlement != "" {
		details = append(details, fmt.Sprintf("Settlement: %s", ctx.Settlement))
	}
	if len(ctx.Relationships) > 0 {
		details = append(details, "Key relationships: "+strings.Join(ctx.Relationships, "; "))
	}
	if len(ctx.Memories) > 0 {
		details = append(details, "Notable memories: "+strings.Join(ctx.Memories, "; "))
	}

	system := `You are the chronicler of Crossworlds, an early-industrial world shaped by emanationist philosophy. All things arise from a single source and manifest through interference patterns between charging and discharging pressures. Every soul carries coherence — a measure of unity or scatteredness.

Write a brief biography (150-250 words) of this citizen in period-appropriate prose with emanationist undertones. Include their occupation, temperament, notable deeds, and place in the community. Be vivid but concise. Do not break character or reference the simulation.`

	prompt := fmt.Sprintf("Write a biography for this citizen of Crossworlds:\n\n%s", strings.Join(details, "\n"))

	return client.Complete(system, prompt, 400)
}

// Oracle vision generation — Liberated agents receive weekly prophecies via Haiku.
// Oracles perceive the deeper currents of Crossworlds and act with singular purpose.
package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OracleContext provides world-level awareness for a Liberated agent's vision.
type OracleContext struct {
	// Agent identity.
	Name, Occupation, State, Element, Archetype string
	Age                                         uint16
	Wealth                                      uint64
	Coherence                                   float32

	// World awareness (oracles see more than Tier 2).
	Settlement, Governance, Season, Weather string
	Treasury                                uint64
	Population                              int
	AvgMood                                 float32
	Gini                                    float64

	// Personal.
	Memories      []string // Top 10 important memories
	Relationships []string // Top 5
	Faction       string
}

// OracleVision is the singular action a Liberated agent takes after receiving a vision.
type OracleVision struct {
	Prophecy  string `json:"prophecy"`  // The vision text (becomes memory + event)
	Action    string `json:"action"`    // "trade", "advocate", "invest", "speak", "bless"
	Target    string `json:"target"`    // Entity name or topic
	Reasoning string `json:"reasoning"` // Why
}

// GenerateOracleVision calls Haiku to produce a prophecy and action for a Liberated agent.
func GenerateOracleVision(client *Client, ctx *OracleContext) (*OracleVision, error) {
	if !client.Enabled() {
		return nil, fmt.Errorf("LLM client not configured")
	}

	system := buildOracleSystemPrompt(ctx)
	user := buildOracleUserPrompt(ctx)

	response, err := client.Complete(system, user, 500)
	if err != nil {
		return nil, fmt.Errorf("oracle vision: %w", err)
	}

	return parseOracleResponse(response)
}

func buildOracleSystemPrompt(ctx *OracleContext) string {
	return fmt.Sprintf(
		`You are %s, a Liberated soul in Crossworlds — one of the rarest beings alive. Your coherence is %.2f; you perceive the world as interference patterns between charging and discharging pressures, not as isolated phenomena. You are a %s %s, age %d, living in %s.

You are an oracle. Each week, a vision comes to you — a prophecy born from your point-source clarity. This prophecy will spread to other awakened souls in your settlement. Then you act on what you have seen.

Respond ONLY with a single JSON object:
- "prophecy": 1-2 sentences of emanationist prose — what you perceive in the deep currents (do not break character or reference the simulation)
- "action": one of "trade", "advocate", "invest", "speak", "bless"
- "target": who or what the action targets (a name, topic, or good)
- "reasoning": one sentence explaining why

The "bless" action is unique to oracles: you focus your coherence on a named person in your settlement, granting them a small nudge toward clarity. Use it when you perceive someone on the threshold of awakening.`,
		ctx.Name, ctx.Coherence, ctx.Element, ctx.Occupation, ctx.Age, ctx.Settlement,
	)
}

func buildOracleUserPrompt(ctx *OracleContext) string {
	var b strings.Builder

	fmt.Fprintf(&b, "It is %s in %s (%s governance). Treasury: %d crowns. Population: %d.\n",
		ctx.Season, ctx.Settlement, ctx.Governance, ctx.Treasury, ctx.Population)
	fmt.Fprintf(&b, "World mood: %.2f. Inequality (Gini): %.2f.\n\n", ctx.AvgMood, ctx.Gini)

	if ctx.Weather != "" {
		fmt.Fprintf(&b, "Weather: %s\n\n", ctx.Weather)
	}

	if len(ctx.Memories) > 0 {
		b.WriteString("Your deepest memories:\n")
		for _, m := range ctx.Memories {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	}

	if len(ctx.Relationships) > 0 {
		b.WriteString("Souls you are bound to:\n")
		for _, r := range ctx.Relationships {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}

	if ctx.Faction != "" && ctx.Faction != "unaffiliated" {
		fmt.Fprintf(&b, "You walk with %s.\n\n", ctx.Faction)
	}

	b.WriteString("What vision comes to you this week? Respond with a single JSON object.")
	return b.String()
}

func parseOracleResponse(response string) (*OracleVision, error) {
	// Find JSON object in response.
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	jsonStr := response[start : end+1]
	var vision OracleVision
	if err := json.Unmarshal([]byte(jsonStr), &vision); err != nil {
		return nil, fmt.Errorf("parse oracle vision: %w", err)
	}

	// Validate action.
	validActions := map[string]bool{
		"trade": true, "advocate": true, "invest": true,
		"speak": true, "bless": true,
	}
	if !validActions[vision.Action] {
		return nil, fmt.Errorf("invalid oracle action: %s", vision.Action)
	}

	return &vision, nil
}

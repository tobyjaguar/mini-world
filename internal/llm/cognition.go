// Tier 2 individual LLM cognition — named characters make weekly decisions via Haiku.
// See design doc Section 4.2 (Tier 2 — Individual).
package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Tier2Decision is a single action decided by an LLM-powered agent.
type Tier2Decision struct {
	Action    string `json:"action"`
	Target    string `json:"target"`
	Reasoning string `json:"reasoning"`
}

// Tier2Context provides the situational data an LLM needs to make decisions.
type Tier2Context struct {
	Name       string
	Age        uint16
	Occupation string
	Wealth     uint64
	Mood       string // e.g. "content", "anxious", "elated"
	Coherence  float32
	State      string // "Embodied", "Centered", "Liberated"
	Archetype  string

	Settlement string
	Governance string
	Treasury   uint64
	Season     string

	Memories      []string // Recent memory descriptions
	Relationships []string // "Name (sentiment: 0.5, trust: 0.7)"
	Faction       string   // Faction name or "unaffiliated"
	Weather       string   // Weather description if available
}

// GenerateTier2Decision calls Haiku to produce 1-3 weekly actions for a Tier 2 agent.
func GenerateTier2Decision(client *Client, ctx *Tier2Context) ([]Tier2Decision, error) {
	if !client.Enabled() {
		return nil, fmt.Errorf("LLM client not configured")
	}

	system := buildTier2SystemPrompt(ctx)
	user := buildTier2UserPrompt(ctx)

	response, err := client.Complete(system, user, 500)
	if err != nil {
		return nil, fmt.Errorf("tier 2 decision: %w", err)
	}

	return parseTier2Response(response)
}

func buildTier2SystemPrompt(ctx *Tier2Context) string {
	return fmt.Sprintf(
		`You are %s, a %d-year-old %s living in %s. You are %s.
Your coherence is %.2f (%s). You have %d crowns.
You belong to %s.

You exist in Crossworlds, an early-industrial world shaped by emanationist philosophy.
Every soul carries coherence — scattered souls react to circumstances, while unified souls shape them.
Make decisions that reflect your personality, circumstances, and inner state.

Respond ONLY with a JSON array of 1-3 actions. Each action has:
- "action": one of "work", "trade", "socialize", "advocate", "invest", "recruit", "speak"
- "target": who or what the action targets (a name, good, or topic)
- "reasoning": one sentence explaining why

Valid actions:
- work: spend the week producing goods from the land (farming, mining, fishing, hunting)
- trade: buy or sell goods at the market
- socialize: strengthen a relationship with someone
- advocate: push for a policy change in your settlement (taxes, governance)
- invest: spend wealth on settlement infrastructure
- recruit: try to bring someone into your faction
- speak: make a public statement (generates narrative color)`,
		ctx.Name, ctx.Age, ctx.Occupation, ctx.Settlement, ctx.Mood,
		ctx.Coherence, ctx.State, ctx.Wealth,
		ctx.Faction,
	)
}

func buildTier2UserPrompt(ctx *Tier2Context) string {
	var b strings.Builder

	fmt.Fprintf(&b, "It is %s in %s. The settlement treasury holds %d crowns under %s governance.\n\n",
		ctx.Season, ctx.Settlement, ctx.Treasury, ctx.Governance)

	if ctx.Weather != "" {
		fmt.Fprintf(&b, "Weather: %s\n\n", ctx.Weather)
	}

	if len(ctx.Memories) > 0 {
		b.WriteString("Recent experiences:\n")
		for _, m := range ctx.Memories {
			fmt.Fprintf(&b, "- %s\n", m)
		}
		b.WriteString("\n")
	}

	if len(ctx.Relationships) > 0 {
		b.WriteString("Key relationships:\n")
		for _, r := range ctx.Relationships {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}

	b.WriteString("What do you do this week? Respond with a JSON array of 1-3 actions.")
	return b.String()
}

func parseTier2Response(response string) ([]Tier2Decision, error) {
	// Find JSON array in response (the LLM might include explanation text).
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}

	jsonStr := response[start : end+1]
	var decisions []Tier2Decision
	if err := json.Unmarshal([]byte(jsonStr), &decisions); err != nil {
		return nil, fmt.Errorf("parse decisions: %w", err)
	}

	// Validate and cap at 3.
	if len(decisions) > 3 {
		decisions = decisions[:3]
	}

	// Validate action types.
	validActions := map[string]bool{
		"work": true, "trade": true, "socialize": true, "advocate": true,
		"invest": true, "recruit": true, "speak": true,
	}
	var valid []Tier2Decision
	for _, d := range decisions {
		if validActions[d.Action] {
			valid = append(valid, d)
		}
	}

	return valid, nil
}

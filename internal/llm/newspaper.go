// Newspaper generation — converts world events into narrative prose.
// See design doc Section 8.4.
package llm

import (
	"fmt"
	"strings"
	"time"
)

// NewspaperData holds the raw data needed to generate a newspaper.
type NewspaperData struct {
	SimTime     string
	Season      string
	Population  int
	Settlements int
	TotalWealth uint64
	AvgMood     float32

	// Recent events by category.
	Deaths    []string
	Births    []string
	Crimes    []string
	Social    []string
	Economy   []string
	Political []string
	Weather   string

	// Top settlements by population.
	TopSettlements []SettlementSummary

	// Notable characters.
	NotableAgents []AgentSummary

	// Market data — top price movers across all settlements.
	MarketPrices []MarketPriceSummary

	// Faction dynamics.
	FactionNews []string

	// Wheeler coherence state of the world.
	AvgCoherence    float32
	CoherenceCounts CoherenceDistribution
}

// SettlementSummary is a brief description of a settlement for the newspaper.
type SettlementSummary struct {
	Name       string
	Population uint32
	Treasury   uint64
	Governance string
	Health     float64 // Conjugate field health ratio
}

// AgentSummary is a brief description of a notable agent.
type AgentSummary struct {
	Name        string
	Age         uint16
	Occupation  string
	Wealth      uint64
	Mood        string
	State       string // State of Being: Embodied, Centered, Liberated
	Element     string // Elemental type: Helium, Hydrogen, Gold, Uranium
	Coherence   float32
}

// MarketPriceSummary describes a notable market price.
type MarketPriceSummary struct {
	Good       string
	Settlement string
	Price      float64
	PriceRatio float64 // Current price / base price (>1 means inflated, <1 deflated)
}

// CoherenceDistribution counts agents by State of Being.
type CoherenceDistribution struct {
	Embodied  int
	Centered  int
	Liberated int
}

// Newspaper holds a generated newspaper issue.
type Newspaper struct {
	GeneratedAt time.Time `json:"generated_at"`
	SimTime     string    `json:"sim_time"`
	Content     string    `json:"content"`
}

// GenerateNewspaper creates a daily newspaper from world events using Haiku.
func GenerateNewspaper(client *Client, data *NewspaperData) (*Newspaper, error) {
	if !client.Enabled() {
		// Fallback: generate a simple text newspaper without LLM.
		return &Newspaper{
			GeneratedAt: time.Now(),
			SimTime:     data.SimTime,
			Content:     generateFallbackNewspaper(data),
		}, nil
	}

	system := `You are the editor of "The Crossworlds Chronicle", the daily broadsheet of an early-industrial world (1700s–1850s) called Crossworlds — a place where alchemical philosophy and mercantile ambition coexist. The world operates under an emanationist cosmology: all things arise from a single source and manifest through interference patterns between charging (centripetal) and discharging (centrifugal) pressures.

Every soul carries a coherence — a measure of how unified or scattered their being is. The Embodied are identified with phenomena, living ordinary lives among desires and routines — not suffering, simply scattered. The Centered are stable and introspective, materially successful but still attached. The rare Liberated souls have achieved self-similarity, a point-source clarity that gives them disproportionate influence.

Write in an engaging, period-appropriate style — broadsheet prose with a philosophical undercurrent. Reference the breathing of the economy (supply and demand as conjugate pressures), the coherence of the populace, and the deeper currents beneath surface events. Keep it concise (under 600 words). Do not break character or reference the simulation.`

	prompt := buildNewspaperPrompt(data)

	content, err := client.Complete(system, prompt, 1000)
	if err != nil {
		// Fall back to simple newspaper on API failure.
		return &Newspaper{
			GeneratedAt: time.Now(),
			SimTime:     data.SimTime,
			Content:     generateFallbackNewspaper(data),
		}, nil
	}

	return &Newspaper{
		GeneratedAt: time.Now(),
		SimTime:     data.SimTime,
		Content:     content,
	}, nil
}

func buildNewspaperPrompt(data *NewspaperData) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Write today's edition of The Crossworlds Chronicle.\n\n")
	fmt.Fprintf(&b, "DATE: %s (%s)\n", data.SimTime, data.Season)
	fmt.Fprintf(&b, "WORLD: %d souls across %d settlements. Total treasury: %d crowns.\n\n", data.Population, data.Settlements, data.TotalWealth)

	// World Spirit — Wheeler coherence overview.
	fmt.Fprintf(&b, "WORLD SPIRIT:\n")
	fmt.Fprintf(&b, "Average coherence: %.2f\n", data.AvgCoherence)
	fmt.Fprintf(&b, "States of Being — Embodied: %d, Centered: %d, Liberated: %d\n\n",
		data.CoherenceCounts.Embodied, data.CoherenceCounts.Centered, data.CoherenceCounts.Liberated)

	if len(data.Deaths) > 0 {
		fmt.Fprintf(&b, "RECENT DEATHS:\n")
		for i, d := range data.Deaths {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteString("\n")
	}

	if len(data.Births) > 0 {
		fmt.Fprintf(&b, "BIRTHS: %d new citizens\n\n", len(data.Births))
	}

	if len(data.Crimes) > 0 {
		fmt.Fprintf(&b, "CRIME REPORTS:\n")
		for i, c := range data.Crimes {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	if len(data.Social) > 0 {
		fmt.Fprintf(&b, "SOCIAL NEWS:\n")
		for i, s := range data.Social {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}

	if len(data.MarketPrices) > 0 {
		fmt.Fprintf(&b, "MARKET REPORT (notable prices):\n")
		for _, mp := range data.MarketPrices {
			direction := "steady"
			if mp.PriceRatio > 1.5 {
				direction = "surging"
			} else if mp.PriceRatio > 1.2 {
				direction = "rising"
			} else if mp.PriceRatio < 0.7 {
				direction = "collapsed"
			} else if mp.PriceRatio < 0.9 {
				direction = "falling"
			}
			fmt.Fprintf(&b, "- %s in %s: %.1f crowns (%s, %.1fx base)\n", mp.Good, mp.Settlement, mp.Price, direction, mp.PriceRatio)
		}
		b.WriteString("\n")
	}

	if data.Weather != "" {
		fmt.Fprintf(&b, "WEATHER: %s\n\n", data.Weather)
	}

	if len(data.Political) > 0 {
		fmt.Fprintf(&b, "POLITICAL AFFAIRS:\n")
		for i, p := range data.Political {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}

	if len(data.FactionNews) > 0 {
		fmt.Fprintf(&b, "FACTION DYNAMICS:\n")
		for _, fn := range data.FactionNews {
			fmt.Fprintf(&b, "- %s\n", fn)
		}
		b.WriteString("\n")
	}

	if len(data.TopSettlements) > 0 {
		fmt.Fprintf(&b, "TOP SETTLEMENTS:\n")
		for _, s := range data.TopSettlements {
			healthDesc := "balanced"
			if s.Health > 0.7 {
				healthDesc = "thriving"
			} else if s.Health < 0.3 {
				healthDesc = "strained"
			}
			fmt.Fprintf(&b, "- %s: pop %d, treasury %d crowns (%s, economic health: %s)\n", s.Name, s.Population, s.Treasury, s.Governance, healthDesc)
		}
		b.WriteString("\n")
	}

	if len(data.NotableAgents) > 0 {
		fmt.Fprintf(&b, "NOTABLE FIGURES:\n")
		for _, a := range data.NotableAgents {
			fmt.Fprintf(&b, "- %s, age %d, %s, %d crowns — %s-type, %s (coherence %.2f)\n",
				a.Name, a.Age, a.Occupation, a.Wealth, a.Element, a.State, a.Coherence)
		}
	}

	return b.String()
}

func generateFallbackNewspaper(data *NewspaperData) string {
	var b strings.Builder

	fmt.Fprintf(&b, "THE CROSSROADS CHRONICLE\n")
	fmt.Fprintf(&b, "========================\n")
	fmt.Fprintf(&b, "%s — %s\n\n", data.SimTime, data.Season)

	fmt.Fprintf(&b, "POPULATION REPORT\n")
	fmt.Fprintf(&b, "The realm counts %d souls across %d settlements.\n", data.Population, data.Settlements)
	fmt.Fprintf(&b, "Total wealth in circulation: %d crowns.\n\n", data.TotalWealth)

	// World Spirit.
	fmt.Fprintf(&b, "THE STATE OF SOULS\n")
	fmt.Fprintf(&b, "Average coherence stands at %.2f.\n", data.AvgCoherence)
	fmt.Fprintf(&b, "Embodied: %d — Centered: %d — Liberated: %d\n\n",
		data.CoherenceCounts.Embodied, data.CoherenceCounts.Centered, data.CoherenceCounts.Liberated)

	if len(data.Deaths) > 0 {
		fmt.Fprintf(&b, "OBITUARIES\n")
		for i, d := range data.Deaths {
			if i >= 5 {
				fmt.Fprintf(&b, "...and %d more.\n", len(data.Deaths)-5)
				break
			}
			fmt.Fprintf(&b, "- %s\n", d)
		}
		b.WriteString("\n")
	}

	if len(data.Births) > 0 {
		fmt.Fprintf(&b, "BIRTHS: %d new citizens welcomed.\n\n", len(data.Births))
	}

	if len(data.MarketPrices) > 0 {
		fmt.Fprintf(&b, "MARKET REPORT\n")
		for _, mp := range data.MarketPrices {
			fmt.Fprintf(&b, "- %s in %s: %.1f crowns (%.1fx base price)\n", mp.Good, mp.Settlement, mp.Price, mp.PriceRatio)
		}
		b.WriteString("\n")
	}

	if len(data.FactionNews) > 0 {
		fmt.Fprintf(&b, "FACTION AFFAIRS\n")
		for _, fn := range data.FactionNews {
			fmt.Fprintf(&b, "- %s\n", fn)
		}
		b.WriteString("\n")
	}

	if data.Weather != "" {
		fmt.Fprintf(&b, "WEATHER REPORT\n")
		fmt.Fprintf(&b, "%s\n\n", data.Weather)
	}

	if len(data.Political) > 0 {
		fmt.Fprintf(&b, "POLITICAL AFFAIRS\n")
		for i, p := range data.Political {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}

	if len(data.Crimes) > 0 {
		fmt.Fprintf(&b, "CRIME BLOTTER\n")
		for i, c := range data.Crimes {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	if len(data.Social) > 0 {
		fmt.Fprintf(&b, "SOCIAL REGISTER\n")
		for i, s := range data.Social {
			if i >= 3 {
				break
			}
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}

	if len(data.TopSettlements) > 0 {
		fmt.Fprintf(&b, "SETTLEMENTS OF NOTE\n")
		for _, s := range data.TopSettlements {
			fmt.Fprintf(&b, "- %s: pop %d, treasury %d crowns (%s)\n", s.Name, s.Population, s.Treasury, s.Governance)
		}
		b.WriteString("\n")
	}

	if len(data.NotableAgents) > 0 {
		fmt.Fprintf(&b, "NOTABLE FIGURES\n")
		for _, a := range data.NotableAgents {
			fmt.Fprintf(&b, "- %s, age %d, %s — %s-type, %s\n", a.Name, a.Age, a.Occupation, a.Element, a.State)
		}
	}

	return b.String()
}

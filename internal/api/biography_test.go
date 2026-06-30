package api

import (
	"strings"
	"testing"

	"github.com/talgya/mini-world/internal/llm"
)

func TestTemplateBiography(t *testing.T) {
	base := llm.BiographyContext{
		Name: "Fern Cross", Age: 22, Occupation: "Farmer",
		Settlement: "Newbridge", Faction: "Ashen Path", Wealth: 120,
	}

	t.Run("liberated, prosperous, with a tie", func(t *testing.T) {
		c := base
		c.Coherence = 0.96
		c.Wealth = 5000
		c.Relationships = []string{"close to Mira Vale"}
		got := templateBiography(c)
		for _, want := range []string{"Fern Cross", "farmer", "Newbridge", "22 years", "Ashen Path", "coherent soul", "prospered", "close to Mira Vale"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in: %s", want, got)
			}
		}
	})

	t.Run("embodied, poor, no settlement/faction", func(t *testing.T) {
		c := llm.BiographyContext{Name: "Tam", Age: 19, Occupation: "Miner", Coherence: 0.2, Wealth: 10}
		got := templateBiography(c)
		for _, want := range []string{"Tam", "the outer wilds", "rhythm of labor", "close to the bone"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in: %s", want, got)
			}
		}
		if strings.Contains(got, "sworn to") {
			t.Errorf("no faction should mean no 'sworn to': %s", got)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		c := base
		c.Coherence = 0.5
		if templateBiography(c) != templateBiography(c) {
			t.Error("template output must be deterministic")
		}
	})
}

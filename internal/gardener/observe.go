// Package gardener implements the autonomous world steward.
// It observes world state via the API, decides on interventions via Haiku,
// and acts via the admin intervention endpoint.
package gardener

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WorldSnapshot holds all data collected during an observation cycle.
type WorldSnapshot struct {
	Status      WorldStatus      `json:"status"`
	Economy     EconomyData      `json:"economy"`
	Settlements []SettlementInfo `json:"settlements"`
	Factions    []FactionInfo    `json:"factions"`
	History     []StatsHistoryRow `json:"history"`
}

// WorldStatus mirrors GET /api/v1/status.
type WorldStatus struct {
	Name        string  `json:"name"`
	Tick        uint64  `json:"tick"`
	SimTime     string  `json:"sim_time"`
	Season      string  `json:"season"`
	Speed       float64 `json:"speed"`
	Running     bool    `json:"running"`
	Population  int     `json:"population"`
	Deaths      int     `json:"deaths"`
	Births      int     `json:"births"`
	Settlements int     `json:"settlements"`
	Factions    int     `json:"factions"`
	AvgMood         float32 `json:"avg_mood"`
	AvgSatisfaction float32 `json:"avg_satisfaction"`
	AvgAlignment    float32 `json:"avg_alignment"`
	TotalWealth     uint64  `json:"total_wealth"`
	Weather     struct {
		Description  string  `json:"description"`
		TempModifier float64 `json:"temp_modifier"`
	} `json:"weather"`
}

// EconomyData mirrors GET /api/v1/economy.
type EconomyData struct {
	TotalCrowns      uint64  `json:"total_crowns"`
	AgentWealth      uint64  `json:"agent_wealth"`
	TreasuryWealth   uint64  `json:"treasury_wealth"`
	AvgMarketHealth  float64 `json:"avg_market_health"`
	TradeVolume      uint64  `json:"trade_volume"`
	MostInflated     []PriceDeviation `json:"most_inflated"`
	MostDeflated     []PriceDeviation `json:"most_deflated"`
	WealthDistribution struct {
		Poorest50PctShare float64 `json:"poorest_50_pct_share"`
		Richest10PctShare float64 `json:"richest_10_pct_share"`
	} `json:"wealth_distribution"`
}

// PriceDeviation represents a notable price deviation from base.
type PriceDeviation struct {
	Good       string  `json:"good"`
	Settlement string  `json:"settlement"`
	Price      float64 `json:"price"`
	BasePrice  float64 `json:"base_price"`
	Ratio      float64 `json:"ratio"`
}

// SettlementInfo mirrors items from GET /api/v1/settlements.
type SettlementInfo struct {
	ID         uint64  `json:"id"`
	Name       string  `json:"name"`
	Q          int     `json:"q"`
	R          int     `json:"r"`
	Population uint32  `json:"population"`
	Governance string  `json:"governance"`
	Treasury   uint64  `json:"treasury"`
	Health     float64 `json:"health"`
}

// FactionInfo mirrors items from GET /api/v1/factions.
type FactionInfo struct {
	ID        uint64             `json:"id"`
	Name      string             `json:"name"`
	Kind      string             `json:"kind"`
	Treasury  uint64             `json:"treasury"`
	Influence map[string]float64 `json:"top_influence"`
}

// StatsHistoryRow mirrors items from GET /api/v1/stats/history.
type StatsHistoryRow struct {
	Tick            uint64  `json:"tick"`
	Population      int     `json:"population"`
	TotalWealth     uint64  `json:"total_wealth"`
	AvgMood         float64 `json:"avg_mood"`
	AvgSurvival     float64 `json:"avg_survival"`
	Births          int     `json:"births"`
	Deaths          int     `json:"deaths"`
	TradeVolume     uint64  `json:"trade_volume"`
	AvgCoherence    float64 `json:"avg_coherence"`
	SettlementCount int     `json:"settlement_count"`
	Gini            float64 `json:"gini"`
	AvgSatisfaction float64 `json:"avg_satisfaction"`
	AvgAlignment    float64 `json:"avg_alignment"`
}

// Observer fetches world state from the API.
type Observer struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewObserver creates an Observer targeting the given API base URL.
func NewObserver(baseURL string) *Observer {
	return &Observer{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Observe fetches all five endpoints and returns a WorldSnapshot.
func (o *Observer) Observe() (*WorldSnapshot, error) {
	snap := &WorldSnapshot{}

	if err := o.fetchJSON("/api/v1/status", &snap.Status); err != nil {
		return nil, fmt.Errorf("fetch status: %w", err)
	}
	if err := o.fetchJSON("/api/v1/economy", &snap.Economy); err != nil {
		return nil, fmt.Errorf("fetch economy: %w", err)
	}
	if err := o.fetchJSON("/api/v1/settlements", &snap.Settlements); err != nil {
		return nil, fmt.Errorf("fetch settlements: %w", err)
	}
	if err := o.fetchJSON("/api/v1/factions", &snap.Factions); err != nil {
		return nil, fmt.Errorf("fetch factions: %w", err)
	}
	if err := o.fetchJSON("/api/v1/stats/history?limit=10", &snap.History); err != nil {
		return nil, fmt.Errorf("fetch stats history: %w", err)
	}

	return snap, nil
}

// fetchJSON GETs a path and decodes the JSON response into target.
func (o *Observer) fetchJSON(path string, target any) error {
	resp, err := o.HTTPClient.Get(o.BaseURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

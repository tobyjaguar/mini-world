// Package sentinel implements a read-only structural health monitor for Crossworlds.
// It watches for recurring failure patterns seen across 29 tuning rounds
// and raises alerts on state transitions. It never modifies the simulation.
package sentinel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WorldSnapshot holds all data collected during an observation cycle.
type WorldSnapshot struct {
	Status      StatusData       `json:"status"`
	Economy     EconomyData      `json:"economy"`
	Settlements []SettlementInfo `json:"settlements"`
	Social      SocialData       `json:"social"`
	History     []StatsHistoryRow `json:"history"`
	Agents      []AgentInfo      `json:"agents"`
}

// StatusData mirrors relevant fields from GET /api/v1/status.
type StatusData struct {
	Tick            uint64                       `json:"tick"`
	SimTime         string                       `json:"sim_time"`
	Population      int                          `json:"population"`
	Deaths          int                          `json:"deaths"`
	Births          int                          `json:"births"`
	AvgSatisfaction float64                      `json:"avg_satisfaction"`
	Occupations     map[string]OccupationDetail  `json:"occupations"`
}

// OccupationDetail holds per-occupation stats from the status endpoint.
type OccupationDetail struct {
	Count           int     `json:"count"`
	AvgSatisfaction float64 `json:"avg_satisfaction"`
}

// EconomyData mirrors relevant fields from GET /api/v1/economy.
type EconomyData struct {
	TotalCrowns    uint64  `json:"total_crowns"`
	TreasuryWealth uint64  `json:"treasury_wealth"`
	AvgMarketHealth float64 `json:"avg_market_health"`
	ProducerHealth  struct {
		Total    int     `json:"total"`
		Working  int     `json:"working"`
		Idle     int     `json:"idle"`
		WorkRate float64 `json:"work_rate"`
	} `json:"producer_health"`
}

// SettlementInfo mirrors relevant fields from GET /api/v1/settlements.
type SettlementInfo struct {
	ID         uint64  `json:"id"`
	Name       string  `json:"name"`
	Population uint32  `json:"population"`
	Governance string  `json:"governance"`
	Health     float64 `json:"health"`
}

// SocialData mirrors relevant fields from GET /api/v1/social.
type SocialData struct {
	Governance struct {
		AvgScore float64 `json:"avg_score"`
	} `json:"governance"`
}

// StatsHistoryRow mirrors items from GET /api/v1/stats/history.
type StatsHistoryRow struct {
	Tick            uint64  `json:"tick"`
	Population      int     `json:"population"`
	Births          int     `json:"births"`
	Deaths          int     `json:"deaths"`
	AvgSatisfaction float64 `json:"avg_satisfaction"`
}

// AgentInfo mirrors relevant fields from GET /api/v1/agents (Tier 2 default).
type AgentInfo struct {
	ID         uint64  `json:"id"`
	Name       string  `json:"name"`
	Occupation string  `json:"occupation"`
	Alive      bool    `json:"alive"`
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

// Observe fetches all six endpoints and returns a WorldSnapshot.
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
	if err := o.fetchJSON("/api/v1/social", &snap.Social); err != nil {
		return nil, fmt.Errorf("fetch social: %w", err)
	}
	if err := o.fetchJSON("/api/v1/stats/history?limit=20", &snap.History); err != nil {
		return nil, fmt.Errorf("fetch stats history: %w", err)
	}
	if err := o.fetchJSON("/api/v1/agents", &snap.Agents); err != nil {
		return nil, fmt.Errorf("fetch agents: %w", err)
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

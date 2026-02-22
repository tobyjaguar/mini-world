package gardener

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// InterventionResult is the response from POST /api/v1/intervention.
type InterventionResult struct {
	Success bool   `json:"success"`
	Details string `json:"details"`
}

// Actor executes interventions via the admin API.
type Actor struct {
	BaseURL    string
	AdminKey   string
	HTTPClient *http.Client
}

// NewActor creates an Actor targeting the given API base URL with admin auth.
func NewActor(baseURL, adminKey string) *Actor {
	return &Actor{
		BaseURL:  baseURL,
		AdminKey: adminKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Act sends an intervention to POST /api/v1/intervention.
func (a *Actor) Act(intervention *Intervention) (*InterventionResult, error) {
	body, err := json.Marshal(intervention)
	if err != nil {
		return nil, fmt.Errorf("marshal intervention: %w", err)
	}

	req, err := http.NewRequest("POST", a.BaseURL+"/api/v1/intervention", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.AdminKey)

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST intervention: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("intervention failed (%d): %s", resp.StatusCode, string(respBody))
	}

	var result InterventionResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

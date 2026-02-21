// Package llm provides the Claude Haiku API client for agent cognition and narrative generation.
// See design doc Section 8.5.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	apiVersion = "2023-06-01"
	model      = "claude-haiku-4-5-20251001"
)

// Client wraps the Anthropic Messages API for Haiku calls.
type Client struct {
	apiKey     string
	httpClient *http.Client

	// Rate limiting: max calls per minute.
	mu        sync.Mutex
	callCount int
	resetAt   time.Time
	maxPerMin int
}

// NewClient creates a new Haiku API client.
// Returns nil if apiKey is empty (LLM features disabled).
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxPerMin: 20, // Conservative rate limit
	}
}

// Enabled returns true if the client has a valid API key.
func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// request is the API request body.
type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

// response is the API response body.
type response struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete sends a prompt to Haiku and returns the response text.
func (c *Client) Complete(system, userPrompt string, maxTokens int) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("LLM client not configured")
	}

	// Rate limiting.
	c.mu.Lock()
	now := time.Now()
	if now.After(c.resetAt) {
		c.callCount = 0
		c.resetAt = now.Add(time.Minute)
	}
	if c.callCount >= c.maxPerMin {
		c.mu.Unlock()
		return "", fmt.Errorf("rate limit exceeded (%d calls/min)", c.maxPerMin)
	}
	c.callCount++
	c.mu.Unlock()

	req := request{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []Message{
			{Role: "user", Content: userPrompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}

	slog.Debug("haiku call",
		"input_tokens", apiResp.Usage.InputTokens,
		"output_tokens", apiResp.Usage.OutputTokens,
	)

	return apiResp.Content[0].Text, nil
}

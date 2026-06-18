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

	// Usage tracking.
	trackMu          sync.Mutex
	callsByTag       map[string]int64
	tokensByTag      map[string][2]int64 // [input, output] uncached tokens per tag
	cacheTokensByTag map[string][2]int64 // [cache_creation, cache_read] tokens per tag
	trackStart       time.Time

	// Visibility: per-provider failure counters (W-15 incident, 2026-05-16).
	// Pre-fix, all LLM failures logged at slog.Debug; the spend-cap suspension
	// was invisible at INFO level for ~30 wall-hours. We now warn on the first
	// failure of a provider's streak and every 100th thereafter, reset on
	// success — keyed by provider so a chain failure names the source.
	failureMu          sync.Mutex
	failuresByProvider map[string]int

	// Provider chain (2026-06-18). Tried in priority order per call; each entry
	// has its own post-cap cooldown. Resolved from LLM_PROVIDERS / per-provider
	// key env vars. See providers.go.
	providers  []*provider
	cooldownMu sync.Mutex
	cooldowns  map[string]time.Time // provider name → skip-until time
}

// NewClient creates an LLM client backed by a provider chain resolved from the
// environment (LLM_PROVIDERS + per-provider key vars), with apiKey supplying the
// Anthropic entry. Returns nil only when the chain is empty (no provider has a
// usable key — LLM features fully disabled).
func NewClient(apiKey string) *Client {
	providers := resolveProviders(apiKey)
	if len(providers) == 0 {
		return nil
	}
	c := &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		maxPerMin:          20, // Conservative rate limit (shared across providers)
		callsByTag:         make(map[string]int64),
		tokensByTag:        make(map[string][2]int64),
		cacheTokensByTag:   make(map[string][2]int64),
		trackStart:         time.Now(),
		providers:          providers,
		cooldowns:          make(map[string]time.Time),
		failuresByProvider: make(map[string]int),
	}

	slog.Info("LLM provider chain configured", "order", providerNames(providers))

	// Start hourly usage summary logger.
	go c.logUsagePeriodically()

	return c
}

// Enabled returns true if the client has at least one configured provider.
func (c *Client) Enabled() bool {
	return c != nil && len(c.providers) > 0
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// request is the legacy API request body (system as string, no caching).
type request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

// systemBlock is one content block in the System array. When cache_control is
// set on the last block, Anthropic caches the prefix up to and including this
// block. Hit on subsequent calls with the same prefix bills cached tokens at
// ~10% of the base input rate.
type systemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"` // "ephemeral" (5-min default TTL)
}

// cachedRequest is the request body when using prompt caching. The System field
// is an array of blocks rather than a string; cacheable blocks carry
// cache_control. The wire protocol accepts either form.
type cachedRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []systemBlock `json:"system,omitempty"`
	Messages  []Message     `json:"messages"`
}

// response is the API response body. CacheCreationInputTokens and
// CacheReadInputTokens are non-zero only on cached requests; their billing rate
// is ~1.25× and ~0.10× the base input rate respectively (Haiku 4.5: $1.25/M
// cache write, $0.10/M cache read vs $1/M input).
type response struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// Complete sends a prompt to Haiku and returns the response text.
// For tracked usage, prefer CompleteTagged.
func (c *Client) Complete(system, userPrompt string, maxTokens int) (string, error) {
	return c.CompleteTagged(system, userPrompt, maxTokens, "unknown")
}

// CompleteTagged sends a prompt to Haiku and records usage under the given tag.
// The system prompt is sent uncached. For repeated calls with a stable system
// prompt, prefer CompleteTaggedCached.
func (c *Client) CompleteTagged(system, userPrompt string, maxTokens int, tag string) (string, error) {
	return c.dispatch(system, userPrompt, maxTokens, tag, false)
}

// CompleteTaggedCached sends a prompt to Haiku with the system prompt marked
// for ephemeral (5-min TTL) prompt caching. Subsequent calls with the same
// system prompt bill cached input tokens at ~10% of the base rate. Cache
// write on the first call costs ~125% of the base rate, so caching is a net
// loss for one-off calls and a net win for repeated batches (oracle batches
// ~10 calls/TickWeek; tier2 batches ~5 calls/TickDay).
func (c *Client) CompleteTaggedCached(system, userPrompt string, maxTokens int, tag string) (string, error) {
	return c.dispatch(system, userPrompt, maxTokens, tag, true)
}

// rateLimit enforces the shared max-calls-per-minute budget across all
// providers. Returns an error when the budget is exhausted for the current
// window; logged at Debug (a benign local throttle, not a provider failure, so
// it does not advance the chain's failure counters).
func (c *Client) rateLimit(tag string) error {
	c.mu.Lock()
	now := time.Now()
	if now.After(c.resetAt) {
		c.callCount = 0
		c.resetAt = now.Add(time.Minute)
	}
	if c.callCount >= c.maxPerMin {
		c.mu.Unlock()
		slog.Debug("LLM local rate limit hit", "tag", tag, "max_per_min", c.maxPerMin)
		return fmt.Errorf("rate limit exceeded (%d calls/min)", c.maxPerMin)
	}
	c.callCount++
	c.mu.Unlock()
	return nil
}

// doRequest sends a pre-marshalled Anthropic body, enforces rate limiting,
// parses the response, records usage and cache stats, and updates the named
// provider's failure counter. Returns the response text or a wrapped error.
func (c *Client) doRequest(body []byte, tag, providerName string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("anthropic provider not configured")
	}

	if err := c.rateLimit(tag); err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		c.recordProviderFailure(providerName, tag, err)
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.recordProviderFailure(providerName, tag, err)
		return "", fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordProviderFailure(providerName, tag, err)
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		c.recordProviderFailure(providerName, tag, apiErr)
		return "", apiErr
	}

	var apiResp response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		c.recordProviderFailure(providerName, tag, err)
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		err := fmt.Errorf("empty response")
		c.recordProviderFailure(providerName, tag, err)
		return "", err
	}

	slog.Debug("haiku call",
		"tag", tag,
		"input_tokens", apiResp.Usage.InputTokens,
		"output_tokens", apiResp.Usage.OutputTokens,
		"cache_write", apiResp.Usage.CacheCreationInputTokens,
		"cache_read", apiResp.Usage.CacheReadInputTokens,
	)

	// Record usage. Cache write/read tokens are tracked separately because
	// they bill at different rates (1.25× and 0.10× respectively).
	c.trackMu.Lock()
	c.callsByTag[tag]++
	tok := c.tokensByTag[tag]
	tok[0] += int64(apiResp.Usage.InputTokens)
	tok[1] += int64(apiResp.Usage.OutputTokens)
	c.tokensByTag[tag] = tok
	cacheTok := c.cacheTokensByTag[tag]
	cacheTok[0] += int64(apiResp.Usage.CacheCreationInputTokens)
	cacheTok[1] += int64(apiResp.Usage.CacheReadInputTokens)
	c.cacheTokensByTag[tag] = cacheTok
	c.trackMu.Unlock()

	c.resetProviderFailures(providerName)
	return apiResp.Content[0].Text, nil
}

// UsageSummary returns current tracking period counters.
func (c *Client) UsageSummary() map[string]any {
	if c == nil {
		return nil
	}
	c.trackMu.Lock()
	defer c.trackMu.Unlock()

	totalCalls := int64(0)
	totalInput := int64(0)
	totalOutput := int64(0)
	totalCacheWrite := int64(0)
	totalCacheRead := int64(0)
	perTag := make(map[string]map[string]int64)

	for tag, calls := range c.callsByTag {
		totalCalls += calls
		tok := c.tokensByTag[tag]
		cacheTok := c.cacheTokensByTag[tag]
		totalInput += tok[0]
		totalOutput += tok[1]
		totalCacheWrite += cacheTok[0]
		totalCacheRead += cacheTok[1]
		perTag[tag] = map[string]int64{
			"calls":              calls,
			"input_tokens":       tok[0],
			"output_tokens":      tok[1],
			"cache_write_tokens": cacheTok[0],
			"cache_read_tokens":  cacheTok[1],
		}
	}

	return map[string]any{
		"period_start":             c.trackStart.UTC().Format(time.RFC3339),
		"period_duration":          time.Since(c.trackStart).Truncate(time.Second).String(),
		"total_calls":              totalCalls,
		"total_input_tokens":       totalInput,
		"total_output_tokens":      totalOutput,
		"total_cache_write_tokens": totalCacheWrite,
		"total_cache_read_tokens":  totalCacheRead,
		"by_tag":                   perTag,
	}
}

// logUsagePeriodically logs a usage summary every hour.
func (c *Client) logUsagePeriodically() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		c.trackMu.Lock()
		totalCalls := int64(0)
		var totalInput, totalOutput, totalCacheWrite, totalCacheRead int64
		tags := make([]any, 0, len(c.callsByTag)*2)
		for tag, calls := range c.callsByTag {
			totalCalls += calls
			tok := c.tokensByTag[tag]
			cacheTok := c.cacheTokensByTag[tag]
			totalInput += tok[0]
			totalOutput += tok[1]
			totalCacheWrite += cacheTok[0]
			totalCacheRead += cacheTok[1]
			tags = append(tags, tag, calls)
		}

		// Reset counters for next period.
		c.callsByTag = make(map[string]int64)
		c.tokensByTag = make(map[string][2]int64)
		c.cacheTokensByTag = make(map[string][2]int64)
		c.trackStart = time.Now()
		c.trackMu.Unlock()

		args := []any{
			"period", "1h",
			"total_calls", totalCalls,
			"input_tokens", totalInput,
			"output_tokens", totalOutput,
			"cache_write_tokens", totalCacheWrite,
			"cache_read_tokens", totalCacheRead,
		}
		args = append(args, tags...)
		slog.Info("llm usage summary", args...)
	}
}

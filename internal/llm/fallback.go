package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Fallback provider support (2026-06-18).
//
// Anthropic enforces a monthly spend cap that, when tripped, fails every call
// with a 400 "specified API usage limits" error until the 1st of the next
// month (W-15, recurring). With the cap shared across the operator's projects,
// the world's cognition/narrative layer can go dark for ~2/3 of each month.
//
// This adds an OpenAI-compatible fallback provider (DeepSeek, xAI/Grok, Venice
// — all speak the same chat-completions wire format). When the primary
// (Anthropic) call fails, the request is retried against the fallback. On a
// usage-cap error specifically, the primary is placed in a short cooldown so we
// stop issuing doomed requests; it is re-probed once per cooldown so the world
// auto-returns to Anthropic when the cap resets.
//
// The fallback is configured via env (set in deploy/config.local):
//
//	LLM_FALLBACK_PROVIDER  deepseek | xai | venice | custom
//	LLM_FALLBACK_API_KEY   provider API key
//	LLM_FALLBACK_MODEL     optional model override (else a per-provider default)
//	LLM_FALLBACK_BASE_URL  required only when PROVIDER=custom (full endpoint URL)
//
// If unset, behavior is unchanged (Anthropic only).

const (
	// primaryCooldown is how long to skip Anthropic after a usage-cap error
	// before re-probing. The cap lasts until the monthly reset (days), so a
	// short cooldown costs one doomed probe per interval and auto-recovers
	// within one interval of the reset.
	primaryCooldown = 30 * time.Minute
)

// fallbackProvider holds the resolved OpenAI-compatible endpoint config.
type fallbackProvider struct {
	name    string // provider label for logs/usage tags
	baseURL string // full chat-completions URL
	apiKey  string
	model   string
}

// providerDefaults maps a provider name to its chat-completions URL and a
// sensible default model. Model is overridable via LLM_FALLBACK_MODEL because
// provider model slugs evolve (e.g. xAI grok-3/grok-4, Venice open-model slugs).
var providerDefaults = map[string]struct{ baseURL, model string }{
	"deepseek": {"https://api.deepseek.com/chat/completions", "deepseek-chat"},
	"xai":      {"https://api.x.ai/v1/chat/completions", "grok-3"},
	"venice":   {"https://api.venice.ai/api/v1/chat/completions", "llama-3.3-70b"},
}

// fallbackFromEnv resolves a fallback provider from environment variables, or
// returns nil if none is configured. Misconfiguration (unknown provider, or
// custom without a base URL) is logged and treated as "no fallback" rather than
// fatal — the world should still boot on Anthropic alone.
func fallbackFromEnv() *fallbackProvider {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_FALLBACK_PROVIDER")))
	if name == "" {
		return nil
	}
	key := strings.TrimSpace(os.Getenv("LLM_FALLBACK_API_KEY"))
	if key == "" {
		slog.Warn("LLM_FALLBACK_PROVIDER set but LLM_FALLBACK_API_KEY empty — fallback disabled", "provider", name)
		return nil
	}

	baseURL := strings.TrimSpace(os.Getenv("LLM_FALLBACK_BASE_URL"))
	modelOverride := strings.TrimSpace(os.Getenv("LLM_FALLBACK_MODEL"))

	fp := &fallbackProvider{name: name, apiKey: key, model: modelOverride}
	if name == "custom" {
		if baseURL == "" {
			slog.Warn("LLM_FALLBACK_PROVIDER=custom requires LLM_FALLBACK_BASE_URL — fallback disabled")
			return nil
		}
		fp.baseURL = baseURL
		if fp.model == "" {
			slog.Warn("LLM_FALLBACK_PROVIDER=custom with no LLM_FALLBACK_MODEL — set one; fallback disabled")
			return nil
		}
		return fp
	}

	def, ok := providerDefaults[name]
	if !ok {
		slog.Warn("unknown LLM_FALLBACK_PROVIDER — fallback disabled",
			"provider", name, "known", "deepseek, xai, venice, custom")
		return nil
	}
	if baseURL != "" {
		fp.baseURL = baseURL // allow override of the default endpoint
	} else {
		fp.baseURL = def.baseURL
	}
	if fp.model == "" {
		fp.model = def.model
	}
	return fp
}

// openAIRequest is the OpenAI-compatible chat-completions request body, shared
// by DeepSeek, xAI, and Venice.
type openAIRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

// openAIResponse is the OpenAI-compatible response body.
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// isCapError reports whether an error looks like an Anthropic usage/spend-cap
// suspension (as opposed to a transient network error or rate limit). These
// persist until the monthly reset, so we cool down the primary on them.
func isCapError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "specified api usage") ||
		strings.Contains(s, "usage limit") ||
		strings.Contains(s, "credit balance")
}

// inPrimaryCooldown reports whether Anthropic is currently in a post-cap
// cooldown window (skip straight to fallback).
func (c *Client) inPrimaryCooldown() bool {
	c.cooldownMu.Lock()
	defer c.cooldownMu.Unlock()
	return time.Now().Before(c.primaryCooldownUntil)
}

func (c *Client) setPrimaryCooldown() {
	c.cooldownMu.Lock()
	c.primaryCooldownUntil = time.Now().Add(primaryCooldown)
	c.cooldownMu.Unlock()
}

// callFallback issues one request to the configured OpenAI-compatible provider,
// records usage (tag suffixed with the provider name so the source is visible
// in /api/v1/llm-usage), and updates the fallback-failure visibility counter.
func (c *Client) callFallback(system, userPrompt string, maxTokens int, tag string) (string, error) {
	fb := c.fallback
	if fb == nil {
		return "", fmt.Errorf("no fallback provider configured")
	}

	if err := c.rateLimit(tag); err != nil {
		return "", err
	}

	msgs := make([]Message, 0, 2)
	if system != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: userPrompt})

	body, err := json.Marshal(openAIRequest{Model: fb.model, MaxTokens: maxTokens, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("marshal fallback request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", fb.baseURL, bytes.NewReader(body))
	if err != nil {
		c.recordFallbackFailure(tag, err)
		return "", fmt.Errorf("create fallback request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+fb.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.recordFallbackFailure(tag, err)
		return "", fmt.Errorf("fallback API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordFallbackFailure(tag, err)
		return "", fmt.Errorf("read fallback response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := fmt.Errorf("fallback API error %d: %s", resp.StatusCode, string(respBody))
		c.recordFallbackFailure(tag, apiErr)
		return "", apiErr
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		c.recordFallbackFailure(tag, err)
		return "", fmt.Errorf("unmarshal fallback response: %w", err)
	}
	if apiResp.Error != nil {
		apiErr := fmt.Errorf("fallback API error: %s", apiResp.Error.Message)
		c.recordFallbackFailure(tag, apiErr)
		return "", apiErr
	}
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		err := fmt.Errorf("empty fallback response")
		c.recordFallbackFailure(tag, err)
		return "", err
	}

	// Record usage under "<tag>:<provider>" so the fallback source is visible
	// alongside Anthropic tags in the usage summary.
	usageTag := tag + ":" + fb.name
	c.trackMu.Lock()
	c.callsByTag[usageTag]++
	tok := c.tokensByTag[usageTag]
	tok[0] += int64(apiResp.Usage.PromptTokens)
	tok[1] += int64(apiResp.Usage.CompletionTokens)
	c.tokensByTag[usageTag] = tok
	c.trackMu.Unlock()

	// Success on the fallback resets its visibility streak.
	c.failureMu.Lock()
	c.fallbackFailures = 0
	c.failureMu.Unlock()

	slog.Debug("fallback call",
		"provider", fb.name, "tag", tag,
		"prompt_tokens", apiResp.Usage.PromptTokens,
		"completion_tokens", apiResp.Usage.CompletionTokens,
	)
	return apiResp.Choices[0].Message.Content, nil
}

// recordFallbackFailure mirrors recordFailure for the fallback path: Warn on
// the first failure of a streak and every 100th. When BOTH providers are down,
// both counters climb and both Warn — making a full outage unmistakable.
func (c *Client) recordFallbackFailure(tag string, err error) {
	c.failureMu.Lock()
	c.fallbackFailures++
	n := c.fallbackFailures
	c.failureMu.Unlock()
	if n == 1 || n%100 == 0 {
		slog.Warn("LLM fallback call failed",
			"provider", c.fallback.name, "tag", tag,
			"consecutive_failures", n, "error", err.Error())
	}
}

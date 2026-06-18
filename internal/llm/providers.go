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

// Multi-provider chain (2026-06-18).
//
// The LLM layer runs an ordered chain of providers. Each call tries them in
// priority order until one succeeds. This serves two goals:
//
//   - Cost: put a cheap provider first (e.g. LLM_PROVIDERS=deepseek,anthropic
//     runs DeepSeek primary and only touches Anthropic if DeepSeek fails).
//   - Resilience: Anthropic enforces a monthly spend cap that fails every call
//     with a 400 "specified API usage limits" error until the 1st of the next
//     month (W-15, recurring, shared across the operator's projects). With a
//     chain, the world keeps thinking on the next provider.
//
// DeepSeek, xAI/Grok, and Venice all speak the OpenAI-compatible
// chat-completions format, so one code path covers all three; Anthropic uses
// its native Messages format (and is the only provider that supports prompt
// caching). When a provider returns a usage/quota cap error it is placed in a
// 30-minute cooldown and skipped; it is re-probed when the cooldown expires, so
// the chain auto-returns to a provider once its cap resets.
//
// Config (deploy/config.local):
//
//	LLM_PROVIDERS    ordered CSV, e.g. "deepseek,anthropic" (default: anthropic)
//	ANTHROPIC_API_KEY / DEEPSEEK_API_KEY / XAI_API_KEY / VENICE_API_KEY
//	<PROVIDER>_MODEL optional per-provider model override
//	CUSTOM_BASE_URL  required if "custom" is in the chain
//
// Back-compat: if LLM_PROVIDERS is unset, the chain is [anthropic] (when
// ANTHROPIC_API_KEY is set) plus any single provider named by the older
// LLM_FALLBACK_PROVIDER/LLM_FALLBACK_API_KEY vars.

const (
	// providerCooldown is how long to skip a provider after a usage/quota cap
	// error before re-probing it.
	providerCooldown = 30 * time.Minute
)

type providerKind int

const (
	kindAnthropic providerKind = iota // native Messages API, supports caching
	kindOpenAI                        // OpenAI-compatible chat-completions
)

// provider is one resolved entry in the chain.
type provider struct {
	name    string // "anthropic" | "deepseek" | "xai" | "venice" | "custom"
	kind    providerKind
	baseURL string
	apiKey  string
	model   string
}

// providerCatalog maps a provider name to its kind, endpoint, default model,
// and the env vars that supply its key and model override. Model defaults are
// overridable because provider slugs evolve (xAI grok-3/4, Venice open models).
var providerCatalog = map[string]struct {
	kind             providerKind
	baseURL, model   string
	keyEnv, modelEnv string
}{
	"anthropic": {kindAnthropic, apiURL, model, "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL"},
	"deepseek":  {kindOpenAI, "https://api.deepseek.com/chat/completions", "deepseek-chat", "DEEPSEEK_API_KEY", "DEEPSEEK_MODEL"},
	"xai":       {kindOpenAI, "https://api.x.ai/v1/chat/completions", "grok-3", "XAI_API_KEY", "XAI_MODEL"},
	"venice":    {kindOpenAI, "https://api.venice.ai/api/v1/chat/completions", "llama-3.3-70b", "VENICE_API_KEY", "VENICE_MODEL"},
	"custom":    {kindOpenAI, "", "", "CUSTOM_API_KEY", "CUSTOM_MODEL"}, // baseURL from CUSTOM_BASE_URL
}

// resolveProviders builds the ordered provider chain from the environment.
// anthropicKey is passed explicitly (already read by main) and used for the
// "anthropic" entry. Providers without a usable key/URL are warned and skipped.
func resolveProviders(anthropicKey string) []*provider {
	order := splitCSV(os.Getenv("LLM_PROVIDERS"))
	if len(order) == 0 {
		// Back-compat default: Anthropic, plus any legacy single fallback.
		if anthropicKey != "" {
			order = append(order, "anthropic")
		}
		if fb := strings.ToLower(strings.TrimSpace(os.Getenv("LLM_FALLBACK_PROVIDER"))); fb != "" {
			order = append(order, fb)
		}
	}

	var chain []*provider
	seen := map[string]bool{}
	for _, name := range order {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		cat, ok := providerCatalog[name]
		if !ok {
			slog.Warn("unknown LLM provider in LLM_PROVIDERS — skipped",
				"provider", name, "known", "anthropic, deepseek, xai, venice, custom")
			continue
		}

		// Resolve key: anthropic uses the passed key; legacy fallback vars
		// supply the key when this provider was named via LLM_FALLBACK_*.
		key := strings.TrimSpace(os.Getenv(cat.keyEnv))
		if name == "anthropic" {
			key = anthropicKey
		} else if key == "" && strings.EqualFold(os.Getenv("LLM_FALLBACK_PROVIDER"), name) {
			key = strings.TrimSpace(os.Getenv("LLM_FALLBACK_API_KEY"))
		}
		if key == "" {
			slog.Warn("LLM provider has no API key — skipped", "provider", name, "key_env", cat.keyEnv)
			continue
		}

		p := &provider{name: name, kind: cat.kind, baseURL: cat.baseURL, apiKey: key, model: cat.model}

		// Model override (per-provider env, or legacy LLM_FALLBACK_MODEL).
		if m := strings.TrimSpace(os.Getenv(cat.modelEnv)); m != "" {
			p.model = m
		} else if strings.EqualFold(os.Getenv("LLM_FALLBACK_PROVIDER"), name) {
			if m := strings.TrimSpace(os.Getenv("LLM_FALLBACK_MODEL")); m != "" {
				p.model = m
			}
		}

		// Base URL: custom requires one; the legacy fallback var may override.
		if name == "custom" {
			p.baseURL = firstNonEmpty(os.Getenv("CUSTOM_BASE_URL"), os.Getenv("LLM_FALLBACK_BASE_URL"))
			if p.baseURL == "" || p.model == "" {
				slog.Warn("custom provider needs CUSTOM_BASE_URL and a model — skipped")
				continue
			}
		} else if u := strings.TrimSpace(os.Getenv("LLM_FALLBACK_BASE_URL")); u != "" && strings.EqualFold(os.Getenv("LLM_FALLBACK_PROVIDER"), name) {
			p.baseURL = u
		}

		chain = append(chain, p)
	}
	return chain
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}

// providerNames returns the resolved chain order for logging.
func providerNames(chain []*provider) []string {
	names := make([]string, len(chain))
	for i, p := range chain {
		names[i] = p.name + "(" + p.model + ")"
	}
	return names
}

// dispatch runs one logical completion across the provider chain in priority
// order, skipping providers that are in a post-cap cooldown. A usage-cap error
// arms that provider's cooldown. `cached` requests Anthropic prompt caching;
// it is ignored by OpenAI-compatible providers.
func (c *Client) dispatch(system, userPrompt string, maxTokens int, tag string, cached bool) (string, error) {
	if len(c.providers) == 0 {
		return "", fmt.Errorf("no LLM providers configured")
	}

	// Prefer providers not in cooldown; if all are cooling down, try them all
	// anyway (the cooldown is advisory, not a hard block).
	candidates := make([]*provider, 0, len(c.providers))
	for _, p := range c.providers {
		if !c.inCooldown(p.name) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		candidates = c.providers
	}

	var lastErr error
	for _, p := range candidates {
		out, err := c.callProvider(p, system, userPrompt, maxTokens, tag, cached)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if isCapError(err) {
			c.setCooldown(p.name)
			slog.Warn("LLM provider usage cap hit — advancing chain",
				"provider", p.name, "cooldown", providerCooldown.String())
		}
	}
	return "", lastErr
}

// callProvider dispatches one attempt to a single provider by kind.
func (c *Client) callProvider(p *provider, system, userPrompt string, maxTokens int, tag string, cached bool) (string, error) {
	if p.kind == kindAnthropic {
		return c.callAnthropic(p, system, userPrompt, maxTokens, tag, cached)
	}
	return c.callOpenAI(p, system, userPrompt, maxTokens, tag)
}

// callAnthropic builds the native Messages body (cached or plain) for the given
// provider's model and sends it via doRequest.
func (c *Client) callAnthropic(p *provider, system, userPrompt string, maxTokens int, tag string, cached bool) (string, error) {
	var body []byte
	var err error
	if cached {
		body, err = json.Marshal(cachedRequest{
			Model:     p.model,
			MaxTokens: maxTokens,
			System:    []systemBlock{{Type: "text", Text: system, CacheControl: &cacheControl{Type: "ephemeral"}}},
			Messages:  []Message{{Role: "user", Content: userPrompt}},
		})
	} else {
		body, err = json.Marshal(request{
			Model:     p.model,
			MaxTokens: maxTokens,
			System:    system,
			Messages:  []Message{{Role: "user", Content: userPrompt}},
		})
	}
	if err != nil {
		return "", fmt.Errorf("marshal anthropic request: %w", err)
	}
	return c.doRequest(body, tag, p.name)
}

// --- OpenAI-compatible providers (DeepSeek / xAI / Venice / custom) ---

// openAIRequest is the OpenAI-compatible chat-completions request body.
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

// callOpenAI issues one request to an OpenAI-compatible provider, records usage
// under "<tag>:<provider>", and updates that provider's failure counter.
func (c *Client) callOpenAI(p *provider, system, userPrompt string, maxTokens int, tag string) (string, error) {
	if err := c.rateLimit(tag); err != nil {
		return "", err
	}

	msgs := make([]Message, 0, 2)
	if system != "" {
		msgs = append(msgs, Message{Role: "system", Content: system})
	}
	msgs = append(msgs, Message{Role: "user", Content: userPrompt})

	body, err := json.Marshal(openAIRequest{Model: p.model, MaxTokens: maxTokens, Messages: msgs})
	if err != nil {
		return "", fmt.Errorf("marshal %s request: %w", p.name, err)
	}

	httpReq, err := http.NewRequest("POST", p.baseURL, bytes.NewReader(body))
	if err != nil {
		c.recordProviderFailure(p.name, tag, err)
		return "", fmt.Errorf("create %s request: %w", p.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.recordProviderFailure(p.name, tag, err)
		return "", fmt.Errorf("%s API call: %w", p.name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordProviderFailure(p.name, tag, err)
		return "", fmt.Errorf("read %s response: %w", p.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := fmt.Errorf("%s API error %d: %s", p.name, resp.StatusCode, string(respBody))
		c.recordProviderFailure(p.name, tag, apiErr)
		return "", apiErr
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		c.recordProviderFailure(p.name, tag, err)
		return "", fmt.Errorf("unmarshal %s response: %w", p.name, err)
	}
	if apiResp.Error != nil {
		apiErr := fmt.Errorf("%s API error: %s", p.name, apiResp.Error.Message)
		c.recordProviderFailure(p.name, tag, apiErr)
		return "", apiErr
	}
	if len(apiResp.Choices) == 0 || apiResp.Choices[0].Message.Content == "" {
		err := fmt.Errorf("empty %s response", p.name)
		c.recordProviderFailure(p.name, tag, err)
		return "", err
	}

	usageTag := tag + ":" + p.name
	c.trackMu.Lock()
	c.callsByTag[usageTag]++
	tok := c.tokensByTag[usageTag]
	tok[0] += int64(apiResp.Usage.PromptTokens)
	tok[1] += int64(apiResp.Usage.CompletionTokens)
	c.tokensByTag[usageTag] = tok
	c.trackMu.Unlock()

	c.resetProviderFailures(p.name)
	slog.Debug("llm call",
		"provider", p.name, "tag", tag,
		"prompt_tokens", apiResp.Usage.PromptTokens,
		"completion_tokens", apiResp.Usage.CompletionTokens,
	)
	return apiResp.Choices[0].Message.Content, nil
}

// --- cap detection & per-provider cooldown ---

// isCapError reports whether an error is a usage/spend/quota cap suspension
// (which persists for a while) rather than a transient error or rate limit.
func isCapError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "specified api usage") ||
		strings.Contains(s, "usage limit") ||
		strings.Contains(s, "credit balance") ||
		strings.Contains(s, "quota") ||
		strings.Contains(s, "insufficient balance")
}

func (c *Client) inCooldown(name string) bool {
	c.cooldownMu.Lock()
	defer c.cooldownMu.Unlock()
	until, ok := c.cooldowns[name]
	return ok && time.Now().Before(until)
}

func (c *Client) setCooldown(name string) {
	c.cooldownMu.Lock()
	c.cooldowns[name] = time.Now().Add(providerCooldown)
	c.cooldownMu.Unlock()
}

// --- per-provider failure visibility (W-15: failures must be visible) ---

// recordProviderFailure increments a provider's consecutive-failure counter and
// Warns on the first failure of a streak and every 100th thereafter. When every
// provider in the chain is failing, each Warns — making a full outage obvious.
func (c *Client) recordProviderFailure(name, tag string, err error) {
	c.failureMu.Lock()
	c.failuresByProvider[name]++
	n := c.failuresByProvider[name]
	c.failureMu.Unlock()
	if n == 1 || n%100 == 0 {
		slog.Warn("LLM call failed",
			"provider", name, "tag", tag,
			"consecutive_failures", n, "error", err.Error())
	}
}

func (c *Client) resetProviderFailures(name string) {
	c.failureMu.Lock()
	c.failuresByProvider[name] = 0
	c.failureMu.Unlock()
}

package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFallbackFromEnv(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantNil  bool
		wantBase string
		wantMdl  string
	}{
		{"unset", nil, true, "", ""},
		{"deepseek default", map[string]string{
			"LLM_FALLBACK_PROVIDER": "deepseek", "LLM_FALLBACK_API_KEY": "k",
		}, false, "https://api.deepseek.com/chat/completions", "deepseek-chat"},
		{"xai model override", map[string]string{
			"LLM_FALLBACK_PROVIDER": "xai", "LLM_FALLBACK_API_KEY": "k", "LLM_FALLBACK_MODEL": "grok-4",
		}, false, "https://api.x.ai/v1/chat/completions", "grok-4"},
		{"provider set but no key", map[string]string{
			"LLM_FALLBACK_PROVIDER": "venice",
		}, true, "", ""},
		{"unknown provider", map[string]string{
			"LLM_FALLBACK_PROVIDER": "bogus", "LLM_FALLBACK_API_KEY": "k",
		}, true, "", ""},
		{"custom needs base url", map[string]string{
			"LLM_FALLBACK_PROVIDER": "custom", "LLM_FALLBACK_API_KEY": "k", "LLM_FALLBACK_MODEL": "m",
		}, true, "", ""},
		{"custom ok", map[string]string{
			"LLM_FALLBACK_PROVIDER": "custom", "LLM_FALLBACK_API_KEY": "k",
			"LLM_FALLBACK_MODEL": "m", "LLM_FALLBACK_BASE_URL": "https://x/v1/chat",
		}, false, "https://x/v1/chat", "m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"LLM_FALLBACK_PROVIDER", "LLM_FALLBACK_API_KEY", "LLM_FALLBACK_MODEL", "LLM_FALLBACK_BASE_URL"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			fp := fallbackFromEnv()
			if tc.wantNil {
				if fp != nil {
					t.Fatalf("want nil, got %+v", fp)
				}
				return
			}
			if fp == nil {
				t.Fatal("want non-nil provider")
			}
			if fp.baseURL != tc.wantBase {
				t.Errorf("baseURL = %q, want %q", fp.baseURL, tc.wantBase)
			}
			if fp.model != tc.wantMdl {
				t.Errorf("model = %q, want %q", fp.model, tc.wantMdl)
			}
		})
	}
}

func TestIsCapError(t *testing.T) {
	capErr := errorString("API error 400: You have reached your specified API usage limits. You will regain access on 2026-07-01")
	if !isCapError(capErr) {
		t.Error("usage-cap message should be detected")
	}
	if isCapError(errorString("rate limit exceeded (20 calls/min)")) {
		t.Error("rate-limit is not a cap error")
	}
	if isCapError(nil) {
		t.Error("nil is not a cap error")
	}
}

// errorString is a tiny error helper for the test.
type errorString string

func (e errorString) Error() string { return string(e) }

// TestFallbackRoundTrip drives the OpenAI-compatible fallback path against a
// mock server and asserts the request shape and response parsing.
func TestFallbackRoundTrip(t *testing.T) {
	var gotAuth, gotModel string
	var gotRoles []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var req openAIRequest
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		for _, m := range req.Messages {
			gotRoles = append(gotRoles, m.Role)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"a vision of grain"}}],"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
	}))
	defer srv.Close()

	c := &Client{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		maxPerMin:  20,
		resetAt:    time.Now().Add(time.Minute),
		callsByTag: map[string]int64{}, tokensByTag: map[string][2]int64{}, cacheTokensByTag: map[string][2]int64{},
		fallback: &fallbackProvider{name: "deepseek", baseURL: srv.URL, apiKey: "secret", model: "deepseek-chat"},
	}

	out, err := c.callFallback("you are an oracle", "speak", 64, "oracle")
	if err != nil {
		t.Fatalf("callFallback: %v", err)
	}
	if out != "a vision of grain" {
		t.Errorf("content = %q", out)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotModel != "deepseek-chat" {
		t.Errorf("model = %q", gotModel)
	}
	if strings.Join(gotRoles, ",") != "system,user" {
		t.Errorf("roles = %v, want system,user", gotRoles)
	}
	// Usage recorded under "<tag>:<provider>".
	if c.callsByTag["oracle:deepseek"] != 1 {
		t.Errorf("usage not recorded under oracle:deepseek: %+v", c.callsByTag)
	}
}

// TestCooldownSkipsPrimary verifies that once the post-cap cooldown is armed,
// completeWithFallback routes straight to the fallback without touching the
// primary (doRequest uses the const Anthropic URL, so any primary attempt would
// fail the offline test — reaching the mock fallback proves the skip).
func TestCooldownSkipsPrimary(t *testing.T) {
	var fallbackHits int
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits++
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer fallback.Close()

	c := &Client{
		apiKey: "anthropic-key", httpClient: &http.Client{Timeout: 5 * time.Second},
		maxPerMin: 20, resetAt: time.Now().Add(time.Minute),
		callsByTag: map[string]int64{}, tokensByTag: map[string][2]int64{}, cacheTokensByTag: map[string][2]int64{},
		fallback: &fallbackProvider{name: "deepseek", baseURL: fallback.URL, apiKey: "k", model: "m"},
	}

	c.setPrimaryCooldown()
	if !c.inPrimaryCooldown() {
		t.Fatal("cooldown should be armed")
	}
	out, err := c.completeWithFallback("s", "u", 16, "tier2", []byte(`{}`))
	if err != nil {
		t.Fatalf("completeWithFallback: %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %q", out)
	}
	if fallbackHits != 1 {
		t.Errorf("fallback hits = %d, want 1", fallbackHits)
	}
}

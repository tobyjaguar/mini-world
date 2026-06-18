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

func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"LLM_PROVIDERS", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL",
		"DEEPSEEK_API_KEY", "DEEPSEEK_MODEL", "XAI_API_KEY", "XAI_MODEL",
		"VENICE_API_KEY", "VENICE_MODEL", "CUSTOM_API_KEY", "CUSTOM_MODEL", "CUSTOM_BASE_URL",
		"LLM_FALLBACK_PROVIDER", "LLM_FALLBACK_API_KEY", "LLM_FALLBACK_MODEL", "LLM_FALLBACK_BASE_URL",
	} {
		t.Setenv(k, "")
	}
}

func TestResolveProviders(t *testing.T) {
	t.Run("explicit chain deepseek,anthropic", func(t *testing.T) {
		clearProviderEnv(t)
		t.Setenv("LLM_PROVIDERS", "deepseek, anthropic")
		t.Setenv("DEEPSEEK_API_KEY", "ds")
		chain := resolveProviders("ant")
		if len(chain) != 2 {
			t.Fatalf("len = %d, want 2: %+v", len(chain), chain)
		}
		if chain[0].name != "deepseek" || chain[0].kind != kindOpenAI || chain[0].model != "deepseek-chat" {
			t.Errorf("chain[0] = %+v", chain[0])
		}
		if chain[1].name != "anthropic" || chain[1].kind != kindAnthropic || chain[1].apiKey != "ant" {
			t.Errorf("chain[1] = %+v", chain[1])
		}
	})

	t.Run("skips keyless providers", func(t *testing.T) {
		clearProviderEnv(t)
		t.Setenv("LLM_PROVIDERS", "deepseek,xai,anthropic")
		t.Setenv("XAI_API_KEY", "x")
		chain := resolveProviders("ant") // deepseek has no key → skipped
		got := make([]string, len(chain))
		for i, p := range chain {
			got[i] = p.name
		}
		if strings.Join(got, ",") != "xai,anthropic" {
			t.Errorf("chain = %v, want [xai anthropic]", got)
		}
	})

	t.Run("model override", func(t *testing.T) {
		clearProviderEnv(t)
		t.Setenv("LLM_PROVIDERS", "xai")
		t.Setenv("XAI_API_KEY", "x")
		t.Setenv("XAI_MODEL", "grok-4")
		chain := resolveProviders("")
		if len(chain) != 1 || chain[0].model != "grok-4" {
			t.Fatalf("chain = %+v", chain)
		}
	})

	t.Run("default is anthropic only", func(t *testing.T) {
		clearProviderEnv(t)
		chain := resolveProviders("ant")
		if len(chain) != 1 || chain[0].name != "anthropic" {
			t.Fatalf("chain = %+v", chain)
		}
	})

	t.Run("legacy LLM_FALLBACK shim", func(t *testing.T) {
		clearProviderEnv(t)
		t.Setenv("LLM_FALLBACK_PROVIDER", "deepseek")
		t.Setenv("LLM_FALLBACK_API_KEY", "ds")
		chain := resolveProviders("ant") // default [anthropic] + legacy fallback
		got := make([]string, len(chain))
		for i, p := range chain {
			got[i] = p.name
		}
		if strings.Join(got, ",") != "anthropic,deepseek" {
			t.Errorf("chain = %v", got)
		}
		if chain[1].apiKey != "ds" {
			t.Errorf("deepseek key = %q", chain[1].apiKey)
		}
	})

	t.Run("empty when no keys", func(t *testing.T) {
		clearProviderEnv(t)
		if chain := resolveProviders(""); len(chain) != 0 {
			t.Fatalf("want empty chain, got %+v", chain)
		}
	})
}

func TestIsCapError(t *testing.T) {
	for _, msg := range []string{
		"API error 400: You have reached your specified API usage limits.",
		"insufficient balance",
		"quota exceeded",
	} {
		if !isCapError(errorString(msg)) {
			t.Errorf("should be cap error: %q", msg)
		}
	}
	if isCapError(errorString("rate limit exceeded (20 calls/min)")) {
		t.Error("rate-limit is not a cap error")
	}
	if isCapError(nil) {
		t.Error("nil is not a cap error")
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

// newTestClient builds a Client with a given chain and initialized maps.
func newTestClient(providers ...*provider) *Client {
	return &Client{
		httpClient:         &http.Client{Timeout: 5 * time.Second},
		maxPerMin:          20,
		resetAt:            time.Now().Add(time.Minute),
		callsByTag:         map[string]int64{},
		tokensByTag:        map[string][2]int64{},
		cacheTokensByTag:   map[string][2]int64{},
		cooldowns:          map[string]time.Time{},
		failuresByProvider: map[string]int{},
		providers:          providers,
	}
}

// TestOpenAIRoundTrip drives the OpenAI-compatible path against a mock server.
func TestOpenAIRoundTrip(t *testing.T) {
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
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"a vision of grain"}}],"usage":{"prompt_tokens":12,"completion_tokens":4}}`)
	}))
	defer srv.Close()

	c := newTestClient(&provider{name: "deepseek", kind: kindOpenAI, baseURL: srv.URL, apiKey: "secret", model: "deepseek-chat"})
	out, err := c.dispatch("you are an oracle", "speak", 64, "oracle", false)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
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
		t.Errorf("roles = %v", gotRoles)
	}
	if c.callsByTag["oracle:deepseek"] != 1 {
		t.Errorf("usage not recorded under oracle:deepseek: %+v", c.callsByTag)
	}
}

// TestChainAdvancesOnCap verifies a primary cap error advances to the next
// provider AND cools the capped one down so it is skipped next call.
func TestChainAdvancesOnCap(t *testing.T) {
	var primaryHits, backupHits int
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryHits++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"insufficient balance"}}`)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backupHits++
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer backup.Close()

	c := newTestClient(
		&provider{name: "deepseek", kind: kindOpenAI, baseURL: primary.URL, apiKey: "k", model: "m"},
		&provider{name: "venice", kind: kindOpenAI, baseURL: backup.URL, apiKey: "k", model: "m"},
	)

	// First call: deepseek caps → advance to venice.
	out, err := c.dispatch("s", "u", 16, "tier2", false)
	if err != nil || out != "ok" {
		t.Fatalf("dispatch 1: out=%q err=%v", out, err)
	}
	if primaryHits != 1 || backupHits != 1 {
		t.Fatalf("after call 1: primary=%d backup=%d", primaryHits, backupHits)
	}
	if !c.inCooldown("deepseek") {
		t.Error("deepseek should be cooled down after cap error")
	}

	// Second call: deepseek in cooldown → skipped, straight to venice.
	out, err = c.dispatch("s", "u", 16, "tier2", false)
	if err != nil || out != "ok" {
		t.Fatalf("dispatch 2: out=%q err=%v", out, err)
	}
	if primaryHits != 1 {
		t.Errorf("primary should stay skipped during cooldown, hits=%d", primaryHits)
	}
	if backupHits != 2 {
		t.Errorf("backup hits = %d, want 2", backupHits)
	}
}

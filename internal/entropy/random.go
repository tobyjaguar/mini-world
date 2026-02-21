// Package entropy provides true randomness via random.org for critical stochastic events.
// Falls back to crypto/rand when API is unavailable.
// See design doc Section 7.2.
package entropy

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Client provides true random numbers from random.org with a local pool.
type Client struct {
	apiKey string
	client *http.Client

	mu   sync.Mutex
	pool []float64
}

// NewClient creates a random.org client. Returns nil if apiKey is empty.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Float returns a random float64 in [0, 1). Uses the pool, refilling from
// random.org when low. Falls back to crypto/rand on API failure.
func (c *Client) Float() float64 {
	if c == nil {
		return cryptoRandFloat()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.pool) < 10 {
		c.refill()
	}

	if len(c.pool) == 0 {
		return cryptoRandFloat()
	}

	val := c.pool[0]
	c.pool = c.pool[1:]
	return val
}

func (c *Client) refill() {
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  "generateDecimalFractions",
		"params": map[string]any{
			"apiKey":        c.apiKey,
			"n":             100,
			"decimalPlaces": 6,
		},
		"id": 1,
	}

	body, err := json.Marshal(req)
	if err != nil {
		slog.Debug("random.org marshal failed", "error", err)
		return
	}

	resp, err := c.client.Post("https://api.random.org/json-rpc/4/invoke", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Debug("random.org fetch failed", "error", err)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Debug("random.org read failed", "error", err)
		return
	}

	var result struct {
		Result struct {
			Random struct {
				Data []float64 `json:"data"`
			} `json:"random"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		slog.Debug("random.org parse failed", "error", err)
		return
	}

	if result.Error != nil {
		slog.Debug("random.org API error", "error", result.Error.Message)
		return
	}

	c.pool = append(c.pool, result.Result.Random.Data...)
	slog.Debug("random.org pool refilled", "count", len(result.Result.Random.Data))
}

// cryptoRandFloat generates a random float64 using crypto/rand as fallback.
func cryptoRandFloat() float64 {
	var buf [8]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		// This should never happen but return 0.5 as a safe default.
		return 0.5
	}
	// Use only 53 bits for a uniform float64 in [0, 1).
	n := binary.LittleEndian.Uint64(buf[:]) >> 11
	return float64(n) / float64(1<<53)
}

// CryptoFloat returns a random float using crypto/rand (no API needed).
// Used as a standalone fallback when no Client is available.
func CryptoFloat() float64 {
	return cryptoRandFloat()
}

// Enabled returns true if the client has a valid API key.
func (c *Client) Enabled() bool {
	return c != nil && c.apiKey != ""
}

// FloatFromSource returns a random float from the client if available, or crypto/rand.
func FloatFromSource(c *Client) float64 {
	if c != nil && c.Enabled() {
		return c.Float()
	}
	return cryptoRandFloat()
}


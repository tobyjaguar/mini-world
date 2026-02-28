// Package weather provides real-world weather data integration.
// Maps OpenWeatherMap conditions to simulation modifiers.
// See design doc Section 7.1.
package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client fetches weather data from OpenWeatherMap.
type Client struct {
	apiKey   string
	location string
	client   *http.Client

	mu          sync.Mutex
	cached      *Conditions
	cachedAt    time.Time
	cacheTTL    time.Duration
	lastFailAt  time.Time
	failBackoff time.Duration
}

// NewClient creates a weather API client. Returns nil if apiKey is empty.
func NewClient(apiKey, location string) *Client {
	if apiKey == "" {
		return nil
	}
	if location == "" {
		location = "San Diego,US"
	}
	return &Client{
		apiKey:   apiKey,
		location: location,
		client:   &http.Client{Timeout: 10 * time.Second},
		cacheTTL: 5 * time.Minute,
	}
}

// Conditions holds parsed weather data from the API.
type Conditions struct {
	Temp        float64 `json:"temp"`         // Celsius
	Description string  `json:"description"`
	WindSpeed   float64 `json:"wind_speed"`   // m/s
	IsStorm     bool    `json:"is_storm"`
	IsSnow      bool    `json:"is_snow"`
	IsRain      bool    `json:"is_rain"`
}

// Fetch retrieves current weather conditions, using cache if fresh.
func (c *Client) Fetch() (*Conditions, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cached != nil && time.Since(c.cachedAt) < c.cacheTTL {
		return c.cached, nil
	}

	// Backoff on repeated failures (up to 10 minutes).
	if c.failBackoff > 0 && time.Since(c.lastFailAt) < c.failBackoff {
		if c.cached != nil {
			return c.cached, nil
		}
		return nil, fmt.Errorf("weather API backoff (%s remaining)", c.failBackoff-time.Since(c.lastFailAt))
	}

	conditions, err := c.fetchFromAPI()
	if err != nil {
		c.lastFailAt = time.Now()
		if c.failBackoff == 0 {
			c.failBackoff = 1 * time.Minute
		} else if c.failBackoff < 10*time.Minute {
			c.failBackoff *= 2
		}
		if c.cached != nil {
			return c.cached, nil
		}
		return nil, err
	}

	c.cached = conditions
	c.cachedAt = time.Now()
	c.failBackoff = 0 // Reset backoff on success.
	return conditions, nil
}

func (c *Client) fetchFromAPI() (*Conditions, error) {
	apiURL := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric",
		url.QueryEscape(c.location), c.apiKey)

	resp, err := c.client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("weather API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read weather response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse OpenWeatherMap response.
	var owm struct {
		Main struct {
			Temp float64 `json:"temp"`
		} `json:"main"`
		Weather []struct {
			Main        string `json:"main"`
			Description string `json:"description"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
	}

	if err := json.Unmarshal(body, &owm); err != nil {
		return nil, fmt.Errorf("parse weather: %w", err)
	}

	conditions := &Conditions{
		Temp:      owm.Main.Temp,
		WindSpeed: owm.Wind.Speed,
	}

	if len(owm.Weather) > 0 {
		conditions.Description = owm.Weather[0].Description
		main := strings.ToLower(owm.Weather[0].Main)
		conditions.IsRain = main == "rain" || main == "drizzle"
		conditions.IsSnow = main == "snow"
		conditions.IsStorm = main == "thunderstorm" || conditions.WindSpeed > 15
	}

	slog.Debug("weather fetched", "temp", conditions.Temp, "desc", conditions.Description)
	return conditions, nil
}

// SimWeather holds simulation-mapped weather modifiers.
type SimWeather struct {
	TempModifier  float32 // -1 cold to +1 hot
	FoodDecayMod  float32 // Multiplier on food spoilage
	TravelPenalty float32 // Multiplier on travel time
	Description   string
}

// MapToSim converts real weather conditions to simulation modifiers.
func MapToSim(c *Conditions, season uint8) SimWeather {
	sw := SimWeather{
		FoodDecayMod:  1.0,
		TravelPenalty: 1.0,
	}

	if c == nil {
		sw.Description = seasonDefault(season)
		return sw
	}

	sw.Description = c.Description

	// Temperature modifier: map celsius to -1..+1 (0C = -0.5, 20C = 0, 40C = +1).
	sw.TempModifier = float32((c.Temp - 20) / 20)
	if sw.TempModifier < -1 {
		sw.TempModifier = -1
	}
	if sw.TempModifier > 1 {
		sw.TempModifier = 1
	}

	// Hot weather spoils food faster.
	if c.Temp > 30 {
		sw.FoodDecayMod = 1.5
	} else if c.Temp > 25 {
		sw.FoodDecayMod = 1.2
	} else if c.Temp < 0 {
		sw.FoodDecayMod = 0.7 // Cold preserves
	}

	// Storm/snow slows travel.
	if c.IsStorm {
		sw.TravelPenalty = 2.0
	} else if c.IsSnow {
		sw.TravelPenalty = 1.5
	} else if c.IsRain {
		sw.TravelPenalty = 1.2
	}

	return sw
}

func seasonDefault(season uint8) string {
	switch season {
	case 0:
		return "mild spring weather"
	case 1:
		return "warm summer sun"
	case 2:
		return "cool autumn breeze"
	case 3:
		return "cold winter chill"
	default:
		return "fair weather"
	}
}

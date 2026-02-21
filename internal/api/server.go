// Package api provides the HTTP API for querying world state.
// See design doc Section 8.4.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/engine"
)

// Server serves the world state over HTTP.
type Server struct {
	Sim  *engine.Simulation
	Eng  *engine.Engine
	Port int
}

// Start begins serving the HTTP API in a goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/settlements", s.handleSettlements)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agent/", s.handleAgent)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/speed", s.handleSpeed)

	addr := fmt.Sprintf(":%d", s.Port)
	slog.Info("HTTP API starting", "addr", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"name":        "Crossroads",
		"tick":        s.Sim.CurrentTick(),
		"sim_time":    engine.SimTime(s.Sim.CurrentTick()),
		"speed":       s.Eng.Speed,
		"running":     s.Eng.Running,
		"population":  s.Sim.Stats.TotalPopulation,
		"deaths":      s.Sim.Stats.Deaths,
		"settlements": len(s.Sim.Settlements),
		"avg_mood":    s.Sim.Stats.AvgMood,
		"total_wealth": s.Sim.Stats.TotalWealth,
	}
	writeJSON(w, status)
}

func (s *Server) handleSettlements(w http.ResponseWriter, r *http.Request) {
	type settlementSummary struct {
		ID         uint64  `json:"id"`
		Name       string  `json:"name"`
		Q          int     `json:"q"`
		R          int     `json:"r"`
		Population uint32  `json:"population"`
		Governance string  `json:"governance"`
		Treasury   uint64  `json:"treasury"`
		Health     float64 `json:"health"`
	}

	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}

	var result []settlementSummary
	for _, st := range s.Sim.Settlements {
		result = append(result, settlementSummary{
			ID:         st.ID,
			Name:       st.Name,
			Q:          st.Position.Q,
			R:          st.Position.R,
			Population: st.Population,
			Governance: govNames[uint8(st.Governance)],
			Treasury:   st.Treasury,
			Health:     st.Health(),
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	// Return Tier 2 agents by default, or filter by query params.
	tier := r.URL.Query().Get("tier")

	type agentSummary struct {
		ID         agents.AgentID `json:"id"`
		Name       string         `json:"name"`
		Age        uint16         `json:"age"`
		Occupation string         `json:"occupation"`
		Tier       int            `json:"tier"`
		Coherence  float32        `json:"coherence"`
		Mood       float32        `json:"mood"`
		Wealth     uint64         `json:"wealth"`
		Alive      bool           `json:"alive"`
	}

	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}

	var result []agentSummary
	for _, a := range s.Sim.Agents {
		// Filter by tier if specified.
		if tier != "" {
			t, _ := strconv.Atoi(tier)
			if int(a.Tier) != t {
				continue
			}
		} else if a.Tier < agents.Tier2 {
			continue // Default: only show Tier 2
		}

		occName := "Unknown"
		if int(a.Occupation) < len(occNames) {
			occName = occNames[a.Occupation]
		}

		result = append(result, agentSummary{
			ID:         a.ID,
			Name:       a.Name,
			Age:        a.Age,
			Occupation: occName,
			Tier:       int(a.Tier),
			Coherence:  a.Soul.CittaCoherence,
			Mood:       a.Mood,
			Wealth:     a.Wealth,
			Alive:      a.Alive,
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from path: /api/v1/agent/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "missing agent id", http.StatusBadRequest)
		return
	}
	idStr := parts[4]
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid agent id", http.StatusBadRequest)
		return
	}

	agent, ok := s.Sim.AgentIndex[agents.AgentID(id)]
	if !ok {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	writeJSON(w, agent)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	events := s.Sim.Events
	start := 0
	if len(events) > limit {
		start = len(events) - limit
	}

	writeJSON(w, events[start:])
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Sim.Stats)
}

func (s *Server) handleSpeed(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Speed float64 `json:"speed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if req.Speed < 0 || req.Speed > 1000 {
			http.Error(w, "speed must be 0–1000", http.StatusBadRequest)
			return
		}
		s.Eng.Speed = req.Speed
		slog.Info("speed changed", "speed", req.Speed)
	}

	writeJSON(w, map[string]float64{"speed": s.Eng.Speed})
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

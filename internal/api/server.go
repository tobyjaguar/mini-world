// Package api provides the HTTP API for querying world state.
// GET endpoints are public (read-only observation).
// POST endpoints require a bearer token (admin control plane).
// See design doc Section 8.4.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/talgya/mini-world/internal/agents"
	"github.com/talgya/mini-world/internal/engine"
	"github.com/talgya/mini-world/internal/llm"
	"github.com/talgya/mini-world/internal/social"
)

// Server serves the world state over HTTP.
type Server struct {
	Sim      *engine.Simulation
	Eng      *engine.Engine
	LLM      *llm.Client
	Port     int
	AdminKey string // Bearer token for POST endpoints. Empty = POST disabled.

	// Cached newspaper (regenerated at most once per sim-day).
	newspaperMu    sync.Mutex
	cachedPaper    *llm.Newspaper
	lastPaperTick  uint64
}

// Start begins serving the HTTP API in a goroutine.
func (s *Server) Start() {
	mux := http.NewServeMux()

	// Public endpoints (GET, read-only — anyone can check in on the world).
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/settlements", s.handleSettlements)
	mux.HandleFunc("/api/v1/agents", s.handleAgents)
	mux.HandleFunc("/api/v1/agent/", s.handleAgent)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/newspaper", s.handleNewspaper)
	mux.HandleFunc("/api/v1/factions", s.handleFactions)

	// Admin endpoints (POST, require bearer token).
	mux.HandleFunc("/api/v1/speed", s.adminOnly(s.handleSpeed))

	addr := fmt.Sprintf(":%d", s.Port)
	slog.Info("HTTP API starting", "addr", addr, "admin_auth", s.AdminKey != "")

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()
}

// adminOnly wraps a handler to require bearer token auth on POST requests.
// GET requests pass through (for endpoints that support both GET and POST).
func (s *Server) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if s.AdminKey == "" {
				http.Error(w, "admin endpoints disabled (no WORLDSIM_ADMIN_KEY set)", http.StatusForbidden)
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.AdminKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next(w, r)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"name":         "Crossroads",
		"tick":         s.Sim.CurrentTick(),
		"sim_time":     engine.SimTime(s.Sim.CurrentTick()),
		"season":       engine.SeasonName(s.Sim.CurrentSeason),
		"speed":        s.Eng.Speed,
		"running":      s.Eng.Running,
		"population":   s.Sim.Stats.TotalPopulation,
		"deaths":       s.Sim.Stats.Deaths,
		"births":       s.Sim.Stats.Births,
		"settlements":  len(s.Sim.Settlements),
		"factions":     len(s.Sim.Factions),
		"avg_mood":     s.Sim.Stats.AvgMood,
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
		if tier != "" {
			t, _ := strconv.Atoi(tier)
			if int(a.Tier) != t {
				continue
			}
		} else if a.Tier < agents.Tier2 {
			continue
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
			http.Error(w, "speed must be 0-1000", http.StatusBadRequest)
			return
		}
		s.Eng.Speed = req.Speed
		slog.Info("speed changed", "speed", req.Speed)
	}

	writeJSON(w, map[string]float64{"speed": s.Eng.Speed})
}

func (s *Server) handleNewspaper(w http.ResponseWriter, r *http.Request) {
	s.newspaperMu.Lock()
	defer s.newspaperMu.Unlock()

	currentTick := s.Sim.CurrentTick()
	currentDay := currentTick / engine.TicksPerSimDay

	// Return cached newspaper if still from today.
	if s.cachedPaper != nil && s.lastPaperTick/engine.TicksPerSimDay == currentDay {
		writeJSON(w, s.cachedPaper)
		return
	}

	// Build newspaper data from current world state.
	data := s.buildNewspaperData()

	paper, err := llm.GenerateNewspaper(s.LLM, data)
	if err != nil {
		slog.Error("newspaper generation failed", "error", err)
		http.Error(w, "newspaper generation failed", http.StatusInternalServerError)
		return
	}

	s.cachedPaper = paper
	s.lastPaperTick = currentTick
	writeJSON(w, paper)
}

func (s *Server) buildNewspaperData() *llm.NewspaperData {
	govNames := map[uint8]string{0: "Monarchy", 1: "Council", 2: "Merchant Republic", 3: "Commune"}
	occNames := []string{
		"Farmer", "Miner", "Crafter", "Merchant", "Soldier",
		"Scholar", "Alchemist", "Laborer", "Fisher", "Hunter",
	}
	goodNames := map[agents.GoodType]string{
		agents.GoodGrain: "Grain", agents.GoodTimber: "Timber", agents.GoodIronOre: "Iron Ore",
		agents.GoodStone: "Stone", agents.GoodFish: "Fish", agents.GoodHerbs: "Herbs",
		agents.GoodGems: "Gems", agents.GoodFurs: "Furs", agents.GoodCoal: "Coal",
		agents.GoodExotics: "Exotics", agents.GoodTools: "Tools", agents.GoodWeapons: "Weapons",
		agents.GoodClothing: "Clothing", agents.GoodMedicine: "Medicine", agents.GoodLuxuries: "Luxuries",
	}
	stateNames := map[agents.StateOfBeing]string{
		agents.Torment: "in Torment", agents.WellBeing: "in WellBeing", agents.Liberation: "Liberated",
	}
	elementNames := map[agents.ElementType]string{
		agents.ElementHelium: "Helium", agents.ElementHydrogen: "Hydrogen",
		agents.ElementGold: "Gold", agents.ElementUranium: "Uranium",
	}
	kindNames := []string{"Political", "Economic", "Military", "Religious", "Criminal"}

	data := &llm.NewspaperData{
		SimTime:     engine.SimTime(s.Sim.CurrentTick()),
		Season:      engine.SeasonName(s.Sim.CurrentSeason),
		Population:  s.Sim.Stats.TotalPopulation,
		Settlements: len(s.Sim.Settlements),
		TotalWealth: s.Sim.Stats.TotalWealth,
		AvgMood:     s.Sim.Stats.AvgMood,
	}

	// Collect recent events by category.
	for _, e := range s.Sim.Events {
		switch e.Category {
		case "death":
			data.Deaths = append(data.Deaths, e.Description)
		case "birth":
			data.Births = append(data.Births, e.Description)
		case "crime":
			data.Crimes = append(data.Crimes, e.Description)
		case "social":
			data.Social = append(data.Social, e.Description)
		case "economy":
			data.Economy = append(data.Economy, e.Description)
		}
	}

	// Top 5 settlements by population (sort first).
	sorted := make([]*social.Settlement, len(s.Sim.Settlements))
	copy(sorted, s.Sim.Settlements)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Population > sorted[j].Population
	})
	for i, st := range sorted {
		if i >= 5 {
			break
		}
		data.TopSettlements = append(data.TopSettlements, llm.SettlementSummary{
			Name:       st.Name,
			Population: st.Population,
			Treasury:   st.Treasury,
			Governance: govNames[uint8(st.Governance)],
			Health:     st.Health(),
		})
	}

	// Market prices — collect all entries, find the most notable (furthest from base price).
	type priceEntry struct {
		good       string
		settlement string
		price      float64
		ratio      float64
	}
	var allPrices []priceEntry
	for _, st := range s.Sim.Settlements {
		if st.Market == nil {
			continue
		}
		for goodType, entry := range st.Market.Entries {
			if entry.BasePrice <= 0 {
				continue
			}
			ratio := entry.Price / entry.BasePrice
			gn := goodNames[goodType]
			if gn == "" {
				gn = fmt.Sprintf("Good#%d", goodType)
			}
			allPrices = append(allPrices, priceEntry{
				good:       gn,
				settlement: st.Name,
				price:      entry.Price,
				ratio:      ratio,
			})
		}
	}
	// Sort by distance from 1.0 (most notable deviations first).
	sort.Slice(allPrices, func(i, j int) bool {
		return math.Abs(allPrices[i].ratio-1.0) > math.Abs(allPrices[j].ratio-1.0)
	})
	for i, pe := range allPrices {
		if i >= 8 {
			break
		}
		data.MarketPrices = append(data.MarketPrices, llm.MarketPriceSummary{
			Good:       pe.good,
			Settlement: pe.settlement,
			Price:      pe.price,
			PriceRatio: pe.ratio,
		})
	}

	// Faction news.
	for _, f := range s.Sim.Factions {
		kindName := "Unknown"
		if int(f.Kind) < len(kindNames) {
			kindName = kindNames[f.Kind]
		}

		// Find top settlement by influence.
		var topSett string
		var topInf float64
		for settID, inf := range f.Influence {
			if inf > topInf {
				if sett, ok := s.Sim.SettlementIndex[settID]; ok {
					topSett = sett.Name
					topInf = inf
				}
			}
		}

		// Build faction summary line.
		line := fmt.Sprintf("%s (%s): treasury %d crowns", f.Name, kindName, f.Treasury)
		if topSett != "" {
			line += fmt.Sprintf(", strongest in %s (influence %.0f)", topSett, topInf)
		}

		// Note any strong inter-faction tensions or alliances.
		for otherID, rel := range f.Relations {
			if rel > 50 {
				for _, other := range s.Sim.Factions {
					if other.ID == otherID {
						line += fmt.Sprintf("; allied with %s", other.Name)
						break
					}
				}
			} else if rel < -50 {
				for _, other := range s.Sim.Factions {
					if other.ID == otherID {
						line += fmt.Sprintf("; hostile toward %s", other.Name)
						break
					}
				}
			}
		}

		data.FactionNews = append(data.FactionNews, line)
	}

	// Wheeler coherence — count states and compute average.
	var totalCoherence float32
	aliveCount := 0
	for _, a := range s.Sim.Agents {
		if !a.Alive {
			continue
		}
		aliveCount++
		totalCoherence += a.Soul.CittaCoherence
		switch a.Soul.State {
		case agents.Torment:
			data.CoherenceCounts.Torment++
		case agents.WellBeing:
			data.CoherenceCounts.WellBeing++
		case agents.Liberation:
			data.CoherenceCounts.Liberation++
		}
	}
	if aliveCount > 0 {
		data.AvgCoherence = totalCoherence / float32(aliveCount)
	}

	// Notable agents (Tier 2) with Wheeler descriptions.
	for _, a := range s.Sim.Agents {
		if a.Tier >= agents.Tier2 && a.Alive {
			occName := "Unknown"
			if int(a.Occupation) < len(occNames) {
				occName = occNames[a.Occupation]
			}

			stateName := stateNames[a.Soul.State]
			elemName := elementNames[a.Soul.Element()]

			data.NotableAgents = append(data.NotableAgents, llm.AgentSummary{
				Name:       a.Name,
				Age:        a.Age,
				Occupation: occName,
				Wealth:     a.Wealth,
				Mood:       fmt.Sprintf("%.2f", a.Mood),
				State:      stateName,
				Element:    elemName,
				Coherence:  a.Soul.CittaCoherence,
			})
		}
	}

	return data
}

func (s *Server) handleFactions(w http.ResponseWriter, r *http.Request) {
	if s.Sim.Factions == nil {
		writeJSON(w, []any{})
		return
	}

	type factionSummary struct {
		ID        uint64             `json:"id"`
		Name      string             `json:"name"`
		Kind      string             `json:"kind"`
		Treasury  uint64             `json:"treasury"`
		Influence map[string]float64 `json:"top_influence"` // settlement name → influence
	}

	kindNames := []string{"Political", "Economic", "Military", "Religious", "Criminal"}

	var result []factionSummary
	for _, f := range s.Sim.Factions {
		kindName := "Unknown"
		if int(f.Kind) < len(kindNames) {
			kindName = kindNames[f.Kind]
		}

		// Convert settlement ID influence to settlement names (top 5).
		topInf := make(map[string]float64)
		for settID, inf := range f.Influence {
			if sett, ok := s.Sim.SettlementIndex[settID]; ok {
				if len(topInf) < 5 || inf > 5 {
					topInf[sett.Name] = inf
				}
			}
		}

		result = append(result, factionSummary{
			ID:        uint64(f.ID),
			Name:      f.Name,
			Kind:      kindName,
			Treasury:  f.Treasury,
			Influence: topInf,
		})
	}
	writeJSON(w, result)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}
